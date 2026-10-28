package session

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Khaymah/dhow/apps/server/internal/Database/Tx"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Session/Models"
)

const (
	fieldName          = "Name"
	messagePasskeyName = "errors.passkeyName"
	uniqueViolation    = "23505"
)

func validPasskeyName(name string) (string, *apperrors.AppError) {
	name = strings.TrimSpace(name)
	if !auth.ValidDisplayName(name, config.PasskeyNameMaxLength) {
		return "", apperrors.ValidationFailed(apperrors.IErrorDetail{Field: fieldName, Message: messagePasskeyName})
	}
	return name, nil
}

func (s *Service) BeginStepUp(ctx context.Context) (models.PasskeyChallenge, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	user, err := s.loadPasskeyUser(ctx, s.queries(), p.UserID, false)
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	options, session, err := s.beginAssertion(user)
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	userID, sessionID := p.UserID, p.SessionID
	return s.storeChallenge(ctx, kindPasskeyStepUp, &userID, &sessionID, options, session)
}

func (s *Service) BeginRegistration(ctx context.Context) (models.PasskeyChallenge, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	user, err := s.loadPasskeyUser(ctx, s.queries(), p.UserID, true)
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	creation, data, err := s.webauthn.BeginRegistration(user,
		webauthn.WithExclusions(webauthn.Credentials(user.WebAuthnCredentials()).CredentialDescriptors()),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey: protocol.ResidentKeyRequirementRequired, RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification: protocol.VerificationPreferred,
		}),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
	)
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	options, session, err := encodeCeremony(creation, data)
	if err != nil {
		return models.PasskeyChallenge{}, err
	}
	userID, sessionID := p.UserID, p.SessionID
	return s.storeChallenge(ctx, kindPasskeyRegister, &userID, &sessionID, options, session)
}

func (s *Service) AddPasskey(ctx context.Context, in models.AddPasskeyInput) (models.AddPasskeyResult, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return models.AddPasskeyResult{}, err
	}
	name, ae := validPasskeyName(in.Name)
	if ae != nil {
		return models.AddPasskeyResult{}, ae
	}
	before, err := s.factors(ctx, p.UserID)
	if err != nil {
		return models.AddPasskeyResult{}, err
	}
	if err := s.confirmFactor(ctx, p, in.Factor); err != nil {
		return models.AddPasskeyResult{}, err
	}
	cred, err := s.createCredential(ctx, p, in)
	if err != nil {
		return models.AddPasskeyResult{}, err
	}
	var result models.AddPasskeyResult
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		row, err := q.InsertPasskey(ctx, insertParams(p.UserID, name, cred, s.now()))
		if err != nil {
			return err
		}
		result.Passkey = models.Passkey{ID: row.Id, Name: name, CreatedAt: row.CreatedAt, Transports: transportsOf(cred), Synced: cred.Flags.BackupEligible}
		if err := q.ClearMfaReset(ctx, sqlc.ClearMfaResetParams{ID: p.UserID, Now: s.now()}); err != nil {
			return err
		}
		result.Change, err = s.finishChange(ctx, q, p, !hasMfa(before))
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return models.AddPasskeyResult{}, apperrors.PasskeyRejected()
	}
	if err != nil {
		return models.AddPasskeyResult{}, err
	}
	return result, s.publishTarget(ctx, eventbus.AuthPasskeyAdded, p, "passkey", result.Passkey.ID.String(), map[string]string{
		"Name": name, "RevokedSessions": itoa(result.Change.Revoked),
	})
}

func (s *Service) createCredential(ctx context.Context, p auth.Principal, in models.AddPasskeyInput) (*webauthn.Credential, error) {
	q := s.queries()
	userID, sessionID := p.UserID, p.SessionID
	raw, found, err := s.takeChallenge(ctx, q, in.ChallengeID, kindPasskeyRegister, &userID, &sessionID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, apperrors.EnrollmentExpired()
	}
	data, err := decodeSession(raw)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(in.Credential)
	if err != nil {
		s.log.DebugContext(ctx, "passkey attestation did not parse", "error", err)
		return nil, apperrors.PasskeyRejected()
	}
	user, err := s.loadPasskeyUser(ctx, q, p.UserID, false)
	if err != nil {
		return nil, err
	}
	cred, err := s.webauthn.CreateCredential(user, data, parsed)
	if err != nil {
		s.log.DebugContext(ctx, "passkey attestation rejected", "error", err)
		return nil, apperrors.PasskeyRejected()
	}
	return cred, nil
}

