package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
)

type Step struct {
	Title    string
	Files    []string
	Packages []string
	Commands []string
	Done     func(ctx context.Context) (bool, error)
	Apply    func(ctx context.Context) error
	Undo     func(ctx context.Context) error
}

type Plan struct {
	Steps    []Step
	Notes    []string
	Blockers []string
}

func (p Plan) Packages() []string {
	var out []string
	for _, s := range p.Steps {
		out = append(out, s.Packages...)
	}
	return out
}

func (p Plan) Files() []string {
	var out []string
	for _, s := range p.Steps {
		out = append(out, s.Files...)
	}
	return out
}

type Runner struct {
	Out io.Writer
	M   Messages
}

func (r Runner) say(format string, args ...any) { _, _ = fmt.Fprintf(r.Out, format, args...) }

func (r Runner) Execute(ctx context.Context, steps []Step) error {
	for i, s := range steps {
		prefix := fmt.Sprintf("[%d/%d]", i+1, len(steps))
		if s.Done != nil {
			done, err := s.Done(ctx)
			if err != nil {
				return fmt.Errorf("%s %s: %w", prefix, s.Title, err)
			}
			if done {
				r.say("%s %s: %s\n", prefix, s.Title, r.M.T(KeyStepAlreadyDone))
				continue
			}
		}
		r.say("%s %s\n", prefix, s.Title)
		if err := s.Apply(ctx); err != nil {
			if s.Undo != nil {
				if uerr := s.Undo(context.WithoutCancel(ctx)); uerr != nil {
					return fmt.Errorf("%s %s: %w (%s: %w)", prefix, s.Title, err, r.M.T(KeyRollbackFailed), uerr)
				}
				r.say("      %s\n", r.M.T(KeyStepRolledBack))
			}
			return fmt.Errorf("%s %s: %w", prefix, s.Title, err)
		}
	}
	return nil
}

func sameFile(sys System, path string, data []byte, perm fs.FileMode) (bool, error) {
	info, err := sys.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode.Perm() != perm {
		return false, nil
	}
	current, err := sys.ReadFile(path)
	if err != nil {
		return false, err
	}
	return bytes.Equal(current, data), nil
}

func FileStep(sys System, title, path string, data []byte, mode FileMode) Step {
	var backup []byte
	var hadFile bool
	return Step{
		Title: title,
		Files: []string{path},
		Done:  func(context.Context) (bool, error) { return sameFile(sys, path, data, mode.Perm) },
		Apply: func(context.Context) error {
			if old, err := sys.ReadFile(path); err == nil {
				backup, hadFile = old, true
			}
			return sys.WriteFile(path, data, mode)
		},
		Undo: func(context.Context) error {
			if hadFile {
				return sys.WriteFile(path, backup, mode)
			}
			return sys.Remove(path)
		},
	}
}

func SecretFileStep(sys System, title, path string, generate func() ([]byte, error)) Step {
	return Step{
		Title: title,
		Files: []string{path},
		Done:  func(context.Context) (bool, error) { return exists(sys, path), nil },
		Apply: func(context.Context) error {
			data, err := generate()
			if err != nil {
				return err
			}
			return sys.WriteFile(path, data, modeRootPrivate)
		},
		Undo: func(context.Context) error { return sys.Remove(path) },
	}
}

func DirStep(sys System, title, path string, mode FileMode) Step {
	created := false
	return Step{
		Title: title,
		Files: []string{path + "/"},
		Done: func(context.Context) (bool, error) {
			info, err := sys.Stat(path)
			if errors.Is(err, fs.ErrNotExist) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			return info.IsDir && info.Mode.Perm() == mode.Perm, nil
		},
		Apply: func(context.Context) error {
			created = !exists(sys, path)
			return sys.MkdirAll(path, mode)
		},
		Undo: func(context.Context) error {
			if created {
				return sys.Remove(path)
			}
			return nil
		},
	}
}

func CommandStep(sys System, title string, done *Command, apply []Command, undo []Command) Step {
	s := Step{Title: title}
	for _, c := range apply {
		s.Commands = append(s.Commands, c.String())
	}
	if done != nil {
		check := *done
		s.Done = func(ctx context.Context) (bool, error) { return succeeds(ctx, sys, check), nil }
	}
	s.Apply = func(ctx context.Context) error {
		for _, c := range apply {
			if _, err := run(ctx, sys, c); err != nil {
				return err
			}
		}
		return nil
	}
	if len(undo) > 0 {
		s.Undo = func(ctx context.Context) error {
			var errs []error
			for _, c := range undo {
				if _, err := run(ctx, sys, c); err != nil {
					errs = append(errs, err)
				}
			}
			return errors.Join(errs...)
		}
	}
	return s
}
