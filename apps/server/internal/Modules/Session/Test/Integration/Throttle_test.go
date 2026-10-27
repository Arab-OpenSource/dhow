package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

func attempt(env *apptest.Env, email, password, remote string) apptest.Response {
	env.T.Helper()
	return env.Do(apptest.Request{
		Method: http.MethodPost, Path: "/sessions", RemoteAddr: remote,
		Headers: map[string]string{"Origin": apptest.PublicOrigin},
		Body:    map[string]string{"Email": email, "Password": password},
	})
}

func retryAfter(t *testing.T, res apptest.Response) int {
	t.Helper()
	v := res.Header.Get("Retry-After")
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	require.NoError(t, err)
	return n
}

func setupEnv(t *testing.T) (*apptest.Env, apptest.SetupDetails) {
	t.Helper()
	env := apptest.New(t)
	d := apptest.DefaultSetup()
	require.Equal(t, http.StatusOK, env.Setup(d).Status)
	return env, d
}

func throttleRow(t *testing.T, env *apptest.Env, key string) (failures int, lockedUntil *time.Time) {
	t.Helper()
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT "Failures", "LockedUntil" FROM "LoginThrottles" WHERE "Key" = $1`, key).Scan(&failures, &lockedUntil))
	return failures, lockedUntil
}

func TestAccountLockoutWithExponentialBackoff(t *testing.T) {
	env, d := setupEnv(t)
	const ip = "203.0.113.10:5000"

	for i := 1; i <= 5; i++ {
		res := attempt(env, d.Email, "wrong password "+strconv.Itoa(i), ip)
		assert.Equal(t, http.StatusUnauthorized, res.Status)
		assert.Equal(t, "INVALID_CREDENTIALS", res.Error.Code)
		assert.Zero(t, retryAfter(t, res), "attempt %d", i)
	}

	locked := attempt(env, d.Email, d.Password, ip)
	assert.Equal(t, http.StatusUnauthorized, locked.Status, "even the right password is refused while locked")
	assert.Equal(t, "INVALID_CREDENTIALS", locked.Error.Code, "a lock uses the same generic error")
	assert.Equal(t, 30, retryAfter(t, locked))
	assert.Empty(t, locked.Header.Get("Set-Cookie"))

	env.Clock.Advance(20 * time.Second)
	still := attempt(env, d.Email, d.Password, ip)
	assert.Equal(t, 10, retryAfter(t, still), "the wait shrinks as time passes and attempts do not extend it")

	otherIP := attempt(env, d.Email, d.Password, "198.51.100.20:5000")
	assert.Equal(t, 10, retryAfter(t, otherIP), "the account lock follows the account, not the address")

	env.Clock.Advance(11 * time.Second)
	next := attempt(env, d.Email, "still wrong", ip)
	assert.Zero(t, retryAfter(t, next))
	assert.Equal(t, "INVALID_CREDENTIALS", next.Error.Code)
	failures, _ := throttleRow(t, env, onlyAccountKey(t, env))
	assert.Equal(t, 6, failures)

	doubled := attempt(env, d.Email, d.Password, ip)
	assert.Equal(t, 60, retryAfter(t, doubled), "the second lock lasts twice as long")

	env.Clock.Advance(61 * time.Second)
	ok := attempt(env, d.Email, d.Password, ip)
	assert.Equal(t, http.StatusOK, ok.Status, string(ok.Raw))
	failures, locked2 := throttleRow(t, env, onlyAccountKey(t, env))
	assert.Zero(t, failures, "a successful sign-in resets the account counter")
	assert.Nil(t, locked2)
}

func onlyAccountKey(t *testing.T, env *apptest.Env) string {
	t.Helper()
	var key string
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT "Key" FROM "LoginThrottles" WHERE "Key" LIKE 'account:%' ORDER BY "CreatedAt" LIMIT 1`).Scan(&key))
	return key
}

func TestUnknownAccountsAreThrottledTheSameWay(t *testing.T) {
	env, _ := setupEnv(t)
	const ip = "203.0.113.11:5000"
	for i := 1; i <= 5; i++ {
		res := attempt(env, "ghost@example.com", "wrong password", ip)
		assert.Zero(t, retryAfter(t, res), "attempt %d", i)
	}
	locked := attempt(env, "ghost@example.com", "wrong password", ip)
	assert.Equal(t, 30, retryAfter(t, locked), "an account that does not exist locks exactly like one that does")
	assert.Equal(t, "INVALID_CREDENTIALS", locked.Error.Code)
}

