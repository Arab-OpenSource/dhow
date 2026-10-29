package e2e_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	identity "github.com/Khaymah/dhow/apps/server/internal/Modules/Identity"
)

func createOrg(c *apptest.Client, name, slug string) apptest.Response {
	return c.Do(apptest.Request{Method: http.MethodPost, Path: "/organizations", Body: map[string]string{"Name": name, "Slug": slug}})
}

func TestOnlyTheInstanceOwnerCreatesOrganizationsByDefault(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	first := env.OrganizationID(d.OrgSlug)
	admin, _ := env.SignedIn(first, "admin@example.com", "admin")
	assert.True(t, owner.Me().CanCreateOrganizations)
	assert.False(t, admin.Me().CanCreateOrganizations)

	denied := createOrg(admin, "Admin Corp", "admin-corp")
	assert.Equal(t, http.StatusForbidden, denied.Status)

	created := createOrg(owner, "Second Org", "second-org")
	require.Equal(t, http.StatusCreated, created.Status, string(created.Raw))
	var o org
	created.Decode(t, &o)
	assert.Equal(t, "owner", owner.RoleIn(uuid.MustParse(o.Id)))
	assert.Len(t, owner.Me().Memberships, 2)
	assert.Equal(t, http.StatusConflict, createOrg(owner, "Again", "second-org").Status, "slugs are unique")
	assert.Equal(t, http.StatusUnprocessableEntity, createOrg(owner, "Bad", "Not A Slug").Status)
	assert.Equal(t, 1, env.AuditCount("identity.organization.created"))
	assert.Equal(t, http.StatusNotFound, admin.Get("/organizations/"+o.Id).Status, "the admin of the first org sees nothing of the second")
}

func TestAnyUserCreatesOrganizationsWhenTheInstanceAllowsIt(t *testing.T) {
	env := apptest.NewWithOptions(t, apptest.Options{Profile: func(c *config.Config) { c.SetEnvSetting(config.EnvOrgCreation, config.OrgCreationAnyUser) }})
	_, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	viewer, _ := env.SignedIn(org, "viewer@example.com", "viewer")
	assert.True(t, viewer.Me().CanCreateOrganizations)
	res := createOrg(viewer, "Viewer Labs", "viewer-labs")
	assert.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
}

func TestLeaveOrganization(t *testing.T) {
	env := apptest.New(t)
	_, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	dev, _ := env.SignedIn(org, "dev@example.com", "developer")
	res := dev.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/leave")})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	assert.Empty(t, dev.Me().Memberships)
	assert.Equal(t, http.StatusNotFound, dev.Get(apptest.OrgPath(org, "")).Status)
	assert.Equal(t, 1, env.AuditCount("identity.member.left"))
}

type foreignIDs struct {
	user, team, binding, invitation string
}

func seedForeign(t *testing.T, env *apptest.Env) foreignIDs {
	t.Helper()
	other := env.SeedOrganization("Other", "other")
	bob, bobID := env.SignedIn(other, "bob@example.com", "owner")
	team := createTeam(t, bob, other, "Bob's team", "viewer")
	require.Equal(t, http.StatusOK, addToTeam(t, bob, other, team, bobID.String()).Status)
	var view teamView
	bob.Get(apptest.OrgPath(other, "/teams/"+team)).Decode(t, &view)
	inv := bob.MustInvite(other, "someone@example.com", "viewer")
	return foreignIDs{user: bobID.String(), team: team, binding: view.Bindings[0].Id, invitation: inv.Invitation.Id}
}

func TestCrossOrganizationIdsAreNotFoundOnEveryRoute(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	f := seedForeign(t, env)
	checked := 0
	for _, rule := range identity.Policies() {
		if !strings.HasPrefix(rule.Path, "/organizations/{id}/") || !strings.Contains(rule.Path[len("/organizations/{id}"):], "{") {
			continue
		}
		path := strings.NewReplacer("{id}", org.String(), "{userId}", f.user, "{teamId}", f.team,
			"{bindingId}", f.binding, "{invitationId}", f.invitation).Replace(rule.Path)
		body := map[string]any{
			"Role": "viewer", "Name": "x", "UserId": f.user, "Scope": "org",
			"CurrentFactor": apptest.PasswordFactor(d.Password),
		}
		res := owner.Do(apptest.Request{Method: rule.Method, Path: path, Body: body})
		assert.Equal(t, http.StatusNotFound, res.Status, "%s with another organization's ids: %s", rule.Key(), res.Raw)
		checked++
	}
	assert.GreaterOrEqual(t, checked, 12)

	bob, _ := env.Login("bob@example.com", "a long password for bob@example.com")
	var list []teamView
	bob.Get(apptest.OrgPath(env.OrganizationID("other"), "/teams")).Decode(t, &list)
	require.Len(t, list, 1)
	assert.Equal(t, 1, list[0].MemberCount, "nothing changed on the other side")
	var invites []apptest.Invitation
	bob.Get(apptest.OrgPath(env.OrganizationID("other"), "/invitations")).Decode(t, &invites)
	assert.Len(t, invites, 1)
}
