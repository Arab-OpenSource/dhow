package e2e_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

func TestResendMakesTheResenderTheInviter(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	admin, adminID := env.SignedIn(org, "admin@example.com", "admin")
	issued := admin.MustInvite(org, "new@example.com", "developer")
	gone := owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/members/"+adminID.String())})
	require.Equal(t, http.StatusOK, gone.Status, string(gone.Raw))
	assert.Equal(t, http.StatusGone, env.Preview(issued.Token).Status, "the inviter left, so the link stopped")

	resent := owner.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/invitations/"+issued.Invitation.Id+"/resend")})
	require.Equal(t, http.StatusOK, resent.Status, string(resent.Raw))
	var again apptest.Issued
	resent.Decode(t, &again)
	_, token, _ := strings.Cut(again.Link, "#")
	require.Equal(t, http.StatusOK, env.AcceptNew(token, "New", "a long password for new@example.com").Status)

	var inviter uuid.UUID
	require.NoError(t, env.Pool.QueryRow(t.Context(), `SELECT "InvitedByUserId" FROM "Invitations" WHERE "Id" = $1`, issued.Invitation.Id).Scan(&inviter))
	assert.Equal(t, owner.Me().User.Id, inviter.String())
}
