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
	limitPerHook            = "webhook-hook"
	unknownHook             = "unknown"
)

type webhookHandler struct {
	service *Service
	limiter ratelimit.Limiter
	log     *slog.Logger
}

func (h *webhookHandler) provider(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { h.receive(w, r, name) }
}

func hookSubject(hookID string) string {
	if hookIDPattern.MatchString(hookID) {
		return hookID
	}
	return unknownHook
}

func (h *webhookHandler) limit(ctx context.Context, hookID string) *apperrors.AppError {
	if h.limiter == nil {
		return nil
	}
	checks := []struct {
		limit   ratelimit.Limit
		subject string
	}{
		{ratelimit.Limit{Name: limitPerIP, Max: config.WebhookPerIPMax, Window: config.WebhookRateWindow}, ratelimit.Network(middleware.ClientIPFrom(ctx))},
		{ratelimit.Limit{Name: limitPerHook, Max: config.WebhookPerHookMax, Window: config.WebhookRateWindow}, hookSubject(hookID)},
	}
	for _, c := range checks {
		d, err := h.limiter.Take(ctx, c.limit, c.subject)
		if err != nil {
			return apperrors.Internal(err)
		}
		if !d.Allowed {
			return apperrors.RateLimited(d.RetryAfter)
		}
	}
	return nil
}

func (h *webhookHandler) fail(w http.ResponseWriter, r *http.Request, ae *apperrors.AppError) {
	if ae.Status >= http.StatusInternalServerError {
		h.log.ErrorContext(r.Context(), "webhook failed", "request_id", middleware.RequestIDFrom(r.Context()), "error", ae)
	}
	response.Error(w, r, ae)
}

func (h *webhookHandler) receive(w http.ResponseWriter, r *http.Request, provider string) {
	ctx := r.Context()
	hookID := chi.URLParam(r, "hookId")
	if ae := h.limit(ctx, hookID); ae != nil {
		h.fail(w, r, ae)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			h.fail(w, r, apperrors.PayloadTooLarge())
			return
		}
		h.fail(w, r, apperrors.BadRequest())
		return
	}
	res, err := h.service.Receive(ctx, Incoming{Provider: provider, HookID: hookID, Header: r.Header, Body: body})
	if err != nil {
		h.fail(w, r, apperrors.From(err))
		return
	}
	if res.Duplicate {
		response.Success(w, http.StatusOK, nil, messageWebhookDuplicate, nil)
		return
	}
	response.Success(w, http.StatusAccepted, nil, messageWebhookAccepted, nil)
}
