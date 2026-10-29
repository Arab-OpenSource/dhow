package installer_test

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	installer "github.com/Khaymah/dhow/apps/runner/internal/Installer"
	engine "github.com/Khaymah/dhow/internal/installer"
	"github.com/Khaymah/dhow/internal/installer/fakesys"
)

func install(t *testing.T, f *fakesys.Sys, p *fakeProtocol, yaml string) (string, error) {
	t.Helper()
	writeAnswers(f, yaml)
	app, out := newApp(f, p, "")
	err := app.Install(context.Background(), installer.InstallOptions{Answers: answersPath})
	return out.String(), err
}

func TestInstallFromAnswersBringsTheRunnerOnline(t *testing.T) {
	f := runnerHost(debian13)
	p := &fakeProtocol{}
	out, err := install(t, f, p, basicAnswers)
	require.NoError(t, err, out)

	assert.Contains(t, out, "Registered runner "+testRunnerID)
	assert.Contains(t, out, "encrypted with systemd-creds")
	require.Len(t, p.targets, 1)
	assert.True(t, p.targets[0].Encrypted, "systemd-creds works, so the identity is encrypted")
	assert.Equal(t, "https://dhow.example.com", p.targets[0].Config.URL.String())
	assert.Equal(t, []string{testToken}, p.tokens)

	for _, want := range []string{
		"useradd --system --user-group --no-create-home --home-dir /var/lib/dhow-runner --shell /usr/sbin/nologin --comment Dhow runner dhow-runner",
		"usermod --add-subuids 1048576-2097151 --add-subgids 1048576-2097151 dhow-runner",
		"systemctl daemon-reload", "systemctl enable dhow-runner", "systemctl restart dhow-runner",
	} {
		assert.True(t, f.Ran(want), want)
	}
	assert.Equal(t, engine.FileMode{Perm: 0o700, Owner: "dhow-runner", Group: "dhow-runner"}, f.ModeOf(installer.StateDir))
	assert.Equal(t, engine.FileMode{Perm: 0o640, Owner: "root", Group: "dhow-runner"}, f.ModeOf(installer.EnvFilePath))
	assert.Equal(t, engine.FileMode{Perm: 0o750, Owner: "root", Group: "dhow-runner"}, f.ModeOf(installer.ConfigDir))
	assert.Equal(t, "DHOW_RUNNER_URL=https://dhow.example.com\nDHOW_RUNNER_DRIVER=podman\nDHOW_RUNNER_CAPACITY=1\n", f.Content(installer.EnvFilePath))
	assert.Equal(t, string(installer.UnitFile), f.Content(installer.UnitPath))
	assert.Equal(t, "[Service]\nLoadCredentialEncrypted=identity:/var/lib/dhow-runner/identity.cred\n", f.Content(installer.CredDropInPath))
	assert.Equal(t, "dhow-runner binary", f.Content(installer.BinaryPath))
	assert.True(t, f.Has(installer.SealedPath))
	assert.False(t, f.Has(installer.IdentityPath))
	saved := f.Content(installer.AnswersPath)
	assert.Contains(t, saved, "url: https://dhow.example.com")
	assert.NotContains(t, saved, "token_file")
}

func TestTheTokenNeverReachesArgvOutputOrDisk(t *testing.T) {
	for _, source := range []string{"file", "stdin"} {
		t.Run(source, func(t *testing.T) {
			f := runnerHost(debian13)
			p := &fakeProtocol{}
			yaml := basicAnswers
			stdin := ""
			if source == "stdin" {
				yaml = strings.Replace(basicAnswers, "token_file: /root/token", "token_file: \"-\"", 1)
				stdin = testToken + "\n"
			}
			writeAnswers(f, yaml)
			if source == "stdin" {
				require.NoError(t, f.Remove(tokenPath))
			}
			app, out := newApp(f, p, stdin)
			require.NoError(t, app.Install(context.Background(), installer.InstallOptions{Answers: answersPath}), out.String())
			require.Equal(t, []string{testToken}, p.tokens)
			assert.NotContains(t, out.String(), testToken)
			for _, c := range f.Commands() {
				assert.NotContains(t, c.String(), testToken, "argv")
				assert.NotContains(t, strings.Join(c.Env, " "), testToken, "environment")
				assert.NotContains(t, string(c.Stdin), testToken, "stdin of a command")
			}
			for path, file := range f.Files {
				if path != tokenPath {
					assert.NotContains(t, string(file.Data), "SuperSecret", path)
				}
			}
		})
	}
}

