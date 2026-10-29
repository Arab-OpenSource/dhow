//go:build dhowtest

package app_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/Khaymah/dhow/apps/runner/internal/App"
	runnertest "github.com/Khaymah/dhow/apps/runner/internal/App/Testing"
	client "github.com/Khaymah/dhow/apps/runner/internal/Client"
	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	identity "github.com/Khaymah/dhow/apps/runner/internal/Identity"
	journal "github.com/Khaymah/dhow/apps/runner/internal/Journal"
	fake "github.com/Khaymah/dhow/apps/runner/internal/Sandbox/Fake"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const rotatedSecret = "rotated-secret-value"

var errCrash = errors.New("simulated crash")

func authenticate(t *testing.T, r *runnertest.Runner, id *identity.Identity) error {
	t.Helper()
	u, err := config.ParseServerURL(r.Server.URL, false)
	require.NoError(t, err)
	c, err := client.New(client.Options{BaseURL: u, CAFile: r.Server.CAFile, Version: "test", Identity: id})
	require.NoError(t, err)
	defer c.CloseIdle()
	return c.Authenticate(context.Background())
}

func pendingPath(r *runnertest.Runner) string {
	return filepath.Join(r.StateDir, config.PendingFileName)
}

func requireNoPending(t *testing.T, r *runnertest.Runner) {
	t.Helper()
	_, err := os.Lstat(pendingPath(r))
	require.ErrorIs(t, err, os.ErrNotExist, "no pending identity is left behind")
}

func recoverIdentity(t *testing.T, r *runnertest.Runner) *identity.Identity {
	t.Helper()
	got, err := app.Recovery{Config: r.Config(), Backoff: runnertest.FastBackoff()}.Run(context.Background(), r.Identity())
	require.NoError(t, err)
	return got.Identity
}

func requireJobWithSecretRuns(t *testing.T, r *runnertest.Runner) {
	t.Helper()
	spec := runnertest.Spec(&runnerv1.Step{Name: "print", Run: "env"})
	spec.Secrets = []*runnerv1.SecretRef{{Name: "API_TOKEN", Target: &runnerv1.SecretRef_AsEnv{AsEnv: "API_TOKEN"}}}
	r.Server.Enqueue(spec, map[string][]byte{"API_TOKEN": []byte(rotatedSecret)})
	running := r.Start(fake.Recording{Steps: []fake.Step{{Output: []fake.Output{{Data: "ok\n"}}}}}, runnertest.StartOptions{})
	out := running.Next(t, finishWithin)
	require.Equal(t, runnerv1.AttemptResult_ATTEMPT_RESULT_SUCCEEDED, out.Result, "the job opens its secrets with the identity on disk")
	require.NoError(t, running.Stop())
}

func requireServerKey(t *testing.T, r *runnertest.Runner, id *identity.Identity, version int32) {
	t.Helper()
	signing, encryption, v := r.Server.RunnerKey()
	assert.Equal(t, id.SigningPublicKey(), []byte(signing))
	assert.Equal(t, id.EncryptionPublicKey(), encryption)
	assert.Equal(t, version, v)
}

func TestRotateKeysSwapsTheIdentityAndTheOldKeyStopsWorking(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	old := r.Identity()
	assert.Equal(t, int32(identity.FirstKeyVersion), old.KeyVersion)

	var out, errOut bytes.Buffer
	err := app.Main(context.Background(), []string{app.CommandRotateKeys}, app.Env{
		Getenv: r.Env.Get, Stdio: app.Stdio{In: bytes.NewReader(nil), Out: &out, Err: &errOut},
		Machine: func() (config.Machine, error) { return config.Machine{}, nil }, Harden: func() error { return nil },
	})
	require.NoError(t, err, errOut.String())
	assert.Contains(t, out.String(), "key version 2")

	rotated := r.Identity()
	assert.False(t, rotated.SameKeys(old))
	assert.Equal(t, int32(2), rotated.KeyVersion)
	assert.Equal(t, old.RunnerID, rotated.RunnerID)
	requireServerKey(t, r, rotated, 2)
	requireNoPending(t, r)
	info, err := os.Stat(filepath.Join(r.StateDir, config.IdentityFileName))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	for _, seed := range [][]byte{old.Signing.Seed(), rotated.Signing.Seed()} {
		assert.NotContains(t, out.String()+errOut.String(), string(seed))
	}

	assert.ErrorIs(t, authenticate(t, r, old), client.ErrCredentials, "the server refuses the old key")
	require.NoError(t, authenticate(t, r, rotated))
	requireJobWithSecretRuns(t, r)
}

