package installer_test

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	installer "github.com/Khaymah/dhow/apps/runner/internal/Installer"
	engine "github.com/Khaymah/dhow/internal/installer"
	"github.com/Khaymah/dhow/internal/installer/fakesys"
)

func installed(t *testing.T, release string) (*fakesys.Sys, *fakeProtocol) {
	t.Helper()
	f := runnerHost(release)
	p := &fakeProtocol{}
	out, err := install(t, f, p, basicAnswers)
	require.NoError(t, err, out)
	return f, p
}

func doctor(f *fakesys.Sys, p *fakeProtocol, opts installer.DoctorOptions) (string, error) {
	app, out := newApp(f, p, "")
	err := app.Doctor(context.Background(), opts)
	return out.String(), err
}

func TestDoctorIsCleanAfterAnInstall(t *testing.T) {
	f, p := installed(t, debian13)
	out, err := doctor(f, p, installer.DoctorOptions{})
	require.NoError(t, err, out)
	assert.Contains(t, out, "Everything looks good.")
	assert.Contains(t, out, "5.2 MEDIUM")
	assert.Contains(t, out, "encrypted with systemd-creds")
	assert.Contains(t, out, "signed in when the service started")
	assert.Equal(t, 1, p.reaches)
	assert.Zero(t, p.signIns, "doctor never signs in next to the running service, which would replace its session")
	assert.NotContains(t, out, "FAIL")
}

func TestDoctorSignsInItselfWhenTheServiceIsStopped(t *testing.T) {
	f, p := installed(t, debian13)
	f.OK["systemctl is-active --quiet dhow-runner"] = false
	out, err := doctor(f, p, installer.DoctorOptions{})
	require.ErrorIs(t, err, installer.ErrUnhealthy, out)
	assert.Equal(t, 1, p.signIns)
	assert.Contains(t, out, "Runner "+testRunnerID+" signed in.")
	assert.Contains(t, out, "FAIL  The service is enabled and running")

	p.signInFn = func() error { return errTest }
	out, _ = doctor(f, p, installer.DoctorOptions{})
	assert.Contains(t, out, "FAIL  The runner signs in")
	assert.Contains(t, out, "simulated failure")
}

func TestDoctorFindsAndFixesProblems(t *testing.T) {
	tests := []struct {
		name   string
		break_ func(f *fakesys.Sys)
		title  string
		fixed  func(t *testing.T, f *fakesys.Sys)
	}{
		{
			"drifted unit", func(f *fakesys.Sys) { f.Put(installer.UnitPath, "[Service]\n", 0o644) }, "The systemd unit matches",
			func(t *testing.T, f *fakesys.Sys) {
				assert.Equal(t, string(installer.UnitFile), f.Content(installer.UnitPath))
			},
		},
		{
			"missing credential drop-in", func(f *fakesys.Sys) { _ = f.Remove(installer.CredDropInPath) }, "Identity storage",
			func(t *testing.T, f *fakesys.Sys) { assert.True(t, f.Has(installer.CredDropInPath)) },
		},
		{"open env file", func(f *fakesys.Sys) {
			_ = f.Chmod(installer.EnvFilePath, engine.FileMode{Perm: 0o644, Owner: "root", Group: "root"})
		}, "File ownership and permissions", func(t *testing.T, f *fakesys.Sys) {
			assert.Equal(t, engine.FileMode{Perm: 0o640, Owner: "root", Group: "dhow-runner"}, f.ModeOf(installer.EnvFilePath))
		}},
		{
			"lost sub-ids", func(f *fakesys.Sys) { f.Put(installer.SubUIDPath, "", 0o644) }, "Sub-ids",
			func(t *testing.T, f *fakesys.Sys) {
				assert.Contains(t, f.Content(installer.SubUIDPath), "dhow-runner:")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, p := installed(t, debian13)
			tt.break_(f)
			out, err := doctor(f, p, installer.DoctorOptions{})
			require.ErrorIs(t, err, installer.ErrUnhealthy)
			assert.Regexp(t, regexp.MustCompile(`(FAIL|WARN)\s+`+regexp.QuoteMeta(tt.title)), out)
			assert.Contains(t, out, "doctor --fix")
			out, err = doctor(f, p, installer.DoctorOptions{Fix: true})
			require.NoError(t, err, out)
			tt.fixed(t, f)
			out, err = doctor(f, p, installer.DoctorOptions{})
			require.NoError(t, err, out)
		})
	}
}

