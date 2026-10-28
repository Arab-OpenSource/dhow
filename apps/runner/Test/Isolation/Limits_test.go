//go:build isolation

package isolation_test

import (
	"context"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sandbox "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func TestAForkBombStopsAtThePidsLimit(t *testing.T) {
	r := defaultResources()
	r.Pids = 64
	box, _ := start(t, r)
	out := run(t, box, probe+" forkbomb\n", nil)
	assert.Equal(t, int32(exitBlocked), out.result.ExitCode, "fork failed with EAGAIN")
	assert.Contains(t, out.stdout, "stopped after")
	alive := run(t, box, busybox+" true\n", nil)
	assert.Zero(t, alive.result.ExitCode, "the sandbox works again once the bomb is gone")
}

func TestMemoryAboveTheLimitIsOOMKilled(t *testing.T) {
	r := defaultResources()
	r.MemoryBytes, r.DiskBytes = 128<<20, 16<<20
	box, _ := start(t, r)
	out := run(t, box, probe+" oom\n", nil)
	assert.Equal(t, syscall.SIGKILL, out.result.Signal)
	assert.Equal(t, runnerv1.FailureReason_FAILURE_REASON_OOM_KILLED, out.result.Reason)
}

func TestCPUIsThrottledToTheLimit(t *testing.T) {
	r := defaultResources()
	r.CPUMillis = 500
	box, _ := start(t, r)
	out := run(t, box, probe+" cpu 3\n", nil)
	require.Zero(t, out.result.ExitCode)
	stat := map[string]int64{}
	for _, line := range strings.Split(out.stdout, "\n") {
		if k, v, ok := strings.Cut(line, " "); ok {
			n, _ := strconv.ParseInt(v, 10, 64)
			stat[k] = n
		}
	}
	require.Positive(t, stat["wall_usec"])
	assert.Positive(t, stat["nr_throttled"], "the cgroup throttled the job")
	share := float64(stat["usage_usec"]) / float64(stat["wall_usec"])
	assert.LessOrEqual(t, share, 0.5*1.2, "half a CPU, with room for accounting slack")
}

func TestFillingTheWorkspaceEndsInENOSPC(t *testing.T) {
	box, _ := start(t, defaultResources())
	out := run(t, box, probe+" fill /workspace\n", nil)
	assert.Equal(t, int32(exitBlocked), out.result.ExitCode, "the write failed with ENOSPC")
}

func TestAStepEndsOnTermAndOneThatIgnoresItOnKill(t *testing.T) {
	box, _ := start(t, defaultResources())
	polite := signalled(t, box, probe+" sleep 600\n", syscall.SIGTERM)
	assert.Equal(t, syscall.SIGTERM, polite.Signal)

	stubborn := signalled(t, box, probe+" ignoreterm 600\n", syscall.SIGTERM, syscall.SIGKILL)
	assert.Equal(t, syscall.SIGKILL, stubborn.Signal)
}

func signalled(t *testing.T, box sandbox.Sandbox, script string, sigs ...syscall.Signal) sandbox.StepResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), stepTimeout)
	defer cancel()
	started := make(chan struct{})
	done := make(chan sandbox.StepResult, 1)
	go func() {
		res, err := box.Exec(ctx, sandbox.StepSpec{Script: script, Started: func() { close(started) }})
		assert.NoError(t, err)
		done <- res
	}()
	<-started
	for _, sig := range sigs {
		time.Sleep(2 * time.Second)
		require.NoError(t, box.Signal(ctx, sig))
		select {
		case res := <-done:
			return res
		case <-time.After(5 * time.Second):
		}
	}
	select {
	case res := <-done:
		return res
	case <-ctx.Done():
		t.Fatal("the step outlived every signal")
		return sandbox.StepResult{}
	}
}

func TestLargeOutputArrivesWhole(t *testing.T) {
	box, _ := start(t, defaultResources())
	var n countingWriter
	ctx, cancel := context.WithTimeout(context.Background(), stepTimeout)
	defer cancel()
	res, err := box.Exec(ctx, sandbox.StepSpec{
		Script: busybox + " head -c 67108864 /dev/zero\n" + busybox + " head -c 10485760 /dev/zero | " + busybox + " tr '\\0' x\n",
		Stdout: &n,
	})
	require.NoError(t, err)
	assert.Zero(t, res.ExitCode)
	assert.Equal(t, int64(64<<20+10<<20), n.n, "64 MiB and one 10 MiB line, byte for byte")
}

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

func TestCleanupRemovesEverySandboxOfTheRunner(t *testing.T) {
	start(t, defaultResources())
	ctx := context.Background()
	require.NoError(t, shared.driver.Cleanup(ctx, runnerID))
	out, err := shared.driver.Podman(ctx, "ps", "--all", "--filter", "label=dhow.runner="+runnerID, "--format", "{{.ID}}")
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(string(out)))
}
