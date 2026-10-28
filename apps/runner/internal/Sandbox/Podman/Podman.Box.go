package podman

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	sandbox "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox"
	wire "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox/Wire"
	agentv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/agent/v1"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const helloTimeout = 30 * time.Second

type box struct {
	d        *Driver
	name     string
	spec     sandbox.PrepareSpec
	socket   string
	listener net.Listener

	mu        sync.Mutex
	conn      *wire.Conn
	started   bool
	destroyed bool
}

func newBox(ctx context.Context, d *Driver, l layout, spec sandbox.PrepareSpec) (*box, error) {
	socket := filepath.Join(l.controlDir, controlSocketName)
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "unix", socket)
	if err != nil {
		return nil, fmt.Errorf("open the control channel: %w", err)
	}
	if err := os.Chmod(socket, config.PrivateFileMode); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("lock down the control channel: %w", err)
	}
	return &box{d: d, name: l.name, spec: spec, socket: socket, listener: ln}, nil
}

func (b *box) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.destroyed {
		b.mu.Unlock()
		return sandbox.ErrDestroyed
	}
	b.mu.Unlock()
	if _, err := b.d.runner().run(ctx, "start", "--", b.name); err != nil {
		return err
	}
	conn, err := b.accept(ctx)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.conn = conn
	b.started = true
	b.mu.Unlock()
	return nil
}

func (b *box) accept(ctx context.Context) (*wire.Conn, error) {
	deadline := time.Now().Add(helloTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = b.listener.(*net.UnixListener).SetDeadline(deadline)
	raw, err := b.listener.Accept()
	_ = b.listener.Close()
	if err != nil {
		return nil, fmt.Errorf("the agent did not connect to the control channel: %w", err)
	}
	conn := wire.NewConn(raw)
	frame, err := conn.Read()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("the agent did not greet the runner: %w", err)
	}
	hello := frame.GetHello()
	if hello == nil || hello.GetProtocolVersion() != wire.ProtocolVersion {
		_ = conn.Close()
		return nil, fmt.Errorf("the agent speaks protocol %d, not %d", hello.GetProtocolVersion(), wire.ProtocolVersion)
	}
	return conn, nil
}

func (b *box) Exec(ctx context.Context, step sandbox.StepSpec) (sandbox.StepResult, error) {
	conn := b.connIfRunning()
	if conn == nil {
		return sandbox.StepResult{}, sandbox.ErrDestroyed
	}
	if err := conn.Write(runStepFrame(step)); err != nil {
		return sandbox.StepResult{}, fmt.Errorf("send a step to the agent: %w", err)
	}
	if step.Started != nil {
		step.Started()
	}
	return b.readStep(ctx, conn, step)
}

func (b *box) readStep(ctx context.Context, conn *wire.Conn, step sandbox.StepSpec) (sandbox.StepResult, error) {
	for {
		frame, err := conn.Read()
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return sandbox.StepResult{}, sandbox.ErrDestroyed
			}
			return sandbox.StepResult{}, fmt.Errorf("read the step result from the agent: %w", err)
		}
		switch body := frame.GetBody().(type) {
		case *agentv1.Frame_Output:
			writeOutput(step, body.Output)
		case *agentv1.Frame_StepExit:
			return b.stepResult(ctx, body.StepExit)
		}
	}
}

func writeOutput(step sandbox.StepSpec, o *agentv1.Output) {
	w := step.Stdout
	if o.GetStream() == agentv1.OutputStream_OUTPUT_STREAM_STDERR {
		w = step.Stderr
	}
	if w != nil {
		_, _ = w.Write(o.GetData())
	}
}

func (b *box) stepResult(ctx context.Context, exit *agentv1.StepExit) (sandbox.StepResult, error) {
	if exit.GetFailure() == agentv1.StepFailure_STEP_FAILURE_SHELL_NOT_FOUND {
		return sandbox.StepResult{}, sandbox.ErrShellMissing
	}
	res := sandbox.StepResult{ExitCode: exit.GetExitCode(), Signal: syscall.Signal(exit.GetSignal())}
	if exit.GetSignal() == int32(syscall.SIGKILL) && b.oomKilled(ctx) {
		res.Reason = runnerv1.FailureReason_FAILURE_REASON_OOM_KILLED
	}
	return res, nil
}

func (b *box) oomKilled(ctx context.Context) bool {
	in, err := inspect(ctx, b.d.runner(), b.name)
	return err == nil && in.State.OOMKilled
}

func runStepFrame(step sandbox.StepSpec) *agentv1.Frame {
	files := make([]*agentv1.SecretFile, 0, len(step.Files))
	for name, data := range step.Files {
		files = append(files, &agentv1.SecretFile{Name: name, Data: data})
	}
	return &agentv1.Frame{Body: &agentv1.Frame_RunStep{RunStep: &agentv1.RunStep{
		Index: step.Index, Script: step.Script, Env: step.Env, SecretFiles: files,
	}}}
}

func (b *box) connIfRunning() *wire.Conn {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.destroyed || !b.started {
		return nil
	}
	return b.conn
}

func (b *box) Signal(ctx context.Context, sig syscall.Signal) error {
	conn := b.connIfRunning()
	if conn == nil {
		return nil
	}
	ps := agentv1.ProcessSignal_PROCESS_SIGNAL_TERM
	if sig == syscall.SIGKILL {
		ps = agentv1.ProcessSignal_PROCESS_SIGNAL_KILL
	}
	_ = ctx
	return conn.Write(&agentv1.Frame{Body: &agentv1.Frame_Signal{Signal: &agentv1.Signal{Signal: ps}}})
}

func (b *box) Kill(ctx context.Context) error {
	b.mu.Lock()
	if b.destroyed {
		b.mu.Unlock()
		return nil
	}
	conn := b.conn
	b.started = false
	b.mu.Unlock()
	_, err := b.d.runner().run(ctx, "kill", "--signal", "KILL", "--", b.name)
	if conn != nil {
		_ = conn.Close()
	}
	if err != nil && !isGone(err) {
		return err
	}
	return nil
}

func (b *box) Destroy(ctx context.Context) error {
	b.mu.Lock()
	if b.destroyed {
		b.mu.Unlock()
		return nil
	}
	b.destroyed = true
	conn := b.conn
	ln := b.listener
	b.conn = nil
	b.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
	if ln != nil {
		_ = ln.Close()
	}
	_, err := b.d.runner().run(ctx, "rm", "--force", "--volumes", "--", b.name)
	_ = os.Remove(b.socket)
	if err != nil && !isGone(err) {
		return err
	}
	return nil
}

func (b *box) Inspect(ctx context.Context) (sandbox.Inspection, error) {
	b.mu.Lock()
	destroyed := b.destroyed
	b.mu.Unlock()
	if destroyed {
		return sandbox.Inspection{Destroyed: true}, nil
	}
	in, err := inspect(ctx, b.d.runner(), b.name)
	if err != nil {
		if isGone(err) {
			return sandbox.Inspection{Destroyed: true}, nil
		}
		return sandbox.Inspection{}, err
	}
	return sandbox.Inspection{Running: in.State.Running}, nil
}

func isGone(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "no such container") || strings.Contains(s, "no container with name")
}
