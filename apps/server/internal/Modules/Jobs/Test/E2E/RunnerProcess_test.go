package e2e_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

const (
	procSecret      = `e2e-s3cret/value+"quoted"`
	procWait        = 60 * time.Second
	procBuildWait   = 5 * time.Minute
	procRegisterCmd = "register"
)

func procModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "go.mod not found")
		dir = parent
	}
}

func procBuild(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dhow-runner")
	ctx, cancel := context.WithTimeout(t.Context(), procBuildWait)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-tags", "dhowtest", "-o", path, "./apps/runner/cmd/dhow-runner")
	cmd.Dir = procModuleRoot(t)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build the runner with the fake driver: %s", out)
	return path
}

type procRunner struct {
	t      *testing.T
	bin    string
	dir    string
	env    []string
	cmd    *exec.Cmd
	stderr *apptest.SyncBuffer
}

func procServe(t *testing.T, w world) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewUnstartedServer(w.env.Handler)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	ca := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0o600))
	return srv, ca
}

func newProcRunner(t *testing.T, w world, srv *httptest.Server, ca string) *procRunner {
	t.Helper()
	base := t.TempDir()
	state := filepath.Join(base, "state")
	r := &procRunner{t: t, bin: procBuild(t), dir: base, env: []string{
		"DHOW_RUNNER_STATE_DIR=" + state, "DHOW_RUNNER_CA_FILE=" + ca, "DHOW_RUNNER_SERVER_ORIGIN=" + apptest.PublicOrigin,
		"DHOW_RUNNER_DRIVER=fake", "DHOW_RUNNER_MAX_MEMORY=512MiB", "DHOW_RUNNER_MAX_DISK=512MiB", "DHOW_RUNNER_MAX_PIDS=512",
		"DHOW_RUNNER_CANCEL_GRACE=1s", "DHOW_RUNNER_LOG_LEVEL=debug",
	}}
	token, _ := w.env.CreateRegistrationToken(w.owner, w.org, apptest.RegistrationTokenSpec{})
	tokenFile := filepath.Join(base, "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte(token+"\n"), 0o600))
	cmd := exec.CommandContext(t.Context(), r.bin, procRegisterCmd, "--url", srv.URL, "--token-file", tokenFile, "--name", "e2e")
	cmd.Env = r.env
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	require.Contains(t, string(out), "Registered runner")
	require.NoError(t, os.Remove(tokenFile))
	return r
}

func (r *procRunner) start(recording string) {
	r.t.Helper()
	path := filepath.Join(r.dir, "recording-"+uuid.NewString()+".json")
	require.NoError(r.t, os.WriteFile(path, []byte(recording), 0o600))
	r.stderr = &apptest.SyncBuffer{}
	r.cmd = exec.CommandContext(context.WithoutCancel(r.t.Context()), r.bin, "run")
	r.cmd.Env = append(append([]string(nil), r.env...), "DHOW_RUNNER_FAKE_RECORDING="+path)
	r.cmd.Stdout, r.cmd.Stderr = r.stderr, r.stderr
	require.NoError(r.t, r.cmd.Start())
	cmd := r.cmd
	r.t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
}

func (r *procRunner) kill() {
	r.t.Helper()
	require.NoError(r.t, r.cmd.Process.Kill())
	_ = r.cmd.Wait()
}

func (r *procRunner) stop() {
	r.t.Helper()
	require.NoError(r.t, r.cmd.Process.Signal(os.Interrupt))
	done := make(chan error, 1)
	go func() { done <- r.cmd.Wait() }()
	select {
	case err := <-done:
		require.NoError(r.t, err, r.stderr.String())
	case <-time.After(procWait):
		r.t.Fatal("the runner did not stop")
	}
}

func procWaitFor(t *testing.T, r *procRunner, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(procWait)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s; runner output:\n%s", what, r.stderr.String())
}

func procForms() map[string]string {
	raw := []byte(procSecret)
	j, _ := json.Marshal(procSecret)
	return map[string]string{
		"raw": procSecret, "base64": base64.StdEncoding.EncodeToString(raw), "base64url": base64.RawURLEncoding.EncodeToString(raw),
		"hex": hex.EncodeToString(raw), "url": url.QueryEscape(procSecret), "json": strings.Trim(string(j), `"`),
	}
}

func procLogText(t *testing.T, w world, job uuid.UUID) string {
	t.Helper()
	res := w.owner.Get(w.path("/jobs/" + job.String() + "/logs?Limit=500"))
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var page struct {
		Chunks []struct{ Text string }
	}
	res.Decode(t, &page)
	var b strings.Builder
	for _, c := range page.Chunks {
		b.WriteString(c.Text)
	}
	return b.String()
}

func TestTheRealRunnerRunsAJobEndToEnd(t *testing.T) {
	w := newWorldWithSecrets(t, map[string]string{"API_TOKEN": procSecret})
	srv, ca := procServe(t, w)
	r := newProcRunner(t, w, srv, ca)
	job := w.job(t, apptest.JobSpec{
		"Image":     apptest.TestImage,
		"Steps":     apptest.Steps(map[string]any{"Name": "build", "Run": "make"}, map[string]any{"Name": "print", "Run": "env"}),
		"Resources": map[string]any{"MemoryBytes": 256 << 20, "DiskBytes": 128 << 20, "Pids": 256},
		"Secrets":   []map[string]any{{"Name": "API_TOKEN", "AsEnv": "API_TOKEN", "Steps": []int{1}}},
	}, nil)
	var printed []map[string]any
	for _, enc := range []string{"raw", "base64", "base64url", "hex", "url", "json"} {
		printed = append(printed, map[string]any{"Data": enc + "=", "FromEnv": "API_TOKEN", "Encoding": enc}, map[string]any{"Data": "\n"})
	}
	recording, err := json.Marshal(map[string]any{"Steps": []map[string]any{
		{"Output": []map[string]any{{"Data": "compiling dhow\n"}, {"Stream": "stderr", "Data": "warning: none\n"}}},
		{"Output": printed},
	}})
	require.NoError(t, err)
	r.start(string(recording))

	procWaitFor(t, r, "the job to succeed", func() bool { return w.get(t, job).Job.State == "succeeded" })
	v := w.get(t, job)
	require.Len(t, v.Attempts, 1)
	assert.Equal(t, "succeeded", v.Attempts[0].State)
	require.Len(t, v.Attempts[0].Steps, 2)
	for _, s := range v.Attempts[0].Steps {
		assert.Equal(t, "succeeded", s.State)
	}
	r.stop()
	runnerLog := r.stderr.String()

	text := procLogText(t, w, job)
	for _, want := range []string{"compiling dhow\n", "warning: none\n", "dhow: step 1: build", "dhow: step 2: print", "raw=***\n", "json=\"***\"\n"} {
		assert.Contains(t, text, want)
	}
	var stored [][]byte
	rows, err := w.env.Pool.Query(context.Background(), `SELECT "Data" FROM "LogChunks" ORDER BY "Seq"`)
	require.NoError(t, err)
	for rows.Next() {
		var data []byte
		require.NoError(t, rows.Scan(&data))
		stored = append(stored, data)
	}
	require.NoError(t, rows.Err())
	require.NotEmpty(t, stored)
	all := string(bytes.Join(stored, nil))
	for name, form := range procForms() {
		assert.NotContains(t, all, form, "%s form in LogChunks", name)
		assert.NotContains(t, runnerLog, form, "%s form in the runner's own log", name)
		assert.NotContains(t, w.env.Logs.String(), form, "%s form in the server log", name)
	}
	procScan(t, r.dir)
}

func procScan(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasPrefix(d.Name(), "recording-") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for name, form := range procForms() {
			assert.NotContains(t, string(raw), form, "%s form in %s", name, path)
		}
		return nil
	}))
}

