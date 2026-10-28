package runners

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	runnerauth "github.com/Khaymah/dhow/apps/server/internal/Core/RunnerAuth"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Runners/Models"
)

const registerCommand = "sudo dhow-runner register --url %s --token-file -"

func principalOf(ctx context.Context) (auth.Principal, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return auth.Principal{}, apperrors.Unauthenticated()
	}
	return p, nil
}

func (s *Service) CreateRegistrationToken(ctx context.Context, org uuid.UUID, in models.TokenInput) (models.CreatedToken, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return models.CreatedToken{}, err
	}
	settings, ae := validateTokenInput(in)
	if ae != nil {
		return models.CreatedToken{}, ae
	}
	secret, raw, err := runnerauth.NewRegistrationToken()
	if err != nil {
		return models.CreatedToken{}, err
	}
	now := s.now().UTC().Truncate(timePrecision)
	row, err := s.queries().InsertRegistrationToken(ctx, sqlc.InsertRegistrationTokenParams{
		OrganizationID: org, CreatedByUserID: p.UserID, Description: settings.description,
		TokenHash: s.keys.RegistrationTokenHash(raw), Labels: settings.labels, MaxUses: int32(settings.maxUses), //nolint:gosec // Bounded to 1..50 by validateTokenInput.
		ExpiresAt: now.Add(settings.ttl), Now: now,
	})
	if err != nil {
		return models.CreatedToken{}, err
	}
	actor := p.UserID
	err = s.bus.Publish(ctx, eventbus.Event{Name: eventbus.RunnerRegistrationTokenCreated, Payload: eventbus.Audited{
		OrganizationIDs: []uuid.UUID{org}, ActorUserID: &actor, TargetType: "runner_registration_token", TargetID: row.Id.String(),
		IP: auth.RequestMetaFrom(ctx).IPString(), Data: map[string]string{
			"Labels": strings.Join(settings.labels, ","), "MaxUses": strconv.Itoa(settings.maxUses),
			"ExpiresAt": row.ExpiresAt.UTC().Format(timeFormat),
		},
	}})
	if err != nil {
		s.revokeAfterAuditFailure(ctx, org, row.Id, p.UserID)
		return models.CreatedToken{}, err
	}
	return models.CreatedToken{
		Token: tokenOf(row, now), Secret: secret, RegisterCommand: strings.Replace(registerCommand, "%s", s.publicURL, 1),
	}, nil
}

func (s *Service) revokeAfterAuditFailure(ctx context.Context, org, id, user uuid.UUID) {
	_, err := s.queries().RevokeRegistrationToken(context.WithoutCancel(ctx), sqlc.RevokeRegistrationTokenParams{
		ID: id, OrganizationID: org, UserID: user, Now: s.now(),
	})
	if err != nil {
		s.log.ErrorContext(ctx, "revoke registration token after audit failure", "error", err)
	}
}

func (s *Service) ListRegistrationTokens(ctx context.Context, org uuid.UUID, page, limit int) ([]models.RegistrationToken, int64, error) {
	q := s.queries()
	total, err := q.CountRegistrationTokens(ctx, org)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.ListRegistrationTokens(ctx, sqlc.ListRegistrationTokensParams{
		OrganizationID: org, PageLimit: int32(limit), PageOffset: int32((page - 1) * limit), //nolint:gosec // limit and page are bounded by response.PageParams.
	})
	if err != nil {
		return nil, 0, err
	}
	now := s.now()
	out := make([]models.RegistrationToken, 0, len(rows))
	for _, r := range rows {
		out = append(out, tokenOf(r, now))
	}
	return out, total, nil
}

func (s *Service) RevokeRegistrationToken(ctx context.Context, org, id uuid.UUID) (models.RegistrationToken, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return models.RegistrationToken{}, err
	}
	now := s.now()
	q := s.queries()
	row, err := q.RevokeRegistrationToken(ctx, sqlc.RevokeRegistrationTokenParams{ID: id, OrganizationID: org, UserID: p.UserID, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, getErr := q.GetRegistrationToken(ctx, sqlc.GetRegistrationTokenParams{ID: id, OrganizationID: org})
		if errors.Is(getErr, pgx.ErrNoRows) {
			return models.RegistrationToken{}, apperrors.NotFound()
		}
		if getErr != nil {
			return models.RegistrationToken{}, getErr
		}
		return tokenOf(existing, now), nil
	}
	if err != nil {
		return models.RegistrationToken{}, err
	}
	actor := p.UserID
	err = s.bus.Publish(ctx, eventbus.Event{Name: eventbus.RunnerRegistrationTokenRevoked, Payload: eventbus.Audited{
		OrganizationIDs: []uuid.UUID{org}, ActorUserID: &actor, TargetType: "runner_registration_token", TargetID: id.String(),
		IP: auth.RequestMetaFrom(ctx).IPString(), Data: map[string]string{"Uses": strconv.Itoa(int(row.Uses))},
	}})
	if err != nil {
		return models.RegistrationToken{}, err
	}
	return tokenOf(row, now), nil
}
