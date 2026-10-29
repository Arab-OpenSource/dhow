package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strings"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	"github.com/Khaymah/dhow/internal/buildinfo"
	"github.com/Khaymah/dhow/internal/logging"
)

const (
	CommandRegister = "register"
	CommandRun      = "run"
	CommandVersion  = "version"
)

var (
	ErrUsage        = errors.New("usage: dhow-runner register --url <server> --token-file <path|-> [--name <name>] [--dev] | dhow-runner run [--dev] | dhow-runner rotate-keys [--dev] | dhow-runner version")
	ErrTokenOnFlags = errors.New("the registration token is never passed on the command line, where shell history and ps would keep it; use --token-file <path> or --token-file - to read it from stdin")
)

type Stdio struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

type Env struct {
	Getenv  func(string) string
	Stdio   Stdio
	Machine func() (config.Machine, error)
	Harden  func() error
}

func Main(ctx context.Context, args []string, env Env) error {
	if len(args) == 0 {
		return ErrUsage
	}
	if env.Machine == nil {
		env.Machine = config.DetectMachine
	}
	if env.Harden == nil {
		env.Harden = hardenProcess
	}
	switch args[0] {
	case CommandAgent:
		return runAgent(ctx, env)
	case CommandConnect:
		return runConnect(ctx, args[1:], env)
	case CommandRegister:
		return runRegister(ctx, args[1:], env)
	case CommandRun:
		return runRun(ctx, args[1:], env)
	case CommandRotateKeys:
		return runRotateKeys(ctx, args[1:], env)
	case CommandVersion:
		info := buildinfo.Get()
		_, err := fmt.Fprintf(env.Stdio.Out, "dhow-runner %s (%s, %s)\n", info.Version, info.Commit, info.GoVersion)
		return err
	default:
		return fmt.Errorf("unknown command %q: %w", args[0], ErrUsage)
	}
}

func refuseTokenFlags(args []string) error {
	for _, a := range args {
		name := strings.TrimLeft(a, "-")
		name, _, _ = strings.Cut(name, "=")
		if strings.HasPrefix(a, "-") && (name == "token" || name == "registration-token") {
			return ErrTokenOnFlags
		}
		if strings.HasPrefix(a, config.RegistrationTokenPrefix) {
			return ErrTokenOnFlags
		}
	}
	return nil
}

func newFlags(name string, errOut io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(errOut)
	return fs
}

func newLogger(w io.Writer, level slog.Level) *slog.Logger {
	return logging.New(w, level).With("component", "dhow-runner")
}
