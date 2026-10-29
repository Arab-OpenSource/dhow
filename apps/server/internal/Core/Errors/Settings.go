package apperrors

import "net/http"

const (
	CodeSettingLockedByEnv = "SETTING_LOCKED_BY_ENV"
	CodeSettingChanged     = "SETTING_CHANGED"
	CodeSettingInvalid     = "SETTING_INVALID"
	CodeMailNotConfigured  = "MAIL_NOT_CONFIGURED"
	CodeMailTestFailed     = "MAIL_TEST_FAILED"

	KeySettingLockedByEnv          = "errors.settingLockedByEnv"
	KeySettingChanged              = "errors.settingChanged"
	KeySettingInvalid              = "errors.settingInvalid"
	KeyMailNotConfigured           = "errors.mailNotConfigured"
	KeyMailTestFailed              = "errors.mailTestFailed"
	KeySettingsSecondFactorMissing = "errors.settingsSecondFactor"
)

func SettingLockedByEnv(details ...IErrorDetail) *AppError {
	e := New(http.StatusConflict, CodeSettingLockedByEnv, KeySettingLockedByEnv)
	e.Details = details
	return e
}

func SettingChanged() *AppError {
	return New(http.StatusConflict, CodeSettingChanged, KeySettingChanged)
}

func SettingInvalid(details ...IErrorDetail) *AppError {
	e := New(http.StatusBadRequest, CodeSettingInvalid, KeySettingInvalid)
	e.Details = details
	return e
}

func MailNotConfigured() *AppError {
	return New(http.StatusConflict, CodeMailNotConfigured, KeyMailNotConfigured)
}

func MailTestFailed(err error) *AppError {
	e := New(http.StatusBadGateway, CodeMailTestFailed, KeyMailTestFailed)
	e.Err = err
	return e
}

func SettingsSecondFactorMissing() *AppError {
	return New(http.StatusForbidden, CodeSecondFactorRequired, KeySettingsSecondFactorMissing)
}
