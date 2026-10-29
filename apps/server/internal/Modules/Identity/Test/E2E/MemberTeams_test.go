package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

type memberTeam struct{ Id, Name string }

func TestTheMemberListCarriesEachMembersTeams(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	dev, devID := env.SignedIn(org, "dev@example.com", "developer")
	viewer, viewerID := env.SignedIn(org, "viewer@example.com", "viewer")
	beta := createTeam(t, owner, org, "beta", "")
	alpha := createTeam(t, owner, org, "Alpha", "")
	gone := createTeam(t, owner, org, "Gone", "")
	for _, team := range []string{beta, alpha, gone} {
		res := addToTeam(t, owner, org, team, devID.String())
		require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	}
	require.Equal(t, http.StatusOK, addToTeam(t, owner, org, beta, viewerID.String()).Status)
	require.Equal(t, http.StatusOK, owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/teams/"+gone)}).Status)

	assert.Equal(t, http.StatusForbidden, viewer.Get(apptest.OrgPath(org, "/members")).Status, "without member.read, no teams either")
	res := dev.Get(apptest.OrgPath(org, "/members"))
	require.Equal(t, http.StatusOK, res.Status, "member.read covers the teams too: %s", res.Raw)
	var members []struct {
		UserId string
		Teams  []memberTeam
	}
	res.Decode(t, &members)
	byUser := map[string][]memberTeam{}
	for _, m := range members {
		byUser[m.UserId] = m.Teams
	}
	assert.Equal(t, []memberTeam{{alpha, "Alpha"}, {beta, "beta"}}, byUser[devID.String()], "ordered by name, without a deleted team")
	assert.Equal(t, []memberTeam{{beta, "beta"}}, byUser[viewerID.String()])
	ownerTeams, ok := byUser[owner.UserID]
	require.True(t, ok)
	assert.Equal(t, []memberTeam{}, ownerTeams, "a member without teams gets an empty list, not null")
	assert.Contains(t, string(res.Raw), `"Teams":[]`)
}
