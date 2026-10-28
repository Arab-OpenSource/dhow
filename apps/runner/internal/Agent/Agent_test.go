package agent

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	wire "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox/Wire"
	agentv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/agent/v1"
)

func startAgent(t *testing.T) (*wire.Conn, chan error) {
	t.Helper()
	runnerSide, agentSide := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), Options{
			Harden: func() error { return nil },
			Dialer: func(context.Context, string) (io.ReadWriteCloser, error) { return agentSide, nil },
		})
	}()
	conn := wire.NewConn(runnerSide)
	t.Cleanup(func() { _ = conn.Close() })
	if f, err := conn.Read(); err != nil || f.GetHello() == nil {
		t.Fatalf("no hello from the agent: %v", err)
	}
	return conn, done
}

func stepExitOf(t *testing.T, conn *wire.Conn, within time.Duration) *agentv1.StepExit {
	t.Helper()
	got := make(chan *agentv1.StepExit, 1)
	go func() {
		for {
			f, err := conn.Read()
			if err != nil {
				close(got)
				return
			}
			if e := f.GetStepExit(); e != nil {
				got <- e
				return
			}
		}
	}()
	select {
	case e, ok := <-got:
		if !ok {
			t.Fatal("the channel closed before the step ended")
		}
		return e
	case <-time.After(within):
		t.Fatalf("the step did not end within %s", within)
		return nil
	}
}

func TestASignalReachesARunningStep(t *testing.T) {
	tests := []struct {
		name   string
		script string
		signal agentv1.ProcessSignal
		want   int32
	}{
		{"term ends a step that does not handle it", "exec sleep 30\n", agentv1.ProcessSignal_PROCESS_SIGNAL_TERM, 15},
		{"kill ends a step that ignores term", "trap '' TERM\nsleep 30\n", agentv1.ProcessSignal_PROCESS_SIGNAL_KILL, 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, _ := startAgent(t)
			if err := conn.Write(&agentv1.Frame{Body: &agentv1.Frame_RunStep{RunStep: &agentv1.RunStep{Index: 1, Script: tt.script}}}); err != nil {
				t.Fatal(err)
			}
			time.Sleep(300 * time.Millisecond)
			if err := conn.Write(&agentv1.Frame{Body: &agentv1.Frame_Signal{Signal: &agentv1.Signal{Signal: tt.signal}}}); err != nil {
				t.Fatal(err)
			}
			if e := stepExitOf(t, conn, 5*time.Second); e.GetSignal() != tt.want {
				t.Fatalf("want signal %d, got exit %d signal %d", tt.want, e.GetExitCode(), e.GetSignal())
			}
		})
	}
}

func TestClosingTheChannelKillsTheStepAndEndsTheAgent(t *testing.T) {
	conn, done := startAgent(t)
	if err := conn.Write(&agentv1.Frame{Body: &agentv1.Frame_RunStep{RunStep: &agentv1.RunStep{Index: 1, Script: "sleep 30\n"}}}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	_ = conn.Close()
	select {
	case err := <-done:
		if !errors.Is(err, ErrChannelClosed) {
			t.Fatalf("want ErrChannelClosed, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the agent outlived its control channel")
	}
}
