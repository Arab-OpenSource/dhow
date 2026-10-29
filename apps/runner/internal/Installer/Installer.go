package installer

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"time"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	engine "github.com/Khaymah/dhow/internal/installer"
)

const (
	CommandInstall   = "install"
	CommandDoctor    = "doctor"
	CommandUninstall = "uninstall"
)

var (
	Commands       = []string{CommandInstall, CommandDoctor, CommandUninstall}
	ErrUsage       = errors.New("invalid usage")
	ErrBlocked     = errors.New("the install cannot continue")
	ErrUnhealthy   = engine.ErrUnhealthy
	errUnknownLang = errors.New("--lang must be en or ar")
)

type Target struct {
	Config    config.Config
	Encrypted bool
}

type Registered struct {
	RunnerID       string
	OrganizationID string
	Labels         []string
}

type RevokeResult int

const (
	RevokeDone RevokeResult = iota
	RevokeAlready
)

type Protocol interface {
	Register(ctx context.Context, t Target, token, name string) (Registered, error)
	Seal(ctx context.Context, t Target) error
	SignIn(ctx context.Context, t Target) (string, error)
	Reach(ctx context.Context, t Target) (string, error)
	Revoke(ctx context.Context, t Target) (string, RevokeResult, error)
}

type App struct {
	Sys      engine.System
	Protocol Protocol
	In       io.Reader
	InFD     int
	Out      io.Writer
	Locale   string
	Version  string
	Machine  func() (config.Machine, error)
	Sleep    func(time.Duration)
}

type InstallOptions struct {
	Answers string
	DryRun  bool
	Yes     bool
	Lang    string
}

type DoctorOptions struct {
	Fix  bool
	Lang string
}

type UninstallOptions struct {
	RemoveUser bool
	Yes        bool
	Lang       string
}

func IsCommand(name string) bool { return slices.Contains(Commands, name) }

func Main(ctx context.Context, args []string, app App, errOut io.Writer) error {
	if len(args) == 0 || !IsCommand(args[0]) {
		return fmt.Errorf("%w: expected one of %v", ErrUsage, Commands)
	}
	fs := flag.NewFlagSet("dhow-runner "+args[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	lang := fs.String("lang", "", "language of the questions and messages: en or ar (default: from the locale)")
	var run func() error
	switch args[0] {
	case CommandInstall:
		answers := fs.String("answers", "", "read every answer from this YAML file and ask nothing")
		dry := fs.Bool("dry-run", false, "detect, ask and print the plan without changing anything")
		yes := fs.Bool("yes", false, "do not ask for the final confirmation")
		run = func() error {
			return app.Install(ctx, InstallOptions{Answers: *answers, DryRun: *dry, Yes: *yes, Lang: *lang})
		}
	case CommandDoctor:
		fix := fs.Bool("fix", false, "apply every proposed fix without asking")
		run = func() error { return app.Doctor(ctx, DoctorOptions{Fix: *fix, Lang: *lang}) }
	case CommandUninstall:
		removeUser := fs.Bool("remove-user", false, "also delete the dhow-runner system user and its sub-ids")
		yes := fs.Bool("yes", false, "do not ask for confirmation")
		run = func() error {
			return app.Uninstall(ctx, UninstallOptions{RemoveUser: *removeUser, Yes: *yes, Lang: *lang})
		}
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
	if *lang != "" && *lang != engine.LangEn && *lang != engine.LangAr {
		return errUnknownLang
	}
	return run()
}

func (app App) say(format string, args ...any) { _, _ = fmt.Fprintf(app.Out, format, args...) }

func (app App) machine() (config.Machine, error) {
	if app.Machine != nil {
		return app.Machine()
	}
	return config.DetectMachine()
}

func (app App) target(env []byte, dev bool, encrypted bool) (Target, error) {
	machine, err := app.machine()
	if err != nil {
		return Target{}, err
	}
	cfg, err := config.Load(EnvGetter(env), config.Options{Dev: dev, Machine: machine, Privileged: true})
	if err != nil {
		return Target{}, err
	}
	return Target{Config: cfg, Encrypted: encrypted}, nil
}

func (app App) installedAnswers() (Answers, bool) {
	data, err := app.Sys.ReadFile(AnswersPath)
	if err != nil {
		return DefaultAnswers(), false
	}
	a, err := ParseAnswers(bytes.NewReader(data))
	if err != nil {
		return DefaultAnswers(), false
	}
	return a, true
}

func newMessages(lang string) engine.Messages {
	return engine.NewMessages(lang, enMessages, arMessages)
}

func Catalog(lang string) map[engine.Key]string {
	if lang == engine.LangAr {
		return arMessages
	}
	return enMessages
}
