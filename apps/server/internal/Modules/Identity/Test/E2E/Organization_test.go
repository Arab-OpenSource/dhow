package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

type org struct{ Id, Name, Slug, CreatedAt string }

func TestOrganizationReadAndRename(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	id := env.OrganizationID(d.OrgSlug).String()

	res := client.Get("/organizations/" + id)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var got org
	res.Decode(t, &got)
	assert.Equal(t, "Dhow Co", got.Name)
	assert.Equal(t, "dhow-co", got.Slug)
	assert.Equal(t, id, got.Id)

	renamed := client.Do(apptest.Request{Method: http.MethodPatch, Path: "/organizations/" + id, Body: map[string]string{"Name": "Dhow Cooperative"}})
	require.Equal(t, http.StatusOK, renamed.Status, string(renamed.Raw))
	renamed.Decode(t, &got)
	assert.Equal(t, "Dhow Cooperative", got.Name)
	assert.Equal(t, "dhow-co", got.Slug, "the slug is not editable")

	bad := client.Do(apptest.Request{Method: http.MethodPatch, Path: "/organizations/" + id, Body: map[string]string{"Name": "  "}})
	assert.Equal(t, http.StatusUnprocessableEntity, bad.Status)
	assert.Equal(t, "Name", bad.Error.Details[0].Field)
}

func TestMembers(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	orgID := env.OrganizationID(d.OrgSlug)
	for _, role := range []string{"admin", "viewer"} {
		env.SeedUser(orgID, role+"@example.com", "a long password for "+role, role)
	}
	res := client.Get("/organizations/" + orgID.String() + "/members?Limit=2&Page=1")
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var members []struct{ UserId, Name, Email, Role, JoinedAt string }
	res.Decode(t, &members)
	require.Len(t, members, 2)
	assert.Equal(t, "owner", members[0].Role)
	require.NotNil(t, res.Meta)
	assert.Equal(t, 1, res.Meta.Page)
	assert.Equal(t, 2, res.Meta.Limit)
	assert.Equal(t, 3, res.Meta.Total)
	assert.NotContains(t, string(res.Raw), "Hash")
	assert.NotContains(t, string(res.Raw), "argon2")

	second := client.Get("/organizations/" + orgID.String() + "/members?Limit=2&Page=2")
	second.Decode(t, &members)
	assert.Len(t, members, 1)
}

func TestTenantIsolation(t *testing.T) {
	env := apptest.New(t)
	alice, d := env.SetupAndLogin()
	aliceOrg := env.OrganizationID(d.OrgSlug).String()
	bobOrg := env.SeedOrganization("Bob Inc", "bob-inc")
	env.SeedUser(bobOrg, "bob@example.com", "bob has a long password", "owner")
	bob, res := env.Login("bob@example.com", "bob has a long password")
	require.Equal(t, http.StatusOK, res.Status)

	for _, path := range []string{"", "/members", "/audit-events", "/audit-events/verify"} {
		cross := bob.Get("/organizations/" + aliceOrg + path)
		assert.Equal(t, http.StatusNotFound, cross.Status, "bob reading alice's %q", path)
		assert.Equal(t, "NOT_FOUND", cross.Error.Code)
		mine := alice.Get("/organizations/" + aliceOrg + path)
		assert.Equal(t, http.StatusOK, mine.Status, "alice reading her own %q", path)
	}
	rename := bob.Do(apptest.Request{Method: http.MethodPatch, Path: "/organizations/" + aliceOrg, Body: map[string]string{"Name": "Hijacked"}})
	assert.Equal(t, http.StatusNotFound, rename.Status)
	var got org
	alice.Get("/organizations/"+aliceOrg).Decode(t, &got)
	assert.Equal(t, "Dhow Co", got.Name)

	unknown := alice.Get("/organizations/0198a000-0000-7000-8000-000000000000")
	assert.Equal(t, http.StatusNotFound, unknown.Status, "an organization that does not exist looks exactly like one that belongs to someone else")
	assert.JSONEq(t, string(unknown.Raw), string(bob.Get("/organizations/"+aliceOrg).Raw))
}

func TestRoleBoundariesOnOrganizationActions(t *testing.T) {
	env := apptest.New(t)
	_, d := env.SetupAndLogin()
	orgID := env.OrganizationID(d.OrgSlug)
	id := orgID.String()

	type expectation struct{ read, rename, members, audit int }
	want := map[string]expectation{
		"owner":      {200, 200, 200, 200},
		"admin":      {200, 200, 200, 200},
		"maintainer": {200, 403, 200, 403},
		"developer":  {200, 403, 200, 403},
		"viewer":     {200, 403, 403, 403},
	}
	for role, exp := range want {
		t.Run(role, func(t *testing.T) {
			email := role + "-matrix@example.com"
			env.SeedUser(orgID, email, "a long password for "+role, role)
			c, res := env.Login(email, "a long password for "+role)
			require.Equal(t, http.StatusOK, res.Status)
			assert.Equal(t, exp.read, c.Get("/organizations/"+id).Status, "org.read")
			assert.Equal(t, exp.members, c.Get("/organizations/"+id+"/members").Status, "member.read")
			assert.Equal(t, exp.audit, c.Get("/organizations/"+id+"/audit-events").Status, "audit.read")
			rename := c.Do(apptest.Request{Method: http.MethodPatch, Path: "/organizations/" + id, Body: map[string]string{"Name": role + " was here"}})
			assert.Equal(t, exp.rename, rename.Status, "org.update")
		})
	}
}
