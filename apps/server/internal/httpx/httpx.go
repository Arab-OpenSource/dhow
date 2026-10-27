package httpx

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Arab-OpenSource/dhow/apps/server/internal/api/gen"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	clientIPKey
)

const (
	CodeInternal         = "internal"
	CodeNotFound         = "not_found"
	CodeMethodNotAllowed = "method_not_allowed"
	CodeBadRequest       = "bad_request"
	CodePayloadTooLarge  = "payload_too_large"
	CodeUnavailable      = "unavailable"

	MessageInternal = "Something went wrong. Try again later."
)

const fallbackErrorBody = `{"Success":false,"Data":null,"Error":{"Code":"internal","Message":"Something went wrong. Try again later."}}` + "\n"

func NewError(code, message string) gen.ErrorResponse {
	return gen.ErrorResponse{Success: false, Data: nil, Error: &gen.ErrorDetail{Code: code, Message: message}}
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	body, err := json.Marshal(NewError(code, message))
	if err != nil {
		body = []byte(fallbackErrorBody)
	} else {
		body = append(body, '\n')
	}
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		return
	}
}

func writeInternal(w http.ResponseWriter) {
	WriteError(w, http.StatusInternalServerError, CodeInternal, MessageInternal)
}

func NotFound(w http.ResponseWriter, _ *http.Request) {
	WriteError(w, http.StatusNotFound, CodeNotFound, "The requested resource does not exist.")
}

func MethodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	WriteError(w, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "This method is not allowed here.")
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := rand.Text()
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func ClientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	isTrusted := func(a netip.Addr) bool {
		for _, p := range trusted {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := peerAddr(r.RemoteAddr)
			if ip.IsValid() && isTrusted(ip) {
				hops := forwardedHops(r.Header.Values("X-Forwarded-For"))
				for i := len(hops) - 1; i >= 0; i-- {
					ip = hops[i]
					if !isTrusted(ip) {
						break
					}
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey, ip)))
		})
	}
}

func ClientIPFrom(ctx context.Context) netip.Addr {
	ip, _ := ctx.Value(clientIPKey).(netip.Addr)
	return ip
}

func peerAddr(remote string) netip.Addr {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

func forwardedHops(values []string) []netip.Addr {
	var out []netip.Addr
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			a, err := netip.ParseAddr(strings.TrimSpace(part))
			if err != nil {
				out = out[:0]
				continue
			}
			out = append(out, a.Unmap())
		}
	}
	return out
}

func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			defer func() {
				if rec := recover(); rec != nil {
					handlePanic(ctx, log, w, r.URL.Path, rec)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func handlePanic(ctx context.Context, log *slog.Logger, w http.ResponseWriter, path string, rec any) {
	if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
		log.WarnContext(ctx, "handler aborted", "request_id", RequestIDFrom(ctx), "path", path)
		return
	}
	log.ErrorContext(ctx, "panic serving request",
		"request_id", RequestIDFrom(ctx), "path", path, "panic", rec, "stack", string(debug.Stack()))
	writeInternal(w)
}

func BodyLimit(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > n {
				WriteError(w, http.StatusRequestEntityTooLarge, CodePayloadTooLarge, "The request body is too large.")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

const ContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'; form-action 'self'"

func SecurityHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", ContentSecurityPolicy)
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()")
			h.Set("X-Frame-Options", "DENY")
			if hsts {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			ip := ClientIPFrom(r.Context())
			log.LogAttrs(r.Context(), slog.LevelInfo, "http request",
				slog.String("request_id", RequestIDFrom(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int("bytes", rec.bytes),
				slog.Duration("duration", time.Since(start)),
				slog.String("client_ip", ip.String()),
			)
		})
	}
}
