package auth

import (
	"context"
	"log/slog"
	"unicode/utf8"

	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
)

const (
	messageKeyPasswordLength = "errors.passwordLength"
)

type PasswordPolicy struct {
	checker BreachChecker
	log     *slog.Logger
	min     int
	max     int
}

func NewPasswordPolicy(checker BreachChecker, log *slog.Logger, minLength, maxLength int) *PasswordPolicy {
	return &PasswordPolicy{checker: checker, log: log, min: minLength, max: maxLength}
}

func (p *PasswordPolicy) ValidateLength(field, password string) *apperrors.AppError {
	n := utf8.RuneCountInString(password)
	if n < p.min || n > p.max {
		return apperrors.ValidationFailed(apperrors.IErrorDetail{Field: field, Message: messageKeyPasswordLength})
	}
	return nil
}

func (p *PasswordPolicy) ValidateBreach(ctx context.Context, password string) *apperrors.AppError {
	breached, err := p.checker.IsBreached(ctx, password)
	if err != nil {
		p.log.WarnContext(ctx, "breached-password check unavailable; allowing the password", "error", err)
		return nil
	}
	if breached {
		return apperrors.PasswordBreached()
	}
	return nil
}

func (p *PasswordPolicy) Validate(ctx context.Context, field, password string) *apperrors.AppError {
	if ae := p.ValidateLength(field, password); ae != nil {
		return ae
	}
	return p.ValidateBreach(ctx, password)
}
