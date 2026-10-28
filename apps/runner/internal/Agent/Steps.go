package agent

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sort"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"

	agentv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/agent/v1"
)

const (
	outputChunkBytes = 32 << 10
	defaultPath      = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

type process struct {
	cmd *exec.Cmd
	mu  sync.Mutex
}

func (p *process) signal(sig unix.Signal) {
	p.mu.Lock()
	cmd := p.cmd
	p.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = unix.Kill(-cmd.Process.Pid, sig)
}

func (a *Agent) runStep(ctx context.Context, step *agentv1.RunStep) error {
	if err := writeSecretFiles(step.GetSecretFiles()); err != nil {
		wipeSecretFiles(step.GetSecretFiles())
		return a.conn.Write(stepExit(step.GetIndex(), 1, 0, agentv1.StepFailure_STEP_FAILURE_SECRET_FILES))
	}
	defer wipeSecretFiles(step.GetSecretFiles())
	cmd := exec.CommandContext(ctx, Shell, "-e", "-s")
	cmd.Env = stepEnv(step.GetEnv())
	cmd.Stdin = stdinReader(step.GetScript())
	cmd.SysProcAttr = &unix.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return a.conn.Write(stepExit(step.GetIndex(), 1, 0, agentv1.StepFailure_STEP_FAILURE_START_FAILED))
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return a.conn.Write(stepExit(step.GetIndex(), 1, 0, agentv1.StepFailure_STEP_FAILURE_START_FAILED))
	}
	if err := cmd.Start(); err != nil {
		return a.startFailed(step.GetIndex(), err)
	}
	p := &process{cmd: cmd}
	a.setCurrent(p)
	var wg sync.WaitGroup
	wg.Add(2)
	go a.pump(&wg, step.GetIndex(), agentv1.OutputStream_OUTPUT_STREAM_STDOUT, stdout)
	go a.pump(&wg, step.GetIndex(), agentv1.OutputStream_OUTPUT_STREAM_STDERR, stderr)
	wg.Wait()
	err = cmd.Wait()
	a.clearCurrent()
	code, sig := exitStatus(err)
	return a.conn.Write(stepExit(step.GetIndex(), code, sig, agentv1.StepFailure_STEP_FAILURE_UNSPECIFIED))
}

func (a *Agent) startFailed(index int32, err error) error {
	failure := agentv1.StepFailure_STEP_FAILURE_START_FAILED
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		failure = agentv1.StepFailure_STEP_FAILURE_SHELL_NOT_FOUND
	}
	return a.conn.Write(stepExit(index, 1, 0, failure))
}

func (a *Agent) pump(wg *sync.WaitGroup, index int32, stream agentv1.OutputStream, r io.Reader) {
	defer wg.Done()
	buf := make([]byte, outputChunkBytes)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			_ = a.conn.Write(&agentv1.Frame{Body: &agentv1.Frame_Output{Output: &agentv1.Output{
				StepIndex: index, Stream: stream, Data: append([]byte(nil), buf[:n]...),
			}}})
		}
		if err != nil {
			return
		}
	}
}

func stepEnv(env map[string]string) []string {
	out := make([]string, 0, len(env)+1)
	keys := make([]string, 0, len(env))
	hasPath := false
	for k := range env {
		keys = append(keys, k)
		if k == "PATH" {
			hasPath = true
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}
	if !hasPath {
		out = append(out, "PATH="+defaultPath)
	}
	return out
}

func stdinReader(script string) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		_, _ = io.WriteString(pw, script)
		_ = pw.Close()
	}()
	return pr
}

func exitStatus(err error) (int32, int32) {
	if err == nil {
		return 0, 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		ws, ok := ee.Sys().(syscall.WaitStatus)
		if ok && ws.Signaled() {
			sig := int32(ws.Signal()) //nolint:gosec // Signal numbers are below 64.
			return 128 + sig, sig
		}
		return int32(ee.ExitCode()), 0 //nolint:gosec // Process exit codes are in the range 0-255.
	}
	return 1, 0
}

func stepExit(index, code, sig int32, failure agentv1.StepFailure) *agentv1.Frame {
	return &agentv1.Frame{Body: &agentv1.Frame_StepExit{StepExit: &agentv1.StepExit{
		Index: index, ExitCode: code, Signal: sig, Failure: failure,
	}}}
}
