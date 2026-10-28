package session

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	appcrypto "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Crypto"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	oidcauth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Oidc"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Tx"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session/Models"
)

type DB interface {
	sqlc.DBTX
	dbtx.Beginner
}

type Publisher interface {
	Publish(ctx context.Context, event eventbus.Event) error
}

type Options struct {
	DB        DB
	Hasher    *auth.Hasher
	Passwords *auth.PasswordPolicy
	CSRF      *auth.CSRF
	Sealer    *appcrypto.Sealer
	Recovery  *auth.RecoveryHasher
	WebAuthn  *webauthn.WebAuthn
	Bus       Publisher
	Instance  policy.InstanceSettings
	Log       *slog.Logger
	Now       func() time.Time

	Oidc         *oidcauth.Registry
	OidcKey      []byte
	CallbackBase string
}

type Service struct {
	db        DB
	hasher    *auth.Hasher
	passwords *auth.PasswordPolicy
	csrf      *auth.CSRF
	sealer    *appcrypto.Sealer
	recovery  *auth.RecoveryHasher
	webauthn  *webauthn.WebAuthn
	bus       Publisher
	instance  policy.InstanceSettings
	log       *slog.Logger
	now       func() time.Time

	oidc         *oidcauth.Registry
	oidcKey      []byte
	callbackBase string
}

var (
	_ auth.Resolver        = (*Service)(nil)
	_ auth.FactorConfirmer = (*Service)(nil)
)

func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		db: o.DB, hasher: o.Hasher, passwords: o.Passwords, csrf: o.CSRF, sealer: o.Sealer, recovery: o.Recovery,
		webauthn: o.WebAuthn, bus: o.Bus, instance: o.Instance, log: o.Log, now: now,
		oidc: o.Oidc, oidcKey: o.OidcKey, callbackBase: o.CallbackBase,
	}
}

func (s *Service) queries() *sqlc.Queries { return sqlc.New(s.db) }

func (s *Service) Resolve(ctx context.Context, tokenHash []byte, now time.Time) (auth.Resolved, error) {
	return s.resolve(ctx, tokenHash, now, true)
}

func (s *Service) Peek(ctx context.Context, tokenHash []byte, now time.Time) (auth.Resolved, error) {
	return s.resolve(ctx, tokenHash, now, false)
}

func (s *Service) resolve(ctx context.Context, tokenHash []byte, now time.Time, touch bool) (auth.Resolved, error) {
	q := s.queries()
	row, err := q.ResolveSession(ctx, sqlc.ResolveSessionParams{TokenHash: tokenHash, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Resolved{}, auth.ErrNoSession
	}
	if err != nil {
		return auth.Resolved{}, err
	}
	memberships, err := q.ListUserMemberships(ctx, row.UserId)
	if err != nil {
		return auth.Resolved{}, err
	}
	if touch && now.Sub(row.LastSeenAt) >= config.SessionTouchInterval {
		touchErr := q.TouchSession(ctx, sqlc.TouchSessionParams{
			ID: row.Id, Now: now, IdleExpiresAt: now.Add(config.SessionIdleTimeout), TouchBefore: now.Add(-config.SessionTouchInterval),
		})
		if touchErr != nil {
			s.log.WarnContext(ctx, "touch session failed", "error", touchErr)
		}
	}
	resolved := auth.Resolved{
		SessionID: row.Id, UserID: row.UserId, InstanceOwner: row.IsInstanceOwner, CsrfHash: row.CsrfHash, ExpiresAt: row.ExpiresAt,
		Email: row.Email, Name: row.Name, Locale: row.Locale,
		Memberships: make([]policy.Membership, 0, len(memberships)),
	}
	for _, m := range memberships {
		role, ok := policy.EffectiveOf(m.Role, m.TeamRoles)
		if !ok {
			continue
		}
		resolved.Memberships = append(resolved.Memberships, policy.Membership{OrganizationID: m.OrganizationId, Role: role})
	}
	return resolved, nil
}

func (s *Service) ConfirmFactor(ctx context.Context, p auth.Principal, f auth.Factor) error {
	if p.ViaToken() || p.SessionID == uuid.Nil {
		return apperrors.SessionRequired()
	}
	return s.confirmFactor(ctx, p, models.Factor(f))
}

func principalOf(ctx context.Context) (auth.Principal, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return auth.Principal{}, apperrors.Unauthenticated()
	}
	return p, nil
}

