package installer

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func answers(t *testing.T, yaml string) Answers {
	t.Helper()
	a, err := ParseAnswers(strings.NewReader(yaml))
	require.NoError(t, err)
	return a
}

func commands(p Plan) string {
	var out []string
	for _, s := range p.Steps {
		out = append(out, s.Commands...)
	}
	return strings.Join(out, "\n")
}

func TestMissingPackages(t *testing.T) {
	deb := Detection{Distro: ParseOSRelease([]byte(debianRelease))}
	rocky := Detection{Distro: ParseOSRelease([]byte(rockyRelease))}
	tests := []struct {
		name string
		det  Detection
		yaml string
		want []string
	}{
		{"debian caddy", deb, minimalAnswers, []string{"postgresql-18", "caddy"}},
		{"rocky nginx", rocky, "mode: native\ndomain: d.example.com\ndatabase: {mode: install}\nproxy: {kind: nginx, email: a@b.co}\n", []string{"postgresql18-server", "nginx", "certbot"}},
		{"existing everything", deb, "mode: native\ndomain: d.example.com\ndatabase: {mode: existing, url_file: /x}\nproxy: {kind: existing}\n", nil},
		{"compose needs no postgres", deb, "mode: compose\ndomain: d.example.com\ndatabase: {mode: install}\nproxy: {kind: none}\n", nil},
		{"already installed", Detection{Distro: deb.Distro, Caddy: true, Postgres: PostgresInfo{Installed18: true}}, minimalAnswers, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, MissingPackages(tt.det, answers(t, tt.yaml)))
		})
	}
}

func build(t *testing.T, f *fakeSys, yaml string, tweak func(d *Detection)) Plan {
	t.Helper()
	det := Detect(context.Background(), f)
	if tweak != nil {
		tweak(&det)
	}
	b := &Builder{Sys: f, Det: det, A: answers(t, yaml), M: NewMessages(LangEn), Version: "1.2.3", Exe: f.exe, Sleep: func(time.Duration) {}}
	return b.Build()
}

type planCase struct {
	name         string
	host         func() *fakeSys
	yaml         string
	tweak        func(d *Detection)
	wantBlockers []string
	wantCommands []string
	wantFiles    []string
	wantPackages []string
	absent       []string
}

func planCases() []planCase {
	return []planCase{
		{
			name: "debian native with caddy and ufw",
			host: func() *fakeSys {
				f := debianHost()
				f.bins["ufw"] = true
				f.stdout["ufw status"] = "Status: active\n"
				return f
			},
			yaml:         minimalAnswers + "firewall: true\n",
			wantCommands: []string{"apt-get install -y --no-install-recommends ca-certificates", "apt-get install -y --no-install-recommends postgresql-18 caddy", "useradd --system --user-group --no-create-home --home-dir /var/lib/dhow --shell /usr/sbin/nologin", "caddy validate", "ufw allow 80/tcp", "ufw allow 443/tcp", "systemctl restart dhow"},
			wantFiles:    []string{AptKeyPath, AptSourcePath, SecretKeyPath, DatabaseURLPath, EnvFilePath, BinaryPath, UnitPath, AnswersPath, CaddySitePath, CaddyfilePath},
			wantPackages: []string{"ca-certificates", "postgresql-18", "caddy"},
			absent:       []string{"dnf", "certbot", "docker"},
		},
		{
			name:         "rocky native with nginx and SELinux",
			host:         func() *fakeSys { f := debianHost(); f.put(OSReleasePath, rockyRelease, 0o644); return f },
			yaml:         "mode: native\ndomain: dhow.example.com\ndatabase: {mode: install}\nproxy: {kind: nginx, email: ops@example.com}\ninstall_packages: true\n",
			tweak:        func(d *Detection) { d.SELinux = true },
			wantCommands: []string{"dnf install -y epel-release", "rpm --import " + DnfKeyPath, "dnf -qy module disable postgresql", "dnf install -y postgresql18-server nginx certbot", "/usr/pgsql-18/bin/postgresql-18-setup initdb", "systemctl enable --now postgresql-18", "certbot certonly --webroot --webroot-path /var/www/dhow-acme --non-interactive --agree-tos --keep-until-expiring --email ops@example.com --domain dhow.example.com", "setsebool -P httpd_can_network_connect on"},
			wantFiles:    []string{DnfKeyPath, DnfRepoPath, NginxSitePath, CertbotHookPath},
			absent:       []string{"apt-get", "ufw"},
		},
		{
			name: "fedora keeps the PGDG packages",
			host: func() *fakeSys {
				f := debianHost()
				f.put(OSReleasePath, "ID=fedora\nVERSION_ID=44\nPRETTY_NAME=\"Fedora Linux 44\"\n", 0o644)
				return f
			},
			yaml:         "mode: native\ndomain: dhow.example.com\ndatabase: {mode: install}\nproxy: {kind: none}\ninstall_packages: true\n",
			wantCommands: []string{"dnf install -y postgresql18-server", "/usr/pgsql-18/bin/postgresql-18-setup initdb"},
			wantFiles:    []string{DnfExcludePath, DnfRepoPath},
			absent:       []string{"module disable", "epel-release"},
		},
		{
			name:      "existing database and proxy install nothing",
			host:      debianHost,
			yaml:      "mode: native\ndomain: dhow.example.com\ndatabase: {mode: existing, url_file: /root/db}\nproxy: {kind: existing, trusted_proxies: 10.0.0.2}\n",
			wantFiles: []string{DatabaseURLPath, EnvFilePath},
			absent:    []string{"apt-get", "psql", "caddy", "nginx", "useradd --system --user-group --no-create-home --home-dir /var/lib/dhow --shell /bin"},
		},
	}
}

