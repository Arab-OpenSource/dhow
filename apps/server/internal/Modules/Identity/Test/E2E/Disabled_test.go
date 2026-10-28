package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func TestADisabledOwnerIsNotACoOwner(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	_, ghost := env.SignedIn(org, "ghost@example.com", "owner")
	_, frozen := env.SignedIn(org, "frozen@example.com", "admin")
	setUserFlag(t, env, "Disabled", ghost)
	setUserFlag(t, env, "Disabled", frozen)

	leave := owner.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/leave")})
	assert.Equal(t, "LAST_OWNER", leave.Error.Code, "a disabled owner cannot keep the organization: %s", leave.Raw)
	demote := owner.ChangeRole(org, owner.Me().User.Id, "admin", apptest.PasswordFactor(d.Password))
	assert.NotEqual(t, http.StatusOK, demote.Status)

	transfer := owner.Do(apptest.Request{
		Method: http.MethodPost, Path: apptest.OrgPath(org, "/transfer-ownership"),
		Body: map[string]any{"UserId": frozen.String(), "CurrentFactor": apptest.PasswordFactor(d.Password)},
	})
	assert.Equal(t, "LAST_OWNER", transfer.Error.Code, "ownership never moves to a disabled account alone: %s", transfer.Raw)
	assert.Equal(t, "owner", owner.RoleIn(org))

	demoteGhost := owner.ChangeRole(org, ghost.String(), "admin", apptest.PasswordFactor(d.Password))
	assert.Equal(t, http.StatusOK, demoteGhost.Status, "the active owner can still demote the disabled one: %s", demoteGhost.Raw)
	removeGhost := owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/members/"+ghost.String())})
	assert.Equal(t, http.StatusOK, removeGhost.Status, "and then remove them: %s", removeGhost.Raw)
}

func TestADisabledInviterInvalidatesTheInvitation(t *testing.T) {
	env := apptest.New(t)
	_, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	admin, adminID := env.SignedIn(org, "admin@example.com", "admin")
	issued := admin.MustInvite(org, "new@example.com", "developer")
	require.Equal(t, http.StatusOK, env.Preview(issued.Token).Status)

	setUserFlag(t, env, "Disabled", adminID)
	assert.Equal(t, http.StatusGone, env.Preview(issued.Token).Status)
	assert.Equal(t, http.StatusGone, env.AcceptNew(issued.Token, "New", "a long password for new@example.com").Status)
}
