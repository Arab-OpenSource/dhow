package integration_test

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func owners(t *testing.T, env *apptest.Env, org string) int {
	t.Helper()
	var n int
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM "Memberships" WHERE "OrganizationId" = $1 AND "Role" = 'owner' AND NOT "IsDeleted"`, org).Scan(&n))
	return n
}

func race(n int, fn func(i int) int) []int {
	out := make([]int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			out[i] = fn(i)
		}()
	}
	close(start)
	wg.Wait()
	return out
}

func TestTwoOwnersLeavingAtOnceLeaveOneBehind(t *testing.T) {
	env := apptest.New(t)
	first, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	second, _ := env.SignedIn(org, "second@example.com", "owner")
	clients := []*apptest.Client{first, second}

	statuses := race(2, func(i int) int {
		return clients[i].Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/leave")}).Status
	})
	assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, statuses)
	assert.Equal(t, 1, owners(t, env, org.String()))
}

func TestTwoOwnersDemotingEachOtherLeaveOneBehind(t *testing.T) {
	env := apptest.New(t)
	first, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	second, secondID := env.SignedIn(org, "second@example.com", "owner")
	targets := []string{secondID.String(), first.Me().User.Id}
	clients := []*apptest.Client{first, second}
	factors := []map[string]any{apptest.PasswordFactor(d.Password), apptest.PasswordFactor("a long password for second@example.com")}

	statuses := race(2, func(i int) int {
		return clients[i].ChangeRole(org, targets[i], "admin", factors[i]).Status
	})
	ok := 0
	for _, s := range statuses {
		if s == http.StatusOK {
			ok++
		}
	}
	assert.Equal(t, 1, ok, "%v", statuses)
	assert.Equal(t, 1, owners(t, env, org.String()))
}
