package client

import (
	"context"
	"errors"
	"math/rand/v2"
	"strconv"
	"time"

	"connectrpc.com/connect"

	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
)

const (
	CodeLeaseLost          = "LEASE_LOST"
	CodeCancelRequested    = "CANCEL_REQUESTED"
	CodeAlreadyStarted     = "ATTEMPT_ALREADY_STARTED"
	CodeNotRunning         = "ATTEMPT_NOT_RUNNING"
	CodeSecretsUnavailable = "SECRETS_UNAVAILABLE"
	CodeCompletionConflict = "COMPLETION_CONFLICT"
	CodeRunnerRevoked      = "RUNNER_REVOKED"
	CodeRunnerDisabled     = "RUNNER_DISABLED"
	CodeUpgradeRequired    = "RUNNER_UPGRADE_REQUIRED"
	CodeUnauthenticated    = "UNAUTHENTICATED"
	retryAfterHeader       = "Retry-After"
	maxRetryAfter          = 5 * time.Minute
)

var (
	ErrRevoked         = errors.New("the server revoked this runner; register it again with a new token")
	ErrDisabled        = errors.New("an administrator disabled this runner")
	ErrUpgradeRequired = errors.New("the server needs a newer dhow-runner")
	ErrCredentials     = errors.New("the server refused this runner's identity")
	ErrBadToken        = errors.New("the server sent an access token in an unknown format")
	ErrBadChallenge    = errors.New("the server sent a challenge for another runner")
)

func Code(err error) connect.Code {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Code()
	}
	return connect.CodeUnknown
}

func Reason(err error) string {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Message()
	}
	return ""
}

func Is(err error, reason string) bool { return err != nil && Reason(err) == reason }

func IsLeaseLost(err error) bool {
	return Code(err) == connect.CodeFailedPrecondition && Reason(err) == CodeLeaseLost
}

func Fatal(err error) bool {
	return errors.Is(err, ErrRevoked) || errors.Is(err, ErrUpgradeRequired)
}

func Transient(err error) bool {
	if err == nil || Fatal(err) {
		return false
	}
	if errors.Is(err, ErrDisabled) || errors.Is(err, ErrCredentials) {
		return true
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return !errors.Is(err, context.Canceled)
	}
	switch ce.Code() {
	case connect.CodeUnavailable, connect.CodeDeadlineExceeded, connect.CodeResourceExhausted, connect.CodeUnknown,
		connect.CodeInternal, connect.CodeAborted, connect.CodeCanceled, connect.CodeUnauthenticated:
		return true
	default:
		return false
	}
}

func RetryAfter(err error) time.Duration {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return 0
	}
	n, convErr := strconv.Atoi(ce.Meta().Get(retryAfterHeader))
	if convErr != nil || n <= 0 {
		return 0
	}
	return min(time.Duration(n)*time.Second, maxRetryAfter)
}

func mapAuthError(err error) error {
	switch {
	case err == nil:
		return nil
	case Code(err) == connect.CodePermissionDenied && Reason(err) == CodeRunnerRevoked:
		return errors.Join(ErrRevoked, err)
	case Code(err) == connect.CodePermissionDenied && Reason(err) == CodeRunnerDisabled:
		return errors.Join(ErrDisabled, err)
	case Code(err) == connect.CodeFailedPrecondition && Reason(err) == CodeUpgradeRequired:
		return errors.Join(ErrUpgradeRequired, err)
	default:
		return err
	}
}

type Backoff struct {
	Min, Max time.Duration
}

func DefaultBackoff() Backoff { return Backoff{Min: config.BackoffMin, Max: config.BackoffMax} }

func (b Backoff) Delay(attempt int, err error) time.Duration {
	if d := RetryAfter(err); d > 0 {
		return d
	}
	ceiling := b.Max
	if attempt < 32 && b.Min<<attempt < b.Max && b.Min<<attempt > 0 {
		ceiling = b.Min << attempt
	}
	if ceiling <= b.Min {
		return b.Min
	}
	return b.Min + rand.N(ceiling-b.Min+1) //nolint:gosec // Retry jitter, not a secret.
}

func (b Backoff) Sleep(ctx context.Context, attempt int, err error) error {
	t := time.NewTimer(b.Delay(attempt, err))
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
