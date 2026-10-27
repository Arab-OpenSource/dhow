package session

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Tx"
)

const (
	fieldNewPassword      = "NewPassword"
	messagePasswordSame   = "errors.passwordUnchanged"
	fieldCurrentPassword  = "CurrentPassword"
	maxPasswordInputBytes = 4096
)

func (s *Service) ChangePassword(ctx context.Context, current, next string) (int64, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return 0, err
	}
	if ae := s.passwords.ValidateLength(fieldNewPassword, next); ae != nil {
		return 0, ae
	}
	if len(current) > maxPasswordInputBytes {
		return 0, apperrors.ValidationFailed(apperrors.IErrorDetail{Field: fieldCurrentPassword, Message: "errors.validationFailed"})
	}
	now := s.now()
	account := accountKey(auth.NormalizeEmail(p.Email))
	wait, err := s.lockedFor(ctx, now, account)
	if err != nil {
		return 0, err
	}
	if wait > 0 {
		return 0, apperrors.InvalidCurrentPassword(wait)
	}
	stored, err := s.queries().GetUserPasswordHash(ctx, p.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperrors.Unauthenticated()
	}
	if err != nil {
		return 0, err
	}
	ok, _, err := s.hasher.Verify(ctx, current, stored)
	if err != nil {
		return 0, err
	}
	if !ok {
		if err := s.recordFailure(ctx, now, account); err != nil {
			return 0, err
		}
		return 0, apperrors.InvalidCurrentPassword(0)
	}
	if err := s.resetFailures(ctx, now, account); err != nil {
		return 0, err
	}
	if current == next {
		return 0, apperrors.ValidationFailed(apperrors.IErrorDetail{Field: fieldNewPassword, Message: messagePasswordSame})
	}
	if ae := s.passwords.ValidateBreach(ctx, next); ae != nil {
		return 0, ae
	}
	hash, err := s.hasher.Hash(ctx, next)
	if err != nil {
		return 0, err
	}
	var revoked int64
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := q.SetUserPassword(ctx, sqlc.SetUserPasswordParams{ID: p.UserID, PasswordHash: hash, Now: now}); err != nil {
			return err
		}
		revoked, err = q.RevokeOtherUserSessions(ctx, sqlc.RevokeOtherUserSessionsParams{UserID: p.UserID, ExceptID: p.SessionID, Now: now})
		return err
	})
	if err != nil {
		return 0, err
	}
	return revoked, s.publish(ctx, eventbus.AuthPasswordChanged, p, p.UserID.String(), map[string]string{"RevokedSessions": itoa(revoked)})
}
