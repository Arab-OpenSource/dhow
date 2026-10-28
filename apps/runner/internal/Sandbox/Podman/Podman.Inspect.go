package podman

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	sandbox "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox"
)

var ErrPolicyMismatch = errors.New("the created sandbox does not match the required isolation policy")

type inspection struct {
	State struct {
		Running    bool   `json:"Running"`
		OOMKilled  bool   `json:"OOMKilled"`
		Status     string `json:"Status"`
		CgroupPath string `json:"CgroupPath"`
	} `json:"State"`
	EffectiveCaps []string `json:"EffectiveCaps"`
	BoundingCaps  []string `json:"BoundingCaps"`
	Mounts        []struct {
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		RW          bool   `json:"RW"`
	} `json:"Mounts"`
	HostConfig struct {
		CapDrop        []string          `json:"CapDrop"`
		SecurityOpt    []string          `json:"SecurityOpt"`
		ReadonlyRootfs bool              `json:"ReadonlyRootfs"`
		NetworkMode    string            `json:"NetworkMode"`
		IpcMode        string            `json:"IpcMode"`
		PidMode        string            `json:"PidMode"`
		UTSMode        string            `json:"UTSMode"`
		CgroupMode     string            `json:"CgroupMode"`
		Memory         int64             `json:"Memory"`
		MemorySwap     int64             `json:"MemorySwap"`
		PidsLimit      int64             `json:"PidsLimit"`
		NanoCpus       int64             `json:"NanoCpus"`
		Privileged     bool              `json:"Privileged"`
		Tmpfs          map[string]string `json:"Tmpfs"`
		Annotations    map[string]string `json:"Annotations"`
	} `json:"HostConfig"`
}

func inspect(ctx context.Context, r runner, name string) (*inspection, error) {
	out, err := r.run(ctx, "inspect", "--type=container", "--format", "json", name)
	if err != nil {
		return nil, err
	}
	var all []inspection
	if err := json.Unmarshal(out, &all); err != nil {
		return nil, fmt.Errorf("decode podman inspect: %w", err)
	}
	if len(all) != 1 {
		return nil, fmt.Errorf("podman inspect returned %d containers for %s", len(all), name)
	}
	return &all[0], nil
}

func verify(in *inspection, spec sandbox.PrepareSpec) error {
	var bad []string
	add := func(cond bool, msg string) {
		if !cond {
			bad = append(bad, msg)
		}
	}
	add(len(in.EffectiveCaps) == 0 && len(in.BoundingCaps) == 0, "the sandbox keeps capabilities")
	add(!in.HostConfig.Privileged, "the sandbox is privileged")
	add(hasOpt(in.HostConfig.SecurityOpt, "no-new-privileges"), "no-new-privileges is not set")
	add(in.HostConfig.ReadonlyRootfs, "the root filesystem is writable")
	add(in.HostConfig.NetworkMode == "none", "the network is not none")
	add(private(in.HostConfig.IpcMode), "the IPC namespace is shared")
	add(private(in.HostConfig.PidMode), "the PID namespace is shared")
	add(private(in.HostConfig.UTSMode), "the UTS namespace is shared")
	add(private(in.HostConfig.CgroupMode), "the cgroup namespace is shared")
	add(strings.HasPrefix(in.HostConfig.Annotations["io.podman.annotations.userns"], "auto"), "the user namespace is not isolated per job")
	verifyLimits(in, spec, add)
	verifyMounts(in, spec, add)
	if len(bad) > 0 {
		return fmt.Errorf("%w: %s", ErrPolicyMismatch, strings.Join(bad, "; "))
	}
	return nil
}

func verifyLimits(in *inspection, spec sandbox.PrepareSpec, add func(bool, string)) {
	mem := spec.Resources.MemoryBytes
	if spec.DiskMode == config.DiskModeTmpfs {
		mem += spec.Resources.DiskBytes
	}
	add(in.HostConfig.Memory == mem, "the memory limit is wrong")
	add(in.HostConfig.MemorySwap == mem, "swap is not disabled")
	add(in.HostConfig.PidsLimit == spec.Resources.Pids, "the process limit is wrong")
	add(in.HostConfig.NanoCpus == spec.Resources.CPUMillis*nanosPerMilliCPU, "the CPU limit is wrong")
}

const nanosPerMilliCPU = 1_000_000

func verifyMounts(in *inspection, spec sandbox.PrepareSpec, add func(bool, string)) {
	allowed := map[string]bool{agentMount: true, controlMount: true}
	if spec.DiskMode == config.DiskModeDirectory {
		allowed[workspacePath] = true
	}
	for _, m := range in.Mounts {
		if m.Type == "tmpfs" {
			continue
		}
		add(allowed[m.Destination], "an unexpected mount is present at "+m.Destination)
		if m.Destination == agentMount || m.Destination == controlMount {
			add(!m.RW, "a control mount is writable at "+m.Destination)
		}
	}
	add(hasTmpfs(in, workspacePath) || spec.DiskMode == config.DiskModeDirectory, "the workspace is missing")
	add(hasTmpfs(in, tmpPath), "the /tmp tmpfs is missing")
	add(hasTmpfs(in, secretsMount), "the secrets tmpfs is missing")
}

func hasTmpfs(in *inspection, dst string) bool {
	_, ok := in.HostConfig.Tmpfs[dst]
	return ok
}

func hasOpt(opts []string, want string) bool {
	for _, o := range opts {
		if o == want || strings.HasPrefix(o, want+"=") || strings.HasPrefix(o, want+":") {
			return true
		}
	}
	return false
}

func private(mode string) bool { return mode == "private" || mode == "" }
