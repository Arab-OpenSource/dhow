package session

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Tx"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session/Models"
)

const (
	kindMfaPending      = "mfa_pending"
	kindPasskeyLogin    = "passkey_login"
	kindPasskeyRegister = "passkey_register"
	kindPasskeyStepUp   = "passkey_step_up"
)

var errChallengeGone = errors.New("mfa challenge is gone")

func hasMfa(f sqlc.GetMfaFactorsRow) bool { return f.HasTotp || f.PasskeyCount > 0 }

func methodsOf(f sqlc.GetMfaFactorsRow) []string {
	var out []string
	if f.HasTotp {
		out = append(out, models.MethodTotp)
	}
	if f.PasskeyCount > 0 {
		out = append(out, models.MethodPasskey)
	}
	if f.RecoveryCodesRemaining > 0 {
		out = append(out, models.MethodRecoveryCode)
	}
	return out
}

func mfaKey(userID uuid.UUID) throttleKey {
	return throttleKey{key: auth.MfaThrottleKey(userID.String()), free: config.MfaFreeAttempts}
}

func (s *Service) issuePending(ctx context.Context, now time.Time, user sessionUser, factors sqlc.GetMfaFactorsRow, meta auth.RequestMeta, first string) (models.LoginResult, error) {
	token, tokenHash, err := auth.NewSessionToken(config.SessionTokenBytes)
	if err != nil {
		return models.LoginResult{}, err
	}
	expires := now.Add(config.MfaPendingTTL)
	id := user.ID
	if _, err := s.queries().InsertAuthChallenge(ctx, sqlc.InsertAuthChallengeParams{
		Kind: kindMfaPending, TokenHash: tokenHash, UserID: &id, DeviceHash: auth.DeviceHash(meta.UserAgent),
		FirstMethod: first, ExpiresAt: expires, Now: now,
	}); err != nil {
		return models.LoginResult{}, err
	}
	if err := s.publishFor(ctx, eventbus.AuthMfaPending, user.ID, "user", user.ID.String(), nil); err != nil {
		return models.LoginResult{}, err
	}
	return models.LoginResult{Pending: &models.MfaPending{Methods: methodsOf(factors), ExpiresAt: expires, CookieValue: token}}, nil
}

func (s *Service) findPending(ctx context.Context, now time.Time) (sqlc.FindMfaChallengeRow, error) {
	meta := auth.RequestMetaFrom(ctx)
	tokenHash, ok := auth.HashSessionToken(meta.MfaToken, config.SessionTokenBytes)
	if !ok {
		return sqlc.FindMfaChallengeRow{}, apperrors.MfaChallengeExpired()
	}
	row, err := s.queries().FindMfaChallenge(ctx, sqlc.FindMfaChallengeParams{TokenHash: tokenHash, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.FindMfaChallengeRow{}, apperrors.MfaChallengeExpired()
	}
	if err != nil {
		return sqlc.FindMfaChallengeRow{}, err
	}
	if row.UserId == nil || subtle.ConstantTimeCompare(row.DeviceHash, auth.DeviceHash(meta.UserAgent)) != 1 {
		return sqlc.FindMfaChallengeRow{}, apperrors.MfaChallengeExpired()
	}
	return row, nil
}

func (s *Service) CompleteMfa(ctx context.Context, in models.MfaLoginInput) (models.LoginResult, error) {
	meta := auth.RequestMetaFrom(ctx)
	now := s.now()
	pending, err := s.findPending(ctx, now)
	if err != nil {
		return models.LoginResult{}, err
	}
	userID := *pending.UserId
	account, byIP := mfaKey(userID), ipKey(meta.IPString())
	wait, err := s.reserve(ctx, now, byIP, account)
	if err != nil {
		return models.LoginResult{}, err
	}
	if wait > 0 {
		return models.LoginResult{}, apperrors.InvalidMfaCode(wait)
	}
	var outcome factorOutcome
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		row, err := q.LockMfaChallenge(ctx, sqlc.LockMfaChallengeParams{ID: pending.Id, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return errChallengeGone
		}
		if err != nil {
			return err
		}
		outcome, err = s.checkSecondFactor(ctx, q, userID, in, row.WebauthnSession, now)
		if err != nil {
			return err
		}
		if !outcome.ok {
			return q.FailMfaChallenge(ctx, sqlc.FailMfaChallengeParams{ID: row.Id, MaxAttempts: config.MfaMaxChallengeAttempts, Now: now})
		}
		_, err = q.UseMfaChallenge(ctx, sqlc.UseMfaChallengeParams{ID: row.Id, Now: now})
		return err
	})
	if errors.Is(err, errChallengeGone) {
		return models.LoginResult{}, apperrors.MfaChallengeExpired()
	}
	if err != nil {
		return models.LoginResult{}, err
	}
	if !outcome.ok {
		return models.LoginResult{}, s.failFactor(ctx, userID, in.Method, "login", outcome, apperrors.InvalidMfaCode(0))
	}
	if err := s.settleSuccess(ctx, now, account, byIP); err != nil {
		return models.LoginResult{}, err
	}
	if err := s.afterFactor(ctx, userID, outcome); err != nil {
		return models.LoginResult{}, err
	}
	user, err := s.queries().GetUserForSession(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && user.Disabled) {
		return models.LoginResult{}, apperrors.MfaChallengeExpired()
	}
	if err != nil {
		return models.LoginResult{}, err
	}
	who := sessionUser{ID: user.Id, Email: user.Email, Name: user.Name, Locale: user.Locale}
	return s.openSession(ctx, now, who, meta, outcome.method, pending.FirstMethod)
}

