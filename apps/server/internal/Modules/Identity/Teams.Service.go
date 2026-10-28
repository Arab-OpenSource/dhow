package identity

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Khaymah/dhow/apps/server/internal/Database/Tx"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Identity/Models"
)

const (
	scopeOrg             = "org"
	constraintTeamName   = "UX_Teams_Organization_Name"
	constraintBinding    = "UX_RoleBindings_Team_Target"
	constraintTeamMember = "UX_TeamMembers_Team_User"
)

func validDescription(v string) bool {
	if utf8.RuneCountInString(v) > config.TeamDescriptionMax {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) && r != '\n' {
			return false
		}
	}
	return true
}

func validateTeam(in *models.TeamInput, withRole bool) *apperrors.AppError {
	var details []apperrors.IErrorDetail
	in.Description = strings.TrimSpace(in.Description)
	if !auth.ValidDisplayName(in.Name, maxNameRunes) {
		details = append(details, invalid(fieldName))
	}
	if !validDescription(in.Description) {
		details = append(details, invalid(fieldDescription))
	}
	if withRole && in.Role != "" {
		if r, ok := policy.ParseRole(in.Role); !ok || !policy.Bindable(r) {
			details = append(details, invalid(fieldRole))
		}
	}
	if len(details) > 0 {
		return apperrors.ValidationFailed(details...)
	}
	return nil
}

func uniqueConstraint(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return pgErr.ConstraintName
	}
	return ""
}

func managerIn(ctx context.Context, q *sqlc.Queries, org, user uuid.UUID) (policy.Role, error) {
	if err := lockOrganization(ctx, q, org); err != nil {
		return "", err
	}
	self, err := loadMember(ctx, q, org, user)
	if err != nil {
		return "", err
	}
	if !policy.Allows(self.Effective, policy.MemberManage) {
		return "", apperrors.Forbidden()
	}
	return self.Effective, nil
}

