package edge_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	dbtest "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Testing"
)

type polled struct {
	grant   *apptest.Grant
	err     error
	elapsed time.Duration
}

func pollAsync(r *apptest.TestRunner, wait int32) <-chan polled {
	out := make(chan polled, 1)
	go func() {
		begin := time.Now()
		g, err := r.Lease(context.Background(), 1, wait)
		out <- polled{grant: g, err: err, elapsed: time.Since(begin)}
	}()
	return out
}

func refused(t *testing.T, r *apptest.TestRunner) *connect.Error {
	t.Helper()
	_, err := r.Lease(context.Background(), 1, 1)
	var ce *connect.Error
	require.True(t, errors.As(err, &ce), "want a refusal, got %v", err)
	return ce
}

func TestLeaseLongPollWakesWhenAJobIsQueued(t *testing.T) {
	w := newWorld(t)
	r := w.runner()
	result := pollAsync(r, 20)
	time.Sleep(300 * time.Millisecond)
	created := time.Now()
	id := w.job(t, apptest.SimpleSpec())
	got := <-result
	require.NoError(t, got.err)
	require.NotNil(t, got.grant)
	assert.Equal(t, id.String(), got.grant.Grant.GetJobId())
	assert.Less(t, time.Since(created), 1500*time.Millisecond, "LISTEN wakes the poll before the 2 second fallback")
}

func TestLeaseLongPollReturnsEmptyAfterTheWait(t *testing.T) {
	w := newWorld(t)
	got := <-pollAsync(w.runner(), 1)
	require.NoError(t, got.err)
	assert.Nil(t, got.grant)
	assert.GreaterOrEqual(t, got.elapsed, time.Second)
	assert.Less(t, got.elapsed, 3*time.Second)
}

func TestLeaseOnePollPerRunnerAndAnInstanceLimit(t *testing.T) {
	env := apptest.NewConfigured(t, dbtest.NewPool(t), apptest.TestParams(), func(c *config.Config) { c.RunnerMaxPolls = 1 })
	w := setup(t, env)
	first, second := w.runner(), w.runner()
	held := pollAsync(first, 20)
	time.Sleep(300 * time.Millisecond)

	same := refused(t, first)
	assert.Equal(t, connect.CodeResourceExhausted, same.Code())
	assert.Equal(t, "POLL_IN_PROGRESS", same.Message())
	busy := refused(t, second)
	assert.Equal(t, "TOO_MANY_POLLS", busy.Message())
	assert.Equal(t, "5", busy.Meta().Get("Retry-After"))
	g, err := second.Lease(context.Background(), 1, 0)
	require.NoError(t, err, "a poll that does not wait is never limited")
	assert.Nil(t, g)

	drained := time.Now()
	w.env.Wired.Jobs.Drain()
	got := <-held
	require.NoError(t, got.err)
	assert.Nil(t, got.grant)
	assert.Less(t, time.Since(drained), time.Second, "shutting down ends open polls at once")
}

func TestLeaseRequestValidation(t *testing.T) {
	w := newWorld(t)
	r := w.runner()
	for name, req := range map[string][2]int32{"negative slots": {-1, 0}, "negative wait": {1, -1}} {
		t.Run(name, func(t *testing.T) {
			_, err := r.Lease(context.Background(), req[0], req[1])
			var ce *connect.Error
			require.True(t, errors.As(err, &ce))
			assert.Equal(t, connect.CodeInvalidArgument, ce.Code())
		})
	}
}
