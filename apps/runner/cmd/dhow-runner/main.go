package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	app "github.com/Khaymah/dhow/apps/runner/internal/App"
	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	args := os.Args[1:]
	if filepath.Base(os.Args[0]) == app.ConnectName {
		args = append([]string{app.CommandConnect}, args...)
	}
	err := app.Main(ctx, args, app.Env{
		Getenv: config.FromEnv(),
		Stdio:  app.Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr},
	})
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "dhow-runner:", err)
		os.Exit(1)
	}
}
