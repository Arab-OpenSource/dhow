package installer

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

type Family string

const (
	FamilyApt     Family = "apt"
	FamilyDnf     Family = "dnf"
	FamilyUnknown Family = ""
)

const minFedora = 41

type Distro struct {
	ID         string
	VersionID  string
	Codename   string
	PrettyName string
	Family     Family
	Supported  bool
	EL         bool
	Fedora     bool
}

func ParseOSRelease(data []byte) Distro {
	fields := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		fields[k] = unquote(strings.TrimSpace(v))
	}
	d := Distro{
		ID:         strings.ToLower(fields["ID"]),
		VersionID:  fields["VERSION_ID"],
		Codename:   fields["VERSION_CODENAME"],
		PrettyName: fields["PRETTY_NAME"],
	}
	if d.PrettyName == "" {
		d.PrettyName = strings.TrimSpace(d.ID + " " + d.VersionID)
	}
	classify(&d)
	return d
}

func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		v = v[1 : len(v)-1]
	}
	return strings.ReplaceAll(v, `\"`, `"`)
}

func classify(d *Distro) {
	major := majorVersion(d.VersionID)
	switch d.ID {
	case "debian":
		d.Family = FamilyApt
		d.Supported = d.VersionID == "12"
		if d.Codename == "" && d.Supported {
			d.Codename = "bookworm"
		}
	case "ubuntu":
		d.Family = FamilyApt
		d.Supported = d.VersionID == "22.04" || d.VersionID == "24.04"
	case "rhel", "rocky", "almalinux":
		d.Family = FamilyDnf
		d.EL = true
		d.Supported = major == 9
	case "fedora":
		d.Family = FamilyDnf
		d.Fedora = true
		d.Supported = major >= minFedora
	}
	if d.Family == FamilyApt && d.Codename == "" {
		d.Supported = false
	}
}

func majorVersion(v string) int {
	head, _, _ := strings.Cut(v, ".")
	n, err := strconv.Atoi(head)
	if err != nil {
		return 0
	}
	return n
}

func (d Distro) Major() int { return majorVersion(d.VersionID) }

func (d Distro) PostgresPackage() string {
	if d.Family == FamilyDnf {
		return "postgresql18-server"
	}
	return "postgresql-18"
}

func (d Distro) PostgresService() string {
	if d.Family == FamilyDnf {
		return "postgresql-18"
	}
	return "postgresql"
}

func (d Distro) NologinShell() string { return "/usr/sbin/nologin" }

func (d Distro) CaddyInstallable() bool {
	return d.ExtraPackagesInstallable() && (d.ID != "ubuntu" || d.VersionID != "22.04")
}

func (d Distro) ExtraPackagesInstallable() bool { return d.Supported && d.ID != "rhel" }

func (d Distro) NeedsEPEL() bool { return d.EL && d.ID != "rhel" }
