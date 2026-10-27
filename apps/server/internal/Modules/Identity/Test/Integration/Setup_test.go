package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

func count(t *testing.T, env *apptest.Env, table string) int {
	t.Helper()
	queries := map[string]string{
		"Users": `SELECT count(*) FROM "Users"`, "Organizations": `SELECT count(*) FROM "Organizations"`,
		"Memberships": `SELECT count(*) FROM "Memberships"`, "SetupTokens": `SELECT count(*) FROM "SetupTokens" WHERE "UsedAt" IS NOT NULL`,
	}
	var n int
	require.NoError(t, env.Pool.QueryRow(context.Background(), queries[table]).Scan(&n))
	return n
}

func TestConcurrentSetupsCreateExactlyOneOwner(t *testing.T) {
	env := apptest.New(t)
	token := env.SetupToken()
	const attempts = 12

	results := make(chan apptest.Response, attempts)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			d := apptest.DefaultSetup()
			d.Token = token
			d.Email = fmt.Sprintf("owner%d@example.com", i)
			d.OrgSlug = fmt.Sprintf("org-%d", i)
			results <- env.Setup(d)
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var ok, completed, other int
	for res := range results {
		switch {
		case res.Status == http.StatusOK:
			ok++
		case res.Status == http.StatusConflict && res.Error.Code == "SETUP_COMPLETED":
			completed++
		default:
			other++
			t.Logf("unexpected result: %d %s", res.Status, res.Raw)
		}
	}
	assert.Equal(t, 1, ok, "exactly one setup wins")
	assert.Equal(t, attempts-1, completed, "every other attempt is told setup is complete")
	assert.Zero(t, other)
	assert.Equal(t, 1, count(t, env, "Users"))
	assert.Equal(t, 1, count(t, env, "Organizations"))
	assert.Equal(t, 1, count(t, env, "Memberships"))
	assert.Equal(t, 1, count(t, env, "SetupTokens"))
}

func TestConcurrentSetupsWithTheSameSlugAndEmail(t *testing.T) {
	env := apptest.New(t)
	token := env.SetupToken()
	var wg sync.WaitGroup
	results := make(chan int, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := apptest.DefaultSetup()
			d.Token = token
			results <- env.Setup(d).Status
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	for status := range results {
		if status == http.StatusOK {
			wins++
		}
	}
	assert.Equal(t, 1, wins)
	assert.Equal(t, 1, count(t, env, "Users"))
}

func TestSetupTokenExpiresAfterOneHour(t *testing.T) {
	env := apptest.New(t)
	token := env.SetupToken()
	env.Clock.Advance(59*time.Minute + 59*time.Second)
	d := apptest.DefaultSetup()
	d.Token = token
	valid := env.Setup(d)
	assert.Equal(t, http.StatusOK, valid.Status, "still valid just before the hour")

	env = apptest.New(t)
	token = env.SetupToken()
	env.Clock.Advance(time.Hour + time.Second)
	d.Token = token
	late := env.Setup(d)
	assert.Equal(t, http.StatusForbidden, late.Status)
	assert.Equal(t, "SETUP_TOKEN_INVALID", late.Error.Code)
	assert.Zero(t, count(t, env, "Users"))
}

func TestSetupTokenIsStoredHashedAndNewBootsReplaceIt(t *testing.T) {
	env := apptest.New(t)
	ctx := context.Background()
	first := env.SetupToken()
	var stored int
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT count(*) FROM "SetupTokens" WHERE "TokenHash" = $1`, []byte(first)).Scan(&stored))
	assert.Zero(t, stored, "the raw token is never stored")
	var hashed int
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT count(*) FROM "SetupTokens" WHERE octet_length("TokenHash") = 32`).Scan(&hashed))
	assert.Equal(t, 1, hashed)

	second := env.SetupToken()
	assert.NotEqual(t, first, second)
	d := apptest.DefaultSetup()
	d.Token = first
	old := env.Setup(d)
	assert.Equal(t, http.StatusForbidden, old.Status, "a restart invalidates the previous link")
	assert.Equal(t, "SETUP_TOKEN_INVALID", old.Error.Code)
	d.Token = second
	assert.Equal(t, http.StatusOK, env.Setup(d).Status)
}

