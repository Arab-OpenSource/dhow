package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
}

type DoctorOptions struct {
	Fix  bool
	Yes  bool
	Lang string
}

type Doctor struct {
	Sys      System
	Det      Detection
	A        Answers
	M        Messages
	Probe    ProbeIDs
	Settings SettingsReader
	b        *Builder
}

type ProbeIDs func() (string, error)

func (app App) loadInstalledAnswers() (Answers, bool) {
	data, err := app.Sys.ReadFile(AnswersPath)
	if err != nil {
		return Answers{}, false
	}
	a, err := ParseInstalledAnswers(bytes.NewReader(data), exists(app.Sys, DatabaseURLPath))
	if err != nil {
		return Answers{}, false
	}
	return a, true
}

func (app App) Doctor(ctx context.Context, opts DoctorOptions) error {
	a, found := app.loadInstalledAnswers()
	m := NewMessages(ChooseLang(opts.Lang, a.Lang, app.Locale))
	if !app.Sys.IsRoot() {
		return errors.New(m.T(KeyNeedRoot))
	}
	if !found {
		app.say("%s\n", m.T(KeyDoctorNotInstalled, AnswersPath))
		return ErrUnhealthy
	}
	det := Detect(ctx, app.Sys)
	d := Doctor{Sys: app.Sys, Det: det, A: a, M: m, Probe: func() (string, error) { return RandomHex(8) }}
	if reader, ok := app.DB.(SettingsReader); ok {
		d.Settings = reader
	}
	d.b = &Builder{Sys: app.Sys, Det: det, A: a, M: m, Version: app.Version, Sleep: app.Sleep}
	findings := d.Run(ctx)
	app.printFindings(findings, m)
	remaining := 0
	prompter := NewPrompter(app.In, app.Out, app.InFD, m)
	interactive := IsTerminal(app.InFD)
	for _, f := range findings {
		if f.Severity < SeverityWarn {
			continue
		}
		if f.Fix == nil {
			remaining++
			continue
		}
		apply := opts.Fix
		if !apply && interactive {
			ok, err := prompter.Confirm(m.T(KeyApplyFix, f.Fix.Title), false)
			if err != nil {
				return err
			}
			apply = ok
		}
		if !apply {
			remaining++
			continue
		}
		if err := (Runner{Out: app.Out, M: m}).Execute(ctx, []Step{*f.Fix}); err != nil {
			app.say("  %s\n", err.Error())
			remaining++
		}
	}
	if remaining > 0 {
		if !opts.Fix && !interactive {
			app.say("\n%s\n", m.T(KeyDoctorFixHint))
		}
		return fmt.Errorf("%w: %d", ErrUnhealthy, remaining)
	}
	app.say("\n%s\n", m.T(KeyDoctorClean))
	return nil
}

func (app App) printFindings(findings []Finding, m Messages) {
	labels := map[Severity]string{
		SeverityOK:   m.T(KeySevOK),
		SeverityInfo: m.T(KeySevInfo),
		SeverityWarn: m.T(KeySevWarn),
		SeverityFail: m.T(KeySevFail),
	}
	for _, f := range findings {
		app.say("  %-5s %s\n", labels[f.Severity], f.Title)
		if f.Detail != "" {
			app.say("        %s\n", f.Detail)
		}
		if f.Severity >= SeverityWarn {
			if f.Fix != nil {
				app.say("        %s %s\n", m.T(KeyProposedFix), f.Fix.Title)
				for _, c := range f.Fix.Commands {
					app.say("          %s\n", c)
				}
			}
			if f.Advice != "" {
				app.say("        %s\n", f.Advice)
			}
		}
	}
}

func (d Doctor) Run(ctx context.Context) []Finding {
	var out []Finding
	if d.A.Mode == ModeCompose {
		out = append(out, d.composeChecks(ctx)...)
	} else {
		out = append(out, d.nativeChecks(ctx)...)
	}
	out = append(out, d.trustedProxies())
	out = append(out, d.publicChecks(ctx)...)
	out = append(out, d.resources()...)
	out = append(out, d.disk()...)
	out = append(out, d.settingSources(ctx)...)
	return out
}

func ok(title string) Finding { return Finding{Severity: SeverityOK, Title: title} }

func (d Doctor) nativeChecks(ctx context.Context) []Finding {
	out := []Finding{d.installed()}
	if out[0].Severity == SeverityFail {
		return out
	}
	out = append(out, d.unitDrift(), d.permissions(), d.secrets(), d.service(ctx))
	return append(out, d.health(ctx)...)
}

func (d Doctor) composeChecks(ctx context.Context) []Finding {
	var out []Finding
	if !exists(d.Sys, ComposeFilePath) || !exists(d.Sys, ComposeEnvPath) {
		return []Finding{{Severity: SeverityFail, Title: d.M.T(KeyCheckInstalled), Detail: ComposeFilePath, Advice: d.M.T(KeyAdviceReinstall)}}
	}
	out = append(out, ok(d.M.T(KeyCheckInstalled)), d.permissionsOf([]permCheck{
		{ConfigDir, modeRootDir.Perm}, {ComposeDir, modeRootDir.Perm}, {ComposeEnvPath, modeRootPrivate.Perm},
	}))
	res, err := d.Sys.Run(ctx, composeCmd("ps", "--status", "running", "--services"))
	running := err == nil && bytes.Contains(res.Stdout, []byte("dhow")) && bytes.Contains(res.Stdout, []byte("postgres"))
	if running {
		out = append(out, ok(d.M.T(KeyCheckService)))
	} else {
		fix := CommandStep(d.Sys, d.M.T(KeyFixComposeUp), nil, []Command{composeUp()}, nil)
		out = append(out, Finding{Severity: SeverityFail, Title: d.M.T(KeyCheckService), Fix: &fix})
	}
	return append(out, d.health(ctx)...)
}
