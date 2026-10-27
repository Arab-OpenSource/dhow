package apperrors

import "net/http"

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
