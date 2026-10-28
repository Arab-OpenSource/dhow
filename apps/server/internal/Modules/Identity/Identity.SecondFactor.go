package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Tx"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity/Models"
)

const fieldRequireSecondFactor = "RequireSecondFactor"

func organizationOf(id uuid.UUID, name, slug string, created time.Time, rule string, since *time.Time) models.Organization {
	return models.Organization{
		ID: id, Name: name, Slug: slug, CreatedAt: created, RequireSecondFactor: rule, RequireSecondFactorSince: since,
	}
}

func orNow(t *time.Time, now time.Time) *time.Time {
	if t == nil {
		return &now
	}
	return t
}

func nextSince(current, next policy.SecondFactorRule, since, adminsSince *time.Time, now time.Time) (*time.Time, *time.Time) {
	if next == policy.SecondFactorOff {
		return nil, nil
	}
	if current == policy.SecondFactorOff {
		return &now, &now
	}
	if adminsSince == nil {
		adminsSince = since
	}
	adminsSince = orNow(adminsSince, now)
	switch {
	case next.Reach() > current.Reach():
		return &now, adminsSince
	case next == policy.SecondFactorAdmins:
		return adminsSince, adminsSince
	default:
		return orNow(since, now), adminsSince
	}
}

func (s *Service) UpdateSecondFactorRule(ctx context.Context, org uuid.UUID, rule string, factor auth.Factor) (models.Organization, error) {
	p, _, err := actorIn(ctx, org)
	if err != nil {
		return models.Organization{}, err
	}
	next, ok := policy.ParseSecondFactorRule(rule)
	if !ok {
		return models.Organization{}, apperrors.ValidationFailed(invalid(fieldRequireSecondFactor))
	}
	state, err := s.queries().GetUserMfaState(ctx, p.UserID)
	if err != nil {
		return models.Organization{}, err
	}
	if !state.HasTotp && !state.HasPasskey {
		return models.Organization{}, apperrors.SecondFactorRequired()
	}
	if err := s.factors.ConfirmFactor(ctx, p, factor); err != nil {
		return models.Organization{}, err
	}
	now := s.now()
	var out models.Organization
	var from policy.SecondFactorRule
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		locked, err := q.LockOrganization(ctx, org)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound()
		}
		if err != nil {
			return err
		}
		self, err := loadMember(ctx, q, org, p.UserID)
		if err != nil {
			return err
		}
		if self.Direct != policy.RoleOwner {
			return apperrors.Forbidden()
		}
		current, known := policy.ParseSecondFactorRule(locked.RequireSecondFactor)
		if !known {
			current = policy.SecondFactorEveryone
		}
		from = current
		since, adminsSince := nextSince(current, next, locked.RequireSecondFactorSince, locked.RequireSecondFactorAdminsSince, now)
		row, err := q.SetOrganizationSecondFactor(ctx, sqlc.SetOrganizationSecondFactorParams{
			ID: org, Rule: string(next), Since: since, AdminsSince: adminsSince, Now: now,
		})
		if err != nil {
			return err
		}
		out = organizationOf(row.Id, row.Name, row.Slug, row.CreatedAt, row.RequireSecondFactor, row.RequireSecondFactorSince)
		return nil
	})
	if err != nil {
		return models.Organization{}, err
	}
	if from == next {
		return out, nil
	}
	data := map[string]string{"From": string(from), "To": string(next)}
	if next.Reach() < from.Reach() {
		data[eventbus.SeverityKey] = eventbus.SeverityHigh
	}
	return out, s.audit(ctx, eventbus.IdentitySecondFactorChanged, []uuid.UUID{org}, targetOrganization, org.String(), data)
}
