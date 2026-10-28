package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

const (
	memberIdle     = 12 * time.Hour
	privilegedIdle = 4 * time.Hour
	absolute       = 7 * 24 * time.Hour
)

func signedInAs(t *testing.T, env *apptest.Env, role string) (*apptest.Client, uuid.UUID, uuid.UUID) {
	t.Helper()
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	if role == "owner" {
		return owner, org, uuid.MustParse(owner.UserID)
	}
	c, id := env.SignedIn(org, role+"@example.com", role)
	return c, org, id
}

func setRole(t *testing.T, env *apptest.Env, org, user uuid.UUID, role string) {
	t.Helper()
	_, err := env.Pool.Exec(context.Background(),
		`UPDATE "Memberships" SET "Role" = $3 WHERE "OrganizationId" = $1 AND "UserId" = $2`, org, user, role)
	require.NoError(t, err)
}

func TestIdleTimeoutDependsOnTheRole(t *testing.T) {
	tests := []struct {
		role string
		idle time.Duration
	}{
		{"viewer", memberIdle},
		{"developer", memberIdle},
		{"maintainer", memberIdle},
		{"admin", privilegedIdle},
		{"owner", privilegedIdle},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			env := apptest.New(t)
			client, _, _ := signedInAs(t, env, tt.role)

			env.Clock.Advance(tt.idle - time.Minute)
			assert.Equal(t, http.StatusOK, client.Get("/me").Status, "still inside the idle window")

			env.Clock.Advance(tt.idle - time.Minute)
			assert.Equal(t, http.StatusOK, client.Get("/me").Status, "activity extends the idle window")

			env.Clock.Advance(tt.idle)
			res := client.Get("/me")
			assert.Equal(t, http.StatusUnauthorized, res.Status, "%s without activity ends the session", tt.idle)
			assert.Equal(t, "UNAUTHENTICATED", res.Error.Code)
		})
	}
}

func TestAnAdminRoleInAnyOrganizationShortensTheIdleWindow(t *testing.T) {
	env := apptest.New(t)
	client, _, user := signedInAs(t, env, "developer")
	other := env.SeedOrganization("Second", "second")
	_, err := env.Pool.Exec(context.Background(),
		`INSERT INTO "Memberships" ("UserId", "OrganizationId", "Role") VALUES ($1, $2, 'admin')`, user, other)
	require.NoError(t, err)

	env.Clock.Advance(privilegedIdle + time.Second)
	assert.Equal(t, http.StatusUnauthorized, client.Get("/me").Status)
}

func TestAPromotionShortensTheIdleWindowAtOnce(t *testing.T) {
	env := apptest.New(t)
	client, org, user := signedInAs(t, env, "developer")

	env.Clock.Advance(3 * time.Hour)
	require.Equal(t, http.StatusOK, client.Get("/me").Status)
	stored := loadSessionRow(t, env, client.Cookie).IdleExpiresAt
	assert.WithinDuration(t, env.Clock.Now().Add(memberIdle), stored, 0)

	setRole(t, env, org, user, "admin")
	env.Clock.Advance(privilegedIdle + time.Second)
	assert.True(t, env.Clock.Now().Before(stored), "the stored idle expiry alone would still allow the session")
	assert.Equal(t, http.StatusUnauthorized, client.Get("/me").Status, "the promotion applies the 4 hour window to the running session")
}

func TestAPromotedSessionTouchStoresTheShortWindow(t *testing.T) {
	env := apptest.New(t)
	client, org, user := signedInAs(t, env, "developer")
	setRole(t, env, org, user, "admin")

	env.Clock.Advance(time.Hour)
	require.Equal(t, http.StatusOK, client.Get("/me").Status)
	assert.WithinDuration(t, env.Clock.Now().Add(privilegedIdle), loadSessionRow(t, env, client.Cookie).IdleExpiresAt, 0)
}

func TestADemotionRestoresTheLongIdleWindow(t *testing.T) {
	env := apptest.New(t)
	client, org, user := signedInAs(t, env, "admin")
	setRole(t, env, org, user, "developer")

	env.Clock.Advance(time.Hour)
	require.Equal(t, http.StatusOK, client.Get("/me").Status)
	assert.WithinDuration(t, env.Clock.Now().Add(memberIdle), loadSessionRow(t, env, client.Cookie).IdleExpiresAt, 0)

	env.Clock.Advance(privilegedIdle + time.Hour)
	assert.Equal(t, http.StatusOK, client.Get("/me").Status, "the next touch after a demotion stores the 12 hour window")
}

func TestAnActiveSessionEndsAfterSevenDays(t *testing.T) {
	env := apptest.New(t)
	client, _, _ := signedInAs(t, env, "developer")
	step := 6 * time.Hour
	for elapsed := step; elapsed <= absolute+step; elapsed += step {
		env.Clock.Advance(step)
		res := client.Get("/me")
		if elapsed < absolute {
			require.Equal(t, http.StatusOK, res.Status, "active for %s", elapsed)
			continue
		}
		require.Equal(t, http.StatusUnauthorized, res.Status, "an active session still ends after 7 days (%s)", elapsed)
		return
	}
	t.Fatal("the session never reached its absolute timeout")
}

func TestASessionFromTheFourteenDayPolicyEndsAfterSevenDays(t *testing.T) {
	env := apptest.New(t)
	client, _, _ := signedInAs(t, env, "developer")
	_, err := env.Pool.Exec(context.Background(), `UPDATE "Sessions" SET "ExpiresAt" = "CreatedAt" + interval '14 days'`)
	require.NoError(t, err)
	created := env.Clock.Now()

	var list []struct{ ExpiresAt time.Time }
	res := client.Get("/sessions")
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	res.Decode(t, &list)
	require.Len(t, list, 1)
	assert.WithinDuration(t, created.Add(absolute), list[0].ExpiresAt, 0, "the reported expiry follows the 7 day limit")

	for range 27 {
		env.Clock.Advance(6 * time.Hour)
		require.Equal(t, http.StatusOK, client.Get("/me").Status)
	}
	env.Clock.Advance(6*time.Hour + time.Second)
	assert.Equal(t, http.StatusUnauthorized, client.Get("/me").Status)
}

func TestIdleWindowNeverOutlivesTheAbsoluteTimeout(t *testing.T) {
	env := apptest.New(t)
	client, _, _ := signedInAs(t, env, "developer")
	for range 15 {
		env.Clock.Advance(11 * time.Hour)
		require.Equal(t, http.StatusOK, client.Get("/me").Status)
	}
	row := loadSessionRow(t, env, client.Cookie)
	assert.False(t, row.IdleExpiresAt.After(row.ExpiresAt), "idle expiry is capped by the absolute expiry")
}

func TestTheSessionListLeavesOutSessionsPastTheCurrentIdleWindow(t *testing.T) {
	env := apptest.New(t)
	idle, org, user := signedInAs(t, env, "developer")
	active, _ := env.Login("developer@example.com", "a long password for developer@example.com")
	require.Equal(t, http.StatusOK, idle.Get("/me").Status)

	setRole(t, env, org, user, "admin")
	env.Clock.Advance(privilegedIdle - time.Minute)
	require.Equal(t, http.StatusOK, active.Get("/me").Status)
	env.Clock.Advance(2 * time.Minute)

	var list []struct{ Current bool }
	res := active.Get("/sessions")
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	res.Decode(t, &list)
	require.Len(t, list, 1, "the session idle for more than 4 hours is not listed as active")
	assert.True(t, list[0].Current)
}