func TestARunnerKilledMidJobReportsTheAttemptLostAfterItRestarts(t *testing.T) {
	w := newWorld(t)
	srv, ca := procServe(t, w)
	r := newProcRunner(t, w, srv, ca)
	job := w.job(t, apptest.JobSpec{
		"Image": apptest.TestImage, "Steps": apptest.Steps(map[string]any{"Name": "slow", "Run": "sleep 600"}),
		"Resources": map[string]any{"MemoryBytes": 256 << 20, "DiskBytes": 128 << 20, "Pids": 256},
	}, nil)
	r.start(`{"Steps":[{"Output":[{"Data":"started\n"}],"HangMillis":600000}]}`)
	procWaitFor(t, r, "the step to start", func() bool {
		v := w.get(t, job)
		return len(v.Attempts) == 1 && len(v.Attempts[0].Steps) == 1 && v.Attempts[0].Steps[0].State == "running"
	})
	var fencing int64
	require.NoError(t, w.env.Pool.QueryRow(context.Background(), `SELECT "FencingToken" FROM "JobAttempts"`).Scan(&fencing))
	r.kill()
	assert.Equal(t, "running", w.get(t, job).Attempts[0].State, "nobody has told the server yet")

	r.start(`{"Steps":[{"Output":[{"Data":"second try\n"}]}]}`)
	procWaitFor(t, r, "the job to succeed on a second attempt", func() bool { return w.get(t, job).Job.State == "succeeded" })
	v := w.get(t, job)
	require.Len(t, v.Attempts, 2)
	first := v.Attempts[0]
	if first.Number != 1 {
		first = v.Attempts[1]
	}
	assert.Equal(t, "lost", first.State)
	assert.Equal(t, "RUNNER_RESTARTED", first.FailureReason)
	var reported int64
	require.NoError(t, w.env.Pool.QueryRow(context.Background(),
		`SELECT "FencingToken" FROM "JobAttempts" WHERE "FailureReason" = 'RUNNER_RESTARTED'`).Scan(&reported))
	assert.Equal(t, fencing, reported, "the restart report used the old fencing token")
	assert.Contains(t, r.stderr.String(), "reporting an attempt that the last run of the runner left open")
	r.stop()
}
