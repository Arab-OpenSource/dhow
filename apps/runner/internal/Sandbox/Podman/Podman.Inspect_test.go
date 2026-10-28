package podman

import (
	"debug/elf"
	"errors"
	"strings"
	"testing"

	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	sandbox "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox"
)

func goodInspection(spec sandbox.PrepareSpec, l layout) *inspection {
	in := &inspection{}
	h := &in.HostConfig
	h.SecurityOpt = []string{"no-new-privileges", "seccomp=/state/containers/seccomp.json"}
	h.ReadonlyRootfs = true
	h.NetworkMode = "none"
	h.IpcMode, h.PidMode, h.UTSMode, h.CgroupMode = "private", "private", "private", "private"
	h.Annotations = map[string]string{"io.podman.annotations.userns": "auto:size=65536", "io.container.manager": "libpod"}
	h.Memory = spec.Resources.MemoryBytes + spec.Resources.DiskBytes
	h.MemorySwap = h.Memory
	h.PidsLimit = spec.Resources.Pids
	h.NanoCpus = spec.Resources.CPUMillis * nanosPerMilliCPU
	h.Tmpfs = map[string]string{
		workspacePath: "rw,size=1024,mode=0755,nosuid,nodev",
		tmpPath:       "rw,size=1024,mode=1777,nosuid,nodev",
		secretsMount:  "rw,size=4194304,mode=0700,noexec,nosuid,nodev",
	}
	in.Config.Env = []string{"container=podman", "HOME=/root"}
	in.Mounts = []mountInfo{
		{Type: "bind", Source: l.agentPath, Destination: agentMount},
		{Type: "bind", Source: l.controlDir, Destination: controlMount},
	}
	return in
}

func testSpec() (sandbox.PrepareSpec, layout) {
	spec := sandbox.PrepareSpec{DiskMode: config.DiskModeTmpfs}
	spec.Resources.MemoryBytes = 1 << 30
	spec.Resources.DiskBytes = 1 << 30
	spec.Resources.Pids = 512
	spec.Resources.CPUMillis = 1000
	return spec, layout{agentPath: "/usr/bin/dhow-runner", controlDir: "/state/attempts/a/control"}
}

func TestVerifyRefusesASandboxThatDriftsFromThePolicy(t *testing.T) {
	tests := []struct {
		name   string
		breaks func(in *inspection)
		want   string
	}{
		{"an empty namespace mode", func(in *inspection) { in.HostConfig.PidMode = "" }, "PID namespace"},
		{"no-new-privileges turned off", func(in *inspection) { in.HostConfig.SecurityOpt = []string{"no-new-privileges=false"} }, "no-new-privileges"},
		{"a crun annotation from the image", func(in *inspection) {
			in.Config.Annotations = map[string]string{"run.oci.keep_original_groups": "1"}
		}, "run.oci.keep_original_groups"},
		{"a wasm handler annotation", func(in *inspection) {
			in.HostConfig.Annotations["module.wasm.image/variant"] = "compat"
		}, "module.wasm.image/variant"},
		{"GODEBUG from the image", func(in *inspection) { in.Config.Env = append(in.Config.Env, "GODEBUG=x") }, "GODEBUG"},
		{"LD_PRELOAD from the image", func(in *inspection) { in.Config.Env = append(in.Config.Env, "LD_PRELOAD=/x.so") }, "LD_PRELOAD"},
		{"the agent from another file", func(in *inspection) { in.Mounts[0].Source = "/tmp/evil" }, "unexpected source"},
		{"an extra bind mount", func(in *inspection) {
			in.Mounts = append(in.Mounts, in.Mounts[0])
			in.Mounts[len(in.Mounts)-1].Destination = "/run/secrets"
		}, "unexpected mount"},
		{"an executable secrets tmpfs", func(in *inspection) {
			in.HostConfig.Tmpfs[secretsMount] = "rw,size=4194304,mode=0700,nosuid,nodev"
		}, "secrets tmpfs"},
		{"a /tmp that allows setuid", func(in *inspection) { in.HostConfig.Tmpfs[tmpPath] = "rw,nodev" }, "/tmp tmpfs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, l := testSpec()
			in := goodInspection(spec, l)
			if err := verify(in, spec, l); err != nil {
				t.Fatalf("the baseline must pass: %v", err)
			}
			tt.breaks(in)
			err := verify(in, spec, l)
			if !errors.Is(err, ErrPolicyMismatch) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want a policy mismatch about %q, got %v", tt.want, err)
			}
		})
	}
}

func TestOnlyQualifiedImagesArePulled(t *testing.T) {
	tests := []struct {
		image string
		want  bool
	}{
		{"ghcr.io/org/app@sha256:" + strings.Repeat("a", 64), true},
		{"localhost/dhow-isolation:test", true},
		{"registry:5000/app@sha256:" + strings.Repeat("a", 64), true},
		{"alpine@sha256:" + strings.Repeat("a", 64), false},
		{"containers-storage:alpine@sha256:" + strings.Repeat("a", 64), false},
		{"docker-daemon:x@sha256:" + strings.Repeat("a", 64), false},
		{"oci-archive:foo@sha256:" + strings.Repeat("a", 64), false},
	}
	for _, tt := range tests {
		if got := qualified(tt.image); got != tt.want {
			t.Errorf("qualified(%q) = %v, want %v", tt.image, got, tt.want)
		}
	}
}

func TestTheSandboxIsCreatedFromTheImageIDWithoutImageEnv(t *testing.T) {
	spec, l := testSpec()
	id := strings.Repeat("b", 64)
	args := createArgs(spec, l, "/seccomp.json", id)
	if args[len(args)-1] != id || args[len(args)-2] != "--" {
		t.Fatalf("the image ID must be the last argument after --, got %v", args[len(args)-2:])
	}
	found := false
	for _, a := range args {
		if a == "--unsetenv-all" {
			found = true
		}
	}
	if !found {
		t.Fatal("the image environment must not reach the agent (--unsetenv-all)")
	}
}

func TestADynamicallyLinkedAgentIsRefused(t *testing.T) {
	f, err := elf.Open("/bin/sh")
	if err != nil {
		t.Skip("no ELF /bin/sh on this host")
	}
	dynamic := false
	for _, p := range f.Progs {
		dynamic = dynamic || p.Type == elf.PT_INTERP
	}
	_ = f.Close()
	if !dynamic {
		t.Skip("/bin/sh is static on this host")
	}
	if err := staticAgent("/bin/sh"); !errors.Is(err, ErrDynamicAgent) {
		t.Fatalf("want ErrDynamicAgent, got %v", err)
	}
}
