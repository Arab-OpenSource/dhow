package agent

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"golang.org/x/sys/unix"
)

const (
	sweepTimeout = 10 * time.Second
	sweepPoll    = 5 * time.Millisecond
)

var (
	ErrNotInit       = errors.New("the agent must run as PID 1 of the job's PID namespace")
	ErrSweepTimedOut = errors.New("processes the step left behind did not die")
)

func defaultSweep() (func() error, error) {
	if os.Getpid() != 1 {
		return nil, ErrNotInit
	}
	return sweepNamespace, nil
}

func sweepNamespace() error {
	deadline := time.Now().Add(sweepTimeout)
	for {
		if err := unix.Kill(-1, unix.SIGKILL); err != nil && !errors.Is(err, unix.ESRCH) {
			return fmt.Errorf("kill the processes the step left behind: %w", err)
		}
		var ws unix.WaitStatus
		pid, err := unix.Wait4(-1, &ws, unix.WNOHANG, nil)
		switch {
		case errors.Is(err, unix.ECHILD):
			return nil
		case errors.Is(err, unix.EINTR), err == nil && pid > 0:
			continue
		case err != nil:
			return fmt.Errorf("reap the processes the step left behind: %w", err)
		}
		if time.Now().After(deadline) {
			return ErrSweepTimedOut
		}
		time.Sleep(sweepPoll)
	}
}

func ignoreJobSignals() {
	dropped := make(chan os.Signal, 16)
	go func() {
		for range dropped {
		}
	}()
	signal.Notify(dropped,
		unix.SIGHUP, unix.SIGINT, unix.SIGQUIT, unix.SIGTERM, unix.SIGUSR1, unix.SIGUSR2,
		unix.SIGALRM, unix.SIGVTALRM, unix.SIGPROF, unix.SIGTSTP, unix.SIGTTIN, unix.SIGTTOU,
		unix.SIGXCPU, unix.SIGXFSZ, unix.SIGWINCH, unix.SIGPIPE,
	)
}
