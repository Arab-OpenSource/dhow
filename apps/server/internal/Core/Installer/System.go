package installer

import (
	"context"
	"io"

	engine "github.com/Khaymah/dhow/internal/installer"
)

type (
	Command     = engine.Command
	Output      = engine.Output
	FileMode    = engine.FileMode
	FileInfo    = engine.FileInfo
	Probe       = engine.Probe
	ProbeResult = engine.ProbeResult
	System      = engine.System
	Host        = engine.Host
)

const maxAttempts = engine.MaxAttempts

var (
	ErrCommandFailed = engine.ErrCommandFailed
	ErrNoInput       = engine.ErrNoInput
	ErrTooManyTries  = engine.ErrTooManyTries
)

func Cmd(name string, args ...string) Command { return engine.Cmd(name, args...) }

func NewHost() *Host { return engine.NewHost() }

func IsTerminal(fd int) bool { return engine.IsTerminal(fd) }

type Option struct {
	Value string
	Label string
}

type Prompter struct{ *engine.Prompter }

func NewPrompter(in io.Reader, out io.Writer, fd int, m Messages) *Prompter {
	return &Prompter{Prompter: engine.NewPrompter(in, out, fd, m)}
}

func (p *Prompter) Choose(label string, options []Option, def string) (string, error) {
	converted := make([]engine.Option, len(options))
	for i, o := range options {
		converted[i] = engine.Option{Value: o.Value, Label: o.Label}
	}
	return p.Prompter.Choose(label, converted, def)
}

func run(ctx context.Context, sys System, c Command) (Output, error) { return engine.Run(ctx, sys, c) }

func succeeds(ctx context.Context, sys System, c Command) bool { return engine.Succeeds(ctx, sys, c) }

func exists(sys System, path string) bool { return engine.Exists(sys, path) }