func TestInstallTwiceChangesNothing(t *testing.T) {
	f := runnerHost(debian13)
	p := &fakeProtocol{}
	_, err := install(t, f, p, basicAnswers)
	require.NoError(t, err)
	before := len(f.Commands())
	out, err := install(t, f, p, basicAnswers)
	require.NoError(t, err, out)
	assert.Len(t, p.tokens, 1, "an installed runner does not register again")
	steps := regexp.MustCompile(`(?m)^\[\d+/\d+\] .*$`).FindAllString(out, -1)
	require.NotEmpty(t, steps)
	for _, s := range steps {
		assert.Contains(t, s, "already done", s)
	}
	for _, c := range f.CommandLines()[before:] {
		assert.False(t, strings.HasPrefix(c, "useradd") || strings.HasPrefix(c, "usermod") || c == "systemctl restart dhow-runner", c)
	}
}

func TestPlanPerDistroFamily(t *testing.T) {
	tests := []struct {
		name, release, pkgs string
		blocked             string
	}{
		{"debian 13", debian13, "apt-get install -y --no-install-recommends podman crun uidmap", ""},
		{"ubuntu 26.04", ubuntu2604, "apt-get install -y --no-install-recommends podman crun uidmap", ""},
		{"fedora 43", fedora43, "dnf install -y podman crun shadow-utils", ""},
		{"rocky 10", rocky10, "dnf install -y podman crun shadow-utils", ""},
		{"unknown", "ID=arch\n", "", "cannot install packages on arch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := runnerHost(tt.release)
			for _, b := range []string{"podman", "crun", "newuidmap", "newgidmap"} {
				f.Bins[b] = false
			}
			p := &fakeProtocol{}
			app, out := newApp(f, p, "")
			writeAnswers(f, basicAnswers+"install_packages: true\n")
			err := app.Install(context.Background(), installer.InstallOptions{Answers: answersPath, DryRun: true})
			if tt.blocked != "" {
				require.ErrorIs(t, err, installer.ErrBlocked)
				assert.Contains(t, out.String(), tt.blocked)
				return
			}
			require.NoError(t, err, out.String())
			assert.Contains(t, out.String(), tt.pkgs)
			assert.Contains(t, out.String(), "Check that Podman is 5.4.0 or newer")
			assert.Empty(t, p.tokens, "a dry run registers nothing")
			for _, c := range f.CommandLines() {
				assert.False(t, strings.HasPrefix(c, "useradd") || strings.HasPrefix(c, "apt-get") || strings.HasPrefix(c, "dnf"), c)
			}
		})
	}
}

func TestInstallRefusesHostsTheSandboxCannotUse(t *testing.T) {
	tests := []struct {
		name  string
		tweak func(f *fakesys.Sys)
		yaml  string
		want  string
	}{
		{"old podman on ubuntu 24.04", func(f *fakesys.Sys) {
			f.Put(engine.OSReleasePath, ubuntu2404, 0o644)
			f.Stdout[podmanVersion] = "4.9.3\n"
		}, basicAnswers, "Podman 4.9.3 is older than 5.4.0"},
		{"no systemd", func(f *fakesys.Sys) { _ = f.Remove(installer.SystemdRunDir) }, basicAnswers, "systemd is not running"},
		{"cgroup v1", func(f *fakesys.Sys) { _ = f.Remove(installer.CgroupControllers) }, basicAnswers, "cgroup v2"},
		{"user namespaces off", func(f *fakesys.Sys) { f.Put(installer.MaxUserNamespaces, "0\n", 0o644) }, basicAnswers, "User namespaces are turned off"},
		{"apparmor", func(f *fakesys.Sys) { f.Put(installer.AppArmorUserns, "1\n", 0o644) }, basicAnswers, "apparmor_restrict_unprivileged_userns"},
		{"missing packages declined", func(f *fakesys.Sys) { f.Bins["crun"] = false }, basicAnswers, "you chose not to install them: crun"},
		{"encrypted without systemd-creds", func(f *fakesys.Sys) { f.Bins["systemd-creds"] = false }, basicAnswers + "identity: encrypted\n", "needs systemd-creds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := runnerHost(debian13)
			tt.tweak(f)
			p := &fakeProtocol{}
			out, err := install(t, f, p, tt.yaml)
			require.ErrorIs(t, err, installer.ErrBlocked)
			assert.Contains(t, out, tt.want)
			assert.Empty(t, p.tokens)
			assert.False(t, f.Ran("useradd"))
		})
	}
}

func TestWithoutSystemdCredsTheIdentityIsAPlainFileAndTheOutputSaysSo(t *testing.T) {
	f := runnerHost(fedora43)
	f.Fail["systemd-creds"] = true
	p := &fakeProtocol{}
	out, err := install(t, f, p, basicAnswers)
	require.NoError(t, err, out)
	require.Len(t, p.targets, 1)
	assert.False(t, p.targets[0].Encrypted)
	assert.Contains(t, out, "systemd-creds cannot encrypt on this host")
	assert.Contains(t, out, "The identity is in /var/lib/dhow-runner/identity")
	assert.False(t, f.Has(installer.CredDropInPath))
	assert.True(t, f.Has(installer.IdentityPath))
}

