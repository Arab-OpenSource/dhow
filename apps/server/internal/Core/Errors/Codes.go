package apperrors

const (
	CodeInternal           = "INTERNAL"
	CodeNotFound           = "NOT_FOUND"
	CodeMethodNotAllowed   = "METHOD_NOT_ALLOWED"
	CodeBadRequest         = "BAD_REQUEST"
	CodeValidationFailed   = "VALIDATION_FAILED"
	CodePayloadTooLarge    = "PAYLOAD_TOO_LARGE"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

const (
	CodeUnauthenticated        = "UNAUTHENTICATED"
	CodeForbidden              = "FORBIDDEN"
	CodeCSRFFailed             = "CSRF_FAILED"
	CodeInvalidCredentials     = "INVALID_CREDENTIALS"      //nolint:gosec // A public error code, not a credential.
	CodeInvalidCurrentPassword = "INVALID_CURRENT_PASSWORD" //nolint:gosec // A public error code, not a credential.
	CodePasswordBreached       = "PASSWORD_BREACHED"
	CodeSetupCompleted         = "SETUP_COMPLETED"
	CodeSetupTokenInvalid      = "SETUP_TOKEN_INVALID" //nolint:gosec // A public error code, not a credential.
	CodeConflict               = "CONFLICT"
)

const (
	KeyInternal           = "errors.internal"
	KeyNotFound           = "errors.notFound"
	KeyMethodNotAllowed   = "errors.methodNotAllowed"
	KeyBadRequest         = "errors.badRequest"
	KeyValidationFailed   = "errors.validationFailed"
	KeyPayloadTooLarge    = "errors.payloadTooLarge"
	KeyServiceUnavailable = "errors.serviceUnavailable"
)

const (
	KeyUnauthenticated        = "errors.unauthenticated"
	KeyForbidden              = "errors.forbidden"
	KeyCSRFFailed             = "errors.csrfFailed"
	KeyInvalidCredentials     = "errors.invalidCredentials"
	KeyInvalidCurrentPassword = "errors.invalidCurrentPassword"
	KeyPasswordBreached       = "errors.passwordBreached"
	KeySetupCompleted         = "errors.setupCompleted"
	KeySetupTokenInvalid      = "errors.setupTokenInvalid"
	KeyConflict               = "errors.conflict"
)
