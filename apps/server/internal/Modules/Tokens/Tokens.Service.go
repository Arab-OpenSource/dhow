package tokens

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Khaymah/dhow/apps/server/internal/Database/Tx"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Tokens/Models"
)

const (
	fieldName    = "Name"
	fieldScopes  = "Scopes"
	fieldExpires = "ExpiresInDays"

	messageName    = "errors.apiTokenName"
	messageScopes  = "errors.apiTokenScopes"
	messageExpires = "errors.apiTokenExpiry"

	targetToken = "api_token"
	day         = 24 * time.Hour
)

type DB interface {
	sqlc.DBTX
	dbtx.Beginner
}

type Publisher interface {
	Publish(ctx context.Context, event eventbus.Event) error
}

type Options struct {
	DB      DB
	Tokens  *auth.APITokens
	Factors auth.FactorConfirmer
	Bus     Publisher
	Log     *slog.Logger
	Now     func() time.Time
}

type Service struct {
	db      DB
	tokens  *auth.APITokens
	factors auth.FactorConfirmer
	bus     Publisher
	log     *slog.Logger
	now     func() time.Time
}

var _ auth.TokenResolver = (*Service)(nil)

func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{db: o.DB, tokens: o.Tokens, factors: o.Factors, bus: o.Bus, log: o.Log, now: now}
}

func (s *Service) queries() *sqlc.Queries { return sqlc.New(s.db) }

func principal(ctx context.Context) (auth.Principal, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return auth.Principal{}, apperrors.Unauthenticated()
	}
	return p, nil
}

func validate(in models.CreateInput) ([]string, *apperrors.AppError) {
	var details []apperrors.IErrorDetail
	if !auth.ValidDisplayName(in.Name, config.APITokenNameMax) {
		details = append(details, apperrors.IErrorDetail{Field: fieldName, Message: messageName})
	}
	if in.ExpiresInDays < 1 || in.ExpiresInDays > config.APITokenMaxTTLDays {
		details = append(details, apperrors.IErrorDetail{Field: fieldExpires, Message: messageExpires})
	}
	scopes := make([]string, 0, len(in.Scopes))
	for _, raw := range in.Scopes {
		if !policy.IsTokenAction(policy.Action(raw)) {
			scopes = nil
			break
		}
		if !slices.Contains(scopes, raw) {
			scopes = append(scopes, raw)
		}
	}
	if len(scopes) == 0 || len(in.Scopes) > config.APITokenMaxScopes {
		details = append(details, apperrors.IErrorDetail{Field: fieldScopes, Message: messageScopes})
	}
	if len(details) > 0 {
		return nil, apperrors.ValidationFailed(details...)
	}
	slices.Sort(scopes)
	return scopes, nil
}

func (s *Service) Create(ctx context.Context, org uuid.UUID, in models.CreateInput, factor auth.Factor) (models.Created, error) {
	p, err := principal(ctx)
	if err != nil {
		return models.Created{}, err
	}
	scopes, ae := validate(in)
	if ae != nil {
		return models.Created{}, ae
	}
	if err := s.factors.ConfirmFactor(ctx, p, factor); err != nil {
		return models.Created{}, err
	}
	value, hash, err := s.tokens.New()
	if err != nil {
		return models.Created{}, err
	}
	now := s.now()
	var id uuid.UUID
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if _, err := q.LockUserForTokens(ctx, p.UserID); err != nil {
			return lockError(err)
		}
		m, err := q.GetTokenMembership(ctx, sqlc.GetTokenMembershipParams{UserID: p.UserID, OrganizationID: org})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound()
		}
		if err != nil {
			return err
		}
		role, ok := policy.EffectiveOf(m.Role, m.TeamRoles)
		if !ok {
			return apperrors.NotFound()
		}
		for _, scope := range scopes {
			if !policy.Allows(role, policy.Action(scope)) {
				return apperrors.ScopeNotAllowed()
			}
		}
		active, err := q.CountActiveUserTokens(ctx, sqlc.CountActiveUserTokensParams{UserID: p.UserID, Now: now})
		if err != nil {
			return err
		}
		if active >= config.APITokenMaxPerUser {
			return apperrors.ApiTokenLimit()
		}
		row, err := q.InsertApiToken(ctx, sqlc.InsertApiTokenParams{
			UserID: p.UserID, OrganizationID: org, MembershipID: m.Id, Name: in.Name, TokenHash: hash, Scopes: scopes,
			ExpiresAt: now.Add(time.Duration(in.ExpiresInDays) * day), Now: now,
		})
		id = row.Id
		return err
	})
	if err != nil {
		return models.Created{}, err
	}
	token, err := s.one(ctx, id)
	if err != nil {
		return models.Created{}, err
	}
	err = s.audit(ctx, p, eventbus.AuthApiTokenCreated, token, map[string]string{
		"Scopes": strings.Join(scopes, " "), "ExpiresInDays": strconv.Itoa(in.ExpiresInDays),
	})
	if err != nil {
		return models.Created{}, err
	}
	return models.Created{Token: token, Value: value}, nil
}

