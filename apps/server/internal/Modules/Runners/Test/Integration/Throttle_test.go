package integration_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	runnerauth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/RunnerAuth"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func registerFrom(env *apptest.Env, remote, token string) error {
	_, err := env.RunnerClient(apptest.RunnerClientOptions{Remote: remote}).Registration.Register(context.Background(),
		connect.NewRequest(apptest.NewRunnerIdentity(env.T).RegisterRequest(token)))
	return err
}

func TestRegistrationIsThrottledPerAddress(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	token, _ := env.CreateRegistrationToken(owner, env.OrganizationID(d.OrgSlug), apptest.RegistrationTokenSpec{MaxUses: 3})
	attacker := "192.0.2.66:1000"
	for i := range config.RunnerRegisterFreeAttempts {
		err := registerFrom(env, attacker, "dhow_rrt_guess")
		var ce *connect.Error
		require.ErrorAs(t, err, &ce, "attempt %d", i)
		assert.Equal(t, "INVALID_REGISTRATION_TOKEN", ce.Message())
		if i < config.RunnerRegisterFreeAttempts-1 {
			assert.Empty(t, ce.Meta().Get(runnerauth.RetryAfterHeader), "attempt %d", i)
		}
	}
	err := registerFrom(env, attacker, token)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, "INVALID_REGISTRATION_TOKEN", ce.Message(), "a locked address gets the same generic error, even with a good token")
	retry, convErr := strconv.Atoi(ce.Meta().Get(runnerauth.RetryAfterHeader))
	require.NoError(t, convErr)
	assert.Positive(t, retry)

	assert.NoError(t, registerFrom(env, "192.0.2.67:1000", token), "other addresses are not affected")
	env.Clock.Advance(config.ThrottleBaseLock + time.Second)
	assert.NoError(t, registerFrom(env, attacker, token), "the lock expires")
}

func TestSuccessfulRegistrationsDoNotAccumulate(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	token, _ := env.CreateRegistrationToken(owner, env.OrganizationID(d.OrgSlug), apptest.RegistrationTokenSpec{MaxUses: 50})
	for i := range 30 {
		require.NoError(t, registerFrom(env, "192.0.2.10:1000", token), "registration %d from a fleet behind one address", i)
	}
}

func TestAuthenticationIsThrottledPerAddress(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	token, _ := env.CreateRegistrationToken(owner, env.OrganizationID(d.OrgSlug), apptest.RegistrationTokenSpec{})
	id, reg := env.RegisterRunner(token)
	attacker := env.RunnerClient(apptest.RunnerClientOptions{Remote: "192.0.2.77:1000"})
	ref := &runnerv1.RunnerRef{RunnerId: reg.GetRunnerId()}
	for range config.RunnerAuthFreeAttempts {
		_, err := attacker.Auth.Authenticate(context.Background(), connect.NewRequest(&runnerv1.AuthenticateRequest{Runner: ref}))
		require.Error(t, err)
	}
	_, err := env.Authenticate(attacker, id, reg.GetRunnerId())
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, "INVALID_RUNNER_CREDENTIALS", ce.Message())
	assert.NotEmpty(t, ce.Meta().Get(runnerauth.RetryAfterHeader))
	_, err = env.Authenticate(env.RunnerClient(apptest.RunnerClientOptions{Remote: "192.0.2.78:1000"}), id, reg.GetRunnerId())
	assert.NoError(t, err, "the runner itself is not locked out from its own address")
}
