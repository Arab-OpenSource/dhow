package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	wire "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox/Wire"
	agentv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/agent/v1"
)

const (
	ControlSocketPath = "/run/dhow/control/control.sock"
	SecretsDir        = "/run/dhow/secrets"
	Shell             = "/bin/sh"
	agentVersion      = "1"
	dialTimeout       = 30 * time.Second
)

var ErrChannelClosed = errors.New("the control channel closed, so the agent exits and the container with it")

type Options struct {
	SocketPath string
	Stdio      Stdio
	Dialer     func(ctx context.Context, path string) (io.ReadWriteCloser, error)
	Harden     func() error
	Clock      func() time.Time
}

type Stdio struct {
	Out io.Writer
	Err io.Writer
}

type Agent struct {
	conn    *wire.Conn
	harden  func() error
	mu      sync.Mutex
	current *process
}

func Run(ctx context.Context, o Options) error {
	if o.SocketPath == "" {
		o.SocketPath = ControlSocketPath
	}
	if o.Harden == nil {
		o.Harden = hardenAgent
	}
	if o.Dialer == nil {
		o.Dialer = dialUnix
	}
	if err := o.Harden(); err != nil {
		return err
	}
	dctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	rwc, err := o.Dialer(dctx, o.SocketPath)
	if err != nil {
		return fmt.Errorf("connect to the runner control channel: %w", err)
	}
	a := &Agent{conn: wire.NewConn(rwc), harden: o.Harden}
	defer func() { _ = a.conn.Close() }()
	if err := a.conn.Write(hello()); err != nil {
		return fmt.Errorf("greet the runner: %w", err)
	}
	return a.loop(ctx)
}

func hello() *agentv1.Frame {
	return &agentv1.Frame{Body: &agentv1.Frame_Hello{Hello: &agentv1.Hello{
		ProtocolVersion: wire.ProtocolVersion, AgentVersion: agentVersion,
	}}}
}

func (a *Agent) loop(ctx context.Context) error {
	for {
		frame, err := a.conn.Read()
		if err != nil {
			a.killCurrent(unix.SIGKILL)
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return ErrChannelClosed
			}
			return fmt.Errorf("read a control frame: %w", err)
		}
		switch body := frame.GetBody().(type) {
		case *agentv1.Frame_RunStep:
			if err := a.runStep(ctx, body.RunStep); err != nil {
				return err
			}
		case *agentv1.Frame_Signal:
			a.killCurrent(signalOf(body.Signal.GetSignal()))
		case *agentv1.Frame_Shutdown:
			a.killCurrent(unix.SIGKILL)
			return nil
		case *agentv1.Frame_AddMask:
		default:
		}
	}
}

func signalOf(s agentv1.ProcessSignal) unix.Signal {
	switch s {
	case agentv1.ProcessSignal_PROCESS_SIGNAL_KILL:
		return unix.SIGKILL
	default:
		return unix.SIGTERM
	}
}

func (a *Agent) killCurrent(sig unix.Signal) {
	a.mu.Lock()
	p := a.current
	a.mu.Unlock()
	if p != nil {
		p.signal(sig)
	}
}

func hardenAgent() error {
	if err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0); err != nil {
		return fmt.Errorf("turn off ptrace inside the job (PR_SET_DUMPABLE): %w", err)
	}
	return nil
}

func dialUnix(ctx context.Context, path string) (io.ReadWriteCloser, error) {
	var d net.Dialer
	for {
		conn, err := d.DialContext(ctx, "unix", path)
		if err == nil {
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func (a *Agent) setCurrent(p *process) {
	a.mu.Lock()
	a.current = p
	a.mu.Unlock()
}

func (a *Agent) clearCurrent() {
	a.mu.Lock()
	a.current = nil
	a.mu.Unlock()
}

func writeSecretFiles(files []*agentv1.SecretFile) error {
	if len(files) == 0 {
		return nil
	}
	if err := os.MkdirAll(SecretsDir, 0o700); err != nil {
		return fmt.Errorf("prepare the secrets directory: %w", err)
	}
	for _, f := range files {
		if err := writeSecretFile(f.GetName(), f.GetData()); err != nil {
			return err
		}
	}
	return nil
}

func writeSecretFile(name string, data []byte) error {
	fd, err := unix.Openat(unix.AT_FDCWD, SecretsDir+"/"+name,
		unix.O_WRONLY|unix.O_CREAT|unix.O_TRUNC|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o400)
	if err != nil {
		return fmt.Errorf("create a secret file: %w", err)
	}
	f := os.NewFile(uintptr(fd), name)
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write a secret file: %w", err)
	}
	return f.Close()
}

func wipeSecretFiles(files []*agentv1.SecretFile) {
	for _, f := range files {
		path := SecretsDir + "/" + f.GetName()
		if fd, err := unix.Openat(unix.AT_FDCWD, path, unix.O_WRONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0); err == nil {
			zero := make([]byte, len(f.GetData()))
			file := os.NewFile(uintptr(fd), path)
			_, _ = file.WriteAt(zero, 0)
			_ = file.Close()
		}
		_ = os.Remove(path)
	}
}
