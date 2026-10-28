package podman

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	sandbox "github.com/Khaymah/dhow/apps/runner/internal/Sandbox"
)

func writeCgroup(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	rel := "/system.slice/dhow-runner.service/libpod-payload-abc"
	dir := filepath.Join(root, rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, rel
}

func TestVerifyCgroup(t *testing.T) {
	spec := sandbox.PrepareSpec{
		Resources: sandbox.Resources{CPUMillis: 500, MemoryBytes: 128 << 20, Pids: 64, DiskBytes: 16 << 20},
		DiskMode:  config.DiskModeTmpfs,
	}
	enforced := map[string]string{
		"memory.max": "150994944", "memory.swap.max": "0", "pids.max": "64", "cpu.max": "50000 100000",
	}
	with := func(k, v string) map[string]string {
		out := map[string]string{}
		for a, b := range enforced {
			out[a] = b
		}
		if v == "" {
			delete(out, k)
		} else {
			out[k] = v
		}
		return out
	}
	tests := []struct {
		name  string
		files map[string]string
		path  string
		ok    bool
	}{
		{"every limit is in force", enforced, "", true},
		{"no memory limit", with("memory.max", "max"), "", false},
		{"swap allowed", with("memory.swap.max", "max"), "", false},
		{"no pids limit", with("pids.max", "max"), "", false},
		{"no CPU limit", with("cpu.max", "max 100000"), "", false},
		{"a different CPU limit", with("cpu.max", "100000 100000"), "", false},
		{"the controller is missing", with("pids.max", ""), "", false},
		{"no cgroup path", enforced, "none", false},
		{"the root cgroup", enforced, "/", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, rel := writeCgroup(t, tt.files)
			switch tt.path {
			case "none":
				rel = ""
			case "/":
				rel = "/"
			}
			err := verifyCgroup(root, rel, spec)
			if tt.ok && err != nil {
				t.Fatalf("want the limits accepted, got %v", err)
			}
			if !tt.ok && !errors.Is(err, ErrPolicyMismatch) {
				t.Fatalf("want ErrPolicyMismatch, got %v", err)
			}
		})
	}
}

func TestOomKillsReadsTheCounter(t *testing.T) {
	root, rel := writeCgroup(t, map[string]string{"memory.events": "low 0\nhigh 0\nmax 4\noom 2\noom_kill 2\noom_group_kill 0"})
	if got := oomKills(filepath.Join(root, rel)); got != 2 {
		t.Fatalf("want 2 OOM kills, got %d", got)
	}
	if got := oomKills(""); got != 0 {
		t.Fatalf("no cgroup means no OOM kills, got %d", got)
	}
}
