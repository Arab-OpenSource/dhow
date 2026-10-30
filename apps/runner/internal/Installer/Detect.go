package installer

import (
	"bufio"
	"bytes"
	"context"
	"math"
	"strconv"
	"strings"

	engine "github.com/Khaymah/dhow/internal/installer"
)

type Creds int

const (
	CredsNone Creds = iota
	CredsHost
	CredsTPM2
)

type Detection struct {
	Distro       engine.OSRelease
	Systemd      bool
	Root         bool
	Podman       string
	Crun         bool
	Newuidmap    bool
	CgroupV2     bool
	UserNS       bool
	AppArmorNS   bool
	Creds        Creds
	User         bool
	UserCreated  bool
	UID          int
	GID          int
	SubUIDs      int
	SubGIDs      int
	PlainID      bool
	SealedID     bool
	Installed    bool
	AnalyzeTools bool
	MemoryBytes  int64
}

func (d Detection) Registered() bool { return d.PlainID || d.SealedID }

func Detect(ctx context.Context, sys engine.System) Detection {
	d := Detection{UID: -1, GID: -1}
	if data, err := sys.ReadFile(engine.OSReleasePath); err == nil {
		d.Distro = engine.ParseOSRelease(data)
	}
	d.Systemd = engine.Exists(sys, SystemdRunDir)
	d.Root = sys.IsRoot()
	d.Podman = podmanVersion(ctx, sys)
	d.Crun = sys.LookPath("crun")
	d.Newuidmap = sys.LookPath("newuidmap") && sys.LookPath("newgidmap")
	d.CgroupV2 = engine.Exists(sys, CgroupControllers)
	d.UserNS = readInt(sys, MaxUserNamespaces) > 0
	d.AppArmorNS = readInt(sys, AppArmorUserns) == 1
	d.Creds = detectCreds(ctx, sys)
	d.UID, d.GID, d.UserCreated, d.User = lookupUser(ctx, sys, ServiceUser)
	d.SubUIDs = subIDCount(sys, SubUIDPath)
	d.SubGIDs = subIDCount(sys, SubGIDPath)
	d.PlainID = engine.Exists(sys, IdentityPath)
	d.SealedID = engine.Exists(sys, SealedPath)
	d.Installed = engine.Exists(sys, UnitPath) || engine.Exists(sys, EnvFilePath) || engine.Exists(sys, AnswersPath)
	d.AnalyzeTools = sys.LookPath("systemd-analyze")
	d.MemoryBytes = int64(min(engine.ReadMemTotal(sys), math.MaxInt64))
	return d
}

func podmanVersion(ctx context.Context, sys engine.System) string {
	if !sys.LookPath("podman") {
		return ""
	}
	out, err := sys.Run(ctx, engine.Cmd("podman", "version", "--format", "{{.Client.Version}}"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out.Stdout))
}

func detectCreds(ctx context.Context, sys engine.System) Creds {
	if !sys.LookPath("systemd-creds") {
		return CredsNone
	}
	probe := engine.Command{Name: "systemd-creds", Args: []string{"encrypt", "--name=dhow-probe", "-", "-"}, Stdin: []byte("dhow probe\n")}
	if _, err := sys.Run(ctx, probe); err != nil {
		return CredsNone
	}
	if engine.Succeeds(ctx, sys, engine.Cmd("systemd-creds", "has-tpm2", "--quiet")) {
		return CredsTPM2
	}
	return CredsHost
}

func readInt(sys engine.System, path string) int {
	data, err := sys.ReadFile(path)
	if err != nil {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return -1
	}
	return n
}

func lookupUser(ctx context.Context, sys engine.System, name string) (int, int, bool, bool) {
	out, err := sys.Run(ctx, engine.Cmd("getent", "passwd", name))
	if err != nil {
		return -1, -1, false, false
	}
	fields := strings.Split(strings.TrimSpace(string(out.Stdout)), ":")
	if len(fields) < 5 {
		return -1, -1, false, true
	}
	created := fields[4] == UserComment
	uid, uerr := strconv.Atoi(fields[2])
	gid, gerr := strconv.Atoi(fields[3])
	if uerr != nil || gerr != nil {
		return -1, -1, created, true
	}
	return uid, gid, created, true
}

type SubIDRangeEntry struct {
	Owner string
	Start int
	Count int
}

func ParseSubIDs(data []byte) []SubIDRangeEntry {
	var out []SubIDRangeEntry
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		parts := strings.Split(strings.TrimSpace(sc.Text()), ":")
		if len(parts) != 3 {
			continue
		}
		start, err1 := strconv.Atoi(parts[1])
		count, err2 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || count <= 0 || start < 0 {
			continue
		}
		out = append(out, SubIDRangeEntry{Owner: parts[0], Start: start, Count: count})
	}
	return out
}

func subIDCount(sys engine.System, path string) int {
	data, err := sys.ReadFile(path)
	if err != nil {
		return 0
	}
	total := 0
	for _, r := range ParseSubIDs(data) {
		if r.Owner == ServiceUser {
			total += r.Count
		}
	}
	return total
}

func FreeSubIDStart(files ...[]byte) int {
	start := SubIDFloor
	for _, data := range files {
		for _, r := range ParseSubIDs(data) {
			start = max(start, r.Start+r.Count)
		}
	}
	if rem := start % SubIDsPerJob; rem != 0 {
		start += SubIDsPerJob - rem
	}
	return start
}

func PodmanAtLeast(have, want string) bool {
	h, w := versionParts(have), versionParts(want)
	for i := range 3 {
		if h[i] != w[i] {
			return h[i] > w[i]
		}
	}
	return true
}

func versionParts(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	for i, p := range strings.SplitN(v, ".", 3) {
		if end := strings.IndexFunc(p, func(r rune) bool { return r < '0' || r > '9' }); end >= 0 {
			p = p[:end]
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return out
		}
		out[i] = n
	}
	return out
}
