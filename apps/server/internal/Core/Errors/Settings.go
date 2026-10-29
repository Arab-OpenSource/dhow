package apperrors

import (
	"net/http"
	"strconv"
)

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

const (
	CodeOidcProviderExists     = "OIDC_PROVIDER_EXISTS"
	CodeOidcProviderLimit      = "OIDC_PROVIDER_LIMIT"
	CodeOidcProviderLinked     = "OIDC_PROVIDER_LINKED"
	CodeOidcProviderOnlySignIn = "OIDC_PROVIDER_ONLY_SIGN_IN"

	KeyOidcProviderExists     = "errors.oidcProviderExists"
	KeyOidcProviderLimit      = "errors.oidcProviderLimit"
	KeyOidcProviderLinked     = "errors.oidcProviderLinked"
	KeyOidcProviderOnlySignIn = "errors.oidcProviderOnlySignIn"

	DetailStrandedUsers = "Users"
)

func OidcProviderExists() *AppError {
	return New(http.StatusConflict, CodeOidcProviderExists, KeyOidcProviderExists)
}

func OidcProviderLimit() *AppError {
	return New(http.StatusConflict, CodeOidcProviderLimit, KeyOidcProviderLimit)
}

func OidcProviderLinked(fields ...string) *AppError {
	e := New(http.StatusConflict, CodeOidcProviderLinked, KeyOidcProviderLinked)
	for _, f := range fields {
		e.Details = append(e.Details, IErrorDetail{Field: f, Message: KeyOidcProviderLinked})
	}
	return e
}

func OidcProviderOnlySignIn(users int64) *AppError {
	e := New(http.StatusConflict, CodeOidcProviderOnlySignIn, KeyOidcProviderOnlySignIn)
	e.Details = []IErrorDetail{{Field: DetailStrandedUsers, Message: strconv.FormatInt(users, 10)}}
	return e
}