func blockerCases() []planCase {
	return []planCase{
		{
			name: "unsupported distro needs packages: manual mode blocks",
			host: func() *fakeSys {
				f := debianHost()
				f.put(OSReleasePath, "ID=arch\nPRETTY_NAME=\"Arch Linux\"\n", 0o644)
				return f
			},
			yaml:         minimalAnswers,
			wantBlockers: []string{"Arch Linux is not a supported distribution", "postgresql-18 caddy"},
		},
		{
			name: "unsupported distro without packages continues",
			host: func() *fakeSys {
				f := debianHost()
				f.put(OSReleasePath, "ID=arch\nPRETTY_NAME=\"Arch Linux\"\n", 0o644)
				return f
			},
			yaml:      "mode: native\ndomain: dhow.example.com\ndatabase: {mode: existing, url_file: /root/db}\nproxy: {kind: none}\n",
			absent:    []string{"apt-get", "dnf"},
			wantFiles: []string{UnitPath},
		},
		{
			name:         "declined packages",
			host:         debianHost,
			yaml:         strings.Replace(minimalAnswers, "install_packages: true", "install_packages: false", 1),
			wantBlockers: []string{"you chose not to install them: postgresql-18 caddy"},
		},
		{
			name: "caddy is not packaged on ubuntu 22.04",
			host: func() *fakeSys {
				f := debianHost()
				f.put(OSReleasePath, "ID=ubuntu\nVERSION_ID=22.04\nVERSION_CODENAME=jammy\nPRETTY_NAME=\"Ubuntu 22.04\"\n", 0o644)
				return f
			},
			yaml:         minimalAnswers,
			wantBlockers: []string{"caddy cannot be installed automatically on Ubuntu 22.04"},
		},
		{
			name:         "compose without docker",
			host:         debianHost,
			yaml:         "mode: compose\ndomain: dhow.example.com\ndatabase: {mode: install}\nproxy: {kind: none}\n",
			wantBlockers: []string{"Docker with the Compose plugin"},
		},
		{
			name:         "another postgres on 5432",
			host:         func() *fakeSys { f := debianHost(); f.put("/proc/net/tcp", procNetTCP, 0o444); return f },
			yaml:         minimalAnswers,
			wantBlockers: []string{"port 5432", "Port 8080 is already used"},
		},
		{
			name:         "no systemd",
			host:         func() *fakeSys { f := debianHost(); delete(f.files, "/run/systemd/system"); return f },
			yaml:         "mode: native\ndomain: dhow.example.com\ndatabase: {mode: existing, url_file: /root/db}\nproxy: {kind: none}\n",
			wantBlockers: []string{"systemd is not running"},
		},
		{
			name:         "no systemd on a supported distribution blames systemd only",
			host:         func() *fakeSys { f := debianHost(); delete(f.files, "/run/systemd/system"); return f },
			yaml:         minimalAnswers,
			wantBlockers: []string{"systemd is not running"},
			absent:       []string{"not a supported distribution"},
		},
		{
			name: "compose with caddy trusts the docker gateway",
			host: func() *fakeSys {
				f := debianHost()
				f.bins["docker"] = true
				f.ok["docker info --format {{.ServerVersion}}"] = true
				f.ok["docker compose version"] = true
				return f
			},
			yaml:         "mode: compose\ndomain: dhow.example.com\ndatabase: {mode: install}\nproxy: {kind: caddy}\ninstall_packages: true\n",
			wantCommands: []string{"docker compose --project-directory /etc/dhow/compose --file /etc/dhow/compose/compose.yaml up --detach --wait"},
			wantFiles:    []string{ComposeFilePath, ComposeEnvPath},
			absent:       []string{"postgresql-18", "useradd"},
		},
	}
}

func TestBuildPlan(t *testing.T) {
	for _, tt := range append(planCases(), blockerCases()...) {
		t.Run(tt.name, func(t *testing.T) {
			p := build(t, tt.host(), tt.yaml, tt.tweak)
			blockers := strings.Join(p.Blockers, "\n")
			if tt.wantBlockers != nil {
				for _, w := range tt.wantBlockers {
					assert.Contains(t, blockers, w)
				}
				for _, w := range tt.absent {
					assert.NotContains(t, blockers, w)
				}
				assert.Empty(t, p.Steps)
				return
			}
			assert.Empty(t, p.Blockers)
			cmds := commands(p)
			for _, w := range tt.wantCommands {
				assert.Contains(t, cmds, w)
			}
			for _, w := range tt.wantFiles {
				assert.Contains(t, p.Files(), w)
			}
			if tt.wantPackages != nil {
				assert.Equal(t, tt.wantPackages, p.Packages())
			}
			for _, w := range tt.absent {
				assert.NotContains(t, cmds, w)
			}
		})
	}
}
