package edge_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func TestConcurrentRegistrationHonoursMaxUses(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	token, _ := env.CreateRegistrationToken(owner, org, apptest.RegistrationTokenSpec{MaxUses: 3})

	const callers = 50
	requests := make([]*runnerv1.RegisterRequest, callers)
	for i := range requests {
		requests[i] = apptest.NewRunnerIdentity(t).RegisterRequest(token)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, denied, other := 0, 0, []error{}
	start := make(chan struct{})
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rc := env.RunnerClient(apptest.RunnerClientOptions{Remote: fmt.Sprintf("198.51.100.%d:4000", i+1)})
			<-start
			_, err := rc.Registration.Register(context.Background(), connect.NewRequest(requests[i]))
			mu.Lock()
			defer mu.Unlock()
			var ce *connect.Error
			switch {
			case err == nil:
				ok++
			case errors.As(err, &ce) && ce.Message() == "INVALID_REGISTRATION_TOKEN":
				denied++
			default:
				other = append(other, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	assert.Empty(t, other)
	assert.Equal(t, 3, ok)
	assert.Equal(t, callers-3, denied)

	var runners, uses int
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "Runners"`).Scan(&runners))
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT "Uses" FROM "RunnerRegistrationTokens"`).Scan(&uses))
	assert.Equal(t, 3, runners)
	assert.Equal(t, 3, uses)
}

func TestConcurrentAuthenticationLeavesOneActiveSession(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	token, _ := env.CreateRegistrationToken(owner, env.OrganizationID(d.OrgSlug), apptest.RegistrationTokenSpec{})
	id, reg := env.RegisterRunner(token)

	const callers = 3
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rc := env.RunnerClient(apptest.RunnerClientOptions{Remote: fmt.Sprintf("203.0.113.%d:4000", i+1)})
			_, errs[i] = env.Authenticate(rc, id, reg.GetRunnerId())
		}()
	}
	wg.Wait()
	for _, err := range errs {
		assert.NoError(t, err)
	}
	var active int
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM "RunnerSessions" WHERE "RunnerId" = $1 AND "RevokedAt" IS NULL`, reg.GetRunnerId()).Scan(&active))
	assert.Equal(t, 1, active)
}
