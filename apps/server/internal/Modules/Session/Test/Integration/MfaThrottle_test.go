package integration_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func totpUser(t *testing.T) (*apptest.Env, apptest.SetupDetails, apptest.TotpEnrollment) {
	t.Helper()
	env, d := setupEnv(t)
	c, login := env.Login(d.Email, d.Password)
	require.Equal(t, http.StatusOK, login.Status)
	enrolled := c.EnableTotp(apptest.PasswordFactor(d.Password))
	env.Clock.Advance(31 * time.Second)
	return env, d, enrolled
}

func TestFortyParallelSecondStepGuessesGetFiveTries(t *testing.T) {
	env, d, enrolled := totpUser(t)
	cookies := make([]string, 8)
	for i := range cookies {
		cookies[i] = env.StartMfa(d.Email, d.Password)
	}

	const guesses = 40
	results := make([]apptest.Response, guesses)
	var wg sync.WaitGroup
	for i := range guesses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code := fmt.Sprintf("%06d", (i*7919)%1000000)
			if code == env.TotpCode(enrolled.Secret, 0) {
				code = "999999"
			}
			_, results[i] = env.CompleteMfa(cookies[i%len(cookies)], map[string]string{"Method": "totp", "Code": code}, nil)
		}()
	}
	wg.Wait()

	evaluated, locked := 0, 0
	for _, res := range results {
		require.Equal(t, http.StatusUnauthorized, res.Status, string(res.Raw))
		require.Equal(t, "INVALID_MFA_CODE", res.Error.Code, "every refusal looks the same")
		if retryAfter(t, res) > 0 {
			locked++
		} else {
			evaluated++
		}
	}
	assert.Equal(t, 5, evaluated, "exactly the free attempts are checked, however many arrive at once")
	assert.Equal(t, guesses-5, locked)
	assert.Equal(t, 5, env.AuditCount("auth.mfa_failed"), "refused attempts during a lock are not written one by one")

	_, blocked := env.CompleteMfa(cookies[7], map[string]string{"Method": "totp", "Code": env.TotpCode(enrolled.Secret, 0)}, nil)
	assert.Equal(t, "INVALID_MFA_CODE", blocked.Error.Code, "even the right code waits for the lock")
	assert.Positive(t, retryAfter(t, blocked))

	env.Clock.Advance(31 * time.Second)
	c, ok := env.CompleteMfa(cookies[7], map[string]string{"Method": "totp", "Code": env.TotpCode(enrolled.Secret, 0)}, nil)
	require.Equal(t, http.StatusOK, ok.Status, string(ok.Raw))
	assert.Equal(t, http.StatusOK, c.Get("/me").Status)
}

func TestACorrectPasswordDoesNotResetTheSecondStepCounter(t *testing.T) {
	env, d, enrolled := totpUser(t)
	for i := range 5 {
		cookie := env.StartMfa(d.Email, d.Password)
		_, res := env.CompleteMfa(cookie, map[string]string{"Method": "totp", "Code": "000000"}, nil)
		assert.Zero(t, retryAfter(t, res), "attempt %d", i)
	}
	cookie := env.StartMfa(d.Email, d.Password)
	_, res := env.CompleteMfa(cookie, map[string]string{"Method": "totp", "Code": env.TotpCode(enrolled.Secret, 0)}, nil)
	assert.Equal(t, 30, retryAfter(t, res), "passing the password again buys no new guesses")
}

func TestOneChallengeAllowsFiveWrongCodes(t *testing.T) {
	env, d, enrolled := totpUser(t)
	cookie := env.StartMfa(d.Email, d.Password)
	for range 5 {
		_, res := env.CompleteMfa(cookie, map[string]string{"Method": "totp", "Code": "000000"}, nil)
		require.Equal(t, "INVALID_MFA_CODE", res.Error.Code)
	}
	env.Clock.Advance(time.Minute)
	_, res := env.CompleteMfa(cookie, map[string]string{"Method": "totp", "Code": env.TotpCode(enrolled.Secret, 0)}, nil)
	assert.Equal(t, "MFA_CHALLENGE_EXPIRED", res.Error.Code, "the challenge is burned after five wrong codes")
}

func TestCurrentFactorGuessesAreThrottledToo(t *testing.T) {
	env, d, enrolled := totpUser(t)
	cookie := env.StartMfa(d.Email, d.Password)
	c, res := env.CompleteMfa(cookie, map[string]string{"Method": "totp", "Code": env.TotpCode(enrolled.Secret, 0)}, nil)
	require.Equal(t, http.StatusOK, res.Status)
	env.Clock.Advance(31 * time.Second)
	disable := func(code string) apptest.Response {
		return c.Do(apptest.Request{Method: http.MethodPost, Path: "/me/totp/disable", Body: map[string]any{"CurrentFactor": apptest.CodeFactor("totp", code)}})
	}
	for i := range 5 {
		res := disable("000000")
		assert.Zero(t, retryAfter(t, res), "attempt %d", i)
		assert.Equal(t, "INVALID_CURRENT_FACTOR", res.Error.Code)
	}
	locked := disable(env.TotpCode(enrolled.Secret, 0))
	assert.Equal(t, "INVALID_CURRENT_FACTOR", locked.Error.Code)
	assert.Equal(t, 30, retryAfter(t, locked), "a stolen session cannot brute-force the current factor")
}
