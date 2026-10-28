package apperrors

import (
	"net/http"
	"time"
)

func New(status int, code, messageKey string) *AppError {
	return &AppError{Status: status, Code: code, Message: messageKey}
}

func BadRequest() *AppError {
	return New(http.StatusBadRequest, CodeBadRequest, KeyBadRequest)
}

func ValidationFailed(details ...IErrorDetail) *AppError {
	e := New(http.StatusUnprocessableEntity, CodeValidationFailed, KeyValidationFailed)
	e.Details = details
	return e
}

func NotFound() *AppError {
	return New(http.StatusNotFound, CodeNotFound, KeyNotFound)
}

func MethodNotAllowed() *AppError {
	return New(http.StatusMethodNotAllowed, CodeMethodNotAllowed, KeyMethodNotAllowed)
}

func PayloadTooLarge() *AppError {
	return New(http.StatusRequestEntityTooLarge, CodePayloadTooLarge, KeyPayloadTooLarge)
}

func ServiceUnavailable() *AppError {
	return New(http.StatusServiceUnavailable, CodeServiceUnavailable, KeyServiceUnavailable)
}

func Internal(err error) *AppError {
	e := New(http.StatusInternalServerError, CodeInternal, KeyInternal)
	e.Err = err
	return e
}

func Unauthenticated() *AppError {
	return New(http.StatusUnauthorized, CodeUnauthenticated, KeyUnauthenticated)
}

func Forbidden() *AppError {
	return New(http.StatusForbidden, CodeForbidden, KeyForbidden)
}

func CSRFFailed() *AppError {
	return New(http.StatusForbidden, CodeCSRFFailed, KeyCSRFFailed)
}

func InvalidCredentials(retryAfter time.Duration) *AppError {
	e := New(http.StatusUnauthorized, CodeInvalidCredentials, KeyInvalidCredentials)
	e.RetryAfter = retryAfter
	return e
}

func InvalidCurrentPassword(retryAfter time.Duration) *AppError {
	e := New(http.StatusForbidden, CodeInvalidCurrentPassword, KeyInvalidCurrentPassword)
	e.RetryAfter = retryAfter
	return e
}

func PasswordBreached() *AppError {
	return New(http.StatusUnprocessableEntity, CodePasswordBreached, KeyPasswordBreached)
}

func SetupCompleted() *AppError {
	return New(http.StatusConflict, CodeSetupCompleted, KeySetupCompleted)
}

func SetupTokenInvalid() *AppError {
	return New(http.StatusForbidden, CodeSetupTokenInvalid, KeySetupTokenInvalid)
}

func Conflict() *AppError {
	return New(http.StatusConflict, CodeConflict, KeyConflict)
}

func MfaChallengeExpired() *AppError {
	return New(http.StatusUnauthorized, CodeMfaChallengeExpired, KeyMfaChallengeExpired)
}

func InvalidMfaCode(retryAfter time.Duration) *AppError {
	e := New(http.StatusUnauthorized, CodeInvalidMfaCode, KeyInvalidMfaCode)
	e.RetryAfter = retryAfter
	return e
}

func InvalidCurrentFactor(retryAfter time.Duration) *AppError {
	e := New(http.StatusForbidden, CodeInvalidCurrentFactor, KeyInvalidCurrentFactor)
	e.RetryAfter = retryAfter
	return e
}

func EnrollmentExpired() *AppError {
	return New(http.StatusConflict, CodeEnrollmentExpired, KeyEnrollmentExpired)
}

func PasskeyRejected() *AppError {
	return New(http.StatusUnprocessableEntity, CodePasskeyRejected, KeyPasskeyRejected)
}

func LastOwner() *AppError {
	return New(http.StatusConflict, CodeLastOwner, KeyLastOwner)
}

func RoleNotAllowed() *AppError {
	return New(http.StatusForbidden, CodeRoleNotAllowed, KeyRoleNotAllowed)
}

func AlreadyMember() *AppError {
	return New(http.StatusConflict, CodeAlreadyMember, KeyAlreadyMember)
}

func InvitationPending() *AppError {
	return New(http.StatusConflict, CodeInvitationPending, KeyInvitationPending)
}

func InvitationInvalid() *AppError {
	return New(http.StatusGone, CodeInvitationInvalid, KeyInvitationInvalid)
}

func InvitationEmailMismatch() *AppError {
	return New(http.StatusForbidden, CodeInvitationEmailMismatch, KeyInvitationEmailMismatch)
}

func InvitationAccountExists() *AppError {
	return New(http.StatusConflict, CodeInvitationAccountExists, KeyInvitationAccountExists)
}

func RunnerRevoked() *AppError {
	return New(http.StatusConflict, CodeRunnerRevoked, KeyRunnerRevoked)
}

func SecretsUnavailable() *AppError {
	return New(http.StatusUnprocessableEntity, CodeSecretsUnavailable, KeySecretsUnavailable)
}

func JobFinished() *AppError {
	return New(http.StatusConflict, CodeJobFinished, KeyJobFinished)
}

func JobQueueFull() *AppError {
	return New(http.StatusTooManyRequests, CodeJobQueueFull, KeyJobQueueFull)
}