func (s *Service) BeginMfaPasskey(ctx context.Context) (models.PasskeyChallenge, error) {
	now := s.now()
	pending, err := s.findPending(ctx, now)
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	user, err := s.loadPasskeyUser(ctx, s.queries(), *pending.UserId, false)
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	options, session, err := s.beginAssertion(user)
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	n, err := s.queries().SetMfaChallengeWebauthn(ctx, sqlc.SetMfaChallengeWebauthnParams{ID: pending.Id, WebauthnSession: session, Now: now})
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	if n == 0 {
		return models.PasskeyChallenge{}, apperrors.MfaChallengeExpired()
	}
	return models.PasskeyChallenge{Options: options, ExpiresAt: pending.ExpiresAt}, nil
}

type factorOutcome struct {
	ok        bool
	method    string
	regressed bool
	targetID  string
}

func (s *Service) checkSecondFactor(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, in models.MfaLoginInput, session []byte, now time.Time) (factorOutcome, error) {
	out := factorOutcome{method: in.Method}
	switch in.Method {
	case models.MethodTotp:
		ok, err := s.verifyTotp(ctx, q, userID, in.Code, now)
		out.ok = ok
		return out, err
	case models.MethodRecoveryCode:
		ok, err := s.consumeRecoveryCode(ctx, q, userID, in.Code, now)
		out.ok = ok
		return out, err
	case models.MethodPasskey:
		if len(session) == 0 {
			return out, nil
		}
		user, err := s.loadPasskeyUser(ctx, q, userID, false)
		if err != nil {
			return out, err
		}
		return s.verifyAssertion(ctx, q, user, session, in.Credential, now)
	default:
		return out, nil
	}
}

func (s *Service) failFactor(ctx context.Context, userID uuid.UUID, method, purpose string, outcome factorOutcome, ae *apperrors.AppError) error {
	if outcome.regressed {
		if err := s.publishFor(ctx, eventbus.AuthPasskeySignCountRegressed, userID, "passkey", outcome.targetID, nil); err != nil {
			return err
		}
	}
	data := map[string]string{"Method": method, "Purpose": purpose}
	if err := s.publishFor(ctx, eventbus.AuthMfaFailed, userID, "user", userID.String(), data); err != nil {
		return err
	}
	return ae
}

func (s *Service) afterFactor(ctx context.Context, userID uuid.UUID, outcome factorOutcome) error {
	if outcome.method != models.MethodRecoveryCode {
		return nil
	}
	factors, err := s.queries().GetMfaFactors(ctx, userID)
	if err != nil {
		return err
	}
	return s.publishFor(ctx, eventbus.AuthRecoveryCodeUsed, userID, "user", userID.String(),
		map[string]string{"Remaining": itoa(factors.RecoveryCodesRemaining)})
}

func (s *Service) publishFor(ctx context.Context, name string, userID uuid.UUID, targetType, targetID string, data map[string]string) error {
	memberships, err := s.queries().ListUserMemberships(ctx, userID)
	if err != nil {
		return err
	}
	orgs := make([]uuid.UUID, 0, len(memberships))
	for _, m := range memberships {
		orgs = append(orgs, m.OrganizationId)
	}
	actor := userID
	return s.bus.Publish(ctx, eventbus.Event{Name: name, Payload: eventbus.Audited{
		OrganizationIDs: orgs, ActorUserID: &actor, TargetType: targetType, TargetID: targetID,
		IP: auth.RequestMetaFrom(ctx).IPString(), Data: data,
	}})
}

func encodeSession(v any) ([]byte, error) {
	return json.Marshal(v)
}
