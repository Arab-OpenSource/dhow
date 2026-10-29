//go:build dhowtest

package app_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
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
)

func requireKeysOffDisk(t *testing.T, r *runnertest.Runner, ids ...*identity.Identity) {
	t.Helper()
	for _, id := range ids {
		seed := id.Signing.Seed()
		enc, err := id.Encryption.Bytes()
		require.NoError(t, err)
		for _, secret := range [][]byte{seed, enc, []byte(base64.StdEncoding.EncodeToString(seed)), []byte(hex.EncodeToString(seed))} {
			assert.False(t, r.DiskHolds(secret), "a private key is readable on disk")
		}
	}
}

func sealedPendingPath(r *runnertest.Runner) string {
	return filepath.Join(r.StateDir, config.SealedPendingFileName)
}

func TestASealedIdentityRotatesAndNeverLandsOnDiskInTheClear(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	old := r.Identity()
	r.Seal(runnertest.NewSealer(t))
	_, err := os.Lstat(filepath.Join(r.StateDir, config.IdentityFileName))
	require.ErrorIs(t, err, os.ErrNotExist, "the plain identity is gone once it is sealed")
	requireKeysOffDisk(t, r, old)

	res, err := app.Rotation{Config: r.Config(), Store: r.Store()}.Run(context.Background())
	require.NoError(t, err)
	rotated := r.Identity()
	assert.Equal(t, int32(2), rotated.KeyVersion)
	assert.True(t, rotated.SameKeys(res.Identity))
	requireServerKey(t, r, rotated, 2)
	requireKeysOffDisk(t, r, old, rotated)
	_, err = os.Lstat(sealedPendingPath(r))
	require.ErrorIs(t, err, os.ErrNotExist)
	assert.ErrorIs(t, authenticate(t, r, old), client.ErrCredentials)
	requireJobWithSecretRuns(t, r)
}

func TestTheUnitReadsTheIdentitySystemdOpened(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	r.Seal(runnertest.NewSealer(t))
	unit := r.UnitStore()
	id, err := unit.Load()
	require.NoError(t, err)
	assert.True(t, id.SameKeys(r.Identity()))
	_, err = identity.SealedStore{Dir: r.StateDir}.Load()
	require.ErrorIs(t, err, identity.ErrSealedNoRoot, "without the unit's credential, only root can open it")
	require.ErrorIs(t, unit.Promote(id), identity.ErrSealedNoRoot, "the runner itself never writes the sealed identity")
}

func TestStoresAreChosenFromTheStateDirectory(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	store, err := identity.Open(r.Config(), nil, config.NoOwner)
	require.NoError(t, err)
	assert.False(t, store.Encrypted())
	r.Seal(runnertest.NewSealer(t))
	store, err = identity.Open(r.Config(), nil, config.NoOwner)
	require.NoError(t, err)
	assert.True(t, store.Encrypted())
	require.NoError(t, os.WriteFile(filepath.Join(r.StateDir, config.IdentityFileName), []byte("{}"), 0o600))
	_, err = identity.Open(r.Config(), nil, config.NoOwner)
	require.ErrorIs(t, err, identity.ErrBothKinds)
}

type sealedCrash struct {
	name      string
	rotation  func(store identity.Store, rot *app.Rotation)
	knobs     func(*runnertest.Knobs)
	want      error
	serverHas string
	diskHas   string
	pending   bool
}

func sealedCrashes() []sealedCrash {
	return []sealedCrash{
		{
			name: "after the new keys are sealed, before the server hears of them", want: errCrash, serverHas: "old", diskHas: "old", pending: true,
			rotation: func(store identity.Store, rot *app.Rotation) {
				rot.SavePending = func(_ string, id *identity.Identity) error {
					if err := store.SavePending(id); err != nil {
						return err
					}
					return errCrash
				}
			},
		},
		{
			name: "after the server took the keys, before its reply arrived", want: app.ErrUnconfirmed, serverHas: "new", diskHas: "old",
			pending: true, knobs: func(k *runnertest.Knobs) { k.RotateLoseReply = true },
		},
		{
			name: "after the reply, before the identity was sealed again", want: app.ErrNotSwitched, serverHas: "new", diskHas: "old", pending: true,
			rotation: func(_ identity.Store, rot *app.Rotation) {
				rot.Promote = func(string, string, *identity.Identity) error { return errCrash }
			},
		},
		{
			name: "after the new identity was sealed, before the pending file was removed", want: app.ErrNotSwitched,
			serverHas: "new", diskHas: "new", pending: true,
			rotation: func(store identity.Store, rot *app.Rotation) {
				rot.Promote = func(_, _ string, next *identity.Identity) error {
					if err := store.Save(next); err != nil {
						return err
					}
					return errCrash
				}
			},
		},
	}
}

