package e2e_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

type activeMember struct {
	UserId       string
	LastActiveAt *time.Time
}

func lastActive(t *testing.T, c *apptest.Client, path string) (map[string]*time.Time, apptest.Response) {
	t.Helper()
	res := c.Get(path)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var members []activeMember
	res.Decode(t, &members)
	out := map[string]*time.Time{}
	for _, m := range members {
		out[m.UserId] = m.LastActiveAt
	}
	return out, res
}

func TestTheMemberListShowsWhenEachMemberWasLastActive(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	dev, devID := env.SignedIn(org, "dev@example.com", "developer")
	signedInAt := env.Clock.Now()
	issued := owner.MustInvite(org, "new@example.com", "viewer")
	require.Equal(t, http.StatusOK, env.AcceptNew(issued.Token, "New Person", newcomerPassword).Status)

	path := apptest.OrgPath(org, "/members")
	seen, _ := lastActive(t, owner, path)
	require.NotNil(t, seen[devID.String()])
	assert.True(t, seen[devID.String()].Equal(signedInAt))

	env.Clock.Advance(10 * time.Minute)
	usedAt := env.Clock.Now()
	require.Equal(t, http.StatusOK, dev.Get("/me").Status)
	require.Equal(t, http.StatusOK, dev.Do(apptest.Request{Method: http.MethodDelete, Path: "/sessions/current"}).Status)
	env.Clock.Advance(time.Hour)

	seen, res := lastActive(t, owner, path)
	require.NotNil(t, seen[devID.String()])
	assert.True(t, seen[devID.String()].Equal(usedAt), "a signed-out session still counts: %v", seen[devID.String()])
	assert.Len(t, seen, 3)
	for id, at := range seen {
		if id != devID.String() && id != owner.UserID {
			assert.Nil(t, at, "a member who never signed in has no last activity")
		}
	}
	assert.NotContains(t, string(res.Raw), `"LastActiveAt":null`, "absent, not null")
}

func TestOnlyTheMembersSessionsCountForLastActivity(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	_, devID := env.SignedIn(org, "dev@example.com", "developer")
	env.Clock.Advance(time.Hour)
	_, err := env.Pool.Exec(t.Context(), `UPDATE "Sessions" SET "IsDeleted" = true WHERE "UserId" = $1`, devID)
	require.NoError(t, err)

	seen, _ := lastActive(t, owner, apptest.OrgPath(org, "/members"))
	assert.Nil(t, seen[devID.String()], "a deleted session is not activity")
	require.NotNil(t, seen[owner.UserID])
}
