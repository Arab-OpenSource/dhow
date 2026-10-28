//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	forkCeiling = 100000
	chunkBytes  = 1 << 20
	cgroupStat  = "/sys/fs/cgroup/cpu.stat"
)

func forkbomb() int {
	self, err := os.Executable()
	if err != nil {
		return exitUsage
	}
	var pids []int
	defer func() {
		for _, pid := range pids {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			var ws syscall.WaitStatus
			_, _ = syscall.Wait4(pid, &ws, 0, nil)
		}
	}()
	for len(pids) < forkCeiling {
		pid, err := syscall.ForkExec(self, []string{self, "sleep", "60"}, &syscall.ProcAttr{}) //nolint:gosec // The probe exists to try this from inside the sandbox.
		if err != nil {
			fmt.Printf("stopped after %d processes: %v\n", len(pids), err)
			if errors.Is(err, syscall.EAGAIN) {
				return exitBlocked
			}
			return exitEscaped
		}
		pids = append(pids, pid)
	}
	fmt.Printf("started %d processes without a limit\n", len(pids))
	return exitEscaped
}

func oom() int {
	var hold [][]byte
	for {
		b := make([]byte, chunkBytes)
		for i := range b {
			b[i] = 1
		}
		hold = append(hold, b)
		if len(hold)%64 == 0 {
			fmt.Printf("holding %d MiB\n", len(hold))
		}
	}
}

func cpu(d time.Duration) int {
	start := time.Now()
	before := cpuStat()
	stop := start.Add(d)
	done := make(chan struct{})
	n := runtime.NumCPU()
	for range n {
		go func() {
			x := 0
			for time.Now().Before(stop) {
				x++
			}
			_ = x
			done <- struct{}{}
		}()
	}
	for range n {
		<-done
	}
	after := cpuStat()
	wall := time.Since(start)
	fmt.Printf("wall_usec %d\nusage_usec %d\nthrottled_usec %d\nnr_throttled %d\n", wall.Microseconds(),
		after["usage_usec"]-before["usage_usec"], after["throttled_usec"]-before["throttled_usec"],
		after["nr_throttled"]-before["nr_throttled"])
	return 0
}

func cpuStat() map[string]int64 {
	out := map[string]int64{}
	data, err := os.ReadFile(cgroupStat)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			out[k] = n
		}
	}
	return out
}

func fill(dir string) int {
	f, err := os.Create(filepath.Join(dir, "fill")) //nolint:gosec // The probe fills its own workspace.
	if err != nil {
		return exitUsage
	}
	defer func() { _ = f.Close() }()
	chunk := make([]byte, chunkBytes)
	var written int64
	for {
		n, err := f.Write(chunk)
		written += int64(n)
		if err != nil {
			fmt.Printf("wrote %d bytes: %v\n", written, err)
			if errors.Is(err, syscall.ENOSPC) {
				return exitBlocked
			}
			return exitEscaped
		}
	}
}

func ignoreTerm(d time.Duration) int {
	signal.Ignore(syscall.SIGTERM)
	time.Sleep(d)
	return 0
}
