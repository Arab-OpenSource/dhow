package installer_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	installer "github.com/Khaymah/dhow/apps/runner/internal/Installer"
	engine "github.com/Khaymah/dhow/internal/installer"
	"github.com/Khaymah/dhow/internal/installer/fakesys"
)

const shadow = "/etc/shadow"

func TestDoctorNeverFixesThroughAPlantedSymlink(t *testing.T) {
	for _, planted := range []string{installer.IdentityPath, installer.PendingPath, installer.SealedPendingPath} {
		t.Run(planted, func(t *testing.T) {
			f, p := installed(t, debian13)
			f.Put(shadow, "root:$6$secret", 0o640)
			f.PutLink(planted, shadow)
			before := f.ModeOf(shadow)
			out, err := doctor(f, p, installer.DoctorOptions{Fix: true})
			require.ErrorIs(t, err, installer.ErrUnhealthy)
			assert.Contains(t, out, planted+" is not a regular file")
			assert.Contains(t, out, "doctor never changes such a file")
			assert.Equal(t, before, f.ModeOf(shadow), "the link target keeps its owner and mode")
			assert.Equal(t, "root:$6$secret", f.Content(shadow))
		})
	}
}

func TestUninstallRemovesAPlantedLinkButNotItsTarget(t *testing.T) {
	f := runnerHost(debian13)
	f.Fail["systemd-creds"] = true
	p := &fakeProtocol{}
	_, err := install(t, f, p, basicAnswers)
	require.NoError(t, err)
	f.Put(shadow, "root:$6$secret", 0o640)
	f.PutLink(installer.PendingPath, shadow)
	_, err = uninstall(f, p, installer.UninstallOptions{Yes: true})
	require.NoError(t, err)
	assert.Equal(t, []string{installer.IdentityPath}, f.Shredded, "only the real identity file is overwritten")
	assert.Equal(t, "root:$6$secret", f.Content(shadow))
	assert.False(t, f.Has(installer.PendingPath))
}

func TestTheAnswersFileMustBeRootsAndPrivate(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		tweak func(f *fakesys.Sys)
		want  string
	}{
		{"owned by another user", answersPath, func(f *fakesys.Sys) {
			_ = f.Chmod(answersPath, engine.FileMode{Perm: 0o600, Owner: installer.ServiceUser})
		}, "must belong to root"},
		{"group writable", answersPath, func(f *fakesys.Sys) { _ = f.Chmod(answersPath, engine.FileMode{Perm: 0o660}) }, "mode 0660"},
		{"a symlink", "/root/link.yaml", func(f *fakesys.Sys) { f.PutLink("/root/link.yaml", answersPath) }, "is not a regular file"},
		{"in a directory others can write", "/srv/drop/runner.yaml", func(f *fakesys.Sys) {
			f.PutDir("/srv/drop", engine.FileMode{Perm: 0o777})
			f.Put("/srv/drop/runner.yaml", basicAnswers, 0o600)
		}, "other users can write to"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := runnerHost(debian13)
			writeAnswers(f, basicAnswers)
			tt.tweak(f)
			app, _ := newApp(f, &fakeProtocol{}, "")
			err := app.Install(context.Background(), installer.InstallOptions{Answers: tt.path})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.False(t, f.Ran("useradd"))
		})
	}
	f := runnerHost(debian13)
	writeAnswers(f, basicAnswers)
	f.Put("/tmp/runner.yaml", basicAnswers, 0o600)
	app, out := newApp(f, &fakeProtocol{}, "")
	require.NoError(t, app.Install(context.Background(), installer.InstallOptions{Answers: "/tmp/runner.yaml"}), out.String())
}

func TestTheTokenFileIsCheckedBeforeItIsUsed(t *testing.T) {
	f := runnerHost(debian13)
	writeAnswers(f, basicAnswers)
	_ = f.Chmod(tokenPath, engine.FileMode{Perm: 0o622})
	p := &fakeProtocol{}
	app, _ := newApp(f, p, "")
	err := app.Install(context.Background(), installer.InstallOptions{Answers: answersPath})
	require.ErrorContains(t, err, "Other users can write to the token file")
	assert.Empty(t, p.tokens)

	f = runnerHost(debian13)
	writeAnswers(f, basicAnswers)
	f.PutLink(tokenPath+".link", tokenPath)
	writeAnswers(f, strings.Replace(basicAnswers, tokenPath, tokenPath+".link", 1))
	app, _ = newApp(f, p, "")
	err = app.Install(context.Background(), installer.InstallOptions{Answers: answersPath})
	require.ErrorContains(t, err, "is not a regular file")

	f = runnerHost(debian13)
	writeAnswers(f, basicAnswers)
	_ = f.Chmod(tokenPath, engine.FileMode{Perm: 0o644})
	app, out := newApp(f, p, "")
	require.NoError(t, app.Install(context.Background(), installer.InstallOptions{Answers: answersPath}))
	assert.Contains(t, out.String(), "Warning: other users can read the token file /root/token")
}

func TestValuesThatWouldRewriteRunnerEnvAreRefused(t *testing.T) {
	for _, yaml := range []string{
		"url: https://d.example\nca_file: \"/etc/ca.pem\\nDHOW_RUNNER_DRIVER=fake\"\n",
		"url: https://d.example\nenv: {DHOW_RUNNER_MAX_MEMORY: \"2GiB\\\\\"}\n",
		"url: https://d.example\nenv: {DHOW_RUNNER_LOG_LEVEL: \"'debug'\"}\n",
		"url: https://d.example\nenv: {DHOW_RUNNER_NOT_A_SETTING: x}\n",
		"url: https://d.example\nenv: {LD_PRELOAD: /tmp/x.so}\n",
	} {
		_, err := installer.ParseAnswers(strings.NewReader(yaml))
		require.ErrorIs(t, err, installer.ErrInvalidAnswers, yaml)
	}
	a, err := installer.ParseAnswers(strings.NewReader("url: https://d.example\nenv: {DHOW_RUNNER_MAX_MEMORY: 2GiB, DHOW_RUNNER_EGRESS_ALLOW_PRIVATE_CIDRS: 10.0.0.0/8}\n"))
	require.NoError(t, err)
	assert.Len(t, a.Env, 2)
}

func TestRemoveUserKeepsAnAccountTheInstallerDidNotCreate(t *testing.T) {
	f := runnerHost(debian13)
	f.OK["getent passwd dhow-runner"] = true
	f.Stdout["getent passwd dhow-runner"] = "dhow-runner:x:990:990:CI user:/var/lib/dhow-runner:/usr/sbin/nologin\n"
	p := &fakeProtocol{}
	out, err := install(t, f, p, basicAnswers)
	require.NoError(t, err, out)
	assert.Contains(t, out, "The user dhow-runner already exists, so the install uses it")
	assert.False(t, f.Ran("useradd"))
	out, err = uninstall(f, p, installer.UninstallOptions{Yes: true, RemoveUser: true})
	require.NoError(t, err, out)
	assert.Contains(t, out, "Keep the user dhow-runner: it existed before dhow-runner install")
	assert.False(t, f.Ran("userdel"))
	assert.Contains(t, f.Content(installer.SubUIDPath), "dhow-runner:")
}