func TestRotateKeysRefusesWhileTheRunnerRuns(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	lock, err := config.LockState(filepath.Join(r.StateDir, config.LockFileName))
	require.NoError(t, err)
	_, err = app.Rotation{Config: r.Config()}.Run(context.Background())
	require.ErrorIs(t, err, app.ErrRunnerRunning)
	_, err = cli(t, r.Env, "", app.CommandRun)
	require.ErrorIs(t, err, config.ErrStateBusy, "a second run is refused too")
	require.NoError(t, lock.Release())
	requireNoPending(t, r)
	assert.Equal(t, int32(1), r.Identity().KeyVersion)
}

func TestRotateKeysRefusesWithUnreportedAttempts(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	j, err := journal.Open(filepath.Join(r.StateDir, config.JournalFileName))
	require.NoError(t, err)
	require.NoError(t, j.Add(journal.Entry{
		LeaseID: "0190c8a2-0000-7000-8000-000000000001", AttemptID: "0190c8a2-0000-7000-8000-000000000002",
		JobID: "0190c8a2-0000-7000-8000-000000000003", FencingToken: 1,
	}))
	_, err = app.Rotation{Config: r.Config()}.Run(context.Background())
	require.ErrorIs(t, err, app.ErrJournalBusy)
	requireNoPending(t, r)
	_, _, v := srv.RunnerKey()
	assert.Equal(t, int32(1), v)
}

type crashCase struct {
	name       string
	rotation   func(*runnertest.Runner, *app.Rotation)
	knobs      func(*runnertest.Knobs)
	want       error
	serverHas  string
	diskHas    string
	pendingAll bool
}

func crashCases() []crashCase {
	return []crashCase{
		{
			name: "while the new keys are written", want: errCrash, serverHas: "old", diskHas: "old",
			rotation: func(r *runnertest.Runner, rot *app.Rotation) {
				rot.SavePending = func(path string, _ *identity.Identity) error {
					_ = os.WriteFile(filepath.Join(filepath.Dir(path), ".identity.pending.tmp-1"), []byte("partial"), 0o600)
					return errCrash
				}
			},
		},
		{
			name: "after the new keys are saved, before the server hears of them", want: errCrash, serverHas: "old", diskHas: "old",
			pendingAll: true,
			rotation: func(r *runnertest.Runner, rot *app.Rotation) {
				rot.SavePending = func(path string, id *identity.Identity) error {
					if err := identity.Save(path, id); err != nil {
						return err
					}
					return errCrash
				}
			},
		},
		{
			name: "after the server took the keys, before its reply arrived", want: app.ErrUnconfirmed, serverHas: "new", diskHas: "old",
			pendingAll: true, knobs: func(k *runnertest.Knobs) { k.RotateLoseReply = true },
		},
		{
			name: "after the reply, before the identity file was switched", want: app.ErrNotSwitched, serverHas: "new", diskHas: "old",
			pendingAll: true,
			rotation: func(_ *runnertest.Runner, rot *app.Rotation) {
				rot.Promote = func(string, string) error { return errCrash }
			},
		},
		{
			name: "after the rename, before the directory was synced", want: app.ErrNotSwitched, serverHas: "new", diskHas: "new",
			rotation: func(_ *runnertest.Runner, rot *app.Rotation) {
				rot.Promote = func(from, to string) error {
					if err := os.Rename(from, to); err != nil {
						return err
					}
					return errCrash
				}
			},
		},
	}
}