func TestEveryCrashPointOfASealedRotationLeavesAUsableIdentity(t *testing.T) {
	for _, tc := range sealedCrashes() {
		for _, next := range []string{"the unit starts", "rotate-keys runs again"} {
			t.Run(tc.name+", then "+next, func(t *testing.T) {
				srv := runnertest.NewServer(t)
				r := runnertest.Register(t, srv, srv.URL)
				old := r.Identity()
				r.Seal(runnertest.NewSealer(t))
				rot := app.Rotation{Config: r.Config(), Store: r.Store()}
				if tc.rotation != nil {
					tc.rotation(r.Store(), &rot)
				}
				if tc.knobs != nil {
					srv.Configure(tc.knobs)
				}
				_, err := rot.Run(context.Background())
				require.ErrorIs(t, err, tc.want)
				srv.Configure(func(k *runnertest.Knobs) { k.RotateLoseReply = false })
				assert.Equal(t, tc.diskHas == "old", r.Identity().SameKeys(old), "the sealed identity holds the %s key", tc.diskHas)
				signing, _, _ := srv.RunnerKey()
				assert.Equal(t, tc.serverHas == "old", bytes.Equal(signing, old.SigningPublicKey()), "the server holds the %s key", tc.serverHas)
				_, statErr := os.Lstat(sealedPendingPath(r))
				assert.Equal(t, tc.pending, statErr == nil)
				requireKeysOffDisk(t, r, old)

				if next == "the unit starts" {
					unit := r.UnitStore()
					current, err := unit.Load()
					require.NoError(t, err)
					got, err := app.Recovery{Config: r.Config(), Store: unit, Backoff: runnertest.FastBackoff()}.Run(context.Background(), current)
					if tc.serverHas == "new" && tc.diskHas == "old" {
						require.ErrorIs(t, err, identity.ErrPendingSealed, "only root can finish the rotation, and the runner says how")
						_, statErr := os.Lstat(sealedPendingPath(r))
						require.NoError(t, statErr, "the saved keys the server holds are kept")
					} else {
						require.NoError(t, err)
						require.NoError(t, authenticate(t, r, got.Identity))
						if got.Client != nil {
							got.Client.CloseIdle()
						}
						_, statErr := os.Lstat(sealedPendingPath(r))
						require.ErrorIs(t, statErr, os.ErrNotExist, "keys the server never took are removed")
						return
					}
				}
				res, err := app.Rotation{Config: r.Config(), Store: r.Store()}.Run(context.Background())
				require.NoError(t, err)
				_, statErr = os.Lstat(sealedPendingPath(r))
				require.ErrorIs(t, statErr, os.ErrNotExist)
				rotated := r.Identity()
				assert.True(t, rotated.SameKeys(res.Identity))
				requireServerKey(t, r, rotated, rotated.KeyVersion)
				assert.ErrorIs(t, authenticate(t, r, old), client.ErrCredentials)
				requireKeysOffDisk(t, r, old, rotated)
				requireJobWithSecretRuns(t, r)
			})
		}
	}
}

func TestASymlinkedCredentialIsRefused(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	r.Seal(runnertest.NewSealer(t))
	unit := r.UnitStore()
	link := filepath.Join(t.TempDir(), "identity")
	require.NoError(t, os.Symlink(unit.Unsealed, link))
	_, err := identity.SealedStore{Dir: r.StateDir, Unsealed: link}.Load()
	require.ErrorIs(t, err, config.ErrNotFile)
	sealed := filepath.Join(r.StateDir, config.SealedIdentityFileName)
	require.NoError(t, os.Rename(sealed, sealed+".real"))
	require.NoError(t, os.Symlink(sealed+".real", sealed))
	_, err = r.Store().Load()
	require.ErrorIs(t, err, config.ErrNotFile)
}
