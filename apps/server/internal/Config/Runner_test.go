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

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func writeKeyPair(t *testing.T, keyMode os.FileMode) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "runner.dhow.test"}, DNSNames: []string{"runner.dhow.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
	require.NoError(t, os.Chmod(keyPath, keyMode))
	return certPath, keyPath
}

func with(extra map[string]string) map[string]string {
	m := valid()
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestRunnerPlaneIsOnThePublicListenerByDefault(t *testing.T) {
	cfg, err := config.Load(env(valid()))
	require.NoError(t, err)
	assert.False(t, cfg.RunnerPlaneSeparate())
	assert.False(t, cfg.RunnerPlaneTLS())
}

func TestSeparateRunnerListenerWithTLS(t *testing.T) {
	cert, key := writeKeyPair(t, 0o600)
	cfg, err := config.Load(env(with(map[string]string{
		config.EnvRunnerListenAddr: "0.0.0.0:7443", config.EnvRunnerTLSCertFile: cert, config.EnvRunnerTLSKeyFile: key,
	})))
	require.NoError(t, err)
	assert.True(t, cfg.RunnerPlaneSeparate())
	assert.True(t, cfg.RunnerPlaneTLS())
	assert.Equal(t, "0.0.0.0:7443", cfg.RunnerListenAddr)
}

func TestSeparateRunnerListenerFailsClosed(t *testing.T) {
	cert, key := writeKeyPair(t, 0o600)
	_, openKey := writeKeyPair(t, 0o644)
	otherCert, _ := writeKeyPair(t, 0o600)
	tests := []struct {
		name  string
		extra map[string]string
		want  string
	}{
		{"no TLS outside development", map[string]string{config.EnvRunnerListenAddr: "0.0.0.0:7443"}, "DHOW_RUNNER_TLS_CERT_FILE and DHOW_RUNNER_TLS_KEY_FILE"},
		{"only a certificate", map[string]string{config.EnvRunnerListenAddr: "0.0.0.0:7443", config.EnvRunnerTLSCertFile: cert}, "needs both"},
		{"only a key", map[string]string{config.EnvRunnerListenAddr: "0.0.0.0:7443", config.EnvRunnerTLSKeyFile: key}, "needs both"},
		{"a key other users can read", map[string]string{
			config.EnvRunnerListenAddr: "0.0.0.0:7443", config.EnvRunnerTLSCertFile: cert, config.EnvRunnerTLSKeyFile: openKey,
		}, "readable or writable by other users"},
		{"a missing key", map[string]string{
			config.EnvRunnerListenAddr: "0.0.0.0:7443", config.EnvRunnerTLSCertFile: cert, config.EnvRunnerTLSKeyFile: filepath.Join(t.TempDir(), "none"),
		}, "read key file"},
		{"a certificate that does not match the key", map[string]string{
			config.EnvRunnerListenAddr: "0.0.0.0:7443", config.EnvRunnerTLSCertFile: otherCert, config.EnvRunnerTLSKeyFile: key,
		}, "cannot load the certificate and key"},
		{"TLS files without the listener", map[string]string{config.EnvRunnerTLSCertFile: cert, config.EnvRunnerTLSKeyFile: key}, "DHOW_RUNNER_LISTEN_ADDR is not"},
		{"a bad address", map[string]string{config.EnvRunnerListenAddr: "7443"}, "must be host:port"},
		{"the public address", map[string]string{config.EnvRunnerListenAddr: config.DefaultListenAddr}, "must differ from DHOW_LISTEN_ADDR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.Load(env(with(tt.extra)))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestPlainRunnerListenerOnlyInDevelopment(t *testing.T) {
	cfg, err := config.Load(env(with(map[string]string{
		config.EnvRunnerListenAddr: "127.0.0.1:7443", config.EnvDev: "true", config.EnvDatabaseURL: "postgres://dhow:pw@localhost/dhow?sslmode=disable",
	})))
	require.NoError(t, err)
	assert.True(t, cfg.RunnerPlaneSeparate())
	assert.False(t, cfg.RunnerPlaneTLS())
}