func TestAddressLockoutAcrossAccounts(t *testing.T) {
	env, d := setupEnv(t)
	const attacker = "203.0.113.12:5000"
	for i := range 20 {
		res := attempt(env, fmt.Sprintf("user%d@example.com", i), "wrong password", attacker)
		assert.Zero(t, retryAfter(t, res), "attempt %d", i)
	}
	locked := attempt(env, d.Email, d.Password, attacker)
	assert.Equal(t, http.StatusUnauthorized, locked.Status)
	assert.Equal(t, 30, retryAfter(t, locked), "20 failures from one address lock the address for every account")
	assert.Equal(t, "INVALID_CREDENTIALS", locked.Error.Code)

	elsewhere := attempt(env, d.Email, d.Password, "198.51.100.30:5000")
	assert.Equal(t, http.StatusOK, elsewhere.Status, "other addresses are not affected")
}

func TestLockedAttemptsAreNotCounted(t *testing.T) {
	env, d := setupEnv(t)
	const ip = "203.0.113.13:5000"
	for range 5 {
		attempt(env, d.Email, "wrong password", ip)
	}
	key := onlyAccountKey(t, env)
	failures, until := throttleRow(t, env, key)
	require.NotNil(t, until)
	assert.Equal(t, 5, failures)

	for range 10 {
		res := attempt(env, d.Email, "wrong password", ip)
		assert.Equal(t, 30, retryAfter(t, res))
	}
	after, untilAfter := throttleRow(t, env, key)
	assert.Equal(t, failures, after, "attempts during a lock do not grow the counter")
	assert.True(t, until.Equal(*untilAfter), "and do not extend the lock, so nobody can keep an account locked forever")
}

func TestFailuresAgeOut(t *testing.T) {
	env, d := setupEnv(t)
	const ip = "203.0.113.14:5000"
	for range 4 {
		attempt(env, d.Email, "wrong password", ip)
	}
	env.Clock.Advance(61 * time.Minute)
	res := attempt(env, d.Email, "wrong password", ip)
	assert.Zero(t, retryAfter(t, res))
	failures, until := throttleRow(t, env, onlyAccountKey(t, env))
	assert.Equal(t, 1, failures, "an hour without failures starts the count again")
	assert.Nil(t, until)
}

func TestSuccessResetsTheAccountCounter(t *testing.T) {
	env, d := setupEnv(t)
	const ip = "203.0.113.15:5000"
	for range 4 {
		attempt(env, d.Email, "wrong password", ip)
	}
	require.Equal(t, http.StatusOK, attempt(env, d.Email, d.Password, ip).Status)
	for range 4 {
		res := attempt(env, d.Email, "wrong password", ip)
		assert.Zero(t, retryAfter(t, res))
	}
	assert.Equal(t, http.StatusOK, attempt(env, d.Email, d.Password, ip).Status, "four failures after a success do not lock")
}

func TestLockoutHoldsUnderConcurrentAttacks(t *testing.T) {
	env, d := setupEnv(t)
	var wg sync.WaitGroup
	for i := range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			attempt(env, d.Email, "wrong password", fmt.Sprintf("192.0.2.%d:4000", i+1))
		}()
	}
	wg.Wait()
	_, until := throttleRow(t, env, onlyAccountKey(t, env))
	require.NotNil(t, until)
	res := attempt(env, d.Email, d.Password, "198.51.100.77:1")
	assert.Equal(t, http.StatusUnauthorized, res.Status)
	assert.Positive(t, retryAfter(t, res))
}

func TestPasswordChangeCannotBeUsedToGuessThePassword(t *testing.T) {
	env, d := setupEnv(t)
	client, res := env.Login(d.Email, d.Password)
	require.Equal(t, http.StatusOK, res.Status)
	change := func(current string) apptest.Response {
		return client.Do(apptest.Request{Method: http.MethodPut, Path: "/me/password", Body: map[string]string{
			"CurrentPassword": current, "NewPassword": "a brand new passphrase",
		}})
	}
	for i := range 5 {
		r := change(fmt.Sprintf("guess number %d", i))
		assert.Equal(t, "INVALID_CURRENT_PASSWORD", r.Error.Code)
	}
	locked := change(d.Password)
	assert.Equal(t, http.StatusForbidden, locked.Status)
	assert.Equal(t, "INVALID_CURRENT_PASSWORD", locked.Error.Code)
	assert.Equal(t, 30, retryAfter(t, locked), "the correct password is refused during the lock")

	env.Clock.Advance(31 * time.Second)
	assert.Equal(t, http.StatusOK, change(d.Password).Status)
}
