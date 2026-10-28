package podman

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	sandbox "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox"
)

func (b *box) enforced(ctx context.Context) error {
	in, err := inspect(ctx, b.d.runner(), b.name)
	if err != nil {
		return err
	}
	if err := verifyCgroup(cgroupRoot, in.State.CgroupPath, b.spec); err != nil {
		return err
	}
	if err := b.verifyUserNS(in.State.Pid); err != nil {
		return err
	}
	b.mu.Lock()
	b.cgroup = filepath.Join(cgroupRoot, in.State.CgroupPath)
	b.mu.Unlock()
	return nil
}

func oomKills(dir string) int64 {
	if dir == "" {
		return 0
	}
	raw, err := os.ReadFile(filepath.Join(dir, "memory.events")) //nolint:gosec // The sandbox cgroup that Podman reports, under /sys/fs/cgroup.
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if k, v, ok := strings.Cut(line, " "); ok && k == "oom_kill" {
			n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err == nil {
				return n
			}
		}
	}
	return 0
}

func verifyCgroup(root, path string, spec sandbox.PrepareSpec) error {
	if path == "" || path == "/" || strings.Contains(path, "..") {
		return fmt.Errorf("%w: the sandbox has no cgroup of its own (%q)", ErrPolicyMismatch, path)
	}
	dir := filepath.Join(root, path)
	mem := spec.Resources.MemoryBytes
	if spec.DiskMode == config.DiskModeTmpfs {
		mem += spec.Resources.DiskBytes
	}
	want := map[string]string{
		"memory.max":      strconv.FormatInt(mem, 10),
		"memory.swap.max": "0",
		"pids.max":        strconv.FormatInt(spec.Resources.Pids, 10),
	}
	var bad []string
	for file, value := range want {
		got, err := readCgroupFile(dir, file)
		if err != nil || got != value {
			bad = append(bad, fmt.Sprintf("%s is %q, not %q", file, got, value))
		}
	}
	if millis, err := cpuMillis(dir); err != nil || millis != spec.Resources.CPUMillis {
		bad = append(bad, fmt.Sprintf("cpu.max allows %d millicores, not %d", millis, spec.Resources.CPUMillis))
	}
	if len(bad) > 0 {
		return fmt.Errorf("%w: the kernel does not enforce the limits in %s: %s", ErrPolicyMismatch, dir, strings.Join(bad, "; "))
	}
	return nil
}

func readCgroupFile(dir, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // The path is the sandbox cgroup that Podman reports, under /sys/fs/cgroup.
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func cpuMillis(dir string) (int64, error) {
	raw, err := readCgroupFile(dir, "cpu.max")
	if err != nil {
		return 0, err
	}
	quota, period, ok := strings.Cut(raw, " ")
	if !ok || quota == "max" {
		return 0, fmt.Errorf("cpu.max is %q", raw)
	}
	q, err := strconv.ParseInt(quota, 10, 64)
	if err != nil {
		return 0, err
	}
	p, err := strconv.ParseInt(period, 10, 64)
	if err != nil || p <= 0 {
		return 0, fmt.Errorf("cpu.max is %q", raw)
	}
	return q * config.MillisPerCPU / p, nil
}