func (s *Service) Me(ctx context.Context) (models.Me, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return models.Me{}, err
	}
	q := s.queries()
	rows, err := q.ListUserMemberships(ctx, p.UserID)
	if err != nil {
		return models.Me{}, err
	}
	flags, err := q.GetUserAccountFlags(ctx, p.UserID)
	if err != nil {
		return models.Me{}, err
	}
	factors, err := q.GetMfaFactors(ctx, p.UserID)
	if err != nil {
		return models.Me{}, err
	}
	memberships := make([]models.Membership, 0, len(rows))
	for _, r := range rows {
		role, ok := policy.EffectiveOf(r.Role, r.TeamRoles)
		if !ok {
			continue
		}
		memberships = append(memberships, models.Membership{
			OrganizationID: r.OrganizationId, OrganizationName: r.OrganizationName,
			OrganizationSlug: r.OrganizationSlug, Role: string(role),
		})
	}
	return models.Me{
		User:        models.User{ID: p.UserID, Email: p.Email, Name: p.Name, Locale: p.Locale},
		Memberships: memberships, CSRFToken: p.CSRFToken, SessionID: p.SessionID, ExpiresAt: p.ExpiresAt,
		CanCreateOrganizations: policy.CanInstance(p.Subject, policy.OrgCreate, s.instance),
		MfaReenrollRequired:    flags.MfaResetAt != nil && !hasMfa(factors),
	}, nil
}

func (s *Service) List(ctx context.Context) ([]models.SessionInfo, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries().ListActiveUserSessions(ctx, sqlc.ListActiveUserSessionsParams{UserID: p.UserID, Now: s.now()})
	if err != nil {
		return nil, err
	}
	out := make([]models.SessionInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, models.SessionInfo{
			ID: r.Id, CreatedAt: r.CreatedAt, LastSeenAt: r.LastSeenAt, ExpiresAt: r.ExpiresAt,
			UserAgent: r.UserAgent, IPAddress: r.IpAddress, Current: r.Id == p.SessionID,
		})
	}
	return out, nil
}

func (s *Service) Logout(ctx context.Context) error {
	p, err := principalOf(ctx)
	if err != nil {
		return err
	}
	if _, err := s.queries().RevokeUserSession(ctx, sqlc.RevokeUserSessionParams{ID: p.SessionID, UserID: p.UserID, Now: s.now()}); err != nil {
		return err
	}
	return s.publish(ctx, eventbus.AuthLogout, p, p.SessionID.String(), nil)
}

func (s *Service) Revoke(ctx context.Context, id uuid.UUID) error {
	p, err := principalOf(ctx)
	if err != nil {
		return err
	}
	n, err := s.queries().RevokeUserSession(ctx, sqlc.RevokeUserSessionParams{ID: id, UserID: p.UserID, Now: s.now()})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NotFound()
	}
	return s.publish(ctx, eventbus.AuthSessionRevoked, p, id.String(), nil)
}

func (s *Service) RevokeOthers(ctx context.Context) (int64, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return 0, err
	}
	n, err := s.queries().RevokeOtherUserSessions(ctx, sqlc.RevokeOtherUserSessionsParams{UserID: p.UserID, ExceptID: p.SessionID, Now: s.now()})
	if err != nil {
		return 0, err
	}
	return n, s.publish(ctx, eventbus.AuthSessionsRevokedOthers, p, p.UserID.String(), map[string]string{"Revoked": itoa(n)})
}

func (s *Service) publish(ctx context.Context, name string, p auth.Principal, targetID string, data map[string]string) error {
	return s.publishTarget(ctx, name, p, "session", targetID, data)
}

func (s *Service) publishTarget(ctx context.Context, name string, p auth.Principal, targetType, targetID string, data map[string]string) error {
	meta := auth.RequestMetaFrom(ctx)
	actor := p.UserID
	orgs := make([]uuid.UUID, 0, len(p.Memberships))
	for _, m := range p.Memberships {
		orgs = append(orgs, m.OrganizationID)
	}
	return s.bus.Publish(ctx, eventbus.Event{Name: name, Payload: eventbus.Audited{
		OrganizationIDs: orgs, ActorUserID: &actor, TargetType: targetType, TargetID: targetID, IP: meta.IPString(), Data: data,
	}})
}
