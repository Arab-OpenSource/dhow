package identity

import (
	"context"
	"strconv"

	"github.com/google/uuid"

	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Khaymah/dhow/apps/server/internal/Database/Tx"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Identity/Models"
)

func checkRoleChange(actor policy.Role, target member, next policy.Role) error {
	if !policy.CanManage(actor, target.Direct) || !policy.CanGrant(actor, next) {
		return apperrors.RoleNotAllowed()
	}
	return nil
}

func (s *Service) ChangeRole(ctx context.Context, org, target uuid.UUID, role string, factor *auth.Factor) (models.MemberChange, error) {
	p, actor, err := actorIn(ctx, org)
	if err != nil {
		return models.MemberChange{}, err
	}
	next, ok := policy.ParseRole(role)
	if !ok {
		return models.MemberChange{}, apperrors.ValidationFailed(invalid(fieldRole))
	}
	if target == p.UserID {
		return models.MemberChange{}, apperrors.ValidationFailed(detail(fieldUserID, messageSelfRoleChange))
	}
	current, err := loadMember(ctx, s.queries(), org, target)
	if err != nil {
		return models.MemberChange{}, err
	}
	if err := checkRoleChange(actor, current, next); err != nil {
		return models.MemberChange{}, err
	}
	if current.Direct == next {
		return models.MemberChange{UserID: target, Role: string(next)}, nil
	}
	owner := current.Direct == policy.RoleOwner || next == policy.RoleOwner
	if owner {
		if factor == nil {
			return models.MemberChange{}, apperrors.ValidationFailed(detail(fieldCurrentFactor, messageFactorRequired))
		}
		if err := s.factors.ConfirmFactor(ctx, p, *factor); err != nil {
			return models.MemberChange{}, err
		}
	}
	now := s.now()
	var revoked int64
	var from policy.Role
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := lockOrganization(ctx, q, org); err != nil {
			return err
		}
		self, err := loadMember(ctx, q, org, p.UserID)
		if err != nil {
			return err
		}
		fresh, err := loadMember(ctx, q, org, target)
		if err != nil {
			return err
		}
		if err := checkRoleChange(self.Effective, fresh, next); err != nil {
			return err
		}
		if fresh.Direct == policy.RoleOwner && !owner {
			return apperrors.Conflict()
		}
		if fresh.Direct == policy.RoleOwner && next != policy.RoleOwner {
			if err := protectLastOwner(ctx, q, org, target, fresh.Direct); err != nil {
				return err
			}
		}
		from = fresh.Direct
		if err := setRole(ctx, q, org, target, next, now); err != nil {
			return err
		}
		revoked, err = q.RevokeAllUserSessions(ctx, sqlc.RevokeAllUserSessionsParams{UserID: target, Now: now})
		return err
	})
	if err != nil {
		return models.MemberChange{}, err
	}
	data := map[string]string{"From": string(from), "To": string(next), "RevokedSessions": itoa(revoked)}
	if owner {
		data[eventbus.SeverityKey] = eventbus.SeverityHigh
	}
	change := models.MemberChange{UserID: target, Role: string(next), Revoked: revoked}
	return change, s.audit(ctx, eventbus.IdentityMemberRoleChanged, []uuid.UUID{org}, targetUser, target.String(), data)
}

func (s *Service) RemoveMember(ctx context.Context, org, target uuid.UUID) (models.MemberChange, error) {
	p, actor, err := actorIn(ctx, org)
	if err != nil {
		return models.MemberChange{}, err
	}
	if target == p.UserID {
		if p.ViaToken() {
			return models.MemberChange{}, apperrors.SessionRequired()
		}
		if err := s.removeSelf(ctx, p, org); err != nil {
			return models.MemberChange{}, err
		}
		return models.MemberChange{UserID: target}, s.audit(ctx, eventbus.IdentityMemberLeft, []uuid.UUID{org}, targetUser, target.String(), nil)
	}
	current, err := loadMember(ctx, s.queries(), org, target)
	if err != nil {
		return models.MemberChange{}, err
	}
	if !policy.CanManage(actor, current.Direct) {
		return models.MemberChange{}, apperrors.RoleNotAllowed()
	}
	if current.Direct == policy.RoleOwner {
		return models.MemberChange{}, apperrors.ValidationFailed(detail(fieldUserID, messageOwnerRemoval))
	}
	now := s.now()
	var revoked int64
	var role policy.Role
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := lockOrganization(ctx, q, org); err != nil {
			return err
		}
		self, err := loadMember(ctx, q, org, p.UserID)
		if err != nil {
			return err
		}
		fresh, err := loadMember(ctx, q, org, target)
		if err != nil {
			return err
		}
		if !policy.CanManage(self.Effective, fresh.Direct) {
			return apperrors.RoleNotAllowed()
		}
		if fresh.Direct == policy.RoleOwner {
			return apperrors.ValidationFailed(detail(fieldUserID, messageOwnerRemoval))
		}
		role = fresh.Direct
		if err := detach(ctx, q, org, target, now); err != nil {
			return err
		}
		revoked, err = q.RevokeAllUserSessions(ctx, sqlc.RevokeAllUserSessionsParams{UserID: target, Now: now})
		return err
	})
	if err != nil {
		return models.MemberChange{}, err
	}
	return models.MemberChange{UserID: target, Revoked: revoked}, s.audit(ctx, eventbus.IdentityMemberRemoved, []uuid.UUID{org}, targetUser,
		target.String(), map[string]string{"Role": string(role), "RevokedSessions": itoa(revoked)})
}

