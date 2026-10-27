package installer

import (
	"bufio"
	"bytes"
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	FirewallNone      = ""
	FirewallUfw       = "ufw"
	FirewallFirewalld = "firewalld"
	postgresPort      = 5432
	tcpListenState    = "0A"
)

var (
	WatchedPorts      = []int{80, 443, 8080}
	postgresVersionRe = regexp.MustCompile(`(\d+)(?:\.\d+)?`)
	postgresBinGlobs  = []string{"/usr/lib/postgresql/*/bin/postgres", "/usr/pgsql-*/bin/postgres"}
)

type PostgresInfo struct {
	Versions    []int
	Installed18 bool
	Listening   bool
}

type Detection struct {
	Distro   Distro
	Systemd  bool
	Root     bool
	Sudo     bool
	Docker   bool
	Compose  bool
	Postgres PostgresInfo
	Nginx    bool
	Caddy    bool
	Certbot  bool
	BusyPort map[int]bool
	Firewall string
	SELinux  bool
	Existing bool
}

func Detect(ctx context.Context, sys System) Detection {
	d := Detection{BusyPort: map[int]bool{}}
	if data, err := sys.ReadFile(OSReleasePath); err == nil {
		d.Distro = ParseOSRelease(data)
	}
	d.Systemd = exists(sys, "/run/systemd/system")
	d.Root = sys.IsRoot()
	d.Sudo = sys.LookPath("sudo")
	d.Docker = sys.LookPath("docker") && succeeds(ctx, sys, Cmd("docker", "info", "--format", "{{.ServerVersion}}"))
	d.Compose = d.Docker && succeeds(ctx, sys, Cmd("docker", "compose", "version"))
	d.Nginx = sys.LookPath("nginx")
	d.Caddy = sys.LookPath("caddy")
	d.Certbot = sys.LookPath("certbot")
	for _, p := range listeningPorts(sys) {
		d.BusyPort[p] = true
	}
	d.Postgres = detectPostgres(ctx, sys, d.BusyPort[postgresPort])
	d.Firewall = detectFirewall(ctx, sys)
	d.SELinux = sys.LookPath("getenforce") && outputHas(ctx, sys, Cmd("getenforce"), "Enforcing")
	d.Existing = exists(sys, UnitPath) || exists(sys, AnswersPath) || exists(sys, ComposeEnvPath)
	return d
}

func outputHas(ctx context.Context, sys System, c Command, want string) bool {
	out, err := sys.Run(ctx, c)
	return err == nil && strings.Contains(string(out.Stdout), want)
}

func detectFirewall(ctx context.Context, sys System) string {
	if sys.LookPath("ufw") && outputHas(ctx, sys, Cmd("ufw", "status"), "Status: active") {
		return FirewallUfw
	}
	if sys.LookPath("firewall-cmd") && outputHas(ctx, sys, Cmd("firewall-cmd", "--state"), "running") {
		return FirewallFirewalld
	}
	return FirewallNone
}

func detectPostgres(ctx context.Context, sys System, listening bool) PostgresInfo {
	info := PostgresInfo{Listening: listening}
	for _, pattern := range postgresBinGlobs {
		matches, err := sys.Glob(pattern)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if v := versionFromPath(m); v > 0 {
				info.Versions = append(info.Versions, v)
			}
		}
	}
	if sys.LookPath("postgres") {
		if out, err := sys.Run(ctx, Cmd("postgres", "--version")); err == nil {
			if m := postgresVersionRe.FindStringSubmatch(string(out.Stdout)); m != nil {
				if v, err := strconv.Atoi(m[1]); err == nil {
					info.Versions = append(info.Versions, v)
				}
			}
		}
	}
	slices.Sort(info.Versions)
	info.Versions = slices.Compact(info.Versions)
	info.Installed18 = slices.Contains(info.Versions, RequiredPostgres)
	return info
}

func versionFromPath(p string) int {
	for _, part := range strings.Split(p, "/") {
		part = strings.TrimPrefix(part, "pgsql-")
		if n, err := strconv.Atoi(part); err == nil {
			return n
		}
	}
	return 0
}

func listeningPorts(sys System) []int {
	var out []int
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := sys.ReadFile(f)
		if err != nil {
			continue
		}
		out = append(out, ParseListening(data)...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func ParseListening(data []byte) []int {
	var out []int
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 || fields[3] != tcpListenState {
			continue
		}
		_, portHex, ok := strings.Cut(fields[1], ":")
		if !ok {
			continue
		}
		port, err := strconv.ParseUint(portHex, 16, 16)
		if err != nil {
			continue
		}
		out = append(out, int(port))
	}
	return out
}

func (d Detection) PackagesInstallable() bool {
	return d.Distro.Supported && d.Systemd
}

func (d Detection) CanInstallPostgres() bool {
	return d.Postgres.Installed18 || (d.Distro.Supported && !d.Postgres.Listening)
}
