package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
)

type sessionRow struct {
	LastSeenAt    time.Time
	IdleExpiresAt time.Time
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	TokenHash     []byte
	CsrfHash      []byte
}

func loadSessionRow(t *testing.T, env *apptest.Env, cookie string) sessionRow {
	t.Helper()
	hash, ok := auth.HashSessionToken(cookie, config.SessionTokenBytes)
	require.True(t, ok)
	var r sessionRow
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT "LastSeenAt", "IdleExpiresAt", "ExpiresAt", "RevokedAt", "TokenHash", "CsrfHash" FROM "Sessions" WHERE "TokenHash" = $1`, hash).
		Scan(&r.LastSeenAt, &r.IdleExpiresAt, &r.ExpiresAt, &r.RevokedAt, &r.TokenHash, &r.CsrfHash))
	return r
}

func TestSessionIsStoredHashedWithTheDocumentedTimeouts(t *testing.T) {
	env := apptest.New(t)
	start := env.Clock.Now()
	client, _ := env.SetupAndLogin()
	row := loadSessionRow(t, env, client.Cookie)

	assert.Len(t, row.TokenHash, 32)
	assert.NotContains(t, string(row.TokenHash), client.Cookie)
	assert.Equal(t, auth.HashBytes([]byte(client.CSRFToken)), row.CsrfHash, "only a hash of the CSRF token is stored")
	assert.WithinDuration(t, start.Add(4*time.Hour), row.IdleExpiresAt, 0, "an owner's idle timeout is 4 hours")
	assert.WithinDuration(t, start.Add(7*24*time.Hour), row.ExpiresAt, 0, "absolute timeout is 7 days")
	assert.Nil(t, row.RevokedAt)

	var raw int
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "Sessions" WHERE "TokenHash" = $1`, []byte(client.Cookie)).Scan(&raw))
	assert.Zero(t, raw, "the raw cookie value is never stored")
}

func TestLastSeenIsUpdatedAtMostOncePerMinute(t *testing.T) {
	env := apptest.New(t)
	client, _ := env.SetupAndLogin()
	login := loadSessionRow(t, env, client.Cookie)

	env.Clock.Advance(30 * time.Second)
	require.Equal(t, http.StatusOK, client.Get("/me").Status)
	require.Equal(t, http.StatusOK, client.Get("/sessions").Status)
	assert.WithinDuration(t, login.LastSeenAt, loadSessionRow(t, env, client.Cookie).LastSeenAt, 0, "requests inside a minute write nothing")

	env.Clock.Advance(31 * time.Second)
	require.Equal(t, http.StatusOK, client.Get("/me").Status)
	touched := loadSessionRow(t, env, client.Cookie)
	assert.WithinDuration(t, env.Clock.Now(), touched.LastSeenAt, 0)
	assert.WithinDuration(t, env.Clock.Now().Add(4*time.Hour), touched.IdleExpiresAt, 0)

	env.Clock.Advance(20 * time.Second)
	require.Equal(t, http.StatusOK, client.Get("/me").Status)
	assert.WithinDuration(t, touched.LastSeenAt, loadSessionRow(t, env, client.Cookie).LastSeenAt, 0)
}

func TestRevocationIsImmediateAndPermanent(t *testing.T) {
	env := apptest.New(t)
	client, _ := env.SetupAndLogin()
	require.Equal(t, http.StatusOK, client.Get("/me").Status)

	_, err := env.Pool.Exec(context.Background(), `UPDATE "Sessions" SET "RevokedAt" = now()`)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, client.Get("/me").Status, "no cache keeps a revoked session alive")
	env.Clock.Advance(time.Second)
	assert.Equal(t, http.StatusUnauthorized, client.Get("/me").Status)
}

func TestDisablingTheUserEndsTheirSessions(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	_, err := env.Pool.Exec(context.Background(), `UPDATE "Users" SET "Disabled" = true`)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, client.Get("/me").Status)
	_, login := env.Login(d.Email, d.Password)
	assert.Equal(t, http.StatusUnauthorized, login.Status)
	assert.Equal(t, "INVALID_CREDENTIALS", login.Error.Code, "a disabled account looks like a wrong password")
}

func TestSoftDeletedUserCannotSignIn(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	_, err := env.Pool.Exec(context.Background(), `UPDATE "Users" SET "IsDeleted" = true`)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, client.Get("/me").Status)
	_, login := env.Login(d.Email, d.Password)
	assert.Equal(t, http.StatusUnauthorized, login.Status)
}

func TestLoginRotatesTheSessionAndRevokesTheOldOne(t *testing.T) {
	env := apptest.New(t)
	first, d := env.SetupAndLogin()
	before := loadSessionRow(t, env, first.Cookie)

	again := &apptest.Client{Env: env, Cookie: first.Cookie}
	res := again.Do(apptest.Request{Method: http.MethodPost, Path: "/sessions", Body: map[string]string{"Email": d.Email, "Password": d.Password}})
	require.Equal(t, http.StatusOK, res.Status)
	renewed := apptest.CookieValue(t, res.Header.Get("Set-Cookie"))

	old := loadSessionRow(t, env, first.Cookie)
	assert.NotNil(t, old.RevokedAt, "the presented session is revoked")
	assert.Equal(t, before.TokenHash, old.TokenHash)
	assert.Nil(t, loadSessionRow(t, env, renewed).RevokedAt)

	var active int
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "Sessions" WHERE "RevokedAt" IS NULL`).Scan(&active))
	assert.Equal(t, 1, active)
}

func TestSessionsSurviveAFailedLogin(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	res := client.Do(apptest.Request{Method: http.MethodPost, Path: "/sessions", Body: map[string]string{"Email": d.Email, "Password": "wrong wrong wrong"}})
	assert.Equal(t, http.StatusUnauthorized, res.Status)
	assert.Equal(t, http.StatusOK, client.Get("/me").Status, "a failed attempt must not sign the real user out")
}

func TestPasswordIsRehashedWhenTheParametersChange(t *testing.T) {
	env := apptest.New(t)
	d := apptest.DefaultSetup()
	require.Equal(t, http.StatusOK, env.Setup(d).Status)
	ctx := context.Background()
	weaker := apptest.TestParams()
	weaker.Iterations = 2
	old, err := auth.NewHasher(ctx, weaker, 1)
	require.NoError(t, err)
	hash, err := old.Hash(ctx, d.Password)
	require.NoError(t, err)
	_, err = env.Pool.Exec(ctx, `UPDATE "Users" SET "PasswordHash" = $1`, hash)
	require.NoError(t, err)

	_, login := env.Login(d.Email, d.Password)
	require.Equal(t, http.StatusOK, login.Status)
	var stored string
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT "PasswordHash" FROM "Users"`).Scan(&stored))
	assert.NotEqual(t, hash, stored)
	assert.Contains(t, stored, "t=1,", "the stored hash now uses the configured parameters")
	_, again := env.Login(d.Email, d.Password)
	assert.Equal(t, http.StatusOK, again.Status)
}
