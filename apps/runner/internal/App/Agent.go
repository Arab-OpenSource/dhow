package app

import (
	"context"
	"fmt"

	agent "github.com/Khaymah/dhow/apps/runner/internal/Agent"
)

const CommandAgent = "__agent"

func runAgent(ctx context.Context, env Env) error {
	err := agent.Run(ctx, agent.Options{Stdio: agent.Stdio{Out: env.Stdio.Out, Err: env.Stdio.Err}})
	if err != nil {
		return fmt.Errorf("the job agent stopped: %w", err)
	}
	return nil
}
