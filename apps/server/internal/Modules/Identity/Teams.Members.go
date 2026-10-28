package identity

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Identity/Models"
)

func (s *Service) AddTeamMember(ctx context.Context, org, team, user uuid.UUID) (models.TeamChange, error) {
	now := s.now()
	change, err := s.teamChange(ctx, org, team, []uuid.UUID{user}, true, func(q *sqlc.Queries, _ *teamTx) error {
		if _, err := loadMember(ctx, q, org, user); isNotFound(err) {
			return apperrors.ValidationFailed(detail(fieldUserID, messageNotAMember))
		} else if err != nil {
			return err
		}
		return q.InsertTeamMember(ctx, sqlc.InsertTeamMemberParams{OrganizationID: org, TeamID: team, UserID: user, Now: now})
	})
	if uniqueConstraint(err) == constraintTeamMember {
		return models.TeamChange{}, apperrors.AlreadyMember()
	}
	if err != nil {
		return models.TeamChange{}, err
	}
	return change, s.audit(ctx, eventbus.IdentityTeamMemberAdded, []uuid.UUID{org}, targetTeam, team.String(),
		map[string]string{"UserId": user.String(), "RevokedSessions": itoa(change.Revoked)})
}

func (s *Service) RemoveTeamMember(ctx context.Context, org, team, user uuid.UUID) (models.TeamChange, error) {
	now := s.now()
	change, err := s.teamChange(ctx, org, team, nil, true, func(q *sqlc.Queries, _ *teamTx) error {
		n, err := q.RemoveTeamMember(ctx, sqlc.RemoveTeamMemberParams{TeamID: team, OrganizationID: org, UserID: user, Now: now})
		if err != nil {
			return err
		}
		if n == 0 {
			return apperrors.NotFound()
		}
		return nil
	})
	if err != nil {
		return models.TeamChange{}, err
	}
	return change, s.audit(ctx, eventbus.IdentityTeamMemberRemoved, []uuid.UUID{org}, targetTeam, team.String(),
		map[string]string{"UserId": user.String(), "RevokedSessions": itoa(change.Revoked)})
}

func bindableRole(v string) (policy.Role, *apperrors.AppError) {
	r, ok := policy.ParseRole(v)
	if !ok || !policy.Bindable(r) {
		return "", apperrors.ValidationFailed(invalid(fieldRole))
	}
	return r, nil
}

func (s *Service) CreateRoleBinding(ctx context.Context, org, team uuid.UUID, scope, role string) (models.TeamChange, error) {
	if scope != scopeOrg {
		return models.TeamChange{}, apperrors.ValidationFailed(invalid(fieldScope))
	}
	r, ae := bindableRole(role)
	if ae != nil {
		return models.TeamChange{}, ae
	}
	now := s.now()
	change, err := s.teamChange(ctx, org, team, nil, true, func(q *sqlc.Queries, t *teamTx) error {
		if !policy.CanGrant(t.actor, r) {
			return apperrors.RoleNotAllowed()
		}
		_, err := q.InsertTeamBinding(ctx, sqlc.InsertTeamBindingParams{OrganizationID: org, TeamID: &team, Scope: scope, Role: string(r), Now: now})
		return err
	})
	if uniqueConstraint(err) == constraintBinding {
		return models.TeamChange{}, conflictBecause(fieldScope, messageBindingExists)
	}
	if err != nil {
		return models.TeamChange{}, err
	}
	return change, s.audit(ctx, eventbus.IdentityRoleBindingAdded, []uuid.UUID{org}, targetTeam, team.String(),
		map[string]string{"Scope": scope, "Role": string(r), "RevokedSessions": itoa(change.Revoked)})
}

func lockBinding(ctx context.Context, q *sqlc.Queries, org, team, binding uuid.UUID) (sqlc.LockTeamBindingRow, error) {
	row, err := q.LockTeamBinding(ctx, sqlc.LockTeamBindingParams{ID: binding, TeamID: &team, OrganizationID: org})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, apperrors.NotFound()
	}
	return row, err
}

func (s *Service) UpdateRoleBinding(ctx context.Context, org, team, binding uuid.UUID, role string) (models.TeamChange, error) {
	r, ae := bindableRole(role)
	if ae != nil {
		return models.TeamChange{}, ae
	}
	now := s.now()
	var from string
	change, err := s.teamChange(ctx, org, team, nil, true, func(q *sqlc.Queries, t *teamTx) error {
		row, err := lockBinding(ctx, q, org, team, binding)
		if err != nil {
			return err
		}
		if !policy.CanGrant(t.actor, r) {
			return apperrors.RoleNotAllowed()
		}
		from = row.Role
		_, err = q.SetTeamBindingRole(ctx, sqlc.SetTeamBindingRoleParams{ID: binding, OrganizationID: org, Role: string(r), Now: now})
		return err
	})
	if err != nil {
		return models.TeamChange{}, err
	}
	return change, s.audit(ctx, eventbus.IdentityRoleBindingChanged, []uuid.UUID{org}, targetTeam, team.String(),
		map[string]string{"From": from, "To": string(r), "RevokedSessions": itoa(change.Revoked)})
}

func (s *Service) DeleteRoleBinding(ctx context.Context, org, team, binding uuid.UUID) (models.TeamChange, error) {
	now := s.now()
	var role string
	change, err := s.teamChange(ctx, org, team, nil, true, func(q *sqlc.Queries, _ *teamTx) error {
		row, err := lockBinding(ctx, q, org, team, binding)
		if err != nil {
			return err
		}
		role = row.Role
		_, err = q.DeleteTeamBinding(ctx, sqlc.DeleteTeamBindingParams{ID: binding, OrganizationID: org, Now: now})
		return err
	})
	if err != nil {
		return models.TeamChange{}, err
	}
	return change, s.audit(ctx, eventbus.IdentityRoleBindingRemoved, []uuid.UUID{org}, targetTeam, team.String(),
		map[string]string{"Role": role, "RevokedSessions": itoa(change.Revoked)})
}
