package app

import (
	"context"
	"errors"
	"fmt"

	agent "github.com/Khaymah/dhow/apps/runner/internal/Agent"
)

const (
	CommandConnect = "__connect"
	ConnectName    = "connect"
)

var ErrConnectUsage = errors.New("usage: connect <host> <port>")

func runConnect(ctx context.Context, args []string, env Env) error {
	if len(args) != 2 {
		return ErrConnectUsage
	}
	if err := agent.Connect(ctx, agent.EgressSocketPath, args[0], args[1], env.Stdio.In, env.Stdio.Out); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	return nil
}
