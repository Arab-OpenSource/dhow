package runnerauth

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"strconv"
	"time"

	"connectrpc.com/connect"

	middleware "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Middleware"
)

const (
	CodeInvalidRegistrationToken = "INVALID_REGISTRATION_TOKEN" //nolint:gosec // A public error code, not a credential.
	CodeInvalidRunnerCredentials = "INVALID_RUNNER_CREDENTIALS" //nolint:gosec // A public error code, not a credential.
	CodeUnauthenticated          = "UNAUTHENTICATED"
	CodePermissionDenied         = "PERMISSION_DENIED"
	CodeRunnerRevoked            = "RUNNER_REVOKED"
	CodeRunnerDisabled           = "RUNNER_DISABLED"
	CodeUpgradeRequired          = "RUNNER_UPGRADE_REQUIRED"
	CodeValidationFailed         = "VALIDATION_FAILED"
	CodeKeyInUse                 = "RUNNER_KEY_IN_USE"
	CodeKeyVersionChanged        = "KEY_VERSION_CHANGED"
	CodeInternal                 = "INTERNAL"
)

const RetryAfterHeader = "Retry-After"

func newError(code connect.Code, message string) *connect.Error {
	return connect.NewError(code, errors.New(message))
}

func withRetry(e *connect.Error, retryAfter time.Duration) *connect.Error {
	if retryAfter > 0 {
		e.Meta().Set(RetryAfterHeader, strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
	}
	return e
}

func InvalidRegistrationToken(retryAfter time.Duration) *connect.Error {
	return withRetry(newError(connect.CodeUnauthenticated, CodeInvalidRegistrationToken), retryAfter)
}

func InvalidRunnerCredentials(retryAfter time.Duration) *connect.Error {
	return withRetry(newError(connect.CodeUnauthenticated, CodeInvalidRunnerCredentials), retryAfter)
}

func Unauthenticated() *connect.Error {
	return newError(connect.CodeUnauthenticated, CodeUnauthenticated)
}

func PermissionDenied() *connect.Error {
	return newError(connect.CodePermissionDenied, CodePermissionDenied)
}

func RunnerRevoked() *connect.Error { return newError(connect.CodePermissionDenied, CodeRunnerRevoked) }

func RunnerDisabled() *connect.Error {
	return newError(connect.CodePermissionDenied, CodeRunnerDisabled)
}

func UpgradeRequired() *connect.Error {
	return newError(connect.CodeFailedPrecondition, CodeUpgradeRequired)
}

func ValidationFailed(field string) *connect.Error {
	return newError(connect.CodeInvalidArgument, CodeValidationFailed+": "+field)
}

func KeyInUse() *connect.Error { return newError(connect.CodeAlreadyExists, CodeKeyInUse) }

func KeyVersionChanged() *connect.Error {
	return newError(connect.CodeFailedPrecondition, CodeKeyVersionChanged)
}

func Internal() *connect.Error { return newError(connect.CodeInternal, CodeInternal) }

func Interceptor(log *slog.Logger) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			res, err := next(ctx, req)
			if err == nil {
				return res, nil
			}
			var ce *connect.Error
			if errors.As(err, &ce) && ce.Code() != connect.CodeInternal && ce.Code() != connect.CodeUnknown {
				return nil, err
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, connect.NewError(connect.CodeCanceled, errors.New("CANCELED"))
			}
			log.ErrorContext(ctx, "runner plane handler failed",
				"request_id", middleware.RequestIDFrom(ctx), "procedure", req.Spec().Procedure, "error", err)
			return nil, Internal()
		}
	})
}
