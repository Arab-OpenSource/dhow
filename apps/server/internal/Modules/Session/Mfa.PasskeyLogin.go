package session

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Tx"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session/Models"
)

var errUnknownPasskeyUser = errors.New("no active user owns this passkey")

func (s *Service) BeginPasskeyLogin(ctx context.Context) (models.PasskeyChallenge, error) {
	assertion, data, err := s.webauthn.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	options, session, err := encodeCeremony(assertion, data)
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	return s.storeChallenge(ctx, kindPasskeyLogin, nil, nil, options, session)
}

func (s *Service) PasskeyLogin(ctx context.Context, challengeID uuid.UUID, credential json.RawMessage) (models.LoginResult, error) {
	meta := auth.RequestMetaFrom(ctx)
	now := s.now()
	byIP := ipKey(meta.IPString())
	wait, err := s.reserve(ctx, now, byIP)
	if err != nil {
		return models.LoginResult{}, err
	}
	if wait > 0 {
		return models.LoginResult{}, apperrors.InvalidCredentials(wait)
	}
	var owner *passkeyUser
	var outcome factorOutcome
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		var err error
		owner, outcome, err = s.checkPasskeyLogin(ctx, q, challengeID, credential, now)
		return err
	})
	if err != nil {
		return models.LoginResult{}, err
	}
	if !outcome.ok || owner == nil {
		return models.LoginResult{}, s.failPasskeyLogin(ctx, owner, outcome)
	}
	if err := s.queries().RefundThrottle(ctx, sqlc.RefundThrottleParams{Key: byIP.key, Free: int32(byIP.free), Now: now}); err != nil { //nolint:gosec // byIP.free is a small constant.
		return models.LoginResult{}, err
	}
	who := sessionUser{ID: owner.id, Email: owner.email, Name: owner.name, Locale: owner.locale}
	return s.openSession(ctx, now, who, meta, models.MethodPasskey)
}

func (s *Service) checkPasskeyLogin(ctx context.Context, q *sqlc.Queries, challengeID uuid.UUID, credential json.RawMessage, now time.Time) (*passkeyUser, factorOutcome, error) {
	out := factorOutcome{method: models.MethodPasskey}
	raw, found, err := s.takeChallenge(ctx, q, challengeID, kindPasskeyLogin, nil, nil)
	if err != nil || !found || len(credential) == 0 {
		return nil, out, err
	}
	data, err := decodeSession(raw)
	if err != nil {
		return nil, out, err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(credential)
	if err != nil {
		s.log.DebugContext(ctx, "passkey assertion did not parse", "error", err)
		return nil, out, nil
	}
	var owner *passkeyUser
	handler := func(_, userHandle []byte) (webauthn.User, error) {
		row, err := q.GetUserByWebauthnHandle(ctx, userHandle)
		if err != nil {
			return nil, errUnknownPasskeyUser
		}
		user, err := s.loadPasskeyUser(ctx, q, row.Id, false)
		if err != nil {
			return nil, err
		}
		owner = user
		if user.disabled {
			return nil, errUnknownPasskeyUser
		}
		return user, nil
	}
	_, cred, err := s.webauthn.ValidatePasskeyLogin(handler, data, parsed)
	if err != nil {
		s.log.DebugContext(ctx, "passkey sign-in rejected", "error", err)
		return owner, out, nil
	}
	out, err = s.recordPasskeyUse(ctx, q, owner, cred, now)
	return owner, out, err
}

func (s *Service) failPasskeyLogin(ctx context.Context, owner *passkeyUser, outcome factorOutcome) error {
	meta := auth.RequestMetaFrom(ctx)
	audited := eventbus.Audited{TargetType: "user", IP: meta.IPString(), Data: map[string]string{"Method": models.MethodPasskey}}
	if owner != nil {
		if outcome.regressed {
			if err := s.publishFor(ctx, eventbus.AuthPasskeySignCountRegressed, owner.id, "passkey", outcome.targetID, nil); err != nil {
				return err
			}
		}
		memberships, err := s.queries().ListUserMemberships(ctx, owner.id)
		if err != nil {
			return err
		}
		id := owner.id
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
