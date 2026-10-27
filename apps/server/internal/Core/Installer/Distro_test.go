package installer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOSRelease(t *testing.T) {
	tests := []struct {
		name      string
		release   string
		id        string
		family    Family
		supported bool
		codename  string
	}{
		{"debian 12", debianRelease, "debian", FamilyApt, true, "bookworm"},
		{"debian 11", "ID=debian\nVERSION_ID=\"11\"\nVERSION_CODENAME=bullseye\n", "debian", FamilyApt, false, "bullseye"},
		{"ubuntu 24.04", "ID=ubuntu\nVERSION_ID=\"24.04\"\nVERSION_CODENAME=noble\n", "ubuntu", FamilyApt, true, "noble"},
		{"ubuntu 22.04", "ID=ubuntu\nVERSION_ID=\"22.04\"\nVERSION_CODENAME=jammy\n", "ubuntu", FamilyApt, true, "jammy"},
		{"ubuntu 20.04", "ID=ubuntu\nVERSION_ID=\"20.04\"\nVERSION_CODENAME=focal\n", "ubuntu", FamilyApt, false, "focal"},
		{"ubuntu without codename", "ID=ubuntu\nVERSION_ID=\"24.04\"\n", "ubuntu", FamilyApt, false, ""},
		{"rocky 9.4", rockyRelease, "rocky", FamilyDnf, true, ""},
		{"alma 9", "ID=\"almalinux\"\nVERSION_ID=\"9.5\"\n", "almalinux", FamilyDnf, true, ""},
		{"rhel 9", "ID=\"rhel\"\nVERSION_ID=\"9.4\"\n", "rhel", FamilyDnf, true, ""},
		{"rhel 8", "ID=\"rhel\"\nVERSION_ID=\"8.10\"\n", "rhel", FamilyDnf, false, ""},
		{"fedora 43", "ID=fedora\nVERSION_ID=43\n", "fedora", FamilyDnf, true, ""},
		{"fedora 39", "ID=fedora\nVERSION_ID=39\n", "fedora", FamilyDnf, false, ""},
		{"arch", "ID=arch\nBUILD_ID=rolling\n", "arch", FamilyUnknown, false, ""},
		{"comments and blanks", "# x\n\nID='debian'\nVERSION_ID='12'\n", "debian", FamilyApt, true, "bookworm"},
		{"empty", "", "", FamilyUnknown, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := ParseOSRelease([]byte(tt.release))
			assert.Equal(t, tt.id, d.ID)
			assert.Equal(t, tt.family, d.Family)
			assert.Equal(t, tt.supported, d.Supported)
			assert.Equal(t, tt.codename, d.Codename)
		})
	}
}

func TestDistroPackages(t *testing.T) {
	deb := ParseOSRelease([]byte(debianRelease))
	rocky := ParseOSRelease([]byte(rockyRelease))
	rhel := ParseOSRelease([]byte("ID=rhel\nVERSION_ID=9.4\n"))
	jammy := ParseOSRelease([]byte("ID=ubuntu\nVERSION_ID=22.04\nVERSION_CODENAME=jammy\n"))
	assert.Equal(t, "postgresql-18", deb.PostgresPackage())
	assert.Equal(t, "postgresql18-server", rocky.PostgresPackage())
	assert.Equal(t, "postgresql-18", rocky.PostgresService())
	assert.Equal(t, "postgresql", deb.PostgresService())
	assert.True(t, rocky.NeedsEPEL())
	assert.False(t, rhel.NeedsEPEL())
	assert.False(t, rhel.ExtraPackagesInstallable())
	assert.False(t, jammy.CaddyInstallable())
	assert.True(t, jammy.ExtraPackagesInstallable())
	assert.True(t, deb.CaddyInstallable())
}

const procNetTCP = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 1 1 0 100 0 0 10 0
   1: 0100007F:1538 00000000:0000 0A 00000000:00000000 00:00000000 00000000   999        0 2 1 0 100 0 0 10 0
   2: 0100007F:1F90 0100007F:D2A4 01 00000000:00000000 00:00000000 00000000     0        0 3 1 0 20 4 30 10 -1
   3: garbage