func (s *Service) ListTeams(ctx context.Context, org uuid.UUID) ([]models.Team, error) {
	q := s.queries()
	rows, err := q.ListTeams(ctx, org)
	if err != nil {
		return nil, err
	}
	bindings, err := q.ListOrganizationBindings(ctx, org)
	if err != nil {
		return nil, err
	}
	byTeam := map[uuid.UUID][]models.RoleBinding{}
	for _, b := range bindings {
		if b.TeamId == nil {
			continue
		}
		byTeam[*b.TeamId] = append(byTeam[*b.TeamId], models.RoleBinding{ID: b.Id, Scope: b.Scope, Role: b.Role, CreatedAt: b.CreatedAt})
	}
	out := make([]models.Team, 0, len(rows))
	for _, r := range rows {
		out = append(out, models.Team{
			ID: r.Id, Name: r.Name, Description: r.Description, MemberCount: int(r.MemberCount),
			Bindings: append([]models.RoleBinding{}, byTeam[r.Id]...), CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *Service) GetTeam(ctx context.Context, org, team uuid.UUID) (models.TeamDetail, error) {
	return loadTeam(ctx, s.queries(), org, team)
}

func loadTeam(ctx context.Context, q *sqlc.Queries, org, team uuid.UUID) (models.TeamDetail, error) {
	row, err := q.GetTeam(ctx, sqlc.GetTeamParams{ID: team, OrganizationID: org})
	if errors.Is(err, pgx.ErrNoRows) {
		return models.TeamDetail{}, apperrors.NotFound()
	}
	if err != nil {
		return models.TeamDetail{}, err
	}
	bindings, err := q.ListTeamBindings(ctx, sqlc.ListTeamBindingsParams{TeamID: &team, OrganizationID: org})
	if err != nil {
		return models.TeamDetail{}, err
	}
	members, err := q.ListTeamMembers(ctx, sqlc.ListTeamMembersParams{TeamID: team, OrganizationID: org})
	if err != nil {
		return models.TeamDetail{}, err
	}
	out := models.TeamDetail{Team: models.Team{
		ID: row.Id, Name: row.Name, Description: row.Description, MemberCount: len(members), CreatedAt: row.CreatedAt,
		Bindings: make([]models.RoleBinding, 0, len(bindings)),
	}, Members: make([]models.TeamMember, 0, len(members))}
	for _, b := range bindings {
		out.Bindings = append(out.Bindings, models.RoleBinding{ID: b.Id, Scope: b.Scope, Role: b.Role, CreatedAt: b.CreatedAt})
	}
	for _, m := range members {
		out.Members = append(out.Members, models.TeamMember{UserID: m.UserId, Name: m.Name, Email: m.Email, Role: m.Role, AddedAt: m.AddedAt})
	}
	return out, nil
}

func (s *Service) CreateTeam(ctx context.Context, org uuid.UUID, in models.TeamInput) (models.TeamDetail, error) {
	p, actor, err := actorIn(ctx, org)
	if err != nil {
		return models.TeamDetail{}, err
	}
	if ae := validateTeam(&in, true); ae != nil {
		return models.TeamDetail{}, ae
	}
	if in.Role != "" && !policy.CanGrant(actor, policy.Role(in.Role)) {
		return models.TeamDetail{}, apperrors.RoleNotAllowed()
	}
	now := s.now()
	var team models.TeamDetail
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		fresh, err := managerIn(ctx, q, org, p.UserID)
		if err != nil {
			return err
		}
		if in.Role != "" && !policy.CanGrant(fresh, policy.Role(in.Role)) {
			return apperrors.RoleNotAllowed()
		}
		row, err := q.InsertTeam(ctx, sqlc.InsertTeamParams{OrganizationID: org, Name: in.Name, Description: in.Description, Now: now})
		if err != nil {
			return err
		}
		if in.Role != "" {
			id := row.Id
			if _, err := q.InsertTeamBinding(ctx, sqlc.InsertTeamBindingParams{
				OrganizationID: org, TeamID: &id, Scope: scopeOrg, Role: in.Role, Now: now,
			}); err != nil {
				return err
			}
		}
		team, err = loadTeam(ctx, q, org, row.Id)
		return err
	})
	if uniqueConstraint(err) == constraintTeamName {
		return models.TeamDetail{}, conflictBecause(fieldName, messageTeamNameTaken)
	}
	if err != nil {
		return models.TeamDetail{}, err
	}
	return team, s.audit(ctx, eventbus.IdentityTeamCreated, []uuid.UUID{org}, targetTeam, team.ID.String(),
		map[string]string{"Name": team.Name, "Role": in.Role})
}

func (s *Service) UpdateTeam(ctx context.Context, org, team uuid.UUID, in models.TeamInput) (models.TeamDetail, error) {
	p, _, err := actorIn(ctx, org)
	if err != nil {
		return models.TeamDetail{}, err
	}
	if ae := validateTeam(&in, false); ae != nil {
		return models.TeamDetail{}, ae
	}
	var out models.TeamDetail
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if _, err := managerIn(ctx, q, org, p.UserID); err != nil {
			return err
		}
		_, err := q.UpdateTeam(ctx, sqlc.UpdateTeamParams{ID: team, OrganizationID: org, Name: in.Name, Description: in.Description, Now: s.now()})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound()
		}
		if err != nil {
			return err
		}
		out, err = loadTeam(ctx, q, org, team)
		return err
	})
	if uniqueConstraint(err) == constraintTeamName {
		return models.TeamDetail{}, conflictBecause(fieldName, messageTeamNameTaken)
	}
	if err != nil {
		return models.TeamDetail{}, err
	}
	return out, s.audit(ctx, eventbus.IdentityTeamUpdated, []uuid.UUID{org}, targetTeam, team.String(), map[string]string{"Name": out.Name})
}

