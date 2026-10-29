package e2e_test

import (
	"context"
	"crypto/ed25519"
	"crypto/hpke"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

type procIdentityFile struct {
	Version       int
	RunnerId      string
	KeyVersion    int32
	SigningSeed   []byte
	EncryptionKey []byte
}

func procIdentity(t *testing.T, r *procRunner) procIdentityFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(r.dir, "state", "identity"))
	require.NoError(t, err)
	var f procIdentityFile
	require.NoError(t, json.Unmarshal(raw, &f))
	require.Len(t, f.SigningSeed, ed25519.SeedSize)
	return f
}

func (f procIdentityFile) runnerIdentity(t *testing.T) *apptest.RunnerIdentity {
	t.Helper()
	enc, err := hpke.MLKEM768X25519().NewPrivateKey(f.EncryptionKey)
	require.NoError(t, err)
	return &apptest.RunnerIdentity{Signing: ed25519.NewKeyFromSeed(f.SigningSeed), Encryption: enc}
}

func TestRotateKeysSwapsTheRunnerKeyAndTheNewKeyLeasesJobs(t *testing.T) {
	w := newWorldWithSecrets(t, map[string]string{"API_TOKEN": procSecret})
	srv, ca := procServe(t, w)
	r := newProcRunner(t, w, srv, ca)
	before := procIdentity(t, r)
	assert.Equal(t, 2, before.Version)
	assert.Equal(t, int32(1), before.KeyVersion)

	cmd := exec.CommandContext(t.Context(), r.bin, "rotate-keys")
	cmd.Env = r.env
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "key version 2")
	assert.NotContains(t, string(out), string(before.SigningSeed))

	after := procIdentity(t, r)
	assert.Equal(t, before.RunnerId, after.RunnerId)
	assert.Equal(t, int32(2), after.KeyVersion)
	assert.NotEqual(t, before.SigningSeed, after.SigningSeed)
	_, err = os.Lstat(filepath.Join(r.dir, "state", "identity.pending"))
	assert.ErrorIs(t, err, os.ErrNotExist)
	var keyVersion int32
	var signing []byte
	require.NoError(t, w.env.Pool.QueryRow(context.Background(),
		`SELECT "KeyVersion", "SigningPublicKey" FROM "Runners" WHERE "Id" = $1`, before.RunnerId).Scan(&keyVersion, &signing))
	assert.Equal(t, int32(2), keyVersion)
	assert.Equal(t, after.runnerIdentity(t).SigningPublicKey(), signing)

	rc := w.env.RunnerClient(apptest.RunnerClientOptions{Remote: "198.51.100.77:5555"})
	_, err = w.env.Authenticate(rc, before.runnerIdentity(t), before.RunnerId)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), "the server refuses the old key")

	job := w.job(t, apptest.JobSpec{
		"Image":     apptest.TestImage,
		"Steps":     apptest.Steps(map[string]any{"Name": "print", "Run": "env"}),
		"Resources": map[string]any{"MemoryBytes": 256 << 20, "DiskBytes": 128 << 20, "Pids": 256},
		"Secrets":   []map[string]any{{"Name": "API_TOKEN", "AsEnv": "API_TOKEN"}},
	}, nil)
	r.start(`{"Steps":[{"Output":[{"Data":"token=","FromEnv":"API_TOKEN","Encoding":"raw"},{"Data":"\n"}]}]}`)
	procWaitFor(t, r, "the job to succeed with the new key", func() bool { return w.get(t, job).Job.State == "succeeded" })
	r.stop()
	assert.Contains(t, procLogText(t, w, job), "token=***\n", "the secret sealed for the new key opened and was masked")
	assert.NotContains(t, r.stderr.String(), procSecret)
}
