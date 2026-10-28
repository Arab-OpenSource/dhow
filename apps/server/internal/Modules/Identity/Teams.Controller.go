package identity

import (
	"context"

	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity/Gen"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity/Models"
)

const (
	messageTeams          = "identity.teams.listed"
	messageTeam           = "identity.team.loaded"
	messageTeamCreated    = "identity.team.created"
	messageTeamUpdated    = "identity.team.updated"
	messageTeamDeleted    = "identity.team.deleted"
	messageTeamMemberIn   = "identity.team.memberAdded"
	messageTeamMemberOut  = "identity.team.memberRemoved"
	messageBindingAdded   = "identity.team.bindingAdded"
	messageBindingChanged = "identity.team.bindingChanged"
	messageBindingRemoved = "identity.team.bindingRemoved"
)

func bindingsToGen(in []models.RoleBinding) []gen.RoleBinding {
	out := make([]gen.RoleBinding, 0, len(in))
	for _, b := range in {
		out = append(out, gen.RoleBinding{Id: b.ID, Scope: gen.BindingScope(b.Scope), Role: gen.BindableRole(b.Role), CreatedAt: b.CreatedAt})
	}
	return out
}

func teamToGen(t models.Team) gen.Team {
	return gen.Team{
		Id: t.ID, Name: t.Name, Description: t.Description, MemberCount: t.MemberCount, Bindings: bindingsToGen(t.Bindings),
		CreatedAt: t.CreatedAt,
	}
}

func teamDetailToGen(t models.TeamDetail) *gen.TeamDetail {
	members := make([]gen.TeamMember, 0, len(t.Members))
	for _, m := range t.Members {
		members = append(members, gen.TeamMember{UserId: m.UserID, Name: m.Name, Email: m.Email, Role: gen.Role(m.Role), AddedAt: m.AddedAt})
	}
	return &gen.TeamDetail{
		Id: t.ID, Name: t.Name, Description: t.Description, MemberCount: t.MemberCount, Bindings: bindingsToGen(t.Bindings),
		CreatedAt: t.CreatedAt, Members: members,
	}
}

func teamChangeToGen(c models.TeamChange) *gen.TeamChange {
	return &gen.TeamChange{Team: *teamDetailToGen(c.Team), Revoked: int(c.Revoked)}
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func (c *Controller) ListTeams(ctx context.Context, req gen.ListTeamsRequestObject) (gen.ListTeamsResponseObject, error) {
	teams, err := c.service.ListTeams(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Team, 0, len(teams))
	for _, t := range teams {
		items = append(items, teamToGen(t))
	}
	return gen.ListTeams200JSONResponse{Success: true, Message: ptr(messageTeams), Data: &items}, nil
}

func (c *Controller) GetTeam(ctx context.Context, req gen.GetTeamRequestObject) (gen.GetTeamResponseObject, error) {
	team, err := c.service.GetTeam(ctx, req.Id, req.TeamId)
	if err != nil {
		return nil, err
	}
	return gen.GetTeam200JSONResponse{Success: true, Message: ptr(messageTeam), Data: teamDetailToGen(team)}, nil
}

func (c *Controller) CreateTeam(ctx context.Context, req gen.CreateTeamRequestObject) (gen.CreateTeamResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	in := models.TeamInput{Name: req.Body.Name, Description: deref(req.Body.Description)}
	if req.Body.Role != nil {
		in.Role = string(*req.Body.Role)
	}
	team, err := c.service.CreateTeam(ctx, req.Id, in)
	if err != nil {
		return nil, err
	}
	return gen.CreateTeam201JSONResponse{Success: true, Message: ptr(messageTeamCreated), Data: teamDetailToGen(team)}, nil
}

func (c *Controller) UpdateTeam(ctx context.Context, req gen.UpdateTeamRequestObject) (gen.UpdateTeamResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	team, err := c.service.UpdateTeam(ctx, req.Id, req.TeamId, models.TeamInput{Name: req.Body.Name, Description: deref(req.Body.Description)})
	if err != nil {
		return nil, err
	}
	return gen.UpdateTeam200JSONResponse{Success: true, Message: ptr(messageTeamUpdated), Data: teamDetailToGen(team)}, nil
}

func (c *Controller) DeleteTeam(ctx context.Context, req gen.DeleteTeamRequestObject) (gen.DeleteTeamResponseObject, error) {
	revoked, err := c.service.DeleteTeam(ctx, req.Id, req.TeamId)
	if err != nil {
		return nil, err
	}
	return gen.DeleteTeam200JSONResponse{Success: true, Message: ptr(messageTeamDeleted), Data: &gen.RevokedCount{Revoked: int(revoked)}}, nil
}

func (c *Controller) AddTeamMember(ctx context.Context, req gen.AddTeamMemberRequestObject) (gen.AddTeamMemberResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	change, err := c.service.AddTeamMember(ctx, req.Id, req.TeamId, req.Body.UserId)
	if err != nil {
		return nil, err
	}
	return gen.AddTeamMember200JSONResponse{Success: true, Message: ptr(messageTeamMemberIn), Data: teamChangeToGen(change)}, nil
}

func (c *Controller) RemoveTeamMember(ctx context.Context, req gen.RemoveTeamMemberRequestObject) (gen.RemoveTeamMemberResponseObject, error) {
	change, err := c.service.RemoveTeamMember(ctx, req.Id, req.TeamId, req.UserId)
	if err != nil {
		return nil, err
	}
	return gen.RemoveTeamMember200JSONResponse{Success: true, Message: ptr(messageTeamMemberOut), Data: teamChangeToGen(change)}, nil
}

func (c *Controller) CreateRoleBinding(ctx context.Context, req gen.CreateRoleBindingRequestObject) (gen.CreateRoleBindingResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	change, err := c.service.CreateRoleBinding(ctx, req.Id, req.TeamId, string(req.Body.Scope), string(req.Body.Role))
	if err != nil {
		return nil, err
	}
	return gen.CreateRoleBinding201JSONResponse{Success: true, Message: ptr(messageBindingAdded), Data: teamChangeToGen(change)}, nil
}

func (c *Controller) UpdateRoleBinding(ctx context.Context, req gen.UpdateRoleBindingRequestObject) (gen.UpdateRoleBindingResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	change, err := c.service.UpdateRoleBinding(ctx, req.Id, req.TeamId, req.BindingId, string(req.Body.Role))
	if err != nil {
		return nil, err
	}
	return gen.UpdateRoleBinding200JSONResponse{Success: true, Message: ptr(messageBindingChanged), Data: teamChangeToGen(change)}, nil
}

func (c *Controller) DeleteRoleBinding(ctx context.Context, req gen.DeleteRoleBindingRequestObject) (gen.DeleteRoleBindingResponseObject, error) {
	change, err := c.service.DeleteRoleBinding(ctx, req.Id, req.TeamId, req.BindingId)
	if err != nil {
		return nil, err
	}
	return gen.DeleteRoleBinding200JSONResponse{Success: true, Message: ptr(messageBindingRemoved), Data: teamChangeToGen(change)}, nil
}
