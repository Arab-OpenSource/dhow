package sources

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	middleware "github.com/Khaymah/dhow/apps/server/internal/Core/Middleware"
	ratelimit "github.com/Khaymah/dhow/apps/server/internal/Core/RateLimit"
	response "github.com/Khaymah/dhow/apps/server/internal/Core/Response"
)

const (
	messageWebhookAccepted  = "sources.webhook.accepted"
	messageWebhookDuplicate = "sources.webhook.duplicate"
	limitPerIP              = "webhook-ip"
	limitPerHookClient      = "webhook-hook-client"
	limitHookFailures       = "webhook-hook-fail"
)

type webhookHandler struct {
	service  *Service
	limiter  ratelimit.Limiter
	log      *slog.Logger
	maxBytes int64
	budget   *ByteBudget
}

func (h *webhookHandler) provider(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { h.receive(w, r, name) }
}

func (h *webhookHandler) take(ctx context.Context, name string, most int, subject string) *apperrors.AppError {
	if h.limiter == nil {
		return nil
	}
	d, err := h.limiter.Take(ctx, ratelimit.Limit{Name: name, Max: most, Window: config.WebhookRateWindow}, subject)
	if err != nil {
		return apperrors.Internal(err)
	}
	if !d.Allowed {
		return apperrors.RateLimited(d.RetryAfter)
	}
	return nil
}

func (h *webhookHandler) fail(w http.ResponseWriter, r *http.Request, ae *apperrors.AppError) {
	if ae.Status >= http.StatusInternalServerError && ae.Status != http.StatusServiceUnavailable {
		h.log.ErrorContext(r.Context(), "webhook failed", "request_id", middleware.RequestIDFrom(r.Context()), "error", ae)
	}
	response.Error(w, r, ae)
}

func busy() *apperrors.AppError {
	e := apperrors.ServiceUnavailable()
	e.RetryAfter = config.WebhookBusyRetry
	return e
}

func (h *webhookHandler) read(w http.ResponseWriter, r *http.Request, known bool) ([]byte, *apperrors.AppError) {
	if !known {
		_, _ = io.CopyN(io.Discard, r.Body, config.WebhookDrainBytes)
		return nil, nil
	}
	if r.ContentLength > h.maxBytes {
		return nil, apperrors.PayloadTooLarge()
	}
	weight := h.maxBytes
	if r.ContentLength >= 0 {
		weight = r.ContentLength
	}
	if !h.budget.TryAcquire(weight) {
		return nil, busy()
	}
	defer h.budget.Release(weight)
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, weight))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, apperrors.PayloadTooLarge()
		}
		return nil, apperrors.BadRequest()
	}
	return body, nil
}

func (h *webhookHandler) receive(w http.ResponseWriter, r *http.Request, provider string) {
	ctx := r.Context()
	network := ratelimit.Network(middleware.ClientIPFrom(ctx))
	if ae := h.take(ctx, limitPerIP, config.WebhookPerIPMax, network); ae != nil {
		h.fail(w, r, ae)
		return
	}
	hook, err := h.service.OpenHook(ctx, provider, chi.URLParam(r, "hookId"))
	if err != nil {
		h.fail(w, r, apperrors.From(err))
		return
	}
	if hook.Known() {
		if ae := h.take(ctx, limitPerHookClient, config.WebhookPerHookMax, hook.ID()+"/"+network); ae != nil {
			h.fail(w, r, ae)
			return
		}
	}
	body, ae := h.read(w, r, hook.Known())
	if ae != nil {
		h.fail(w, r, ae)
		return
	}
	res, err := h.service.ReceiveFor(ctx, hook, r.Header, body)
	if err != nil {
		ae := apperrors.From(err)
		if hook.Known() && ae.Code == apperrors.CodeWebhookRejected {
			if limited := h.take(ctx, limitHookFailures, config.WebhookFailuresPerHook, hook.ID()); limited != nil {
				ae = limited
			}
		}
		h.fail(w, r, ae)
		return
	}
	if res.Duplicate {
		response.Success(w, http.StatusOK, nil, messageWebhookDuplicate, nil)
		return
	}
	response.Success(w, http.StatusAccepted, nil, messageWebhookAccepted, nil)
}
