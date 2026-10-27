package session

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session/Models"
)

const messageNoPasskeys = "errors.noPasskeys"

type passkeyUser struct {
	id       uuid.UUID
	handle   []byte
	email    string
	name     string
	locale   string
	disabled bool
	rows     []sqlc.ListUserPasskeysRow
}

func (u *passkeyUser) WebAuthnID() []byte          { return u.handle }
func (u *passkeyUser) WebAuthnName() string        { return u.email }
func (u *passkeyUser) WebAuthnDisplayName() string { return u.name }

func (u *passkeyUser) WebAuthnCredentials() []webauthn.Credential {
	out := make([]webauthn.Credential, 0, len(u.rows))
	for _, r := range u.rows {
		transports := make([]protocol.AuthenticatorTransport, 0, len(r.Transports))
		for _, t := range r.Transports {
			transports = append(transports, protocol.AuthenticatorTransport(t))
		}
		out = append(out, webauthn.Credential{
			ID: r.CredentialId, PublicKey: r.PublicKey, AttestationType: r.AttestationType,
			AttestationFormat: r.AttestationFormat, Transport: transports,
			Flags: webauthn.CredentialFlags{
				UserPresent: r.UserPresent, UserVerified: r.UserVerified, BackupEligible: r.BackupEligible, BackupState: r.BackupState,
			},
			Authenticator: webauthn.Authenticator{
				AAGUID: r.Aaguid, SignCount: uint32(r.SignCount), Attachment: protocol.AuthenticatorAttachment(r.Attachment), //nolint:gosec // The column is checked to fit in 32 bits.
			},
		})
	}
	return out
}

func (u *passkeyUser) row(credentialID []byte) (sqlc.ListUserPasskeysRow, bool) {
	for _, r := range u.rows {
		if bytes.Equal(r.CredentialId, credentialID) {
			return r, true
		}
	}
	return sqlc.ListUserPasskeysRow{}, false
}

func (s *Service) loadPasskeyUser(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, ensureHandle bool) (*passkeyUser, error) {
	user, err := q.GetUserForSession(ctx, userID)
	if err != nil {
		return nil, err
	}
	handle := user.WebauthnHandle
	if handle == nil && ensureHandle {
		fresh := make([]byte, config.WebAuthnHandleBytes)
		if _, err := rand.Read(fresh); err != nil {
			return nil, fmt.Errorf("read random bytes: %w", err)
		}
		if handle, err = q.EnsureWebauthnHandle(ctx, sqlc.EnsureWebauthnHandleParams{ID: userID, Handle: fresh, Now: s.now()}); err != nil {
			return nil, err
		}
	}
	rows, err := q.ListUserPasskeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &passkeyUser{id: user.Id, handle: handle, email: user.Email, name: user.Name, locale: user.Locale, disabled: user.Disabled, rows: rows}, nil
}

func (s *Service) beginAssertion(user *passkeyUser) (options, session []byte, err error) {
	if len(user.rows) == 0 || len(user.handle) == 0 {
		return nil, nil, conflict(messageNoPasskeys)
	}
	assertion, data, err := s.webauthn.BeginLogin(user, webauthn.WithUserVerification(protocol.VerificationPreferred))
	if err != nil {
		return nil, nil, fmt.Errorf("begin passkey assertion: %w", err)
	}
	return encodeCeremony(assertion, data)
}

func encodeCeremony(options any, data *webauthn.SessionData) ([]byte, []byte, error) {
	rawOptions, err := json.Marshal(options)
	if err != nil {
		return nil, nil, fmt.Errorf("encode passkey options: %w", err)
	}
	rawSession, err := encodeSession(data)
	if err != nil {
		return nil, nil, fmt.Errorf("encode passkey session: %w", err)
	}
	return rawOptions, rawSession, nil
}

func decodeSession(raw []byte) (webauthn.SessionData, error) {
	var data webauthn.SessionData
	if err := json.Unmarshal(raw, &data); err != nil {
		return webauthn.SessionData{}, fmt.Errorf("decode passkey session: %w", err)
	}
	return data, nil
}

func (s *Service) verifyAssertion(ctx context.Context, q *sqlc.Queries, user *passkeyUser, rawSession []byte, credential json.RawMessage, now time.Time) (factorOutcome, error) {
	out := factorOutcome{method: models.MethodPasskey}
	data, err := decodeSession(rawSession)
	if err != nil {
		return out, err
	}
	if len(credential) == 0 {
		return out, nil
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(credential)
	if err != nil {
		s.log.DebugContext(ctx, "passkey assertion did not parse", "error", err)
		return out, nil
	}
	cred, err := s.webauthn.ValidateLogin(user, data, parsed)
	if err != nil {
		s.log.DebugContext(ctx, "passkey assertion rejected", "error", err)
		return out, nil
	}
	return s.recordPasskeyUse(ctx, q, user, cred, now)
}

func (s *Service) recordPasskeyUse(ctx context.Context, q *sqlc.Queries, user *passkeyUser, cred *webauthn.Credential, now time.Time) (factorOutcome, error) {
	out := factorOutcome{method: models.MethodPasskey}
	row, ok := user.row(cred.ID)
	if !ok {
		return out, nil
	}
	out.targetID = row.Id.String()
	if cred.Authenticator.CloneWarning {
		out.regressed = true
		return out, nil
	}
	n, err := q.RecordPasskeyUse(ctx, sqlc.RecordPasskeyUseParams{
		ID: row.Id, SignCount: int64(cred.Authenticator.SignCount), UserVerified: cred.Flags.UserVerified,
		BackupState: cred.Flags.BackupState, PreviousSignCount: row.SignCount, Now: now,
	})
	out.ok = n == 1
	return out, err
}

func (s *Service) takeChallenge(ctx context.Context, q *sqlc.Queries, id uuid.UUID, kind string, userID, sessionID *uuid.UUID) ([]byte, bool, error) {
	raw, err := q.TakeWebauthnChallenge(ctx, sqlc.TakeWebauthnChallengeParams{
		ID: id, Kind: kind, UserID: userID, SessionID: sessionID, Now: s.now(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	return raw, err == nil, err
}

func (s *Service) storeChallenge(ctx context.Context, kind string, userID, sessionID *uuid.UUID, options, session []byte) (models.PasskeyChallenge, error) {
	now := s.now()
	expires := now.Add(config.WebAuthnChallengeTTL)
	id, err := s.queries().InsertAuthChallenge(ctx, sqlc.InsertAuthChallengeParams{
		Kind: kind, UserID: userID, SessionID: sessionID, WebauthnSession: session, ExpiresAt: expires, Now: now,
	})
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	return models.PasskeyChallenge{ChallengeID: id, Options: options, ExpiresAt: expires}, nil
}

func toPasskey(r sqlc.ListUserPasskeysRow) models.Passkey {
	transports := r.Transports
	if transports == nil {
		transports = []string{}
	}
	return models.Passkey{
		ID: r.Id, Name: r.Name, CreatedAt: r.CreatedAt, LastUsedAt: r.LastUsedAt, Transports: transports, Synced: r.BackupEligible,
	}
}
