package agent

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	wire "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox/Wire"
	agentv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/agent/v1"
)

func runStepFrame(index int32, script string, files ...*agentv1.SecretFile) *agentv1.Frame {
	return &agentv1.Frame{Body: &agentv1.Frame_RunStep{RunStep: &agentv1.RunStep{Index: index, Script: script, SecretFiles: files}}}
}

func outputUntilExit(t *testing.T, conn *wire.Conn, within time.Duration) (string, *agentv1.StepExit) {
	t.Helper()
	type result struct {
		out  string
		exit *agentv1.StepExit
	}
	got := make(chan result, 1)
	go func() {
		var out bytes.Buffer
		for {
			f, err := conn.Read()
			if err != nil {
				got <- result{out: out.String()}
				return
			}
			if o := f.GetOutput(); o != nil {
				out.Write(o.GetData())
			}
			if e := f.GetStepExit(); e != nil {
				got <- result{out: out.String(), exit: e}
				return
			}
		}
	}()
	select {
	case r := <-got:
		return r.out, r.exit
	case <-time.After(within):
		t.Fatalf("the step did not end within %s", within)
		return "", nil
	}
}

func TestASecretFileExistsOnlyDuringItsStep(t *testing.T) {
	dir := t.TempDir()
	conn, _ := startAgentWith(t, Options{SecretsDir: dir})
	file := &agentv1.SecretFile{Name: "deploy-key", Data: []byte("canary-secret-value")}
	if err := conn.Write(runStepFrame(0, "cat "+filepath.Join(dir, "deploy-key")+"\n", file)); err != nil {
		t.Fatal(err)
	}
	out, exit := outputUntilExit(t, conn, 5*time.Second)
	if exit == nil || exit.GetExitCode() != 0 || out != "canary-secret-value" {
		t.Fatalf("the step did not read its secret file: exit %v, output %q", exit, out)
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatalf("the secrets directory still holds %d entries after the step", len(left))
	}
}

func TestAPlantedFileCannotCatchTheNextStepsSecret(t *testing.T) {
	tests := []struct {
		name  string
		plant func(t *testing.T, path string) func() []byte
	}{
		{"a fifo held open for reading", func(t *testing.T, path string) func() []byte {
			if err := unix.Mkfifo(path, 0o600); err != nil {
				t.Fatal(err)
			}
			fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
			if err != nil {
				t.Fatal(err)
			}
			return drain(t, os.NewFile(uintptr(fd), path))
		}},
		{"a regular file held open", func(t *testing.T, path string) func() []byte {
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			return drain(t, f)
		}},
		{"a symlink out of the directory", func(t *testing.T, path string) func() []byte {
			target := filepath.Join(t.TempDir(), "stolen")
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			return func() []byte {
				b, _ := os.ReadFile(target)
				return b
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "token")
			leaked := tt.plant(t, path)
			conn, _ := startAgentWith(t, Options{SecretsDir: dir})
			file := &agentv1.SecretFile{Name: "token", Data: []byte("fresh-secret")}
			if err := conn.Write(runStepFrame(2, "cat "+path+"\n", file)); err != nil {
				t.Fatal(err)
			}
			out, exit := outputUntilExit(t, conn, 5*time.Second)
			if exit == nil || exit.GetExitCode() != 0 || out != "fresh-secret" {
				t.Fatalf("the step did not get its secret: exit %v, output %q", exit, out)
			}
			if got := leaked(); bytes.Contains(got, []byte("fresh-secret")) {
				t.Fatal("the planted file caught the secret")
			}
		})
	}
}

func drain(t *testing.T, f *os.File) func() []byte {
	t.Cleanup(func() { _ = f.Close() })
	return func() []byte {
		buf := make([]byte, 4096)
		n, _ := unix.Read(int(f.Fd()), buf)
		if n < 0 {
			return nil
		}
		return buf[:n]
	}
}

func TestTheSweepRunsAfterTheShellAndBeforeTheExitIsReported(t *testing.T) {
	swept := make(chan struct{}, 1)
	conn, _ := startAgentWith(t, Options{SecretsDir: t.TempDir(), Sweep: func() error {
		swept <- struct{}{}
		return nil
	}})
	if err := conn.Write(runStepFrame(0, "true\n")); err != nil {
		t.Fatal(err)
	}
	_, exit := outputUntilExit(t, conn, 5*time.Second)
	if exit == nil {
		t.Fatal("no exit reported")
	}
	select {
	case <-swept:
	default:
		t.Fatal("the step exit was reported before the leftover processes were swept")
	}
}

func TestAFailedSweepClosesTheChannelInsteadOfReportingTheStep(t *testing.T) {
	conn, done := startAgentWith(t, Options{SecretsDir: t.TempDir(), Sweep: func() error { return ErrSweepTimedOut }})
	if err := conn.Write(runStepFrame(0, "true\n")); err != nil {
		t.Fatal(err)
	}
	if _, exit := outputUntilExit(t, conn, 5*time.Second); exit != nil {
		t.Fatalf("the agent reported a step whose leftovers survived: %v", exit)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrChannelClosed) && err == nil {
			t.Fatalf("want the agent to stop, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the agent kept running after a failed sweep")
	}
}

func TestTheAgentRefusesToRunOutsideAPIDNamespace(t *testing.T) {
	if os.Getpid() == 1 {
		t.Skip("the test itself runs as PID 1")
	}
	err := Run(t.Context(), Options{Harden: func() error { return nil }})
	if !errors.Is(err, ErrNotInit) {
		t.Fatalf("want ErrNotInit, got %v", err)
	}
}