func TestInstallEncryptsAPlainIdentityLeftByAManualSetup(t *testing.T) {
	f := runnerHost(debian13)
	f.Put(installer.IdentityPath, `{"SigningSeed":"private"}`, 0o600)
	p := &fakeProtocol{}
	out, err := install(t, f, p, basicAnswers)
	require.NoError(t, err, out)
	assert.Empty(t, p.tokens, "a registered runner keeps its identity")
	assert.Contains(t, out, "is not encrypted. The plan encrypts it")
	assert.True(t, f.Ran("systemctl stop dhow-runner") || !strings.Contains(out, "Stop the runner"))
	assert.True(t, f.Has(installer.SealedPath))
	assert.False(t, f.Has(installer.IdentityPath))
	assert.True(t, f.Has(installer.CredDropInPath))
}

func TestAFailedStartShowsTheServiceLog(t *testing.T) {
	f := runnerHost(debian13)
	base := f.OnRun
	f.OnRun = func(s *fakesys.Sys, c engine.Command) (engine.Output, bool, error) {
		out, handled, err := base(s, c)
		if c.String() == "systemctl restart dhow-runner" {
			s.Stdout["journalctl _SYSTEMD_INVOCATION_ID=inv-1 --output cat --no-pager"] = `{"level":"ERROR","msg":"the server stopped this runner","error":"RUNNER_REVOKED"}` + "\n"
			s.Stdout["systemctl show --property ActiveState --property SubState --value dhow-runner"] = "activating\nauto-restart\n"
			s.Stdout["journalctl --unit dhow-runner --lines 25 --no-pager --output cat"] = "dhow-runner: RUNNER_REVOKED\n"
		}
		return out, handled, err
	}
	out, err := install(t, f, &fakeProtocol{}, basicAnswers)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not come online: the server stopped this runner RUNNER_REVOKED")
	assert.Contains(t, err.Error(), "dhow-runner: RUNNER_REVOKED")
	assert.Contains(t, err.Error(), "run the same command again")
	assert.NotContains(t, out, "is installed and online")
}

func TestInstallNeedsRootAndATokenFile(t *testing.T) {
	f := runnerHost(debian13)
	f.Root = false
	_, err := install(t, f, &fakeProtocol{}, basicAnswers)
	require.ErrorContains(t, err, "run it as root")

	f = runnerHost(debian13)
	_, err = install(t, f, &fakeProtocol{}, "url: https://dhow.example.com\n")
	require.ErrorContains(t, err, "needs token_file")

	f = runnerHost(debian13)
	writeAnswers(f, basicAnswers)
	f.Put(tokenPath, "not-a-token\n", 0o600)
	app, _ := newApp(f, &fakeProtocol{}, "")
	err = app.Install(context.Background(), installer.InstallOptions{Answers: answersPath})
	require.ErrorIs(t, err, installer.ErrTokenFormat)
}

func TestInteractiveInstallAsksAndHidesTheToken(t *testing.T) {
	f := runnerHost(debian13)
	p := &fakeProtocol{}
	in := strings.Join([]string{"https://dhow.example.com", testToken, "build-02", "1", "2", "y"}, "\n") + "\n"
	app, out := newApp(f, p, in)
	require.NoError(t, app.Install(context.Background(), installer.InstallOptions{}), out.String())
	assert.Equal(t, []string{testToken}, p.tokens)
	assert.NotContains(t, out.String(), testToken)
	assert.Contains(t, out.String(), "Registration token")
	assert.Contains(t, f.Content(installer.EnvFilePath), "DHOW_RUNNER_CAPACITY=2")
}

func TestInstallInArabic(t *testing.T) {
	f := runnerHost(debian13)
	writeAnswers(f, basicAnswers+"lang: ar\n")
	app, out := newApp(f, &fakeProtocol{}, "")
	require.NoError(t, app.Install(context.Background(), installer.InstallOptions{Answers: answersPath}))
	assert.Contains(t, out.String(), "سجّلنا جهاز التشغيل")
	assert.Contains(t, out.String(), "ثبّتنا جهاز التشغيل")
}

func TestTheEmbeddedUnitIsTheShippedOne(t *testing.T) {
	shipped, err := os.ReadFile("../../../../deploy/systemd/dhow-runner.service")
	require.NoError(t, err)
	assert.True(t, bytes.Equal(shipped, installer.UnitFile), "copy deploy/systemd/dhow-runner.service to Assets/")
}
