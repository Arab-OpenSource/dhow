package e2e_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

type listedInvitation struct {
	Id, Email                        string
	Expired                          bool
	AcceptedAt                       *time.Time
	AcceptedByUserId, AcceptedByName *string
}

func invitations(t *testing.T, c *apptest.Client, path string) []listedInvitation {
	t.Helper()
	res := c.Get(path)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var out []listedInvitation
	res.Decode(t, &out)
	return out
}

func TestAcceptedInvitationsAreListedWithWhoAcceptedThem(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	otherOrg := env.SeedOrganization("Other", "other")
	bob, bobID := env.SignedIn(otherOrg, "bob@example.com", "owner")

	first := owner.MustInvite(org, "new@example.com", "viewer")
	require.Equal(t, http.StatusOK, env.AcceptNew(first.Token, "New Person", newcomerPassword).Status)
	firstAt := env.Clock.Now()
	env.Clock.Advance(time.Minute)
	second := owner.MustInvite(org, "bob@example.com", "developer")
	require.Equal(t, http.StatusOK, bob.AcceptSignedIn(second.Token).Status)
	open := owner.MustInvite(org, "open@example.com", "viewer")
	revoked := owner.MustInvite(org, "gone@example.com", "viewer")
	require.Equal(t, http.StatusOK, owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/invitations/"+revoked.Invitation.Id)}).Status)

	base := apptest.OrgPath(org, "/invitations")
	for _, path := range []string{base, base + "?Status=open"} {
		list := invitations(t, owner, path)
		require.Len(t, list, 1, path)
		assert.Equal(t, open.Invitation.Id, list[0].Id)
		assert.Nil(t, list[0].AcceptedAt, "an open invitation has no acceptance")
	}

	accepted := invitations(t, owner, base+"?Status=accepted")
	require.Len(t, accepted, 2)
	assert.Equal(t, second.Invitation.Id, accepted[0].Id, "newest acceptance first")
	assert.Equal(t, first.Invitation.Id, accepted[1].Id)
	require.NotNil(t, accepted[0].AcceptedByUserId)
	assert.Equal(t, bobID.String(), *accepted[0].AcceptedByUserId)
	require.NotNil(t, accepted[1].AcceptedByName)
	assert.Equal(t, "New Person", *accepted[1].AcceptedByName)
	require.NotNil(t, accepted[1].AcceptedAt)
	assert.True(t, accepted[1].AcceptedAt.Equal(firstAt), "%v", accepted[1].AcceptedAt)
	for _, i := range accepted {
		assert.False(t, i.Expired, "an accepted invitation is not offered for resending")
	}
	assert.Empty(t, invitations(t, bob, apptest.OrgPath(otherOrg, "/invitations")+"?Status=accepted"), "another organization's list stays apart")
}

func TestTheAcceptedListNeedsMemberManageAndAKnownStatus(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	maintainer, _ := env.SignedIn(org, "maint@example.com", "maintainer")
	path := apptest.OrgPath(org, "/invitations?Status=accepted")

	assert.Equal(t, http.StatusForbidden, maintainer.Get(path).Status)
	bad := owner.Get(apptest.OrgPath(org, "/invitations?Status=revoked"))
	assert.Equal(t, http.StatusUnprocessableEntity, bad.Status, string(bad.Raw))
	assert.Equal(t, "VALIDATION_FAILED", bad.Error.Code)
}
