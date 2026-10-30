package podman

import (
	"fmt"
	"strconv"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	sandbox "github.com/Khaymah/dhow/apps/runner/internal/Sandbox"
)

const (
	agentMount    = "/run/dhow/agent"
	controlMount  = "/run/dhow/control"
	connectMount  = "/run/dhow/bin/connect"
	secretsMount  = "/run/dhow/secrets"
	workspacePath = "/workspace"
	tmpPath       = "/tmp"
	secretsTmpfs  = 4 << 20
	shmBytes      = 64 << 20
	nofileLimit   = 4096
	entrypoint    = `["` + agentMount + `","__agent"]`
)

type layout struct {
	name       string
	agentPath  string
	controlDir string
	workDir    string
}

func createArgs(spec sandbox.PrepareSpec, l layout, seccompFile, imageID string) []string {
	r := spec.Resources
	mem := r.MemoryBytes
	diskMount := tmpfsMount(workspacePath, r.DiskBytes, "tmpfs-mode=0755,nosuid,nodev,U")
	if spec.DiskMode == config.DiskModeTmpfs {
		mem += r.DiskBytes
	} else {
		diskMount = fmt.Sprintf("type=bind,src=%s,dst=%s,relabel=private,U=true", l.workDir, workspacePath)
	}
	args := []string{
		"create",
		"--name", l.name,
		"--label", sandbox.LabelRunner + "=" + spec.RunnerID,
		"--label", sandbox.LabelAttempt + "=" + spec.AttemptID,
		"--pull=never",
		"--userns=auto:size=65536",
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges",
		"--security-opt=seccomp=" + seccompFile,
		"--read-only",
		"--read-only-tmpfs=false",
		"--ipc=private",
		"--uts=private",
		"--cgroupns=private",
		"--cgroups=split",
		"--pid=private",
		"--network=none",
		"--no-hosts",
		"--log-driver=none",
		"--init=false",
		"--http-proxy=false",
		"--unsetenv-all",
		"--ulimit=nofile=" + strconv.Itoa(nofileLimit) + ":" + strconv.Itoa(nofileLimit),
		"--ulimit=core=0:0",
		"--shm-size=" + strconv.Itoa(shmBytes),
		"--cpus=" + cpus(r.CPUMillis),
		"--memory=" + strconv.FormatInt(mem, 10),
		"--memory-swap=" + strconv.FormatInt(mem, 10),
		"--pids-limit=" + strconv.FormatInt(r.Pids, 10),
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=%s,ro=true", l.agentPath, agentMount),
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=%s,ro=true", l.agentPath, connectMount),
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=%s,ro=true,relabel=private,U=true", l.controlDir, controlMount),
		"--mount", diskMount,
		"--mount", tmpfsMount(tmpPath, r.DiskBytes, "tmpfs-mode=1777,nosuid,nodev"),
		"--mount", tmpfsMount(secretsMount, secretsTmpfs, "tmpfs-mode=0700,noexec,nosuid,nodev,U"),
		"--workdir=" + workspacePath,
		"--entrypoint=" + entrypoint,
		"--", imageID,
	}
	return args
}

func tmpfsMount(dst string, size int64, opts string) string {
	return fmt.Sprintf("type=tmpfs,dst=%s,tmpfs-size=%d,%s", dst, size, opts)
}

func cpus(millis int64) string {
	return strconv.FormatFloat(float64(millis)/float64(config.MillisPerCPU), 'f', 3, 64)
}