`

func TestParseListening(t *testing.T) {
	assert.Equal(t, []int{8080, 5432}, ParseListening([]byte(procNetTCP)))
	assert.Empty(t, ParseListening(nil))
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name  string
		setup func(f *fakeSys)
		check func(t *testing.T, d Detection)
	}{
		{"bare debian", func(*fakeSys) {}, func(t *testing.T, d Detection) {
			assert.True(t, d.Distro.Supported)
			assert.True(t, d.Systemd)
			assert.True(t, d.Root)
			assert.False(t, d.Docker)
			assert.Empty(t, d.Postgres.Versions)
			assert.True(t, d.CanInstallPostgres())
			assert.Equal(t, FirewallNone, d.Firewall)
			assert.False(t, d.Existing)
		}},
		{"docker without compose", func(f *fakeSys) {
			f.bins["docker"] = true
			f.ok["docker info --format {{.ServerVersion}}"] = true
		}, func(t *testing.T, d Detection) {
			assert.True(t, d.Docker)
			assert.False(t, d.Compose)
		}},
		{"docker with compose", func(f *fakeSys) {
			f.bins["docker"] = true
			f.ok["docker info --format {{.ServerVersion}}"] = true
			f.ok["docker compose version"] = true
		}, func(t *testing.T, d Detection) { assert.True(t, d.Compose) }},
		{"postgres 16 listening", func(f *fakeSys) {
			f.put("/usr/lib/postgresql/16/bin/postgres", "", 0o755)
			f.put("/proc/net/tcp", procNetTCP, 0o444)
		}, func(t *testing.T, d Detection) {
			assert.Equal(t, []int{16}, d.Postgres.Versions)
			assert.True(t, d.Postgres.Listening)
			assert.False(t, d.CanInstallPostgres())
			assert.True(t, d.BusyPort[8080])
		}},
		{"pgdg 18 on rhel layout and distro postgres", func(f *fakeSys) {
			f.put("/usr/pgsql-18/bin/postgres", "", 0o755)
			f.bins["postgres"] = true
			f.stdout["postgres --version"] = "postgres (PostgreSQL) 15.8\n"
			f.put("/proc/net/tcp6", procNetTCP, 0o444)
		}, func(t *testing.T, d Detection) {
			assert.Equal(t, []int{15, 18}, d.Postgres.Versions)
			assert.True(t, d.Postgres.Installed18)
			assert.True(t, d.CanInstallPostgres())
		}},
		{"ufw active", func(f *fakeSys) {
			f.bins["ufw"] = true
			f.stdout["ufw status"] = "Status: active\n"
		}, func(t *testing.T, d Detection) { assert.Equal(t, FirewallUfw, d.Firewall) }},
		{"ufw inactive and firewalld running", func(f *fakeSys) {
			f.bins["ufw"] = true
			f.stdout["ufw status"] = "Status: inactive\n"
			f.bins["firewall-cmd"] = true
			f.stdout["firewall-cmd --state"] = "running\n"
		}, func(t *testing.T, d Detection) { assert.Equal(t, FirewallFirewalld, d.Firewall) }},
		{"selinux, proxies and an earlier install", func(f *fakeSys) {
			f.bins["getenforce"] = true
			f.stdout["getenforce"] = "Enforcing\n"
			f.bins["nginx"] = true
			f.bins["caddy"] = true
			f.bins["certbot"] = true
			f.put(UnitPath, "x", 0o644)
		}, func(t *testing.T, d Detection) {
			assert.True(t, d.SELinux)
			assert.True(t, d.Nginx && d.Caddy && d.Certbot)
			assert.True(t, d.Existing)
		}},
		{"not root, no systemd", func(f *fakeSys) {
			f.root = false
			delete(f.files, "/run/systemd/system")
		}, func(t *testing.T, d Detection) {
			assert.False(t, d.Root)
			assert.False(t, d.Systemd)
			assert.False(t, d.PackagesInstallable())
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := debianHost()
			tt.setup(f)
			tt.check(t, Detect(context.Background(), f))
		})
	}
}

func TestParseServerMajor(t *testing.T) {
	for in, want := range map[string]int{"18.1": 18, "18beta1": 0, "17.6 (Debian 17.6-1.pgdg120+1)": 17, "9.6.24": 9} {
		got, err := ParseServerMajor(in)
		if want == 0 {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
}
