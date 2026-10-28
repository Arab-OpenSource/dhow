package e2e_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

func TestAnOwnerIsNeverRemovedWithoutTheCurrentFactor(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	_, coOwner := env.SignedIn(org, "co@example.com", "owner")
	path := apptest.OrgPath(org, "/members/"+coOwner.String())

	res := owner.Do(apptest.Request{Method: http.MethodDelete, Path: path})
	require.Equal(t, http.StatusUnprocessableEntity, res.Status, string(res.Raw))
	require.Len(t, res.Error.Details, 1)
	assert.Equal(t, "UserId", res.Error.Details[0].Field)

	token := env.SeedAPIToken(uuid.MustParse(owner.Me().User.Id), org, "member.manage")
	viaToken := env.Bearer(token.Value, apptest.Request{Method: http.MethodDelete, Path: path})
	assert.Equal(t, http.StatusUnprocessableEntity, viaToken.Status, "an owner's token cannot remove another owner: %s", viaToken.Raw)
	assert.Equal(t, "owner", roleOf(t, env, org.String(), coOwner.String()))

	demote := owner.ChangeRole(org, coOwner.String(), "admin", apptest.PasswordFactor(d.Password))
	require.Equal(t, http.StatusOK, demote.Status, string(demote.Raw))
	removed := owner.Do(apptest.Request{Method: http.MethodDelete, Path: path})
	assert.Equal(t, http.StatusOK, removed.Status, string(removed.Raw))
}