func lockError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperrors.Unauthenticated()
	}
	return err
}

func (s *Service) audit(ctx context.Context, p auth.Principal, name string, t models.Token, data map[string]string) error {
	actor := p.UserID
	if data == nil {
		data = map[string]string{}
	}
	data["Owner"] = t.Owner.ID.String()
	return s.bus.Publish(ctx, eventbus.Event{Name: name, Payload: eventbus.Audited{
		OrganizationIDs: []uuid.UUID{t.OrganizationID}, ActorUserID: &actor, TargetType: targetToken, TargetID: t.ID.String(),
		IP: auth.RequestMetaFrom(ctx).IPString(), Data: data,
	}})
}

func (s *Service) list(ctx context.Context, params sqlc.ListApiTokensParams) ([]models.Token, error) {
	params.MaxRows = config.APITokenListLimit
	rows, err := s.queries().ListApiTokens(ctx, params)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]models.Token, 0, len(rows))
	for _, r := range rows {
		out = append(out, toModel(r, now))
	}
	return out, nil
}

func (s *Service) one(ctx context.Context, id uuid.UUID) (models.Token, error) {
	list, err := s.list(ctx, sqlc.ListApiTokensParams{ID: &id})
	if err != nil {
		return models.Token{}, err
	}
	if len(list) != 1 {
		return models.Token{}, apperrors.NotFound()
	}
	return list[0], nil
}

func toModel(r sqlc.ListApiTokensRow, now time.Time) models.Token {
	status := models.StatusActive
	switch {
	case r.RevokedAt != nil:
		status = models.StatusRevoked
	case !r.ExpiresAt.After(now):
		status = models.StatusExpired
	case !r.MembershipActive:
		status = models.StatusInactive
	}
	return models.Token{
		ID: r.Id, OrganizationID: r.OrganizationId, OrganizationName: r.OrganizationName, Name: r.Name, Scopes: r.Scopes,
		Status: status, Owner: models.Owner{ID: r.UserId, Name: r.UserName, Email: r.UserEmail},
		CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, LastUsedAt: r.LastUsedAt, LastUsedIP: r.LastUsedIp, RevokedAt: r.RevokedAt,
	}
}

func (s *Service) ListMine(ctx context.Context) ([]models.Token, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	return s.list(ctx, sqlc.ListApiTokensParams{UserID: &p.UserID})
}

func (s *Service) ListOrganization(ctx context.Context, org uuid.UUID) ([]models.Token, error) {
	return s.list(ctx, sqlc.ListApiTokensParams{OrganizationID: &org})
}

func (s *Service) RevokeMine(ctx context.Context, id uuid.UUID) (models.Token, error) {
	p, err := principal(ctx)
	if err != nil {
		return models.Token{}, err
	}
	return s.revoke(ctx, p, id, sqlc.RevokeApiTokenParams{UserID: &p.UserID})
}

func (s *Service) RevokeInOrganization(ctx context.Context, org, id uuid.UUID) (models.Token, error) {
	p, err := principal(ctx)
	if err != nil {
		return models.Token{}, err
	}
	return s.revoke(ctx, p, id, sqlc.RevokeApiTokenParams{OrganizationID: &org})
}

func (s *Service) revoke(ctx context.Context, p auth.Principal, id uuid.UUID, scope sqlc.RevokeApiTokenParams) (models.Token, error) {
	actor := p.UserID
	scope.ID, scope.ActorID, scope.Now = id, &actor, s.now()
	_, err := s.queries().RevokeApiToken(ctx, scope)
	revoked := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return models.Token{}, err
	}
	token, err := s.one(ctx, id)
	if err != nil {
		return models.Token{}, err
	}
	if (scope.UserID != nil && token.Owner.ID != *scope.UserID) || (scope.OrganizationID != nil && token.OrganizationID != *scope.OrganizationID) {
		return models.Token{}, apperrors.NotFound()
	}
	if !revoked {
		return token, nil
	}
	return token, s.audit(ctx, p, eventbus.AuthApiTokenRevoked, token, nil)
}
