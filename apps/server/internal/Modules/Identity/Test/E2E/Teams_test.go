package e2e_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

type teamView struct {
	Id, Name, Description string
	MemberCount           int
	Bindings              []struct{ Id, Scope, Role string }
	Members               []struct{ UserId, Name, Email, Role string }
}

func createTeam(t *testing.T, c *apptest.Client, org uuid.UUID, name, role string) string {
	t.Helper()
	body := map[string]string{"Name": name}
	if role != "" {
		body["Role"] = role
	}
	res := c.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/teams"), Body: body})
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	var team teamView
	res.Decode(t, &team)
	return team.Id
}

func addToTeam(t *testing.T, c *apptest.Client, org uuid.UUID, team, user string) apptest.Response {
	t.Helper()
	return c.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/teams/"+team+"/members"), Body: map[string]string{"UserId": user}})
}

func TestTeamCrud(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	id := createTeam(t, owner, org, "Backend", "developer")

	dup := owner.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/teams"), Body: map[string]string{"Name": "backend"}})
	assert.Equal(t, http.StatusConflict, dup.Status, "names are unique ignoring case")
	bad := owner.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/teams"), Body: map[string]string{"Name": " "}})
	assert.Equal(t, http.StatusUnprocessableEntity, bad.Status)

	renamed := owner.Do(apptest.Request{
		Method: http.MethodPatch, Path: apptest.OrgPath(org, "/teams/"+id),
		Body: map[string]string{"Name": "Platform", "Description": "Runs the payments API"},
	})
	require.Equal(t, http.StatusOK, renamed.Status, string(renamed.Raw))
	var team teamView
	owner.Get(apptest.OrgPath(org, "/teams/"+id)).Decode(t, &team)
	assert.Equal(t, "Platform", team.Name)
	assert.Equal(t, "Runs the payments API", team.Description)
	require.Len(t, team.Bindings, 1)
	assert.Equal(t, "org", team.Bindings[0].Scope)
	assert.Equal(t, "developer", team.Bindings[0].Role)

	var list []teamView
	owner.Get(apptest.OrgPath(org, "/teams")).Decode(t, &list)
	require.Len(t, list, 1)

	del := owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/teams/"+id)})
	require.Equal(t, http.StatusOK, del.Status, string(del.Raw))
	assert.Equal(t, http.StatusNotFound, owner.Get(apptest.OrgPath(org, "/teams/"+id)).Status)
	for _, action := range []string{"identity.team.created", "identity.team.updated", "identity.team.deleted"} {
		assert.Equal(t, 1, env.AuditCount(action), action)
	}
}

func TestTeamBindingRaisesTheEffectiveRole(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	viewer, viewerID := env.SignedIn(org, "viewer@example.com", "viewer")
	team := createTeam(t, owner, org, "Admins", "")

	assert.Equal(t, http.StatusForbidden, viewer.Get(apptest.OrgPath(org, "/members")).Status)
	added := addToTeam(t, owner, org, team, viewerID.String())
	require.Equal(t, http.StatusOK, added.Status, string(added.Raw))
	assert.True(t, env.SessionAlive(viewer), "joining a team without a role changes nothing")

	bind := owner.Do(apptest.Request{
		Method: http.MethodPost, Path: apptest.OrgPath(org, "/teams/"+team+"/role-bindings"),
		Body: map[string]string{"Scope": "org", "Role": "admin"},
	})
	require.Equal(t, http.StatusCreated, bind.Status, string(bind.Raw))
	var change struct {
		Team    teamView
		Revoked int
	}
	bind.Decode(t, &change)
	assert.Equal(t, 1, change.Revoked, "the raised role rotates the member's session")
	assert.False(t, env.SessionAlive(viewer))

	again, _ := env.Login("viewer@example.com", "a long password for viewer@example.com")
	assert.Equal(t, "admin", again.RoleIn(org), "effective role is the higher of direct and team")
	assert.Equal(t, http.StatusOK, again.Get(apptest.OrgPath(org, "/members")).Status)
	assert.Equal(t, http.StatusOK, again.Get(apptest.OrgPath(org, "/audit-events")).Status)
	var members []struct{ Email, Role, EffectiveRole string }
	owner.Get(apptest.OrgPath(org, "/members")).Decode(t, &members)
	for _, m := range members {
		if m.Email == "viewer@example.com" {
			assert.Equal(t, "viewer", m.Role)
			assert.Equal(t, "admin", m.EffectiveRole)
		}
	}

	dup := owner.Do(apptest.Request{
		Method: http.MethodPost, Path: apptest.OrgPath(org, "/teams/"+team+"/role-bindings"),
		Body: map[string]string{"Scope": "org", "Role": "viewer"},
	})
	assert.Equal(t, http.StatusConflict, dup.Status, "one binding per scope")

	drop := owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/teams/"+team+"/role-bindings/"+change.Team.Bindings[0].Id)})
	require.Equal(t, http.StatusOK, drop.Status, string(drop.Raw))
	assert.False(t, env.SessionAlive(again), "the lowered role ends the session too")
	third, _ := env.Login("viewer@example.com", "a long password for viewer@example.com")
	assert.Equal(t, "viewer", third.RoleIn(org))
	assert.Equal(t, http.StatusForbidden, third.Get(apptest.OrgPath(org, "/members")).Status)
}

func TestTeamBindingsCannotEscalate(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	admin, adminID := env.SignedIn(org, "admin@example.com", "admin")
	dev, _ := env.SignedIn(org, "dev@example.com", "developer")
	team := createTeam(t, admin, org, "Ops", "")

	owned := admin.Do(apptest.Request{
		Method: http.MethodPost, Path: apptest.OrgPath(org, "/teams/"+team+"/role-bindings"),
		Body: map[string]string{"Scope": "org", "Role": "owner"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, owned.Status, "a binding never grants owner")
	ownedByOwner := owner.Do(apptest.Request{
		Method: http.MethodPost, Path: apptest.OrgPath(org, "/teams"),
		Body: map[string]string{"Name": "Owners", "Role": "owner"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, ownedByOwner.Status, "not even an owner can make a team of owners")
	project := admin.Do(apptest.Request{
		Method: http.MethodPost, Path: apptest.OrgPath(org, "/teams/"+team+"/role-bindings"),
		Body: map[string]string{"Scope": "project", "Role": "viewer"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, project.Status, "project scope arrives with projects")

	devTeam := dev.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/teams"), Body: map[string]string{"Name": "Mine", "Role": "admin"}})
	assert.Equal(t, http.StatusForbidden, devTeam.Status, "a developer cannot make teams")
	devJoin := dev.Do(apptest.Request{
		Method: http.MethodPost, Path: apptest.OrgPath(org, "/teams/"+team+"/members"),
		Body: map[string]string{"UserId": dev.Me().User.Id},
	})
	assert.Equal(t, http.StatusForbidden, devJoin.Status, "a developer cannot join a team by themselves")

	outsiderOrg := env.SeedOrganization("Other", "other")
	stranger := env.SeedUser(outsiderOrg, "stranger@example.com", "a long password for stranger", "owner")
	foreign := addToTeam(t, admin, org, team, stranger.String())
	assert.Equal(t, http.StatusUnprocessableEntity, foreign.Status, "only members of the organization join its teams")
	assert.Equal(t, http.StatusOK, addToTeam(t, admin, org, team, adminID.String()).Status)
}
