package identity

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Tx"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity/Models"
)

func (s *Service) CreateOrganization(ctx context.Context, name, slug string) (models.Organization, error) {
	p, err := principal(ctx)
	if err != nil {
		return models.Organization{}, err
	}
	var details []apperrors.IErrorDetail
	if !auth.ValidDisplayName(name, maxNameRunes) {
		details = append(details, invalid(fieldName))
	}
	if !validSlug(slug) {
		details = append(details, invalid(fieldSlug))
	}
	if len(details) > 0 {
		return models.Organization{}, apperrors.ValidationFailed(details...)
	}
	now := s.now()
	var org models.Organization
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		row, err := q.InsertOrganization(ctx, sqlc.InsertOrganizationParams{Name: name, Slug: slug, Now: now})
		if err != nil {
			return err
		}
		org = models.Organization{ID: row.Id, Name: row.Name, Slug: row.Slug, CreatedAt: row.CreatedAt}
		return q.InsertMembership(ctx, sqlc.InsertMembershipParams{
			UserID: p.UserID, OrganizationID: row.Id, Role: string(policy.RoleOwner), Now: now,
		})
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == "UX_Organizations_Slug" {
		return models.Organization{}, conflict(fieldSlug, messageSlugTaken)
	}
	if err != nil {
		return models.Organization{}, err
	}
	return org, s.audit(ctx, eventbus.IdentityOrganizationCreated, []uuid.UUID{org.ID}, targetOrganization, org.ID.String(),
		map[string]string{"Name": org.Name, "Slug": org.Slug})
}

func (s *Service) LeaveOrganization(ctx context.Context, org uuid.UUID) error {
	p, _, err := actorIn(ctx, org)
	if err != nil {
		return err
	}
	if err := s.removeSelf(ctx, p, org); err != nil {
		return err
	}
	return s.audit(ctx, eventbus.IdentityMemberLeft, []uuid.UUID{org}, targetUser, p.UserID.String(), nil)
}

func (s *Service) removeSelf(ctx context.Context, p auth.Principal, org uuid.UUID) error {
	now := s.now()
	return dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := lockOrganization(ctx, q, org); err != nil {
			return err
		}
		self, err := loadMember(ctx, q, org, p.UserID)
		if err != nil {
			return err
		}
		if err := protectLastOwner(ctx, q, org, p.UserID, self.Direct); err != nil {
			return err
		}
		return detach(ctx, q, org, p.UserID, now)
	})
}

func protectLastOwner(ctx context.Context, q *sqlc.Queries, org, user uuid.UUID, role policy.Role) error {
	if role != policy.RoleOwner {
		return nil
	}
	others, err := q.CountOtherActiveOwners(ctx, sqlc.CountOtherActiveOwnersParams{OrganizationID: org, UserID: user})
	if err != nil {
		return err
	}
	if others == 0 {
		return apperrors.LastOwner()
	}
	return nil
}

func detach(ctx context.Context, q *sqlc.Queries, org, user uuid.UUID, now time.Time) error {
	n, err := q.RemoveMembership(ctx, sqlc.RemoveMembershipParams{OrganizationID: org, UserID: user, Now: now})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NotFound()
	}
	return q.RemoveMemberTeams(ctx, sqlc.RemoveMemberTeamsParams{OrganizationID: org, UserID: user, Now: now})
}

func (s *Service) TransferOwnership(ctx context.Context, org, target uuid.UUID, factor auth.Factor) (models.MemberChange, error) {
	p, _, err := actorIn(ctx, org)
	if err != nil {
		return models.MemberChange{}, err
	}
	if target == p.UserID {
		return models.MemberChange{}, apperrors.ValidationFailed(detail(fieldUserID, messageTransferToSelf))
	}
	if _, err := loadMember(ctx, s.queries(), org, target); err != nil {
		return models.MemberChange{}, err
	}
	if err := s.factors.ConfirmFactor(ctx, p, factor); err != nil {
		return models.MemberChange{}, err
	}
	now := s.now()
	var revoked int64
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := lockOrganization(ctx, q, org); err != nil {
			return err
		}
		self, err := loadMember(ctx, q, org, p.UserID)
		if err != nil {
			return err
		}
		if self.Direct != policy.RoleOwner {
			return apperrors.Forbidden()
		}
		if _, err := loadMember(ctx, q, org, target); err != nil {
			return err
		}
		if err := setRole(ctx, q, org, target, policy.RoleOwner, now); err != nil {
			return err
		}
		if err := protectLastOwner(ctx, q, org, p.UserID, policy.RoleOwner); err != nil {
			return err
		}
		if err := setRole(ctx, q, org, p.UserID, policy.RoleAdmin, now); err != nil {
			return err
		}
		if revoked, err = q.RevokeAllUserSessions(ctx, sqlc.RevokeAllUserSessionsParams{UserID: target, Now: now}); err != nil {
			return err
		}
		_, err = q.RevokeUserSessionsExcept(ctx, sqlc.RevokeUserSessionsExceptParams{UserID: p.UserID, ExceptID: p.SessionID, Now: now})
		return err
	})
	if err != nil {
		return models.MemberChange{}, err
	}
	change := models.MemberChange{UserID: target, Role: string(policy.RoleOwner), Revoked: revoked}
	return change, s.audit(ctx, eventbus.IdentityOwnershipTransferred, []uuid.UUID{org}, targetUser, target.String(), map[string]string{
		"From": p.UserID.String(), "To": target.String(), "RevokedSessions": itoa(revoked), eventbus.SeverityKey: eventbus.SeverityHigh,
	})
}

func setRole(ctx context.Context, q *sqlc.Queries, org, user uuid.UUID, role policy.Role, now time.Time) error {
	n, err := q.SetMemberRole(ctx, sqlc.SetMemberRoleParams{OrganizationID: org, UserID: user, Role: string(role), Now: now})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NotFound()
	}
	return nil
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
