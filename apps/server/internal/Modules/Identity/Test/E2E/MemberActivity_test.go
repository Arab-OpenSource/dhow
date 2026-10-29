package e2e_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

type activeMember struct {
	UserId       string
	LastActiveAt *time.Time
}

func lastActive(t *testing.T, c *apptest.Client, org uuid.UUID) (map[string]*time.Time, apptest.Response) {
	t.Helper()
	res := c.Get(apptest.OrgPath(org, "/members"))
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var members []activeMember
	res.Decode(t, &members)
	out := map[string]*time.Time{}
	for _, m := range members {
		out[m.UserId] = m.LastActiveAt
	}
	return out, res
}

func addMember(t *testing.T, env *apptest.Env, org, user uuid.UUID, role string) {
	t.Helper()
	_, err := env.Pool.Exec(t.Context(), `INSERT INTO "Memberships" ("UserId", "OrganizationId", "Role") VALUES ($1, $2, $3)`, user, org, role)
	require.NoError(t, err)
}

func visit(t *testing.T, c *apptest.Client, org uuid.UUID) {
	t.Helper()
	res := c.Get(apptest.OrgPath(org, ""))
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
}

func at(t *testing.T, seen map[string]*time.Time, user uuid.UUID) time.Time {
	t.Helper()
	v := seen[user.String()]
	require.NotNil(t, v, "no last activity for %s", user)
	return *v
}

func TestLastActivityBelongsToOneOrganization(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	orgA := env.OrganizationID(d.OrgSlug)
	orgB := env.SeedOrganization("Other", "other")
	ownerB, _ := env.SignedIn(orgB, "owner-b@example.com", "owner")
	_, devID := env.SignedIn(orgA, "dev@example.com", "developer")
	addMember(t, env, orgB, devID, "developer")
	dev, login := env.Login("dev@example.com", "a long password for dev@example.com")
	require.Equal(t, http.StatusOK, login.Status)

	seen, res := lastActive(t, owner, orgA)
	assert.Nil(t, seen[devID.String()], "signing in is not activity in an organization")
	assert.NotContains(t, string(res.Raw), `"LastActiveAt":null`, "absent, not null")

	env.Clock.Advance(5 * time.Minute)
	inB := env.Clock.Now()
	visit(t, dev, orgB)
	seen, _ = lastActive(t, owner, orgA)
	assert.Nil(t, seen[devID.String()], "activity in another organization stays there")
	seenB, _ := lastActive(t, ownerB, orgB)
	assert.True(t, at(t, seenB, devID).Equal(inB))

	env.Clock.Advance(5 * time.Minute)
	inA := env.Clock.Now()
	visit(t, dev, orgA)
	seen, _ = lastActive(t, owner, orgA)
	assert.True(t, at(t, seen, devID).Equal(inA))
	seenB, _ = lastActive(t, ownerB, orgB)
	assert.True(t, at(t, seenB, devID).Equal(inB), "and the other way round")
}

func TestLastActivityIsWrittenAtMostOnceAMinute(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	dev, devID := env.SignedIn(org, "dev@example.com", "developer")

	first := env.Clock.Now()
	visit(t, dev, org)
	env.Clock.Advance(59 * time.Second)
	visit(t, dev, org)
	seen, _ := lastActive(t, owner, org)
	assert.True(t, at(t, seen, devID).Equal(first), "a second request within the minute writes nothing")

	env.Clock.Advance(time.Second)
	again := env.Clock.Now()
	visit(t, dev, org)
	seen, _ = lastActive(t, owner, org)
	assert.True(t, at(t, seen, devID).Equal(again), "a minute later it moves")
}

func TestOnlyManagersSeeLastActivity(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	admin, adminID := env.SignedIn(org, "admin@example.com", "admin")
	maintainer, _ := env.SignedIn(org, "maint@example.com", "maintainer")
	dev, _ := env.SignedIn(org, "dev@example.com", "developer")
	viewer, _ := env.SignedIn(org, "viewer@example.com", "viewer")
	visit(t, admin, org)

	for name, c := range map[string]*apptest.Client{"owner": owner, "admin": admin} {
		seen, _ := lastActive(t, c, org)
		assert.False(t, at(t, seen, adminID).IsZero(), name)
	}
	for name, c := range map[string]*apptest.Client{"maintainer": maintainer, "developer": dev} {
		_, res := lastActive(t, c, org)
		assert.NotContains(t, string(res.Raw), "LastActiveAt", "%s has member.read but not member.manage", name)
	}
	assert.Equal(t, http.StatusForbidden, viewer.Get(apptest.OrgPath(org, "/members")).Status, "a viewer cannot read the list at all")
}
