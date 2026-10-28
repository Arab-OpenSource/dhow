package identity

import (
	"regexp"

	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Identity/Models"
)

const (
	maxNameRunes = 100
	maxSlugLen   = 63

	fieldOwnerName        = "OwnerName"
	fieldEmail            = "Email"
	fieldOrganizationName = "OrganizationName"
	fieldOrganizationSlug = "OrganizationSlug"
	fieldLocale           = "Locale"
	fieldPassword         = "Password"
	fieldName             = "Name"

	messageInvalidValue = "errors.validationFailed"
	messageSlugTaken    = "errors.slugTaken"
	messageEmailTaken   = "errors.emailTaken"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validSlug(v string) bool {
	return len(v) >= 1 && len(v) <= maxSlugLen && slugPattern.MatchString(v)
}

func validLocale(v string) bool { return v == "ar" || v == "en" }

func invalid(field string) apperrors.IErrorDetail {
	return apperrors.IErrorDetail{Field: field, Message: messageInvalidValue}
}

func validateSetup(in *models.SetupInput) *apperrors.AppError {
	var details []apperrors.IErrorDetail
	in.Email = auth.NormalizeEmail(in.Email)
	if !auth.ValidDisplayName(in.OwnerName, maxNameRunes) {
		details = append(details, invalid(fieldOwnerName))
	}
	if !auth.ValidEmail(in.Email) {
		details = append(details, invalid(fieldEmail))
	}
	if !auth.ValidDisplayName(in.OrganizationName, maxNameRunes) {
		details = append(details, invalid(fieldOrganizationName))
	}
	if !validSlug(in.OrganizationSlug) {
		details = append(details, invalid(fieldOrganizationSlug))
	}
	if !validLocale(in.Locale) {
		details = append(details, invalid(fieldLocale))
	}
	if len(details) > 0 {
		return apperrors.ValidationFailed(details...)
	}
	return nil
}
