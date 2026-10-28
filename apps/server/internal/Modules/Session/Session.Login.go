package session

import (
	"context"
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

func (s *Service) Login(ctx context.Context, in models.LoginInput) (models.LoginResult, error) {
	meta := auth.RequestMetaFrom(ctx)
	now := s.now()
	email := auth.NormalizeEmail(in.Email)
	account, byIP := accountKey(email), ipKey(meta.IPString())

	wait, err := s.reserve(ctx, now, byIP, account)
	if err != nil {
		return models.LoginResult{}, err
	}
	if wait > 0 {
		return models.LoginResult{}, apperrors.InvalidCredentials(wait)
	}

	user, found, err := s.findLoginUser(ctx, email)
	if err != nil {
		return models.LoginResult{}, err
	}
	verified, rehash, err := s.checkPassword(ctx, user, found, in.Password)
	if err != nil {
		return models.LoginResult{}, err
	}
	if !verified {
		return models.LoginResult{}, s.failLogin(ctx, email, user, found)
	}
	if err := s.settleSuccess(ctx, now, account, byIP); err != nil {
		return models.LoginResult{}, err
	}
	if rehash {
		s.rehash(ctx, now, user, in.Password)
	}
	who := sessionUser{ID: user.Id, Email: user.Email, Name: user.Name, Locale: user.Locale}
	factors, err := s.queries().GetMfaFactors(ctx, user.Id)
	if err != nil {
		return models.LoginResult{}, err
	}
	if hasMfa(factors) {
		return s.issuePending(ctx, now, who, factors, meta, models.MethodPassword)
	}
	return s.openSession(ctx, now, who, meta, models.MethodPassword, "")
}

type sessionUser struct {
	ID     uuid.UUID
	Email  string
	Name   string
	Locale string
}

func (s *Service) findLoginUser(ctx context.Context, email string) (sqlc.GetUserForLoginRow, bool, error) {
	if !auth.ValidEmail(email) {
		return sqlc.GetUserForLoginRow{}, false, nil
	}
	user, err := s.queries().GetUserForLogin(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.GetUserForLoginRow{}, false, nil
	}
	return user, err == nil, err
}

func (s *Service) checkPassword(ctx context.Context, user sqlc.GetUserForLoginRow, found bool, password string) (ok, rehash bool, err error) {
	if !found || user.Disabled {
		return false, false, s.hasher.VerifyDummy(ctx, password)
	}
	return s.hasher.Verify(ctx, password, user.PasswordHash)
}

func (s *Service) failLogin(ctx context.Context, email string, user sqlc.GetUserForLoginRow, found bool) error {
	meta := auth.RequestMetaFrom(ctx)
	audited := eventbus.Audited{
		TargetType: "user", IP: meta.IPString(), Data: map[string]string{"EmailDigest": emailDigest(email)},
	}
	memberships, err := s.queries().ListUserMemberships(ctx, user.Id)
	if err != nil {
		return err
	}
	if found {
		id := user.Id
		audited.ActorUserID, audited.TargetID = &id, id.String()
		for _, m := range memberships {
			audited.OrganizationIDs = append(audited.OrganizationIDs, m.OrganizationId)
		}
	}
	if err := s.bus.Publish(ctx, eventbus.Event{Name: eventbus.AuthLoginFailed, Payload: audited}); err != nil {
		return err
	}
	return apperrors.InvalidCredentials(0)
}

func (s *Service) rehash(ctx context.Context, now time.Time, user sqlc.GetUserForLoginRow, password string) {
	hash, err := s.hasher.Hash(ctx, password)
	if err != nil {
		s.log.WarnContext(ctx, "password rehash failed", "error", err)
		return
	}
	err = s.queries().RehashUserPassword(ctx, sqlc.RehashUserPasswordParams{ID: user.Id, PasswordHash: hash, OldHash: user.PasswordHash, Now: now})
	if err != nil {
		s.log.WarnContext(ctx, "password rehash not stored", "error", err)
	}
}

func (s *Service) openSession(ctx context.Context, now time.Time, user sessionUser, meta auth.RequestMeta, method, first string) (models.LoginResult, error) {
	cookie, tokenHash, err := auth.NewSessionToken(config.SessionTokenBytes)
	if err != nil {
		return models.LoginResult{}, err
	}
	csrfToken := s.csrf.Token(cookie)
	expires := now.Add(config.SessionAbsoluteTimeout)
	var memberships []sqlc.ListUserMembershipsRow
	var sessionID uuid.UUID
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if old, ok := auth.HashSessionToken(meta.SessionToken, config.SessionTokenBytes); ok {
			if _, err := q.RevokeSessionByTokenHash(ctx, sqlc.RevokeSessionByTokenHashParams{TokenHash: old, Now: now}); err != nil {
				return err
			}
		}
		if memberships, err = q.ListUserMemberships(ctx, user.ID); err != nil {
			return err
		}
		row, err := q.InsertSession(ctx, sqlc.InsertSessionParams{
			UserID: user.ID, TokenHash: tokenHash, CsrfHash: s.csrf.Hash(csrfToken), Now: now, ExpiresAt: expires,
			IdleExpiresAt: now.Add(config.SessionIdleTimeout), UserAgent: meta.UserAgent, IpAddress: meta.IPString(),
		})
		if err != nil {
			return err
		}
		sessionID = row.Id
		return nil
	})
	if err != nil {
		return models.LoginResult{}, err
	}
	orgs := make([]uuid.UUID, 0, len(memberships))
	for _, m := range memberships {
		orgs = append(orgs, m.OrganizationId)
	}
	actor := user.ID
	data := map[string]string{"Method": method}
	if first != "" {
		data["FirstMethod"] = first
	}
	publishErr := s.bus.Publish(ctx, eventbus.Event{Name: eventbus.AuthLogin, Payload: eventbus.Audited{
		OrganizationIDs: orgs, ActorUserID: &actor, TargetType: "session", TargetID: sessionID.String(), IP: meta.IPString(),
		Data: data,
	}})
	if publishErr != nil {
		if _, revokeErr := s.queries().RevokeUserSession(context.WithoutCancel(ctx), sqlc.RevokeUserSessionParams{ID: sessionID, UserID: user.ID, Now: now}); revokeErr != nil {
			s.log.ErrorContext(ctx, "revoke session after audit failure", "error", revokeErr)
		}
		return models.LoginResult{}, publishErr
	}
	return models.LoginResult{
		User:        models.User{ID: user.ID, Email: user.Email, Name: user.Name, Locale: user.Locale},
		CSRFToken:   csrfToken,
		CookieValue: cookie,
		ExpiresAt:   expires,
	}, nil
}
