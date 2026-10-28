package config

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const pidMaxPath = "/proc/sys/kernel/pid_max"

type Machine struct {
	CPUMillis   int64
	MemoryBytes int64
	PidMax      int64
	DiskFree    func(path string) (int64, error)
}

func DetectMachine() (Machine, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return Machine{}, fmt.Errorf("read the machine memory: %w", err)
	}
	m := Machine{
		CPUMillis:   int64(runtime.NumCPU()) * MillisPerCPU,
		MemoryBytes: int64(info.Totalram) * int64(info.Unit), //nolint:gosec // Kernel counters fit in int64 on every supported machine.
		DiskFree:    diskFree,
	}
	if raw, err := os.ReadFile(pidMaxPath); err == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64); err == nil {
			m.PidMax = n
		}
	}
	return m, nil
}

func diskFree(path string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, fmt.Errorf("read free space under %s: %w", path, err)
	}
	return int64(st.Bavail) * st.Bsize, nil //nolint:gosec // Block counts fit in int64 on every supported filesystem.
}
