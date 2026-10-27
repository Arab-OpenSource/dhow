package i18n

var enMessages = map[string]string{
	"errors.internal":           "Something went wrong. Try again later.",
	"errors.notFound":           "The requested resource does not exist.",
	"errors.methodNotAllowed":   "This method is not allowed here.",
	"errors.badRequest":         "The request is not valid. Check the data and try again.",
	"errors.validationFailed":   "Some of the data is not valid. Check it and try again.",
	"errors.payloadTooLarge":    "The request body is too large.",
	"errors.serviceUnavailable": "The service is not ready. Try again later.",
	"system.health.live":        "The server is running.",
	"system.health.ready":       "The server is ready.",
	"system.meta.loaded":        "Instance information loaded.",
}
