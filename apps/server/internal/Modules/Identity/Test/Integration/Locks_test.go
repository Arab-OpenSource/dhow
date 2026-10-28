package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

func waitForLockWaiter(t *testing.T, env *apptest.Env) bool {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		require.NoError(t, env.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&n))
		if n > 0 {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func demoteWhileLocked(t *testing.T, env *apptest.Env, org, user uuid.UUID, role string, request func() int) (int, bool) {
	t.Helper()
	ctx := context.Background()
	tx, err := env.Pool.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT "Id" FROM "Organizations" WHERE "Id" = $1 FOR UPDATE`, org)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE "Memberships" SET "Role" = $3 WHERE "OrganizationId" = $1 AND "UserId" = $2`, org, user, role)
	require.NoError(t, err)
	done := make(chan int, 1)
	go func() { done <- request() }()
	waited := waitForLockWaiter(t, env)
	require.NoError(t, tx.Commit(ctx))
	select {
	case status := <-done:
		return status, waited
	case <-time.After(10 * time.Second):
		t.Fatal("the request never finished")
		return 0, waited
	}
}

func TestAcceptSeesADemotionOfTheInviterThatCommitsWhileItWaits(t *testing.T) {
	env := apptest.New(t)
	_, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	admin, adminID := env.SignedIn(org, "admin@example.com", "admin")
	issued := admin.MustInvite(org, "new@example.com", "maintainer")

	status, waited := demoteWhileLocked(t, env, org, adminID, "developer", func() int {
		return env.AcceptNew(issued.Token, "New", "a long password for new@example.com").Status
	})
	assert.True(t, waited, "accept waits for the organization lock")
	assert.Equal(t, http.StatusGone, status, "the inviter can no longer grant maintainer when the accept commits")

	var members int
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM "Memberships" m JOIN "Users" u ON u."Id" = m."UserId" WHERE u."Email" = 'new@example.com'`).Scan(&members))
	assert.Zero(t, members)
}
