//go:build isolation

package isolation_test

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sandbox "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox"
)

const exitBlocked = 3

func escapeLines(out output) []string {
	var lines []string
	for _, l := range strings.Split(out.stdout, "\n") {
		if strings.HasPrefix(l, "ESCAPED ") {
			lines = append(lines, l)
		}
	}
	return lines
}

func TestEveryEscapeProbeFails(t *testing.T) {
	box, dir := start(t, defaultResources())
	hostPaths := strings.Join([]string{shared.stateDir, dir, filepath.Join(shared.stateDir, "identity-canary")}, ":")
	out := run(t, box, probe+" escape\n", map[string]string{
		"DHOW_PROBE_HOST_PATHS": hostPaths, "DHOW_PROBE_CANARY": reverse(shared.canary),
	})
	assert.Empty(t, escapeLines(out))
	assert.Zero(t, out.result.ExitCode, "every probe must report blocked")
	assert.Contains(t, out.stdout, "blocked no capabilities")
}

func TestASetuidBinaryGainsNothing(t *testing.T) {
	box, _ := start(t, defaultResources())
	out := run(t, box, probe+"-setuid capeff\n", nil)
	assert.Zero(t, out.result.ExitCode, out.stdout)
	assert.Contains(t, out.stdout, "blocked no capabilities")
}

func TestTwoJobsCannotSeeEachOther(t *testing.T) {
	first, _ := start(t, defaultResources())
	second, _ := start(t, defaultResources())
	a, b := token(), token()
	outs := make([]output, 2)
	var wg sync.WaitGroup
	for i, j := range []struct {
		box         sandbox.Sandbox
		mine, other string
	}{{first, a, b}, {second, b, a}} {
		wg.Go(func() {
			outs[i] = run(t, j.box, strings.Join([]string{
				probe + " marker " + j.mine + " 30 &",
				probe + " sleep 5",
				probe + " hunt " + j.other,
				"wait",
				"",
			}, "\n"), nil)
		})
	}
	wg.Wait()
	for _, out := range outs {
		assert.Empty(t, escapeLines(out))
		assert.Zero(t, out.result.ExitCode, "each job set its markers and found none of the other's")
	}
}

func TestSecretsNeverReachPodman(t *testing.T) {
	box, _ := start(t, defaultResources())
	secret := "dhow-secret-" + token()
	ctx := context.Background()
	out := runCtx(ctx, t, box, sandbox.StepSpec{
		Script: busybox + " test -s /run/dhow/secrets/deploy-key && " + busybox + " test -n \"$DEPLOY_TOKEN\"\n",
		Env:    map[string]string{"DEPLOY_TOKEN": secret},
		Files:  map[string][]byte{"deploy-key": []byte(secret)},
	})
	require.Zero(t, out.result.ExitCode, "the step sees its secret env and file")

	inspected, err := shared.driver.Podman(ctx, "inspect", "--type", "container", "--", containerOf(t))
	require.NoError(t, err)
	assert.NotContains(t, string(inspected), secret, "podman inspect")

	_, err = shared.driver.Podman(ctx, "unshare", "grep", "-r", "-l", "-F", "--", secret, shared.stateDir)
	var ee *exec.ExitError
	require.True(t, errors.As(err, &ee), "grep ran and found nothing: %v", err)
	assert.Equal(t, 1, ee.ExitCode(), "no file under the runner state directory, Podman storage included, holds the secret")

	after := run(t, box, busybox+" ls -A /run/dhow/secrets\n", nil)
	assert.Empty(t, strings.TrimSpace(after.stdout), "secret files are wiped after the step")
}

func containerOf(t *testing.T) string {
	t.Helper()
	out, err := shared.driver.Podman(context.Background(), "ps", "--filter", "label=dhow.runner="+runnerID, "--format", "{{.Names}}")
	require.NoError(t, err)
	names := strings.Fields(string(out))
	require.NotEmpty(t, names)
	return names[0]
}
