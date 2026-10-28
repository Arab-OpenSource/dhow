//go:build chaos

package chaos_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	firstSubID = 200000
	lastSubID  = 1199999
)

func tickScript(job string, ticks int) string {
	url := fmt.Sprintf("http://%s:%s/?job=%s&attempt=$DHOW_ATTEMPT_ID&token=$DHOW_FENCING_TOKEN", sinkHost, shared.sink.port, job)
	return fmt.Sprintf("i=0\nwhile [ $i -lt %d ]; do\n  busybox wget -q -O /dev/null \"%s\" || true\n  busybox sleep 1\n  i=$((i+1))\ndone\n", ticks, url)
}

func newJob(t *testing.T, name string, steps ...jobStep) string {
	t.Helper()
	ensureUp(t)
	return shared.api.createJob(t, name, jobSpec{
		Image: shared.image, Steps: steps, TimeoutSeconds: 900,
		Network: &jobNetwork{Egress: []string{sinkHost + ":" + shared.sink.port}},
	})
}

func uniqueName(base string) string { return base + "-" + randomHex(4) }

func ensureUp(t *testing.T) {
	t.Helper()
	s := shared.stack
	s.runnerProxy.Blackhole(false)
	shared.slowPull.Throttle(0)
	s.mu.Lock()
	server, runner := s.server, s.runner
	s.mu.Unlock()
	if server == nil {
		_, err := s.startServer()
		require.NoError(t, err)
	}
	if runner == nil {
		require.NoError(t, s.startRunner())
	}
}

func waitTicks(t *testing.T, job string, n int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if len(shared.sink.of(job)) >= n {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("job %s made %d of %d sink calls within %s", job, len(shared.sink.of(job)), n, within)
}

func waitNoSandboxes(t *testing.T, within time.Duration) time.Duration {
	t.Helper()
	start := time.Now()
	for time.Since(start) < within {
		if len(shared.stack.sandboxes()) == 0 {
			return time.Since(start)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("sandboxes still exist after %s: %v", within, shared.stack.sandboxes())
	return within
}

func jobProcesses() []string {
	entries, _ := os.ReadDir("/proc")
	var out []string
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", e.Name(), "status"))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "Uid:" {
				if uid, err := strconv.Atoi(fields[1]); err == nil && uid >= firstSubID && uid <= lastSubID {
					out = append(out, e.Name())
				}
			}
		}
	}
	return out
}

func TestAJobThatIgnoresTermIsKilledAfterTheGrace(t *testing.T) {
	name := uniqueName("ignore-term")
	id := newJob(t, name, jobStep{Name: "stubborn", Run: tickScript(name, 1) + "trap '' TERM\nwhile :; do busybox sleep 1; done\n"})
	waitTicks(t, name, 1, 3*time.Minute)
	asked := time.Now()
	shared.api.cancel(t, id)
	d := shared.api.waitJob(t, id, time.Minute, finished)
	assert.Equal(t, "canceled", d.Job.State)
	assert.GreaterOrEqual(t, time.Since(asked), 2*time.Second, "the step had its two-second grace before SIGKILL")
	waitNoSandboxes(t, 30*time.Second)
}

func TestAForkThatLeftTheGroupDiesWithTheSandbox(t *testing.T) {
	name := uniqueName("escape-group")
	escape := "busybox setsid busybox sh -c \"trap '' TERM; while :; do busybox sleep 1; done\" &\n"
	id := newJob(t, name, jobStep{Name: "escape", Run: tickScript(name, 1) + escape + "trap '' TERM\nwhile :; do busybox sleep 1; done\n"})
	waitTicks(t, name, 1, 3*time.Minute)
	shared.api.cancel(t, id)
	d := shared.api.waitJob(t, id, time.Minute, finished)
	assert.Equal(t, "canceled", d.Job.State)
	waitNoSandboxes(t, 30*time.Second)
	assert.Empty(t, jobProcesses(), "no process of a job user namespace is left on the host")
}

func TestKillingTheRunnerMidStepRetriesTheJobOnce(t *testing.T) {
	name := uniqueName("runner-kill")
	id := newJob(t, name, jobStep{Name: "ticks", Run: tickScript(name, 20)})
	waitTicks(t, name, 2, 3*time.Minute)
	shared.stack.killRunner()
	gone := waitNoSandboxes(t, 3*time.Second)
	t.Logf("the sandbox went away %s after the runner died", gone)
	require.NoError(t, shared.stack.startRunner())
	d := shared.api.waitJob(t, id, 4*time.Minute, finished)
	assert.Equal(t, "succeeded", d.Job.State)
	assert.Equal(t, 2, d.Job.AttemptCount)
	assert.Len(t, shared.sink.attempts(name), 2)
	assert.Empty(t, shared.sink.overlaps(name))
}