func TestDoctorReportsWhatItCannotFix(t *testing.T) {
	tests := []struct {
		name   string
		break_ func(f *fakesys.Sys)
		want   string
	}{
		{"old podman", func(f *fakesys.Sys) { f.Stdout[podmanVersion] = "5.2.1\n" }, "Podman 5.2.1 is older than 5.4.0"},
		{"loose unit", func(f *fakesys.Sys) {
			f.Stdout["systemd-analyze security --no-pager dhow-runner.service"] = "→ Overall exposure level for dhow-runner.service: 9.2 UNSAFE 😨\n"
		}, "9.2 UNSAFE"},
		{"stale pending identity", func(f *fakesys.Sys) { f.Put(installer.SealedPendingPath, "x", 0o600) }, "key rotation that did not finish"},
		{"not signed in yet", func(f *fakesys.Sys) {
			f.Stdout["journalctl _SYSTEMD_INVOCATION_ID=inv-1 --output cat --no-pager"] = `{"level":"WARN","msg":"could not sign in to the server yet","reason":"x"}` + "\n"
		}, "does not show a sign-in yet"},
		{"apparmor", func(f *fakesys.Sys) { f.Put(installer.AppArmorUserns, "1\n", 0o644) }, "AppArmor blocks user namespaces"},
		{"both identities", func(f *fakesys.Sys) { f.Put(installer.IdentityPath, "{}", 0o600) }, "holds a plain identity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, p := installed(t, debian13)
			tt.break_(f)
			out, err := doctor(f, p, installer.DoctorOptions{Fix: true})
			require.ErrorIs(t, err, installer.ErrUnhealthy)
			assert.Contains(t, out, tt.want)
		})
	}
}

func TestDoctorSaysWhenTheIdentityIsAPlainFile(t *testing.T) {
	f := runnerHost(debian13)
	f.Fail["systemd-creds"] = true
	p := &fakeProtocol{}
	_, err := install(t, f, p, basicAnswers)
	require.NoError(t, err)
	out, err := doctor(f, p, installer.DoctorOptions{})
	require.NoError(t, err, out)
	assert.Contains(t, out, "info  Identity storage")
	assert.Contains(t, out, "not encrypted because systemd-creds cannot encrypt on this host")
}

func TestDoctorNeedsAnInstallAndRoot(t *testing.T) {
	f := runnerHost(debian13)
	out, err := doctor(f, &fakeProtocol{}, installer.DoctorOptions{})
	require.ErrorIs(t, err, installer.ErrUnhealthy)
	assert.Contains(t, out, "not installed")
	f.Root = false
	_, err = doctor(f, &fakeProtocol{}, installer.DoctorOptions{})
	require.ErrorContains(t, err, "run it as root")
}

func TestDoctorPrintsNoSecret(t *testing.T) {
	f, p := installed(t, debian13)
	out, err := doctor(f, p, installer.DoctorOptions{})
	require.NoError(t, err)
	for _, secret := range []string{testToken, "private", sealedPrefix} {
		assert.NotContains(t, out, secret)
	}
}

func uninstall(f *fakesys.Sys, p *fakeProtocol, opts installer.UninstallOptions) (string, error) {
	app, out := newApp(f, p, "")
	err := app.Uninstall(context.Background(), opts)
	return out.String(), err
}

func TestUninstallRevokesAndLeavesTheHostClean(t *testing.T) {
	f, p := installed(t, debian13)
	out, err := uninstall(f, p, installer.UninstallOptions{Yes: true})
	require.NoError(t, err, out)
	assert.Equal(t, 1, p.revokes)
	assert.Contains(t, out, "Revoked runner "+testRunnerID)
	assert.True(t, f.Ran("systemctl disable --now dhow-runner"))
	for _, path := range []string{
		installer.UnitPath, installer.DropInDir, installer.CredDropInPath, installer.StateDir,
		installer.SealedPath, installer.ConfigDir, installer.EnvFilePath, installer.AnswersPath, installer.BinaryPath,
	} {
		assert.False(t, f.Has(path), path)
	}
	assert.Contains(t, out, "The user dhow-runner and its sub-ids stay")
	assert.False(t, f.Ran("userdel"))
	assert.Contains(t, f.Content(installer.SubUIDPath), "dhow-runner:")

	out, err = uninstall(f, p, installer.UninstallOptions{Yes: true})
	require.NoError(t, err)
	assert.Contains(t, out, "Nothing to remove")
	assert.Equal(t, 1, p.revokes)

	out, err = uninstall(f, p, installer.UninstallOptions{Yes: true, RemoveUser: true})
	require.NoError(t, err, out)
	assert.True(t, f.Ran("userdel dhow-runner"))
	assert.Equal(t, "alice:100000:65536\n", f.Content(installer.SubUIDPath))
	assert.Equal(t, "alice:100000:65536\n", f.Content(installer.SubGIDPath))
}

func TestUninstallTellsTheOperatorToRevokeWhenTheRunnerCannot(t *testing.T) {
	f, p := installed(t, debian13)
	p.revokeFn = func() (installer.RevokeResult, error) { return installer.RevokeDone, errTest }
	out, err := uninstall(f, p, installer.UninstallOptions{Yes: true})
	require.NoError(t, err, out)
	assert.Contains(t, out, "Revoke runner "+testRunnerID+" in the dashboard")
	assert.False(t, f.Has(installer.StateDir), "the host is cleaned anyway")

	f, p = installed(t, debian13)
	p.revokeFn = func() (installer.RevokeResult, error) { return installer.RevokeAlready, nil }
	out, err = uninstall(f, p, installer.UninstallOptions{Yes: true})
	require.NoError(t, err)
	assert.Contains(t, out, "was already revoked")
}

