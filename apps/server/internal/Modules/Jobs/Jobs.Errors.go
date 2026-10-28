package jobs

import (
	"errors"
	"math"
	"strconv"
	"time"

	"connectrpc.com/connect"

	runnerauth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/RunnerAuth"
)

const (
	CodeLeaseLost          = "LEASE_LOST"
	CodeAlreadyStarted     = "ATTEMPT_ALREADY_STARTED"
	CodeNotRunning         = "ATTEMPT_NOT_RUNNING"
	CodeCancelRequested    = "CANCEL_REQUESTED"
	CodeCompletionConflict = "COMPLETION_CONFLICT"
	CodeSecretsUnavailable = "SECRETS_UNAVAILABLE"
	CodeTooManyPolls       = "TOO_MANY_POLLS"
	CodePollInProgress     = "POLL_IN_PROGRESS"
	CodeStepNotStarted     = "STEP_NOT_STARTED"
)

var errInvalidTransition = errors.New("state transition is not in the state machine")

func rpcError(code connect.Code, message string) *connect.Error {
	return connect.NewError(code, errors.New(message))
}

func leaseLost() *connect.Error { return rpcError(connect.CodeFailedPrecondition, CodeLeaseLost) }

func alreadyStarted() *connect.Error {
	return rpcError(connect.CodeFailedPrecondition, CodeAlreadyStarted)
}

func notRunning() *connect.Error { return rpcError(connect.CodeFailedPrecondition, CodeNotRunning) }

func cancelRequested() *connect.Error {
	return rpcError(connect.CodeFailedPrecondition, CodeCancelRequested)
}

func completionConflict() *connect.Error {
	return rpcError(connect.CodeAlreadyExists, CodeCompletionConflict)
}

func secretsUnavailable() *connect.Error {
	return rpcError(connect.CodeFailedPrecondition, CodeSecretsUnavailable)
}

func stepNotStarted() *connect.Error {
	return rpcError(connect.CodeFailedPrecondition, CodeStepNotStarted)
}

func exhausted(code string, retryAfter time.Duration) *connect.Error {
	e := rpcError(connect.CodeResourceExhausted, code)
	e.Meta().Set(runnerauth.RetryAfterHeader, strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
	return e
}
