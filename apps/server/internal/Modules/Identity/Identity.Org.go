package identity

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Identity/Models"
)

func (s *Service) GetOrganization(ctx context.Context, id uuid.UUID) (models.Organization, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return models.Organization{}, apperrors.Unauthenticated()
	}
	row, err := s.queries().GetOrganizationForMember(ctx, sqlc.GetOrganizationForMemberParams{ID: id, UserID: p.UserID})
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Organization{}, apperrors.NotFound()
	}
	if err != nil {
		return models.Organization{}, err
	}
	return organizationOf(row.Id, row.Name, row.Slug, row.CreatedAt, row.RequireSecondFactor, row.RequireSecondFactorSince), nil
}

func (s *Service) UpdateOrganization(ctx context.Context, id uuid.UUID, name string) (models.Organization, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return models.Organization{}, apperrors.Unauthenticated()
	}
	if !auth.ValidDisplayName(name, maxNameRunes) {
		return models.Organization{}, apperrors.ValidationFailed(invalid(fieldName))
	}
	row, err := s.queries().UpdateOrganizationName(ctx, sqlc.UpdateOrganizationNameParams{ID: id, UserID: p.UserID, Name: name, Now: s.now()})
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Organization{}, apperrors.NotFound()
	}
	if err != nil {
		return models.Organization{}, err
	}
	actor := p.UserID
	err = s.bus.Publish(ctx, eventbus.Event{Name: eventbus.IdentityOrganizationUpdated, Payload: eventbus.Audited{
		OrganizationIDs: []uuid.UUID{id}, ActorUserID: &actor, TargetType: "organization", TargetID: id.String(),
		IP: auth.RequestMetaFrom(ctx).IPString(), Data: map[string]string{"Name": name},
	}})
	if err != nil {
		return models.Organization{}, err
	}
	return organizationOf(row.Id, row.Name, row.Slug, row.CreatedAt, row.RequireSecondFactor, row.RequireSecondFactorSince), nil
}

func mfaOf(totp, passkey bool) string {
	switch {
	case passkey:
		return "passkey"
	case totp:
		return "totp"
	default:
		return "none"
	}
}

func (s *Service) ListMembers(ctx context.Context, id uuid.UUID, page, limit int) ([]models.Member, int64, error) {
	_, actor, err := actorIn(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	manager := policy.Allows(actor, policy.MemberManage)
	q := s.queries()
	total, err := q.CountMembers(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.ListMembers(ctx, sqlc.ListMembersParams{
		OrganizationID: id, PageLimit: int32(limit), PageOffset: int32((page - 1) * limit), //nolint:gosec // limit and page are bounded by response.PageParams.
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]models.Member, 0, len(rows))
	for _, r := range rows {
		effective, ok := policy.EffectiveOf(r.Role, r.TeamRoles)
		if !ok {
			continue
		}
		m := models.Member{UserID: r.UserId, Name: r.Name, Email: r.Email, Role: r.Role, EffectiveRole: string(effective), JoinedAt: r.JoinedAt}
		if manager {
			m.Mfa = mfaOf(r.HasTotp, r.HasPasskey)
			m.MfaResetPending = r.MfaResetAt != nil && m.Mfa == "none"
		}
		out = append(out, m)
	}
	return out, total, nil
}
