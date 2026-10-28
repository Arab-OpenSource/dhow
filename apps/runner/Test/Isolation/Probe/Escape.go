//go:build linux

package main

import (
	"bufio"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
)

const (
	envHostPaths = "DHOW_PROBE_HOST_PATHS"
	envCanary    = "DHOW_PROBE_CANARY"
	controlPath  = "/run/dhow/control/control.sock"
	maxScanBytes = 1 << 20
)

func escapeProbes() []result {
	out := []result{
		readable("read /etc/shadow", "/etc/shadow"),
		listable("list /proc/1/root", "/proc/1/root"),
		readable("read the agent environment", "/proc/1/environ"),
		hostMounts(),
		writable("write /proc/sys", "/proc/sys/kernel/hostname"),
		writable("write /proc/sysrq-trigger", "/proc/sysrq-trigger"),
		writable("write /sys", "/sys/kernel/uevent_helper"),
		writable("write the cgroup", "/sys/fs/cgroup/cgroup.procs"),
		writable("write /etc/hostname", "/etc/hostname"),
		writable("write /run/.containerenv", "/run/.containerenv"),
		writable("write /etc/resolv.conf", "/etc/resolv.conf"),
		absent("see the host's subscription secrets", "/run/secrets"),
		controlChannel(),
		capabilities(),
	}
	out = append(out, hostPaths()...)
	out = append(out, engineSockets()...)
	out = append(out, syscallProbes()...)
	return append(out, canary())
}

func readable(name, path string) result {
	data, err := os.ReadFile(path) //nolint:gosec // The probe reads fixed paths that must stay out of reach.
	if err == nil {
		return escaped(name, "read %d bytes of %s", len(data), path)
	}
	return blocked(name)
}

func absent(name, path string) result {
	if _, err := os.Lstat(path); err == nil {
		return escaped(name, "%s exists", path)
	}
	return blocked(name)
}

func listable(name, path string) result {
	entries, err := os.ReadDir(path)
	if err == nil {
		return escaped(name, "listed %d entries in %s", len(entries), path)
	}
	return blocked(name)
}

func writable(name, path string) result {
	f, err := os.OpenFile(path, os.O_WRONLY, 0) //nolint:gosec // The probe opens fixed paths that must stay read-only.
	if err != nil {
		return blocked(name)
	}
	_, werr := f.Write([]byte("1"))
	_ = f.Close()
	return blockedUnless(name, werr, "writing "+path)
}

func hostPaths() []result {
	var out []result
	paths := []string{"/var/lib/dhow-runner", "/etc/dhow-runner"}
	for _, p := range strings.Split(os.Getenv(envHostPaths), ":") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	for _, p := range paths {
		name := "reach " + p
		if _, err := os.Lstat(p); err == nil { //nolint:gosec // The probe exists to try this from inside the sandbox.
			out = append(out, escaped(name, "%s exists inside the job", p))
			continue
		}
		link := filepath.Join("/workspace", "probe-link-"+filepath.Base(p))
		if err := os.Symlink(p, link); err == nil {
			_, rerr := os.ReadDir(link)
			_ = os.Remove(link)
			if rerr == nil {
				out = append(out, escaped(name, "a symlink in /workspace reaches %s", p))
				continue
			}
		}
		out = append(out, blocked(name))
	}
	return out
}

func engineSockets() []result {
	candidates := []string{"/var/run/docker.sock", "/run/docker.sock", "/run/podman/podman.sock"}
	if found, err := filepath.Glob("/run/user/*/podman/podman.sock"); err == nil {
		candidates = append(candidates, found...)
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "podman", "podman.sock"))
	}
	out := make([]result, 0, len(candidates))
	for _, p := range candidates {
		if _, err := os.Lstat(p); err == nil { //nolint:gosec // The probe exists to try this from inside the sandbox.
			out = append(out, escaped("engine socket "+p, "%s exists inside the job", p))
			continue
		}
		out = append(out, blocked("engine socket "+p))
	}
	return out
}

func controlChannel() result {
	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", controlPath)
	if err == nil {
		_ = conn.Close()
		return escaped("connect to the control channel", "a job process connected to %s", controlPath)
	}
	return blocked("connect to the control channel")
}

func capabilities() result {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return escaped("no capabilities", "cannot read /proc/self/status: %v", err)
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		for _, field := range []string{"CapInh:", "CapPrm:", "CapEff:", "CapBnd:", "CapAmb:"} {
			if v, ok := strings.CutPrefix(line, field); ok && strings.Trim(strings.TrimSpace(v), "0") != "" {
				return escaped("no capabilities", "%s %s", field, strings.TrimSpace(v))
			}
		}
	}
	return blocked("no capabilities")
}

func allowedMount(point string) bool {
	exact := []string{
		"/", "/proc", "/dev", "/sys", "/workspace", "/tmp", "/run/.containerenv", "/etc/hostname", "/etc/resolv.conf",
		"/etc/hosts", "/run/dhow/agent", "/run/dhow/bin/connect", "/run/dhow/control", "/run/dhow/secrets",
	}
	for _, e := range exact {
		if point == e {
			return true
		}
	}
	for _, prefix := range []string{"/proc/", "/dev/", "/sys/"} {
		if strings.HasPrefix(point, prefix) {
			return true
		}
	}
	return false
}

func hostMounts() result {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return escaped("only the sandbox mounts", "cannot read mountinfo: %v", err)
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 6 {
			continue
		}
		point, opts := fields[4], fields[5]
		if !allowedMount(point) {
			return escaped("only the sandbox mounts", "unexpected mount %s", point)
		}
		if strings.Contains(","+opts+",", ",rw,") && (point == "/run/dhow/agent" || point == "/run/dhow/bin/connect" || point == "/run/dhow/control") {
			return escaped("only the sandbox mounts", "%s is writable", point)
		}
	}
	return blocked("only the sandbox mounts")
}

func canary() result {
	token := reverse(os.Getenv(envCanary))
	if token == "" {
		return blocked("runner secrets out of reach")
	}
	found := ""
	_ = filepath.WalkDir("/", func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (p == "/proc" || p == "/sys" || p == "/dev") {
			return filepath.SkipDir
		}
		if found == "" && d.Type().IsRegular() && containsToken(p, token) {
			found = p
			return errors.New("found")
		}
		return nil
	})
	if found == "" {
		found = environHolding(token)
	}
	if found != "" {
		return escaped("runner secrets out of reach", "the canary is in %s", found)
	}
	return blocked("runner secrets out of reach")
}

func containsToken(path, token string) bool {
	f, err := os.Open(path) //nolint:gosec // The probe scans its own sandbox for a canary.
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, maxScanBytes)
	n, _ := f.Read(buf)
	return strings.Contains(string(buf[:n]), token)
}

func environHolding(token string) string {
	paths, _ := filepath.Glob("/proc/[0-9]*/environ")
	for _, p := range paths {
		if containsToken(p, token) {
			return p
		}
	}
	return ""
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