func TestEveryCrashPointOfRotateKeysLeavesAUsableIdentity(t *testing.T) {
	for _, tc := range crashCases() {
		for _, next := range []string{"the runner starts", "rotate-keys runs again"} {
			t.Run(tc.name+", then "+next, func(t *testing.T) {
				srv := runnertest.NewServer(t)
				r := runnertest.Register(t, srv, srv.URL)
				old := r.Identity()
				rot := app.Rotation{Config: r.Config()}
				if tc.rotation != nil {
					tc.rotation(r, &rot)
				}
				if tc.knobs != nil {
					srv.Configure(tc.knobs)
				}
				_, err := rot.Run(context.Background())
				require.ErrorIs(t, err, tc.want)
				srv.Configure(func(k *runnertest.Knobs) { k.RotateLoseReply = false })

				onDisk := r.Identity()
				assert.Equal(t, tc.diskHas == "old", onDisk.SameKeys(old), "the identity file holds the %s key", tc.diskHas)
				signing, _, _ := srv.RunnerKey()
				assert.Equal(t, tc.serverHas == "old", bytes.Equal(signing, old.SigningPublicKey()), "the server holds the %s key", tc.serverHas)
				_, statErr := os.Lstat(pendingPath(r))
				assert.Equal(t, tc.pendingAll, statErr == nil, "a pending identity is kept until the server confirms it")
				var pending *identity.Identity
				if tc.pendingAll {
					pending, err = identity.Load(pendingPath(r))
					require.NoError(t, err)
				}

				if next == "the runner starts" {
					id := recoverIdentity(t, r)
					stale, err := filepath.Glob(filepath.Join(r.StateDir, ".identity*.tmp-*"))
					require.NoError(t, err)
					assert.Empty(t, stale, "half-written identity files are removed")
					require.NoError(t, authenticate(t, r, id), "the identity the runner starts with is accepted")
					if tc.serverHas == "new" {
						requireNoPending(t, r)
						assert.False(t, r.Identity().SameKeys(old))
					}
					requireJobWithSecretRuns(t, r)
					return
				}
				res, err := app.Rotation{Config: r.Config()}.Run(context.Background())
				require.NoError(t, err)
				requireNoPending(t, r)
				rotated := r.Identity()
				assert.True(t, rotated.SameKeys(res.Identity))
				if pending != nil {
					assert.True(t, rotated.SameKeys(pending), "a resumed rotation reuses the keys it saved")
				}
				version := int32(2)
				if tc.diskHas == "new" {
					version = 3
				}
				assert.Equal(t, version, rotated.KeyVersion, "a finished rotation is not repeated, a new run rotates again")
				requireServerKey(t, r, rotated, version)
				assert.ErrorIs(t, authenticate(t, r, old), client.ErrCredentials)
				stale, err := filepath.Glob(filepath.Join(r.StateDir, ".identity*.tmp-*"))
				require.NoError(t, err)
				assert.Empty(t, stale, "half-written identity files are removed")
				requireJobWithSecretRuns(t, r)
			})
		}
	}
}

func TestRecoveryKeepsTheCurrentKeyWhenTheServerNeverSawTheNewOne(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	old := r.Identity()
	keys, err := identity.GenerateKeys()
	require.NoError(t, err)
	next, err := old.Next(keys)
	require.NoError(t, err)
	require.NoError(t, identity.Save(pendingPath(r), next))

	got := recoverIdentity(t, r)
	assert.True(t, got.SameKeys(old))
	_, err = os.Lstat(pendingPath(r))
	require.NoError(t, err, "the pending keys stay, so rotate-keys can finish with them")
}

func TestRecoveryRefusesAPendingIdentityOfAnotherRunner(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	other := *r.Identity()
	other.RunnerID = "0190c8a2-0000-7000-8000-0000000000ff"
	require.NoError(t, identity.Save(pendingPath(r), &other))
	_, err := app.Recovery{Config: r.Config()}.Run(context.Background(), r.Identity())
	require.ErrorIs(t, err, identity.ErrPendingMismatch)
	_, err = app.Rotation{Config: r.Config()}.Run(context.Background())
	require.ErrorIs(t, err, identity.ErrPendingMismatch)
}

func TestRecoveryDropsAPendingIdentityThatIsAlreadyInUse(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	require.NoError(t, identity.Save(pendingPath(r), r.Identity()))
	got := recoverIdentity(t, r)
	assert.True(t, got.SameKeys(r.Identity()))
	requireNoPending(t, r)
}

func TestRunFinishesARotationTheServerTookBeforeItStarts(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	old := r.Identity()
	srv.Configure(func(k *runnertest.Knobs) { k.RotateLoseReply = true })
	_, err := app.Rotation{Config: r.Config()}.Run(context.Background())
	require.ErrorIs(t, err, app.ErrUnconfirmed)
	srv.Configure(func(k *runnertest.Knobs) { k.RotateLoseReply = false })

	recording := filepath.Join(t.TempDir(), "recording.json")
	require.NoError(t, os.WriteFile(recording, []byte(`{"Steps":[]}`), 0o600))
	env := runnertest.Env{config.EnvFakeRecording: recording}
	for k, v := range r.Env {
		env[k] = v
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var errOut bytes.Buffer
	go func() {
		done <- app.Main(ctx, []string{app.CommandRun}, app.Env{
			Getenv: env.Get, Stdio: app.Stdio{In: bytes.NewReader(nil), Out: &bytes.Buffer{}, Err: &errOut},
			Machine: func() (config.Machine, error) { return config.Machine{}, nil }, Harden: func() error { return nil },
		})
	}()
	runnertest.Eventually(t, finishWithin, func() bool {
		_, statErr := os.Lstat(pendingPath(r))
		return errors.Is(statErr, os.ErrNotExist)
	}, "the runner to switch to the new identity")
	cancel()
	require.NoError(t, <-done)
	assert.False(t, r.Identity().SameKeys(old))
	requireServerKey(t, r, r.Identity(), 2)
	assert.Contains(t, errOut.String(), "finished an interrupted key rotation")
}
