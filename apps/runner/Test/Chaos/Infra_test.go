//go:build chaos

package chaos_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	postgresImage = "docker.io/library/postgres:18"
	registryImage = "docker.io/library/registry:2"
	registryPort  = "5000"
	slowPort      = "5001"
	pgContainer   = "dhow-chaos-postgres"
	regContainer  = "dhow-chaos-registry"
	commandWait   = 5 * time.Minute
)

func podman(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandWait)
	defer cancel()
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("podman %s: %w: %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func writeCerts(dir string) (string, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", err
	}
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "dhow chaos CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return "", err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", err
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
	if err != nil {
		return "", err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", err
	}
	caPath := filepath.Join(dir, "ca.crt")
	files := map[string][]byte{
		caPath:                             pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		filepath.Join(dir, "registry.crt"): pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		filepath.Join(dir, "registry.key"): pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	}
	for name, body := range files {
		if err := os.WriteFile(name, body, 0o600); err != nil {
			return "", err
		}
	}
	return caPath, nil
}

func trustRegistry(home, caPath string) error {
	ca, err := os.ReadFile(caPath)
	if err != nil {
		return err
	}
	for _, port := range []string{registryPort, slowPort} {
		dir := filepath.Join(home, ".config", "containers", "certs.d", "localhost:"+port)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "ca.crt"), ca, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func startPostgres(port, password string) error {
	_, _ = podman("rm", "--force", pgContainer)
	if _, err := podman("run", "--detach", "--name", pgContainer, "--publish", "127.0.0.1:"+port+":5432",
		"--env", "POSTGRES_PASSWORD="+password, "--env", "POSTGRES_DB=dhow", postgresImage); err != nil {
		return err
	}
	return waitPostgres()
}

func waitPostgres() error {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := podman("exec", pgContainer, "pg_isready", "-U", "postgres", "-d", "dhow"); err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("postgres did not become ready")
}

func restartPostgres() error {
	if _, err := podman("restart", "--time", "1", pgContainer); err != nil {
		return err
	}
	return waitPostgres()
}

func startRegistry(certs string) error {
	_, _ = podman("rm", "--force", regContainer)
	_, err := podman("run", "--detach", "--name", regContainer, "--publish", "127.0.0.1:"+registryPort+":5000",
		"--volume", certs+":/certs:ro",
		"--env", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/registry.crt", "--env", "REGISTRY_HTTP_TLS_KEY=/certs/registry.key",
		registryImage)
	return err
}

func pushImage(work, busybox, name string, padding int) (string, error) {
	dir, err := os.MkdirTemp(work, "image-")
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(busybox)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "busybox"), data, 0o755); err != nil {
		return "", err
	}
	lines := []string{"FROM scratch", "COPY --chmod=0755 busybox /bin/busybox", "COPY --chmod=0755 busybox /bin/sh"}
	if padding > 0 {
		pad := make([]byte, padding)
		if _, err := rand.Read(pad); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(dir, "padding"), pad, 0o644); err != nil {
			return "", err
		}
		lines = append(lines, "COPY padding /padding")
	}
	if err := os.WriteFile(filepath.Join(dir, "Containerfile"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return "", err
	}
	tag := "localhost:" + registryPort + "/dhow/" + name + ":chaos"
	if _, err := podman("build", "--pull=never", "--network=none", "--tag", tag, "--file", filepath.Join(dir, "Containerfile"), dir); err != nil {
		return "", err
	}
	digestFile := filepath.Join(dir, "digest")
	if _, err := podman("push", "--cert-dir", filepath.Join(work, "pushcerts"), "--digestfile", digestFile, tag); err != nil {
		return "", err
	}
	digest, err := os.ReadFile(digestFile)
	if err != nil {
		return "", err
	}
	return "dhow/" + name + "@" + strings.TrimSpace(string(digest)), nil
}