func TestKillingTheRunnerAfterAMutatingStepBeganLosesTheJob(t *testing.T) {
	name := uniqueName("mutating-kill")
	id := newJob(t, name, jobStep{Name: "deploy", Mutating: true, Run: tickScript(name, 30)})
	waitTicks(t, name, 2, 3*time.Minute)
	shared.stack.killRunner()
	waitNoSandboxes(t, 3*time.Second)
	require.NoError(t, shared.stack.startRunner())
	d := shared.api.waitJob(t, id, 4*time.Minute, finished)
	assert.Equal(t, "lost", d.Job.State)
	assert.Equal(t, "LOST_AFTER_MUTATION", d.Job.FailureReason)
	assert.Equal(t, 1, d.Job.AttemptCount, "a job that may have changed something is never run again")
	assert.Len(t, shared.sink.attempts(name), 1)
}

func TestAShortServerOutageDoesNotRestartTheJob(t *testing.T) {
	name := uniqueName("server-20s")
	id := newJob(t, name, jobStep{Name: "ticks", Run: tickScript(name, 45)})
	waitTicks(t, name, 2, 3*time.Minute)
	shared.stack.killServer()
	time.Sleep(20 * time.Second)
	_, err := shared.stack.startServer()
	require.NoError(t, err)
	d := shared.api.waitJob(t, id, 4*time.Minute, finished)
	assert.Equal(t, "succeeded", d.Job.State)
	assert.Equal(t, 1, d.Job.AttemptCount)
	assert.Len(t, shared.sink.attempts(name), 1)
}

func TestALongServerOutageFencesTheRunnerAndRetriesWithoutOverlap(t *testing.T) {
	name := uniqueName("server-90s")
	id := newJob(t, name, jobStep{Name: "ticks", Run: tickScript(name, 20)})
	waitTicks(t, name, 2, 3*time.Minute)
	shared.stack.killServer()
	time.Sleep(40 * time.Second)
	assert.Empty(t, shared.stack.sandboxes(), "the runner fenced itself and killed the sandbox within 35 seconds")
	time.Sleep(50 * time.Second)
	_, err := shared.stack.startServer()
	require.NoError(t, err)
	d := shared.api.waitJob(t, id, 5*time.Minute, finished)
	assert.Equal(t, "succeeded", d.Job.State)
	assert.Equal(t, 2, d.Job.AttemptCount)
	assert.Empty(t, shared.sink.overlaps(name))
}

func TestANetworkBlackholeFencesTheRunnerAndRetriesWithoutOverlap(t *testing.T) {
	name := uniqueName("blackhole-60s")
	id := newJob(t, name, jobStep{Name: "ticks", Run: tickScript(name, 20)})
	waitTicks(t, name, 2, 3*time.Minute)
	shared.stack.runnerProxy.Blackhole(true)
	time.Sleep(40 * time.Second)
	assert.Empty(t, shared.stack.sandboxes(), "the runner fenced itself within 35 seconds without the server")
	time.Sleep(20 * time.Second)
	shared.stack.runnerProxy.Blackhole(false)
	d := shared.api.waitJob(t, id, 5*time.Minute, finished)
	assert.Equal(t, "succeeded", d.Job.State)
	assert.Equal(t, 2, d.Job.AttemptCount)
	assert.Empty(t, shared.sink.overlaps(name))
}

func TestAPostgresRestartMidJobDoesNotLoseIt(t *testing.T) {
	name := uniqueName("postgres-restart")
	id := newJob(t, name, jobStep{Name: "ticks", Run: tickScript(name, 30)})
	waitTicks(t, name, 2, 3*time.Minute)
	require.NoError(t, restartPostgres())
	d := shared.api.waitJob(t, id, 4*time.Minute, finished)
	assert.Equal(t, "succeeded", d.Job.State)
	assert.Equal(t, 1, d.Job.AttemptCount)
}

func TestCancelingDuringTheImagePullStartsNoSandbox(t *testing.T) {
	ensureUp(t)
	shared.slowPull.Throttle(100 << 10)
	name := uniqueName("cancel-pull")
	id := shared.api.createJob(t, name, jobSpec{Image: shared.slowImage, Steps: []jobStep{{Name: "never", Run: tickScript(name, 1)}}})
	shared.api.waitJob(t, id, 2*time.Minute, func(d jobDetail) bool { return d.Job.State == "leased" || d.Job.State == "running" })
	time.Sleep(3 * time.Second)
	shared.api.cancel(t, id)
	d := shared.api.waitJob(t, id, 2*time.Minute, finished)
	shared.slowPull.Throttle(0)
	assert.Equal(t, "canceled", d.Job.State)
	assert.Empty(t, shared.sink.of(name), "no step ran")
	waitNoSandboxes(t, 30*time.Second)
}
