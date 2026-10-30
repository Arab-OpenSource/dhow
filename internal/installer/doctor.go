package installer

import (
	"context"
	"errors"
	"fmt"
	"io"
)

type Severity int

const (
	SeverityOK Severity = iota
	SeverityInfo
	SeverityWarn
	SeverityFail
)

var ErrUnhealthy = errors.New("doctor found problems")

type Finding struct {
	Severity Severity
	Title    string
	Detail   string
	Advice   string
	Fix      *Step
	Advisory bool
}

func OK(title string) Finding { return Finding{Severity: SeverityOK, Title: title} }

func PrintFindings(out io.Writer, findings []Finding, m Messages) {
	say := func(format string, args ...any) { _, _ = fmt.Fprintf(out, format, args...) }
	labels := map[Severity]string{
		SeverityOK:   m.T(KeySevOK),
		SeverityInfo: m.T(KeySevInfo),
		SeverityWarn: m.T(KeySevWarn),
		SeverityFail: m.T(KeySevFail),
	}
	for _, f := range findings {
		say("  %-5s %s\n", labels[f.Severity], f.Title)
		if f.Detail != "" {
			say("        %s\n", f.Detail)
		}
		if f.Severity >= SeverityWarn {
			if f.Fix != nil {
				say("        %s %s\n", m.T(KeyProposedFix), f.Fix.Title)
				for _, c := range f.Fix.Commands {
					say("          %s\n", c)
				}
			}
			if f.Advice != "" {
				say("        %s\n", f.Advice)
			}
		}
	}
}

type FixOptions struct {
	Fix         bool
	Interactive bool
	Prompter    *Prompter
	Out         io.Writer
	M           Messages
}

func ApplyFixes(ctx context.Context, findings []Finding, o FixOptions) (int, error) {
	remaining := 0
	for _, f := range findings {
		if f.Severity < SeverityWarn || f.Advisory {
			continue
		}
		if f.Fix == nil {
			remaining++
			continue
		}
		apply := o.Fix
		if !apply && o.Interactive && o.Prompter != nil {
			ok, err := o.Prompter.Confirm(o.M.T(KeyApplyFix, f.Fix.Title), false)
			if err != nil {
				return remaining, err
			}
			apply = ok
		}
		if !apply {
			remaining++
			continue
		}
		if err := (Runner{Out: o.Out, M: o.M}).Execute(ctx, []Step{*f.Fix}); err != nil {
			_, _ = fmt.Fprintf(o.Out, "  %s\n", err.Error())
			remaining++
		}
	}
	return remaining, nil
}

func HasAdvisory(findings []Finding) bool {
	for _, f := range findings {
		if f.Advisory && f.Severity >= SeverityWarn {
			return true
		}
	}
	return false
}

func Worst(findings []Finding) Severity {
	worst := SeverityOK
	for _, f := range findings {
		worst = max(worst, f.Severity)
	}
	return worst
}
