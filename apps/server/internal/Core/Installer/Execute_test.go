package installer

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ciAnswers = "mode: native\ndomain: dhow.example.com\ndatabase: {mode: install}\nproxy: {kind: none}\ninstall_packages: true\nbreached_password_check: false\n"

func newApp(f *fakeSys, in string) (App, *bytes.Buffer) {
	var out bytes.Buffer
	return App{Sys: f, In: strings.NewReader(in), InFD: -1, Out: &out, Version: "1.2.3", Sleep: func(time.Duration) {}}, &out
}

func installedHost(t *testing.T) (*fakeSys, *bytes.Buffer) {
	t.Helper()
	f := liveHost(debianHost())
	f.put("/root/answers.yaml", ciAnswers, 0o600)
	app, out := newApp(f, "")
	require.NoError(t, app.Install(context.Background(), InstallOptions{Answers: "/root/answers.yaml", Yes: true}), out.String())
	return f, out
}

func TestInstallNativeEndToEndOnFakeHost(t *testing.T) {
	f, out := installedHost(t)
	key := f.content(SecretKeyPath)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{64}\n$`), key)
	url := f.content(DatabaseURLPath)
	assert.Regexp(t, regexp.MustCompile(`^postgres://dhow:[0-9a-f]{48}@/dhow\?host=/var/run/postgresql&port=5432&sslmode=require\n$`), url)
	for p, perm := range map[string]uint32{SecretKeyPath: 0o600, DatabaseURLPath: 0o600, EnvFilePath: 0o600, AnswersPath: 0o600, UnitPath: 0o644, BinaryPath: 0o755} {
		require.True(t, f.has(p), p)
		assert.Equal(t, perm, uint32(f.files[p].mode.Perm), p)
	}
	env := f.content(EnvFilePath)
	assert.Contains(t, env, "DHOW_PUBLIC_URL=https://dhow.example.com\n")
	assert.Contains(t, env, "DHOW_BREACHED_PASSWORD_CHECK=false\n")
	assert.Contains(t, env, "DHOW_LISTEN_ADDR=127.0.0.1:8080\n")
	assert.NotContains(t, env, "DHOW_SECRET_KEY")
	assert.Equal(t, string(UnitFile), f.content(UnitPath))
	assert.Equal(t, "dhow-server binary v1", f.content(BinaryPath))
	assert.Contains(t, out.String(), "https://dhow.example.com/setup#TOKEN")

	password := passwordFromURL(url)
	for _, leak := range []string{strings.TrimSpace(key), password} {
		assert.NotContains(t, out.String(), leak, "a secret reached the output")
		assert.NotContains(t, f.content(AnswersPath), leak)
		for _, c := range f.commands {
			assert.NotContains(t, c, leak, "a secret reached a command line")
		}
		for _, in := range f.stdins {
			assert.NotContains(t, in, leak, "the plain password reached psql")
		}
	}
	assert.Contains(t, strings.Join(f.stdins, ""), "\\set verifier 'SCRAM-SHA-256$4096:")
}

func TestInstallRerunKeepsSecretsAndChangesNothing(t *testing.T) {
	f, _ := installedHost(t)
	key, url := f.content(SecretKeyPath), f.content(DatabaseURLPath)
	f.commands = nil
	app, out := newApp(f, "")
	require.NoError(t, app.Install(context.Background(), InstallOptions{Answers: "/root/answers.yaml", Yes: true}), out.String())
	assert.Equal(t, key, f.content(SecretKeyPath))
	assert.Equal(t, url, f.content(DatabaseURLPath))
	assert.False(t, f.ran("systemctl restart dhow"), "an unchanged install must not restart the service")
	assert.False(t, f.ran("apt-get install"))
	assert.False(t, f.ran("useradd"))
	assert.NotRegexp(t, regexp.MustCompile(`\[\d+/\d+\] [^:\n]*\n`), out.String(), "every step must be skipped")
}

func TestInstallUpgradeReplacesTheBinaryAndRestarts(t *testing.T) {
	f, _ := installedHost(t)
	key := f.content(SecretKeyPath)
	f.put(f.exe, "dhow-server binary v2", 0o755)
	f.commands = nil
	app, out := newApp(f, "")
	require.NoError(t, app.Install(context.Background(), InstallOptions{Answers: "/root/answers.yaml", Yes: true}), out.String())
	assert.Equal(t, "dhow-server binary v2", f.content(BinaryPath))
	assert.True(t, f.ran("systemctl restart dhow"))
	assert.Equal(t, key, f.content(SecretKeyPath))
}

