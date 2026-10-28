package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	app "github.com/Arab-OpenSource/dhow/apps/runner/internal/App"
	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := app.Main(ctx, os.Args[1:], app.Env{
		Getenv: config.FromEnv(),
		Stdio:  app.Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr},
	})
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "dhow-runner:", err)
		os.Exit(1)
	}
}
