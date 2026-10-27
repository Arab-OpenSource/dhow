package installer

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
)

const (
	CommandInstall   = "install"
	CommandDoctor    = "doctor"
	CommandUninstall = "uninstall"
)

var (
	Commands       = []string{CommandInstall, CommandDoctor, CommandUninstall}
	ErrUsage       = errors.New("invalid usage")
	errUnknownLang = errors.New("--lang must be en or ar")
)

func IsCommand(name string) bool { return slices.Contains(Commands, name) }

func Main(ctx context.Context, args []string, app App, errOut io.Writer) error {
	if len(args) == 0 || !IsCommand(args[0]) {
		return fmt.Errorf("%w: expected one of %v", ErrUsage, Commands)
	}
	fs := flag.NewFlagSet("dhow-server "+args[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	lang := fs.String("lang", "", "language of the questions and messages: en or ar (default: from the locale)")
	yes := fs.Bool("yes", false, "do not ask for the final confirmation")
	var run func() error
	switch args[0] {
	case CommandInstall:
		answers := fs.String("answers", "", "read every answer from this YAML file instead of asking")
		dry := fs.Bool("dry-run", false, "detect, ask and print the plan without changing anything")
		run = func() error {
			return app.Install(ctx, InstallOptions{Answers: *answers, DryRun: *dry, Yes: *yes, Lang: *lang})
		}
	case CommandDoctor:
		fix := fs.Bool("fix", false, "apply every proposed fix without asking")
		run = func() error { return app.Doctor(ctx, DoctorOptions{Fix: *fix, Yes: *yes, Lang: *lang}) }
	case CommandUninstall:
		purge := fs.Bool("purge", false, "also delete the data, the secrets and the local database (asks you to type a confirmation)")
		run = func() error { return app.Uninstall(ctx, UninstallOptions{Purge: *purge, Yes: *yes, Lang: *lang}) }
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%w: unexpected argument %q", ErrUsage, fs.Arg(0))
	}
	if *lang != "" && *lang != LangEn && *lang != LangAr {
		return errUnknownLang
	}
	return run()
}
