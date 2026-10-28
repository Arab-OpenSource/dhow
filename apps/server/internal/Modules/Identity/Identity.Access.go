package identity

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
)

const (
	fieldUserID        = "UserId"
	fieldRole          = "Role"
	fieldCurrentFactor = "CurrentFactor"
	fieldSlug          = "Slug"
	fieldDescription   = "Description"
	fieldScope         = "Scope"
	fieldToken         = "Token"

	messageSelfRoleChange = "errors.selfRoleChange"
	messageOwnerRemoval   = "errors.ownerRemoval"
	messageSelfMfaReset   = "errors.selfMfaReset"
	messageFactorRequired = "errors.factorRequired"
	messageTransferToSelf = "errors.transferToSelf"
	messageNotAMember     = "errors.notAMember"
	messageMfaResetScope  = "errors.mfaResetScope"
	messageMfaNotEnabled  = "errors.mfaNotEnabled"
	messageTeamNameTaken  = "errors.teamNameTaken"
	messageBindingExists  = "errors.bindingExists"

	targetUser         = "user"
	targetOrganization = "organization"
	targetTeam         = "team"
	targetInvitation   = "invitation"
)

type member struct {
	sqlc.GetMemberRow
	Direct    policy.Role
	Effective policy.Role
}

func principal(ctx context.Context) (auth.Principal, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return auth.Principal{}, apperrors.Unauthenticated()
	}
	return p, nil
}

func actorIn(ctx context.Context, org uuid.UUID) (auth.Principal, policy.Role, error) {
	p, err := principal(ctx)
	if err != nil {
		return auth.Principal{}, "", err
	}
	role, ok := p.RoleIn(org)
	if !ok {
		return auth.Principal{}, "", apperrors.NotFound()
	}
	return p, role, nil
}

func loadMember(ctx context.Context, q *sqlc.Queries, org, user uuid.UUID) (member, error) {
	row, err := q.GetMember(ctx, sqlc.GetMemberParams{OrganizationID: org, UserID: user})
	if errors.Is(err, pgx.ErrNoRows) {
		return member{}, apperrors.NotFound()
	}
	if err != nil {
		return member{}, err
	}
	direct, ok := policy.ParseRole(row.Role)
	if !ok {
		return member{}, apperrors.NotFound()
	}
	return member{GetMemberRow: row, Direct: direct, Effective: policy.Effective(direct, roles(row.TeamRoles)...)}, nil
}

func isNotFound(err error) bool {
	var ae *apperrors.AppError
	return errors.As(err, &ae) && ae.Code == apperrors.CodeNotFound
}

func roles(values []string) []policy.Role {
	out := make([]policy.Role, 0, len(values))
	for _, v := range values {
		if r, ok := policy.ParseRole(v); ok {
			out = append(out, r)
		}
	}
	return out
}

func lockOrganization(ctx context.Context, q *sqlc.Queries, org uuid.UUID) error {
	_, err := q.LockOrganization(ctx, org)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperrors.NotFound()
	}
	return err
}

func detail(field, message string) apperrors.IErrorDetail {
	return apperrors.IErrorDetail{Field: field, Message: message}
}

func forbiddenBecause(message string) *apperrors.AppError {
	return apperrors.New(http.StatusForbidden, apperrors.CodeForbidden, message)
}

func conflictBecause(field, message string) *apperrors.AppError {
	e := apperrors.Conflict()
	if field != "" {
		e.Details = []apperrors.IErrorDetail{detail(field, message)}
		return e
	}
	e.Message = message
	return e
}

func (s *Service) audit(ctx context.Context, name string, orgs []uuid.UUID, targetType, targetID string, data map[string]string) error {
	p, err := principal(ctx)
	if err != nil {
		return err
	}
	return s.auditAs(ctx, p.UserID, name, orgs, targetType, targetID, data)
}

func (s *Service) auditAs(ctx context.Context, actor uuid.UUID, name string, orgs []uuid.UUID, targetType, targetID string, data map[string]string) error {
	return s.bus.Publish(ctx, eventbus.Event{Name: name, Payload: eventbus.Audited{
		OrganizationIDs: orgs, ActorUserID: &actor, TargetType: targetType, TargetID: targetID,
		IP: auth.RequestMetaFrom(ctx).IPString(), Data: data,
	}})
}

func effectiveRoles(ctx context.Context, q *sqlc.Queries, org uuid.UUID, users []uuid.UUID) (map[uuid.UUID]policy.Role, error) {
	out := make(map[uuid.UUID]policy.Role, len(users))
	if len(users) == 0 {
		return out, nil
	}
	rows, err := q.ListMemberRoles(ctx, sqlc.ListMemberRolesParams{OrganizationID: org, UserIds: users})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if role, ok := policy.EffectiveOf(r.Role, r.TeamRoles); ok {
			out[r.UserId] = role
		}
	}
	return out, nil
}

func (s *Service) revokeChanged(ctx context.Context, q *sqlc.Queries, p auth.Principal, before, after map[uuid.UUID]policy.Role) (int64, error) {
	var total int64
	now := s.now()
	for user, was := range before {
		if after[user] == was {
			continue
		}
		var n int64
		var err error
		if user == p.UserID {
			n, err = q.RevokeUserSessionsExcept(ctx, sqlc.RevokeUserSessionsExceptParams{UserID: user, ExceptID: p.SessionID, Now: now})
		} else {
			n, err = q.RevokeAllUserSessions(ctx, sqlc.RevokeAllUserSessionsParams{UserID: user, Now: now})
		}
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}
