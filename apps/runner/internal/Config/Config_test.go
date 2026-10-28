package config_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
)

type env map[string]string

func (e env) get(k string) string { return e[k] }

func base(t *testing.T) env {
	t.Helper()
	return env{config.EnvURL: "https://dhow.example.com", config.EnvStateDir: filepath.Join(t.TempDir(), "state")}
}

func big8() config.Machine {
	return config.Machine{CPUMillis: 8000, MemoryBytes: 16 << 30, PidMax: 4 << 20, DiskFree: func(string) (int64, error) { return 50 << 30, nil }}
}

func load(t *testing.T, e env, dev bool, m config.Machine) (config.Config, error) {
	t.Helper()
	return config.Load(e.get, config.Options{Dev: dev, Machine: m})
}

func TestDefaults(t *testing.T) {
	cfg, err := load(t, base(t), false, big8())
	require.NoError(t, err)
	assert.Equal(t, "https://dhow.example.com", cfg.Origin)
	assert.Equal(t, config.DriverPodman, cfg.Driver)
	assert.Equal(t, config.DiskModeTmpfs, cfg.DiskMode)
	assert.Equal(t, 1, cfg.Capacity)
	assert.EqualValues(t, 8000, cfg.Limits.CPUMillis, "every CPU by default")
	assert.EqualValues(t, 2<<30, cfg.Limits.MemoryBytes)
	assert.Equal(t, 6*time.Hour, cfg.Limits.Timeout)
	assert.Equal(t, 10*time.Second, cfg.CancelGrace)
	assert.False(t, cfg.AllowUnpinnedImages)
}

func TestURLRules(t *testing.T) {
	cases := []struct {
		url string
		dev bool
		ok  bool
	}{
		{"https://dhow.example.com", false, true},
		{"https://dhow.example.com/", false, true},
		{"https://dhow.example.com:8443", false, true},
		{"http://dhow.example.com", false, false},
		{"http://dhow.example.com", true, false},
		{"http://localhost:8080", false, false},
		{"http://localhost:8080", true, true},
		{"http://127.0.0.1:8080", true, true},
		{"http://[::1]:8080", true, true},
		{"https://user:pass@dhow.example.com", false, false},
		{"https://dhow.example.com/sub", false, false},
		{"https://dhow.example.com?x=1", false, false},
		{"ftp://dhow.example.com", false, false},
		{"dhow.example.com", false, false},
	}
	for _, c := range cases {
		_, err := config.ParseServerURL(c.url, c.dev)
		assert.Equal(t, c.ok, err == nil, "%s dev=%v: %v", c.url, c.dev, err)
	}
}

func TestOriginOverride(t *testing.T) {
	e := base(t)
	e[config.EnvURL] = "https://runners.internal:7443"
	e[config.EnvServerOrigin] = "https://dhow.example.com:443"
	cfg, err := load(t, e, false, big8())
	require.NoError(t, err)
	assert.Equal(t, "https://dhow.example.com", cfg.Origin)
	e[config.EnvServerOrigin] = "http://dhow.example.com"
	_, err = load(t, e, false, big8())
	assert.ErrorIs(t, err, config.ErrNotHTTPS)
	_, err = load(t, e, true, big8())
	assert.ErrorIs(t, err, config.ErrDevLocalOnly, "--dev allows a plain http origin on loopback only")
	e[config.EnvServerOrigin] = "http://127.0.0.1:8080"
	_, err = load(t, e, true, big8())
	assert.NoError(t, err)
}

func TestLimitsAboveTheMachineFailClosed(t *testing.T) {
	small := config.Machine{CPUMillis: 2000, MemoryBytes: 4 << 30, PidMax: 1500}
	cases := map[string]env{
		"cpu":    {config.EnvMaxCPU: "4"},
		"memory": {config.EnvMaxMemory: "3GiB", config.EnvMaxDisk: "2GiB"},
		"pids":   {config.EnvMaxPids: "2000", config.EnvMaxMemory: "1GiB", config.EnvMaxDisk: "1GiB"},
	}
	for name, extra := range cases {
		e := base(t)
		e[config.EnvMaxCPU] = "1"
		e[config.EnvMaxMemory], e[config.EnvMaxDisk] = "1GiB", "1GiB"
		for k, v := range extra {
			e[k] = v
		}
		_, err := load(t, e, false, small)
		assert.ErrorIs(t, err, config.ErrAboveMachine, name)
	}
	e := base(t)
	_, err := load(t, e, false, small)
	assert.ErrorIs(t, err, config.ErrAboveMachine, "the tmpfs disk counts as memory, so the defaults do not fit 4 GiB")
	e[config.EnvDiskMode] = config.DiskModeDirectory
	e[config.EnvMaxCPU] = "2"
	small.DiskFree = func(string) (int64, error) { return 1 << 30, nil }
	_, err = load(t, e, false, small)
	assert.ErrorIs(t, err, config.ErrAboveMachine, "the directory disk must fit the free space")
	small.DiskFree = func(string) (int64, error) { return 10 << 30, nil }
	_, err = load(t, e, false, small)
	assert.NoError(t, err)
}

