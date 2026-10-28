package podman

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type runner struct {
	binary string
	env    []string
}

func (r runner) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, r.binary, args...) //nolint:gosec // Every argument is a fixed flag or a validated job value passed as argv after --, never a shell string.
	cmd.Env = r.env
	return cmd
}

func (r runner) run(ctx context.Context, args ...string) ([]byte, error) {
	var out, errb bytes.Buffer
	cmd := r.command(ctx, args...)
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return out.Bytes(), fmt.Errorf("podman %s: %w: %s", args[0], err, trim(errb.Bytes()))
	}
	return out.Bytes(), nil
}

func trim(b []byte) string {
	const maxErr = 2048
	s := strings.TrimSpace(string(b))
	if len(s) > maxErr {
		s = s[:maxErr] + "…"
	}
	return s
}
