package podman

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	MinPodmanMajor = 5
	MinPodmanMinor = 4
	cgroupRoot     = "/sys/fs/cgroup"
)

var RequiredControllers = []string{"cpu", "io", "memory", "pids"}

type Check struct {
	Name   string
	OK     bool
	Detail string
}

type PreflightError struct{ Failed []Check }

func (e *PreflightError) Error() string {
	parts := make([]string, 0, len(e.Failed))
	for _, c := range e.Failed {
		parts = append(parts, c.Name+": "+c.Detail)
	}
	return "the host is not ready for the rootless Podman sandbox: " + strings.Join(parts, "; ")
}

func Preflight(ctx context.Context, binary string) []Check {
	return []Check{
		checkPodmanVersion(ctx, binary),
		checkCgroupV2(),
		checkDelegation(),
		checkSubuid(),
		checkUserNamespaces(),
	}
}

func failed(checks []Check) []Check {
	var out []Check
	for _, c := range checks {
		if !c.OK {
			out = append(out, c)
		}
	}
	return out
}

func checkPodmanVersion(ctx context.Context, binary string) Check {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := runner{binary: binary}.run(cctx, "version", "--format", "{{.Client.Version}}")
	if err != nil {
		return Check{Name: "podman", Detail: err.Error()}
	}
	v := strings.TrimSpace(string(out))
	major, minor, perr := parseVersion(v)
	if perr != nil {
		return Check{Name: "podman", Detail: perr.Error()}
	}
	if major < MinPodmanMajor || (major == MinPodmanMajor && minor < MinPodmanMinor) {
		return Check{Name: "podman", Detail: fmt.Sprintf("Podman %s is older than the required %d.%d", v, MinPodmanMajor, MinPodmanMinor)}
	}
	return Check{Name: "podman", OK: true, Detail: v}
}

func parseVersion(v string) (int, int, error) {
	fields := strings.SplitN(strings.TrimSpace(v), ".", 3)
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("could not read a Podman version from %q", v)
	}
	major, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("could not read the Podman major version from %q", v)
	}
	minor, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("could not read the Podman minor version from %q", v)
	}
	return major, minor, nil
}

func checkCgroupV2() Check {
	var st unix.Statfs_t
	if err := unix.Statfs(cgroupRoot, &st); err != nil {
		return Check{Name: "cgroup2", Detail: fmt.Sprintf("read %s: %v", cgroupRoot, err)}
	}
	if st.Type != unix.CGROUP2_SUPER_MAGIC {
		return Check{Name: "cgroup2", Detail: fmt.Sprintf("%s is not a cgroup v2 mount; the runner needs a unified cgroup v2 hierarchy", cgroupRoot)}
	}
	return Check{Name: "cgroup2", OK: true}
}

func ownCgroupPath() (string, error) {
	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "0::") {
			return strings.TrimPrefix(line, "0::"), nil
		}
	}
	return "", errors.New("no cgroup v2 entry in /proc/self/cgroup")
}

func checkDelegation() Check {
	path, err := ownCgroupPath()
	if err != nil {
		return Check{Name: "cgroup-delegation", Detail: err.Error()}
	}
	controllers, err := os.ReadFile(cgroupRoot + path + "/cgroup.controllers") //nolint:gosec // The path comes from the runner's own /proc/self/cgroup, not from a job.
	if err != nil {
		return Check{Name: "cgroup-delegation", Detail: fmt.Sprintf("read the delegated controllers: %v", err)}
	}
	have := map[string]bool{}
	for _, c := range strings.Fields(string(controllers)) {
		have[c] = true
	}
	var missing []string
	for _, c := range RequiredControllers {
		if !have[c] {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return Check{Name: "cgroup-delegation", Detail: fmt.Sprintf("the service is missing delegated controllers %s; run dhow-runner under systemd with Delegate=yes", strings.Join(missing, ", "))}
	}
	return Check{Name: "cgroup-delegation", OK: true}
}

func checkSubuid() Check {
	u, err := user.Current()
	if err != nil {
		return Check{Name: "subuid", Detail: err.Error()}
	}
	for _, file := range []string{"/etc/subuid", "/etc/subgid"} {
		count, err := subidCount(file, u)
		if err != nil {
			return Check{Name: "subuid", Detail: err.Error()}
		}
		if count < minSubidRange {
			return Check{Name: "subuid", Detail: fmt.Sprintf("%s gives the runner user only %d sub-ids; it needs at least %d", file, count, minSubidRange)}
		}
	}
	return Check{Name: "subuid", OK: true}
}

const minSubidRange = 65536

func subidCount(file string, u *user.User) (uint64, error) {
	f, err := os.Open(file) //nolint:gosec // file is one of the two fixed subid paths, never a job value.
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", file, err)
	}
	defer func() { _ = f.Close() }()
	var total uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Split(sc.Text(), ":")
		if len(parts) != 3 {
			continue
		}
		if parts[0] != u.Username && parts[0] != u.Uid {
			continue
		}
		n, err := strconv.ParseUint(parts[2], 10, 64)
		if err != nil {
			continue
		}
		total += n
	}
	return total, nil
}

func checkUserNamespaces() Check {
	raw, err := os.ReadFile("/proc/sys/user/max_user_namespaces")
	if err != nil {
		return Check{Name: "user-namespaces", Detail: fmt.Sprintf("read the user namespace limit: %v", err)}
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return Check{Name: "user-namespaces", Detail: err.Error()}
	}
	if n <= 0 {
		return Check{Name: "user-namespaces", Detail: "user namespaces are disabled (user.max_user_namespaces is 0)"}
	}
	return Check{Name: "user-namespaces", OK: true}
}
