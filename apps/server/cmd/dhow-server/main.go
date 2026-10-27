package main

import (
	"context"
	"fmt"
	"os"

	app "github.com/Arab-OpenSource/dhow/apps/server/internal/App"
)

const commandHealthcheck = "healthcheck"

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "dhow-server:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	switch {
	case len(args) == 0:
		return app.Run(ctx)
	case len(args) == 1 && args[0] == commandHealthcheck:
		return app.RunHealthcheck(ctx)
	default:
		return fmt.Errorf("unknown command %q; run without arguments to start the server or use %q", args[0], commandHealthcheck)
	}
}