func TestFailedStepRollsBackAndStops(t *testing.T) {
	f := liveHost(debianHost())
	f.put("/root/answers.yaml", ciAnswers, 0o600)
	f.failOnce["systemctl daemon-reload"] = true
	app, out := newApp(f, "")
	err := app.Install(context.Background(), InstallOptions{Answers: "/root/answers.yaml", Yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Install the hardened systemd unit")
	assert.Contains(t, err.Error(), "simulated failure")
	assert.Contains(t, out.String(), "rolled back")
	assert.False(t, f.has(UnitPath), "the failed step must remove the unit it wrote")
	assert.True(t, f.has(SecretKeyPath), "finished steps are kept")
	assert.False(t, f.ran("systemctl restart dhow"), "later steps must not run")

	key := f.content(SecretKeyPath)
	app, out = newApp(f, "")
	require.NoError(t, app.Install(context.Background(), InstallOptions{Answers: "/root/answers.yaml", Yes: true}), out.String())
	assert.Equal(t, key, f.content(SecretKeyPath))
	assert.True(t, f.has(UnitPath))
}

func TestFileStepUndoRestoresThePreviousContent(t *testing.T) {
	f := newFake()
	f.put("/etc/x.conf", "old", 0o640)
	s := FileStep(f, "x", "/etc/x.conf", []byte("new"), FileMode{Perm: 0o644})
	done, err := s.Done(context.Background())
	require.NoError(t, err)
	assert.False(t, done)
	require.NoError(t, s.Apply(context.Background()))
	done, err = s.Done(context.Background())
	require.NoError(t, err)
	assert.True(t, done)
	require.NoError(t, s.Undo(context.Background()))
	assert.Equal(t, "old", f.content("/etc/x.conf"))
}

func TestDryRunChangesNothingAndNeedsNoRoot(t *testing.T) {
	f := debianHost()
	f.root = false
	f.put("/root/answers.yaml", ciAnswers, 0o600)
	before := len(f.files)
	app, out := newApp(f, "")
	require.NoError(t, app.Install(context.Background(), InstallOptions{Answers: "/root/answers.yaml", DryRun: true}))
	assert.Len(t, f.files, before)
	assert.Empty(t, filterChecks(f.commands))
	assert.Contains(t, out.String(), "Plan:")
	assert.Contains(t, out.String(), SecretKeyPath)
	assert.Contains(t, out.String(), "postgresql-18")
	assert.Contains(t, out.String(), "Dry run: nothing was changed.")
}

func filterChecks(cmds []string) []string {
	var out []string
	for _, c := range cmds {
		if strings.HasPrefix(c, "apt-get") || strings.HasPrefix(c, "useradd") || strings.HasPrefix(c, "systemctl enable") || strings.HasPrefix(c, "systemctl restart") || strings.HasPrefix(c, "runuser") {
			out = append(out, c)
		}
	}
	return out
}

func TestInstallNeedsRoot(t *testing.T) {
	f := debianHost()
	f.root = false
	f.put("/root/answers.yaml", ciAnswers, 0o600)
	app, _ := newApp(f, "")
	err := app.Install(context.Background(), InstallOptions{Answers: "/root/answers.yaml", Yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sudo")
}

func TestInstallStopsOnBlockersAndWhenTheUserSaysNo(t *testing.T) {
	f := debianHost()
	f.put("/root/answers.yaml", strings.Replace(ciAnswers, "install_packages: true", "install_packages: false", 1), 0o600)
	app, out := newApp(f, "")
	require.ErrorIs(t, app.Install(context.Background(), InstallOptions{Answers: "/root/answers.yaml", Yes: true}), ErrBlocked)
	assert.Contains(t, out.String(), "The install cannot continue")

	f = liveHost(debianHost())
	f.put("/root/answers.yaml", ciAnswers, 0o600)
	app, _ = newApp(f, "n\n")
	err := app.Install(context.Background(), InstallOptions{Answers: "/root/answers.yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Nothing was changed")
	assert.False(t, f.has(SecretKeyPath))
}

func TestExistingDatabaseFromAnswersFile(t *testing.T) {
	f := liveHost(debianHost())
	f.put("/root/db-url", "postgres://dhow:pw-from-file@db.internal:5432/dhow?sslmode=verify-full\n", 0o600)
	f.put("/root/answers.yaml", "mode: native\ndomain: dhow.example.com\ndatabase: {mode: existing, url_file: /root/db-url}\nproxy: {kind: none}\n", 0o600)
	app, out := newApp(f, "")
	app.DB = fakeDB{major: 18}
	require.NoError(t, app.Install(context.Background(), InstallOptions{Answers: "/root/answers.yaml", Yes: true}), out.String())
	assert.Equal(t, "postgres://dhow:pw-from-file@db.internal:5432/dhow?sslmode=verify-full\n", f.content(DatabaseURLPath))
	assert.NotContains(t, out.String(), "pw-from-file")
	assert.Contains(t, out.String(), "Connected: PostgreSQL 18, sslmode=verify-full.")

	tests := []struct {
		name, content string
		perm          uint32
		major         int
		want          string
	}{
		{"world-readable url file", "postgres://dhow:pw@db/dhow?sslmode=require", 0o644, 18, "readable or writable by other users"},
		{"plaintext sslmode", "postgres://dhow:pw@db/dhow?sslmode=disable", 0o600, 18, "sslmode"},
		{"old server", "postgres://dhow:pw@db/dhow?sslmode=require", 0o600, 16, "PostgreSQL 16; Dhow needs 18"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := liveHost(debianHost())
			g.files["/root/db-url"] = &fakeFile{data: []byte(tt.content), mode: FileMode{Perm: fsMode(tt.perm)}}
			g.put("/root/answers.yaml", "mode: native\ndomain: dhow.example.com\ndatabase: {mode: existing, url_file: /root/db-url}\nproxy: {kind: none}\n", 0o600)
			app, out := newApp(g, "")
			app.DB = fakeDB{major: tt.major}
			err := app.Install(context.Background(), InstallOptions{Answers: "/root/answers.yaml", Yes: true})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.NotContains(t, err.Error()+out.String(), "pw@")
		})
	}
}

func TestLastSetupURL(t *testing.T) {
	logs := []byte("plain text line\n" +
		`{"msg":"x","setup_url":"https://a.example/setup#old"}` + "\n" +
		`dhow-1  | {"msg":"y","setup_url":"https://a.example/setup#new"}` + "\n" +
		`{"msg":"listening"}` + "\n{broken json\n")
	assert.Equal(t, "https://a.example/setup#new", LastSetupURL(logs))
	assert.Empty(t, LastSetupURL([]byte(`{"msg":"no link"}`)))
}
