package installer

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

const OSReleasePath = "/etc/os-release"

type Family string

const (
	FamilyApt     Family = "apt"
	FamilyDnf     Family = "dnf"
	FamilyUnknown Family = ""
)

type OSRelease struct {
	ID         string
	VersionID  string
	Codename   string
	PrettyName string
	Family     Family
	EL         bool
	Fedora     bool
}

func ParseOSRelease(data []byte) OSRelease {
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
		fields[k] = Unquote(strings.TrimSpace(v))
	}
	d := OSRelease{
		ID:         strings.ToLower(fields["ID"]),
		VersionID:  fields["VERSION_ID"],
		Codename:   fields["VERSION_CODENAME"],
		PrettyName: fields["PRETTY_NAME"],
	}
	if d.PrettyName == "" {
		d.PrettyName = strings.TrimSpace(d.ID + " " + d.VersionID)
	}
	switch d.ID {
	case "debian", "ubuntu":
		d.Family = FamilyApt
	case "rhel", "rocky", "almalinux":
		d.Family = FamilyDnf
		d.EL = true
	case "fedora":
		d.Family = FamilyDnf
		d.Fedora = true
	}
	return d
}

func Unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		v = v[1 : len(v)-1]
	}
	return strings.ReplaceAll(v, `\"`, `"`)
}

func MajorVersion(v string) int {
	head, _, _ := strings.Cut(v, ".")
	n, err := strconv.Atoi(head)
	if err != nil {
		return 0
	}
	return n
}

func (d OSRelease) Major() int { return MajorVersion(d.VersionID) }

func (d OSRelease) NologinShell() string { return "/usr/sbin/nologin" }
