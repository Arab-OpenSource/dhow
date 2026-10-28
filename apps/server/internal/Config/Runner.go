package config

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	EnvRunnerListenAddr  = "DHOW_RUNNER_LISTEN_ADDR"
	EnvRunnerTLSCertFile = "DHOW_RUNNER_TLS_CERT_FILE"
	EnvRunnerTLSKeyFile  = "DHOW_RUNNER_TLS_KEY_FILE"
)

const (
	RunnerAPIPrefix       = "/runner-api"
	RunnerBodyMaxBytes    = 1 << 20
	RunnerReadMaxBytes    = 256 << 10
	RunnerProtocolHeader  = "Dhow-Protocol"
	RunnerProtocolVersion = "1"
	RunnerVersionHeader   = "Dhow-Runner-Version"
)

const (
	RunnerRegistrationTokenPrefix = "dhow_rrt_" //nolint:gosec // A token prefix, not a credential.
	RunnerAccessTokenPrefix       = "dhow_rat_" //nolint:gosec // A token prefix, not a credential.
	RunnerTokenBytes              = 32
	RunnerTokenDefaultTTL         = time.Hour
	RunnerTokenMinTTL             = time.Minute
	RunnerTokenMaxTTL             = 24 * time.Hour
	RunnerTokenDefaultUses        = 1
	RunnerTokenMaxUses            = 50
	RunnerChallengeTTL            = 60 * time.Second
	RunnerAccessTokenTTL          = 15 * time.Minute
	RunnerCloneWindow             = 10 * time.Minute
	RunnerCloneMaxSwitches        = 3
	RunnerOnlineWindow            = 2 * time.Minute
	RunnerTouchInterval           = 30 * time.Second
	RunnerRegisterFreeAttempts    = 20
	RunnerAuthFreeAttempts        = 20
	RunnerMaxCapacity             = 16
	RunnerMaxLabels               = 32
	RunnerLabelMaxLength          = 128
	RunnerNameMaxLength           = 100
	RunnerTokenDescriptionMax     = 200
	RunnerServerKeyVersion        = 1
)

const (
	RunnerSigningKeyLabel        = "dhow/runner-plane-signing/v1"
	RunnerRegistrationTokenLabel = "dhow/runner-registration-token/v1" //nolint:gosec // A key derivation label, not a credential.
	RunnerChallengeKeyLabel      = "dhow/runner-challenge/v1"
)

var (
	ErrRunnerTLSRequired = errors.New("needs DHOW_RUNNER_TLS_CERT_FILE and DHOW_RUNNER_TLS_KEY_FILE (plain HTTP is allowed only with DHOW_DEV=true)")
	ErrRunnerTLSWithout  = errors.New("is set but DHOW_RUNNER_LISTEN_ADDR is not; the runner TLS files are used only by the separate runner listener")
	ErrRunnerTLSHalf     = errors.New("needs both DHOW_RUNNER_TLS_CERT_FILE and DHOW_RUNNER_TLS_KEY_FILE")
	ErrRunnerSameAddr    = errors.New("must differ from DHOW_LISTEN_ADDR")
)

type runnerListener struct {
	addr, cert, key string
}

func parseRunnerListener(getenv func(string) string, dev bool, publicAddr string) (runnerListener, error) {
	l := runnerListener{addr: getenv(EnvRunnerListenAddr), cert: getenv(EnvRunnerTLSCertFile), key: getenv(EnvRunnerTLSKeyFile)}
	if l.addr == "" {
		if l.cert != "" || l.key != "" {
			return runnerListener{}, fmt.Errorf("%s: %w", EnvRunnerTLSCertFile+"/"+EnvRunnerTLSKeyFile, ErrRunnerTLSWithout)
		}
		return l, nil
	}
	addr, err := parseListenAddr(l.addr)
	if err != nil {
		return runnerListener{}, fmt.Errorf("%s: %w", EnvRunnerListenAddr, err)
	}
	if addr == publicAddr {
		return runnerListener{}, fmt.Errorf("%s: %w", EnvRunnerListenAddr, ErrRunnerSameAddr)
	}
	switch {
	case l.cert == "" && l.key == "" && dev:
		return l, nil
	case l.cert == "" && l.key == "":
		return runnerListener{}, fmt.Errorf("%s: %w", EnvRunnerListenAddr, ErrRunnerTLSRequired)
	case l.cert == "" || l.key == "":
		return runnerListener{}, fmt.Errorf("%s: %w", EnvRunnerListenAddr, ErrRunnerTLSHalf)
	}
	if err := checkPrivateFile(l.key); err != nil {
		return runnerListener{}, fmt.Errorf("%s: %w", EnvRunnerTLSKeyFile, err)
	}
	if _, err := tls.LoadX509KeyPair(l.cert, l.key); err != nil {
		return runnerListener{}, fmt.Errorf("%s/%s: cannot load the certificate and key: %w", EnvRunnerTLSCertFile, EnvRunnerTLSKeyFile, err)
	}
	return l, nil
}

func checkPrivateFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("read key file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s %w", path, ErrSecretFileNotFile)
	}
	if info.Mode().Perm()&otherUsersPermBits != 0 {
		return fmt.Errorf("%s %w (mode %04o); run chmod 600 %s", path, ErrSecretFileShared, info.Mode().Perm(), path)
	}
	return nil
}

func (c Config) RunnerPlaneSeparate() bool { return c.RunnerListenAddr != "" }

func (c Config) RunnerPlaneTLS() bool { return c.RunnerTLSCertFile != "" && c.RunnerTLSKeyFile != "" }
