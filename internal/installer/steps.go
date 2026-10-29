package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
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

func PrintPlan(ctx context.Context, out io.Writer, plan Plan, m Messages) {
	say := func(format string, args ...any) { _, _ = fmt.Fprintf(out, format, args...) }
	for _, n := range plan.Notes {
		say("\n%s\n", n)
	}
	if len(plan.Blockers) > 0 {
		say("\n%s\n", m.T(KeyBlockers))
		for _, bl := range plan.Blockers {
			say("  - %s\n", bl)
		}
		return
	}
	say("\n%s\n", m.T(KeyPlan))
	for i, s := range plan.Steps {
		state := ""
		if s.Done != nil {
			if done, err := s.Done(ctx); err == nil && done {
				state = " (" + m.T(KeyStepAlreadyDone) + ")"
			}
		}
		say("  %2d. %s%s\n", i+1, s.Title, state)
		for _, f := range s.Files {
			say("        %s %s\n", m.T(KeyPlanFile), f)
		}
		for _, p := range s.Packages {
			say("        %s %s\n", m.T(KeyPlanPackage), p)
		}
		for _, c := range s.Commands {
			say("        %s %s\n", m.T(KeyPlanCommand), c)
		}
	}
	pkgs := plan.Packages()
	sort.Strings(pkgs)
	list := strings.Join(pkgs, " ")
	if list == "" {
		list = m.T(KeyNone)
	}
	say("\n%s %s\n", m.T(KeyPlanPackagesTotal), list)
}

func SameFile(sys System, path string, data []byte, perm fs.FileMode) (bool, error) {
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
		Done:  func(context.Context) (bool, error) { return SameFile(sys, path, data, mode.Perm) },
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

func SecretFileStep(sys System, title, path string, mode FileMode, generate func() ([]byte, error)) Step {
	return Step{
		Title: title,
		Files: []string{path},
		Done:  func(context.Context) (bool, error) { return Exists(sys, path), nil },
		Apply: func(context.Context) error {
			data, err := generate()
			if err != nil {
				return err
			}
			return sys.WriteFile(path, data, mode)
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
			created = !Exists(sys, path)
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
		s.Done = func(ctx context.Context) (bool, error) { return Succeeds(ctx, sys, check), nil }
	}
	s.Apply = func(ctx context.Context) error {
		for _, c := range apply {
			if _, err := Run(ctx, sys, c); err != nil {
				return err
			}
		}
		return nil
	}
	if len(undo) > 0 {
		s.Undo = func(ctx context.Context) error {
			var errs []error
			for _, c := range undo {
				if _, err := Run(ctx, sys, c); err != nil {
					errs = append(errs, err)
				}
			}
			return errors.Join(errs...)
		}
	}
	return s
}

func RemoveStep(sys System, title string, paths ...string) Step {
	return Step{
		Title: title,
		Files: paths,
		Done: func(context.Context) (bool, error) {
			for _, p := range paths {
				if Exists(sys, p) {
					return false, nil
				}
			}
			return true, nil
		},
		Apply: func(context.Context) error {
			var errs []error
			for _, p := range paths {
				errs = append(errs, sys.RemoveAll(p))
			}
			return errors.Join(errs...)
		},
	}
}

type File struct {
	Path string
	Data []byte
	Mode FileMode
}

type savedFile struct {
	data []byte
	mode FileMode
	had  bool
}

func FilesStep(sys System, title string, files []File, after []Command, done func(context.Context) (bool, error)) Step {
	saved := map[string]savedFile{}
	s := Step{Title: title, Done: done}
	for _, f := range files {
		s.Files = append(s.Files, f.Path)
	}
	for _, c := range after {
		s.Commands = append(s.Commands, c.String())
	}
	s.Apply = func(ctx context.Context) error {
		for _, f := range files {
			if old, err := sys.ReadFile(f.Path); err == nil {
				saved[f.Path] = savedFile{data: old, mode: f.Mode, had: true}
			} else {
				saved[f.Path] = savedFile{mode: f.Mode}
			}
			if err := sys.WriteFile(f.Path, f.Data, f.Mode); err != nil {
				return err
			}
		}
		for _, c := range after {
			if _, err := Run(ctx, sys, c); err != nil {
				return err
			}
		}
		return nil
	}
	s.Undo = func(context.Context) error {
		var errs []error
		for path, f := range saved {
			if f.had {
				errs = append(errs, sys.WriteFile(path, f.data, f.mode))
			} else {
				errs = append(errs, sys.Remove(path))
			}
		}
		return errors.Join(errs...)
	}
	return s
}

func FilesMatch(sys System, files []File) (bool, error) {
	for _, f := range files {
		same, err := SameFile(sys, f.Path, f.Data, f.Mode.Perm)
		if err != nil || !same {
			return false, err
		}
	}
	return true, nil
}