func TestUninstallShredsAPlainIdentityAndAsksFirst(t *testing.T) {
	f := runnerHost(debian13)
	f.Fail["systemd-creds"] = true
	p := &fakeProtocol{}
	_, err := install(t, f, p, basicAnswers)
	require.NoError(t, err)
	app, out := newApp(f, p, "n\n")
	require.ErrorContains(t, app.Uninstall(context.Background(), installer.UninstallOptions{}), "Nothing was changed")
	assert.True(t, f.Has(installer.IdentityPath))
	assert.Zero(t, p.revokes)
	assert.Contains(t, out.String(), "Remove the runner from this host?")
	_, err = uninstall(f, p, installer.UninstallOptions{Yes: true})
	require.NoError(t, err)
	assert.Equal(t, []string{installer.IdentityPath}, f.Shredded)
}

func TestMainFlags(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, "expected one of"},
		{[]string{"upgrade"}, "expected one of"},
		{[]string{"install", "--token", "x"}, "flag provided but not defined"},
		{[]string{"doctor", "--remove-user"}, "flag provided but not defined"},
		{[]string{"uninstall", "now"}, "unexpected argument"},
		{[]string{"install", "--lang", "fr"}, "--lang must be en or ar"},
		{[]string{"install", "--help"}, ""},
	}
	for _, tt := range tests {
		var errOut bytes.Buffer
		app, _ := newApp(runnerHost(debian13), &fakeProtocol{}, "")
		err := installer.Main(context.Background(), tt.args, app, &errOut)
		if tt.want == "" {
			require.NoError(t, err)
			continue
		}
		require.ErrorContains(t, err, tt.want, strings.Join(tt.args, " "))
	}
}

func TestParseAnswers(t *testing.T) {
	for yaml, want := range map[string]string{
		"token_file: /t\n":                                    "url: is required",
		"url: http://dhow.example.com\n":                      "must use https://",
		"url: https://d.example\ntoken_file: rel\n":           "token_file: must be an absolute path",
		"url: https://d.example\ncapacity: 17\n":              "capacity: must be from 1 to 16",
		"url: https://d.example\nidentity: tpm\n":             "identity: must be auto, encrypted or file",
		"url: https://d.example\ndriver: docker\n":            "driver: must be podman",
		"url: https://d.example\nenv: {PATH: /x}\n":           "is not a dhow-runner setting",
		"url: https://d.example\nenv: {DHOW_RUNNER_URL: x}\n": "is set from the other answers",
		"url: https://d.example\ntoken: x\n":                  "field token not found",
	} {
		_, err := installer.ParseAnswers(strings.NewReader(yaml))
		require.ErrorIs(t, err, installer.ErrInvalidAnswers, yaml)
		assert.Contains(t, err.Error(), want, yaml)
	}
	a, err := installer.ParseAnswers(strings.NewReader("url: http://127.0.0.1:8080/\ndev: true\nenv: {DHOW_RUNNER_MAX_MEMORY: 2GiB}\n"))
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:8080", a.URL)
	env := string(installer.RenderEnv(a, []byte("DHOW_RUNNER_URL=https://old.example\nDHOW_RUNNER_LOG_LEVEL=debug\n")))
	assert.Equal(t, "DHOW_RUNNER_URL=http://127.0.0.1:8080\nDHOW_RUNNER_DRIVER=podman\nDHOW_RUNNER_CAPACITY=1\nDHOW_RUNNER_MAX_MEMORY=2GiB\nDHOW_RUNNER_LOG_LEVEL=debug\n", env)
}

func TestHelpers(t *testing.T) {
	score, level, ok := installer.ParseExposure([]byte("x\n" + exposureLine))
	require.True(t, ok)
	assert.InDelta(t, 5.2, score, 0.001)
	assert.Equal(t, "MEDIUM", level)
	_, _, ok = installer.ParseExposure([]byte("nothing"))
	assert.False(t, ok)
	assert.True(t, installer.PodmanAtLeast("5.4.0", "5.4.0"))
	assert.True(t, installer.PodmanAtLeast("6.1.4", "5.4.0"))
	assert.True(t, installer.PodmanAtLeast("5.10.0-rc1", "5.4.0"))
	assert.False(t, installer.PodmanAtLeast("5.3.9", "5.4.0"))
	assert.False(t, installer.PodmanAtLeast("4.9.3", "5.4.0"))
	assert.Equal(t, 1<<20, installer.FreeSubIDStart([]byte("a:100000:65536\n")))
	assert.Equal(t, 2162688, installer.FreeSubIDStart([]byte("a:1048576:1048576\n"), []byte("b:2100000:10\n")))
}

var verbRe = regexp.MustCompile(`%[sdqvt]`)

func TestCatalogsHaveTheSameKeysAndVerbs(t *testing.T) {
	en, ar := installer.Catalog(engine.LangEn), installer.Catalog(engine.LangAr)
	assert.Equal(t, engine.Keys(en), engine.Keys(ar))
	for k, v := range en {
		assert.Equal(t, verbRe.FindAllString(v, -1), verbRe.FindAllString(ar[k], -1), string(k))
		assert.NotEmpty(t, ar[k], string(k))
	}
}