type teamTx struct {
	p      auth.Principal
	actor  policy.Role
	org    uuid.UUID
	team   uuid.UUID
	before map[uuid.UUID]policy.Role
	users  []uuid.UUID
}

func (s *Service) beginTeamChange(ctx context.Context, q *sqlc.Queries, t *teamTx, extra ...uuid.UUID) error {
	actor, err := managerIn(ctx, q, t.org, t.p.UserID)
	if err != nil {
		return err
	}
	if _, err := q.LockTeam(ctx, sqlc.LockTeamParams{ID: t.team, OrganizationID: t.org}); errors.Is(err, pgx.ErrNoRows) {
		return apperrors.NotFound()
	} else if err != nil {
		return err
	}
	t.actor = actor
	bindings, err := q.ListTeamBindings(ctx, sqlc.ListTeamBindingsParams{TeamID: &t.team, OrganizationID: t.org})
	if err != nil {
		return err
	}
	for _, b := range bindings {
		if !policy.CanGrant(t.actor, policy.Role(b.Role)) {
			return apperrors.RoleNotAllowed()
		}
	}
	members, err := q.ListTeamMembers(ctx, sqlc.ListTeamMembersParams{TeamID: t.team, OrganizationID: t.org})
	if err != nil {
		return err
	}
	t.users = append([]uuid.UUID{}, extra...)
	for _, m := range members {
		t.users = append(t.users, m.UserId)
	}
	t.before, err = effectiveRoles(ctx, q, t.org, t.users)
	return err
}

func (s *Service) finishTeamChange(ctx context.Context, q *sqlc.Queries, t *teamTx) (int64, error) {
	after, err := effectiveRoles(ctx, q, t.org, t.users)
	if err != nil {
		return 0, err
	}
	return s.revokeChanged(ctx, q, t.p, t.before, after)
}

func (s *Service) teamChange(ctx context.Context, org, team uuid.UUID, extra []uuid.UUID, reload bool, change func(q *sqlc.Queries, t *teamTx) error) (models.TeamChange, error) {
	p, _, err := actorIn(ctx, org)
	if err != nil {
		return models.TeamChange{}, err
	}
	t := &teamTx{p: p, org: org, team: team}
	var out models.TeamChange
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := s.beginTeamChange(ctx, q, t, extra...); err != nil {
			return err
		}
		if err := change(q, t); err != nil {
			return err
		}
		if out.Revoked, err = s.finishTeamChange(ctx, q, t); err != nil {
			return err
		}
		if !reload {
			return nil
		}
		out.Team, err = loadTeam(ctx, q, org, team)
		return err
	})
	return out, err
}

func (s *Service) DeleteTeam(ctx context.Context, org, team uuid.UUID) (int64, error) {
	now := s.now()
	var name string
	change, err := s.teamChange(ctx, org, team, nil, false, func(q *sqlc.Queries, t *teamTx) error {
		row, err := q.GetTeam(ctx, sqlc.GetTeamParams{ID: team, OrganizationID: org})
		if err != nil {
			return err
		}
		name = row.Name
		if _, err := q.DeleteTeam(ctx, sqlc.DeleteTeamParams{ID: team, OrganizationID: org, Now: now}); err != nil {
			return err
		}
		if err := q.RemoveAllTeamMembers(ctx, sqlc.RemoveAllTeamMembersParams{TeamID: team, OrganizationID: org, Now: now}); err != nil {
			return err
		}
		return q.RemoveAllTeamBindings(ctx, sqlc.RemoveAllTeamBindingsParams{TeamID: &team, OrganizationID: org, Now: now})
	})
	if err != nil {
		return 0, err
	}
	return change.Revoked, s.audit(ctx, eventbus.IdentityTeamDeleted, []uuid.UUID{org}, targetTeam, team.String(),
		map[string]string{"Name": name, "RevokedSessions": itoa(change.Revoked)})
}
