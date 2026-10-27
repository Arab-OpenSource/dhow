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
	KeyInternal           = "errors.internal"
	KeyNotFound           = "errors.notFound"
	KeyMethodNotAllowed   = "errors.methodNotAllowed"
	KeyBadRequest         = "errors.badRequest"
	KeyValidationFailed   = "errors.validationFailed"
	KeyPayloadTooLarge    = "errors.payloadTooLarge"
	KeyServiceUnavailable = "errors.serviceUnavailable"
)
