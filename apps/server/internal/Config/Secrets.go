package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	FileSuffix              = "_FILE"
	EnvSecretKeyFile        = "DHOW_SECRET_KEY_FILE"   //nolint:gosec // The name of an environment variable, not a credential.
	EnvDatabaseURLFile      = "DHOW_DATABASE_URL_FILE" //nolint:gosec // The name of an environment variable, not a credential.
	EnvCredentialsDirectory = "CREDENTIALS_DIRECTORY"
	CredentialSecretKey     = "dhow-secret-key"
	CredentialDatabaseURL   = "dhow-database-url" //nolint:gosec // The name of a systemd credential, not a credential.
	MaxSecretFileBytes      = 64 << 10
	otherUsersPermBits      = 0o007
)

var (
	ErrSecretFileShared   = errors.New("is readable or writable by other users")
	ErrSecretFileEmpty    = errors.New("is empty")
	ErrSecretFileTooLarge = errors.New("is too large")
	ErrSecretFileNotFile  = errors.New("is not a regular file")
)

func secretValue(getenv func(string) string, name, credential string) (string, error) {
	value := getenv(name)
	file := getenv(name + FileSuffix)
	credPath := credentialPath(getenv(EnvCredentialsDirectory), credential)

	var sources []string
	if value != "" {
		sources = append(sources, name)
	}
	if file != "" {
		sources = append(sources, name+FileSuffix)
	}
	if credPath != "" {
		sources = append(sources, "the systemd credential "+credential)
	}
	if len(sources) > 1 {
		return "", fmt.Errorf("is set more than once (%s); keep exactly one", strings.Join(sources, ", "))
	}
	switch {
	case file != "":
		return ReadSecretFile(file)
	case credPath != "":
		return ReadSecretFile(credPath)
	default:
		return value, nil
	}
}

func credentialPath(dir, name string) string {
	if dir == "" {
		return ""
	}
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

func ReadSecretFile(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // The path is the operator's own configuration, and the file must be private to the service.
	if err != nil {
		return "", fmt.Errorf("read secret file: %w", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("read secret file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s %w", path, ErrSecretFileNotFile)
	}
	if info.Mode().Perm()&otherUsersPermBits != 0 {
		return "", fmt.Errorf("%s %w (mode %04o); run chmod 600 %s", path, ErrSecretFileShared, info.Mode().Perm(), path)
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxSecretFileBytes+1))
	if err != nil {
		return "", fmt.Errorf("read secret file: %w", err)
	}
	if len(data) > MaxSecretFileBytes {
		return "", fmt.Errorf("%s %w (more than %d bytes)", path, ErrSecretFileTooLarge, MaxSecretFileBytes)
	}
	v := trimOneNewline(string(data))
	if v == "" {
		return "", fmt.Errorf("%s %w", path, ErrSecretFileEmpty)
	}
	return v, nil
}

func trimOneNewline(s string) string {
	if strings.HasSuffix(s, "\r\n") {
		return strings.TrimSuffix(s, "\r\n")
	}
	return strings.TrimSuffix(s, "\n")
}
