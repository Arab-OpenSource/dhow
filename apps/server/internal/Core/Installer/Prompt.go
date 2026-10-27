package installer

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const maxAttempts = 5

var (
	ErrNoInput      = errors.New("no answer on standard input; use --answers for unattended installs")
	ErrTooManyTries = errors.New("too many invalid answers")
)

type Option struct {
	Value string
	Label string
}

type Prompter struct {
	in  *bufio.Reader
	out io.Writer
	fd  int
	m   Messages
}

func NewPrompter(in io.Reader, out io.Writer, fd int, m Messages) *Prompter {
	return &Prompter{in: bufio.NewReader(in), out: out, fd: fd, m: m}
}

func (p *Prompter) line() (string, error) {
	s, err := p.in.ReadString('\n')
	if err != nil && (s == "" || !errors.Is(err, io.EOF)) {
		if errors.Is(err, io.EOF) {
			return "", ErrNoInput
		}
		return "", fmt.Errorf("read answer: %w", err)
	}
	return strings.TrimRight(s, "\r\n"), nil
}

func (p *Prompter) say(format string, args ...any) {
	_, _ = fmt.Fprintf(p.out, format, args...)
}

func (p *Prompter) Ask(label, def string, validate func(string) error) (string, error) {
	for range maxAttempts {
		if def != "" {
			p.say("%s [%s]: ", label, def)
		} else {
			p.say("%s: ", label)
		}
		v, err := p.line()
		if err != nil {
			return "", err
		}
		v = strings.TrimSpace(v)
		if v == "" {
			v = def
		}
		if validate == nil {
			return v, nil
		}
		if err := validate(v); err != nil {
			p.say("  %s\n", err.Error())
			continue
		}
		return v, nil
	}
	return "", ErrTooManyTries
}

func (p *Prompter) Choose(label string, options []Option, def string) (string, error) {
	defIndex := 1
	p.say("%s\n", label)
	for i, o := range options {
		marker := " "
		if o.Value == def {
			marker = "*"
			defIndex = i + 1
		}
		p.say(" %s %d) %s\n", marker, i+1, o.Label)
	}
	answer, err := p.Ask(p.m.T(KeyChoose), strconv.Itoa(defIndex), func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > len(options) {
			return errors.New(p.m.T(KeyChooseInvalid, len(options)))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	n, err := strconv.Atoi(answer)
	if err != nil {
		return "", fmt.Errorf("choose: %w", err)
	}
	return options[n-1].Value, nil
}

func (p *Prompter) Confirm(label string, def bool) (bool, error) {
	hint := p.m.T(KeyYesNoDefaultNo)
	if def {
		hint = p.m.T(KeyYesNoDefaultYes)
	}
	for range maxAttempts {
		p.say("%s %s: ", label, hint)
		v, err := p.line()
		if err != nil {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "":
			return def, nil
		case "y", "yes", "نعم", "اه", "أيوه", "ايوه":
			return true, nil
		case "n", "no", "لا", "لأ":
			return false, nil
		}
		p.say("  %s\n", p.m.T(KeyYesNoInvalid))
	}
	return false, ErrTooManyTries
}

func (p *Prompter) Secret(label string, validate func(string) error) (string, error) {
	for range maxAttempts {
		p.say("%s: ", label)
		restore, err := disableEcho(p.fd)
		v, lerr := p.line()
		if err == nil {
			restore()
			p.say("\n")
		}
		if lerr != nil {
			return "", lerr
		}
		v = strings.TrimSpace(v)
		if err := validate(v); err != nil {
			p.say("  %s\n", err.Error())
			continue
		}
		return v, nil
	}
	return "", ErrTooManyTries
}

func (p *Prompter) Typed(label, phrase string) (bool, error) {
	p.say("%s\n  %s: ", label, phrase)
	v, err := p.line()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(v) == phrase, nil
}