func TestTheStateDiskMustHoldTheLogSpillFiles(t *testing.T) {
	m := big8()
	m.DiskFree = func(string) (int64, error) { return 100 << 20, nil }
	_, err := load(t, base(t), false, m)
	assert.ErrorIs(t, err, config.ErrAboveMachine, "a 256 MiB spill file per job does not fit 100 MiB")
	m.DiskFree = func(string) (int64, error) { return 600 << 20, nil }
	e := base(t)
	_, err = load(t, e, false, m)
	assert.NoError(t, err)
	e[config.EnvCapacity] = "3"
	_, err = load(t, e, false, m)
	assert.ErrorIs(t, err, config.ErrAboveMachine, "three jobs need three spill files")
}

func TestInvalidValuesAreRefused(t *testing.T) {
	cases := map[string]string{
		config.EnvCapacity: "17", config.EnvMaxMemory: "lots", config.EnvMaxTimeout: "48h", config.EnvCancelGrace: "2m",
		config.EnvDriver: "docker", config.EnvDiskMode: "nfs", config.EnvAllowUnpinned: "maybe", config.EnvLogLevel: "loud",
		config.EnvEgressAllowPrivate: "10.0.0.0/8,nope", config.EnvStateDir: "relative/dir", config.EnvRunscPath: "/does/not/exist",
		config.EnvCAFile: "/does/not/exist",
	}
	for k, v := range cases {
		e := base(t)
		e[k] = v
		_, err := load(t, e, false, big8())
		assert.Error(t, err, "%s=%s", k, v)
	}
}

func TestTheFakeDriverExistsOnlyInTestBuilds(t *testing.T) {
	e := base(t)
	e[config.EnvDriver] = config.DriverFake
	_, err := load(t, e, false, big8())
	if fakeBuilt {
		assert.NoError(t, err)
	} else {
		assert.Error(t, err)
	}
}

func TestStateFilesWithOpenPermissionsFailClosed(t *testing.T) {
	e := base(t)
	dir := e[config.EnvStateDir]
	require.NoError(t, os.MkdirAll(dir, 0o700))
	id := filepath.Join(dir, config.IdentityFileName)
	require.NoError(t, os.WriteFile(id, []byte("{}"), 0o600))
	_, err := load(t, e, false, big8())
	require.NoError(t, err)

	require.NoError(t, os.Chmod(id, 0o640))
	_, err = load(t, e, false, big8())
	assert.ErrorIs(t, err, config.ErrOpenMode)

	require.NoError(t, os.Chmod(id, 0o600))
	require.NoError(t, os.Chmod(dir, 0o777))
	_, err = load(t, e, false, big8())
	assert.ErrorIs(t, err, config.ErrSharedDir)
	require.NoError(t, os.Chmod(dir, 0o700))

	require.NoError(t, os.Remove(id))
	require.NoError(t, os.Symlink("/etc/hostname", id))
	_, err = load(t, e, false, big8())
	assert.ErrorIs(t, err, config.ErrNotFile)
}

func TestWriteFileAtomicIsPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	require.NoError(t, config.WriteFileAtomic(path, []byte("one")))
	require.NoError(t, config.WriteFileAtomic(path, []byte("two")))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	raw, err := config.ReadPrivateFile(path)
	require.NoError(t, err)
	assert.Equal(t, "two", string(raw))
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary file is left behind")
}

func TestCAFileMustHoldACertificate(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.pem")
	require.NoError(t, os.WriteFile(bad, []byte("not a cert"), 0o600))
	e := base(t)
	e[config.EnvCAFile] = bad
	_, err := load(t, e, false, big8())
	assert.ErrorIs(t, err, config.ErrCAFileNoCerts)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "ca"}, NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	good := filepath.Join(dir, "good.pem")
	require.NoError(t, os.WriteFile(good, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	e[config.EnvCAFile] = good
	_, err = load(t, e, false, big8())
	assert.NoError(t, err)
}

func TestSizes(t *testing.T) {
	for in, want := range map[string]int64{"512MiB": 512 << 20, "2GiB": 2 << 30, "1024": 1024, "4G": 4 << 30, "64 KiB": 64 << 10} {
		got, err := config.ParseSize(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "-1", "1.5GiB", "GiB", "99999999999TiB"} {
		_, err := config.ParseSize(in)
		assert.Error(t, err, in)
	}
}

func TestDetectMachine(t *testing.T) {
	m, err := config.DetectMachine()
	require.NoError(t, err)
	assert.Positive(t, m.CPUMillis)
	assert.Positive(t, m.MemoryBytes)
	free, err := m.DiskFree(t.TempDir())
	require.NoError(t, err)
	assert.Positive(t, free)
}