func transportsOf(cred *webauthn.Credential) []string {
	out := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		out = append(out, string(t))
	}
	return out
}

func insertParams(userID uuid.UUID, name string, cred *webauthn.Credential, now time.Time) sqlc.InsertPasskeyParams {
	return sqlc.InsertPasskeyParams{
		UserID: userID, Name: name, CredentialID: cred.ID, PublicKey: cred.PublicKey,
		AttestationType: cred.AttestationType, AttestationFormat: cred.AttestationFormat, Aaguid: nonNil(cred.Authenticator.AAGUID),
		SignCount: int64(cred.Authenticator.SignCount), Transports: transportsOf(cred),
		Attachment: string(cred.Authenticator.Attachment), UserPresent: cred.Flags.UserPresent,
		UserVerified: cred.Flags.UserVerified, BackupEligible: cred.Flags.BackupEligible, BackupState: cred.Flags.BackupState, Now: now,
	}
}

func nonNil(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}

func (s *Service) ListPasskeys(ctx context.Context) ([]models.Passkey, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries().ListUserPasskeys(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]models.Passkey, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPasskey(r))
	}
	return out, nil
}

func (s *Service) RenamePasskey(ctx context.Context, id uuid.UUID, name string) (models.Passkey, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return models.Passkey{}, err
	}
	clean, ae := validPasskeyName(name)
	if ae != nil {
		return models.Passkey{}, ae
	}
	row, err := s.queries().RenamePasskey(ctx, sqlc.RenamePasskeyParams{ID: id, UserID: p.UserID, Name: clean, Now: s.now()})
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Passkey{}, apperrors.NotFound()
	}
	if err != nil {
		return models.Passkey{}, err
	}
	passkey := toPasskey(sqlc.ListUserPasskeysRow{
		Id: row.Id, CreatedAt: row.CreatedAt, Name: row.Name, Transports: row.Transports, BackupEligible: row.BackupEligible, LastUsedAt: row.LastUsedAt,
	})
	return passkey, s.publishTarget(ctx, eventbus.AuthPasskeyRenamed, p, "passkey", id.String(), map[string]string{"Name": clean})
}

func (s *Service) RemovePasskey(ctx context.Context, id uuid.UUID, factor models.Factor) (models.MfaChange, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return models.MfaChange{}, err
	}
	owned, err := s.ListPasskeys(ctx)
	if err != nil {
		return models.MfaChange{}, err
	}
	if !containsPasskey(owned, id) {
		return models.MfaChange{}, apperrors.NotFound()
	}
	if err := s.confirmFactor(ctx, p, factor); err != nil {
		return models.MfaChange{}, err
	}
	var change models.MfaChange
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		methods, err := q.LockUserSignInMethods(ctx, p.UserID)
		if err != nil {
			return err
		}
		if signInMethods(methods) <= 1 {
			return apperrors.LastSignInMethod()
		}
		n, err := q.RemovePasskey(ctx, sqlc.RemovePasskeyParams{ID: id, UserID: p.UserID, Now: s.now()})
		if err != nil {
			return err
		}
		if n == 0 {
			return apperrors.NotFound()
		}
		if err := s.dropRecoveryIfLast(ctx, q, p.UserID); err != nil {
			return err
		}
		change, err = s.finishChange(ctx, q, p, false)
		return err
	})
	if err != nil {
		return models.MfaChange{}, err
	}
	return change, s.publishTarget(ctx, eventbus.AuthPasskeyRemoved, p, "passkey", id.String(), map[string]string{"RevokedSessions": itoa(change.Revoked)})
}

func containsPasskey(list []models.Passkey, id uuid.UUID) bool {
	for _, k := range list {
		if k.ID == id {
			return true
		}
	}
	return false
}