func (s *Service) coversEveryMembership(ctx context.Context, q *sqlc.Queries, p auth.Principal, target uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.ListUserRoles(ctx, target)
	if err != nil {
		return nil, err
	}
	orgs := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		role, ok := policy.EffectiveOf(r.Role, r.TeamRoles)
		if !ok {
			continue
		}
		mine, member := p.RoleIn(r.OrganizationId)
		if !member || !policy.CanResetMfa(mine, role, r.IsInstanceOwner) {
			return nil, forbiddenBecause(messageMfaResetScope)
		}
		orgs = append(orgs, r.OrganizationId)
	}
	return orgs, nil
}

func (s *Service) ResetMfa(ctx context.Context, org, target uuid.UUID, factor auth.Factor) (models.MemberChange, error) {
	p, _, err := actorIn(ctx, org)
	if err != nil {
		return models.MemberChange{}, err
	}
	if target == p.UserID {
		return models.MemberChange{}, apperrors.ValidationFailed(detail(fieldUserID, messageSelfMfaReset))
	}
	q := s.queries()
	if _, err := loadMember(ctx, q, org, target); err != nil {
		return models.MemberChange{}, err
	}
	if _, err := s.coversEveryMembership(ctx, q, p, target); err != nil {
		return models.MemberChange{}, err
	}
	state, err := q.GetUserMfaState(ctx, target)
	if err != nil {
		return models.MemberChange{}, err
	}
	if !state.HasTotp && !state.HasPasskey && !state.HasRecoveryCodes {
		return models.MemberChange{}, conflictBecause("", messageMfaNotEnabled)
	}
	if err := s.factors.ConfirmFactor(ctx, p, factor); err != nil {
		return models.MemberChange{}, err
	}
	now := s.now()
	var revoked, totp, passkeys, codes, tokens int64
	var orgs []uuid.UUID
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := lockOrganization(ctx, q, org); err != nil {
			return err
		}
		if _, err := loadMember(ctx, q, org, target); err != nil {
			return err
		}
		if orgs, err = s.coversEveryMembership(ctx, q, p, target); err != nil {
			return err
		}
		if totp, err = q.RetireUserTotp(ctx, sqlc.RetireUserTotpParams{UserID: target, Now: now}); err != nil {
			return err
		}
		if passkeys, err = q.RetireUserPasskeys(ctx, sqlc.RetireUserPasskeysParams{UserID: target, Now: now}); err != nil {
			return err
		}
		if codes, err = q.RetireUserRecoveryCodes(ctx, sqlc.RetireUserRecoveryCodesParams{UserID: target, Now: now}); err != nil {
			return err
		}
		if err := q.ExpireUserAuthChallenges(ctx, sqlc.ExpireUserAuthChallengesParams{UserID: &target, Now: now}); err != nil {
			return err
		}
		if err := q.MarkMfaReset(ctx, sqlc.MarkMfaResetParams{ID: target, Now: now}); err != nil {
			return err
		}
		if tokens, err = q.RevokeUserApiTokens(ctx, sqlc.RevokeUserApiTokensParams{UserID: target, ActorID: &p.UserID, Now: now}); err != nil {
			return err
		}
		revoked, err = q.RevokeAllUserSessions(ctx, sqlc.RevokeAllUserSessionsParams{UserID: target, Now: now})
		return err
	})
	if err != nil {
		return models.MemberChange{}, err
	}
	return models.MemberChange{UserID: target, Revoked: revoked}, s.audit(ctx, eventbus.IdentityMemberMfaReset, orgs, targetUser, target.String(),
		map[string]string{
			"Totp": strconv.FormatBool(totp > 0), "Passkeys": itoa(passkeys), "RecoveryCodes": itoa(codes),
			"RevokedSessions": itoa(revoked), "RevokedPats": itoa(tokens), eventbus.SeverityKey: eventbus.SeverityHigh,
		})
}