func TestSetupTokenLinkShape(t *testing.T) {
	env := apptest.New(t)
	link, created, err := env.Wired.Identity.EnsureSetupToken(context.Background())
	require.NoError(t, err)
	require.True(t, created)
	assert.Regexp(t, `^https://dhow\.example\.com/setup#[A-Za-z0-9_-]{43}$`, link, "the token travels in the fragment, so it never reaches a server or a log")
}

func TestBadSetupTokensAreRejectedBeforeAnyWork(t *testing.T) {
	env := apptest.New(t)
	env.SetupToken()
	tests := map[string]string{
		"empty":         "",
		"not base64":    "%%%",
		"wrong length":  "AAAA",
		"unknown token": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			d := apptest.DefaultSetup()
			d.Token = token
			if token == "" {
				d.Token = " "
			}
			res := env.Setup(d)
			assert.Equal(t, http.StatusForbidden, res.Status)
			assert.Equal(t, "SETUP_TOKEN_INVALID", res.Error.Code)
		})
	}
	assert.Zero(t, count(t, env, "Users"))
}

func TestSetupStoresAnArgon2idHash(t *testing.T) {
	env := apptest.New(t)
	d := apptest.DefaultSetup()
	require.Equal(t, http.StatusOK, env.Setup(d).Status)
	var hash, email string
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT "PasswordHash", "Email" FROM "Users"`).Scan(&hash, &email))
	assert.Regexp(t, `^\$argon2id\$v=19\$m=\d+,t=\d+,p=\d+\$[A-Za-z0-9+/]+\$[A-Za-z0-9+/]+$`, hash)
	assert.NotContains(t, hash, d.Password)
	assert.Equal(t, "owner@example.com", email)
	ok, _, err := env.Hasher.Verify(context.Background(), d.Password, hash)
	require.NoError(t, err)
	assert.True(t, ok)
	var role string
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT "Role" FROM "Memberships"`).Scan(&role))
	assert.Equal(t, "owner", role)
}

func TestSetupRejectsAWeakOrBreachedPasswordWithoutBurningTheToken(t *testing.T) {
	env := apptest.New(t)
	token := env.SetupToken()
	env.Breach.Add("this password is breached")
	d := apptest.DefaultSetup()
	d.Token = token

	d.Password = "short"
	weak := env.Setup(d)
	assert.Equal(t, http.StatusUnprocessableEntity, weak.Status)
	assert.Equal(t, "Password", weak.Error.Details[0].Field)
	d.Password = "this password is breached"
	breached := env.Setup(d)
	assert.Equal(t, http.StatusUnprocessableEntity, breached.Status)
	assert.Equal(t, "PASSWORD_BREACHED", breached.Error.Code)
	assert.Zero(t, count(t, env, "Users"))

	d.Password = "a long unique passphrase"
	assert.Equal(t, http.StatusOK, env.Setup(d).Status, "the token survives validation failures")
}

func TestSetupRollsBackEverythingOnConflict(t *testing.T) {
	env := apptest.New(t)
	ctx := context.Background()
	org := env.SeedOrganization("Taken", "taken")
	require.NotZero(t, org)
	d := apptest.DefaultSetup()
	d.OrgSlug = "taken"
	res := env.Setup(d)
	assert.Equal(t, http.StatusConflict, res.Status)
	assert.Equal(t, "CONFLICT", res.Error.Code)
	assert.Equal(t, "OrganizationSlug", res.Error.Details[0].Field)
	assert.Equal(t, 0, count(t, env, "Users"), "no user is left behind")
	assert.Equal(t, 1, count(t, env, "Organizations"))
	var done *time.Time
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT "SetupCompletedAt" FROM "Instance"`).Scan(&done))
	assert.Nil(t, done, "setup stays open after a failed attempt")

	d.OrgSlug = "fresh"
	assert.Equal(t, http.StatusOK, env.Setup(d).Status, "the same token still works")
}

func TestSetupEmitsAnAuditEvent(t *testing.T) {
	env := apptest.New(t)
	d := apptest.DefaultSetup()
	require.Equal(t, http.StatusOK, env.Setup(d).Status)
	var action, data string
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT "Action", "Data"::text FROM "AuditEvents"`).Scan(&action, &data))
	assert.Equal(t, "system.instance.setupCompleted", action)
	assert.JSONEq(t, `{"OrganizationSlug":"dhow-co"}`, data)
}
