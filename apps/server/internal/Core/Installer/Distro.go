package installer

import (
	engine "github.com/Khaymah/dhow/internal/installer"
)

type Family = engine.Family

const (
	FamilyApt     = engine.FamilyApt
	FamilyDnf     = engine.FamilyDnf
	FamilyUnknown = engine.FamilyUnknown
)

const minFedora = 41

type Distro struct {
	engine.OSRelease
	Supported bool
}

func ParseOSRelease(data []byte) Distro {
	d := Distro{OSRelease: engine.ParseOSRelease(data)}
	classify(&d)
	return d
}

func classify(d *Distro) {
	major := d.Major()
	switch d.ID {
	case "debian":
		d.Supported = d.VersionID == "12"
		if d.Codename == "" && d.Supported {
			d.Codename = "bookworm"
		}
	case "ubuntu":
		d.Supported = d.VersionID == "22.04" || d.VersionID == "24.04"
	case "rhel", "rocky", "almalinux":
		d.Supported = major == 9
	case "fedora":
		d.Supported = major >= minFedora
	}
	if d.Family == FamilyApt && d.Codename == "" {
		d.Supported = false
	}
}

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

func (d Distro) CaddyInstallable() bool {
	return d.ExtraPackagesInstallable() && (d.ID != "ubuntu" || d.VersionID != "22.04")
}

func (d Distro) ExtraPackagesInstallable() bool { return d.Supported && d.ID != "rhel" }

func (d Distro) NeedsEPEL() bool { return d.EL && d.ID != "rhel" }
