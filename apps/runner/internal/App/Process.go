package app

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func hardenProcess() error {
	if err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0); err != nil {
		return fmt.Errorf("turn off core dumps and ptrace for the runner (PR_SET_DUMPABLE): %w", err)
	}
	if err := unix.Setrlimit(unix.RLIMIT_CORE, &unix.Rlimit{Cur: 0, Max: 0}); err != nil {
		return fmt.Errorf("turn off core dumps for the runner (RLIMIT_CORE): %w", err)
	}
	return nil
}
