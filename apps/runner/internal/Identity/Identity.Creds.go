package identity

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const credsTimeout = time.Minute

var ErrCreds = errors.New("systemd-creds failed")

type SystemdCreds struct{}

func (SystemdCreds) run(action, name string, input []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), credsTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemd-creds", action, "--name="+name, "-", "-") //nolint:gosec // A fixed program with argv built here; the name is a constant credential name.
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s %s: %w", ErrCreds, action, strings.TrimSpace(stderr.String()), err)
	}
	return stdout.Bytes(), nil
}

func (s SystemdCreds) Seal(name string, plain []byte) ([]byte, error) {
	return s.run("encrypt", name, plain)
}

func (s SystemdCreds) Open(name string, sealed []byte) ([]byte, error) {
	return s.run("decrypt", name, sealed)
}
