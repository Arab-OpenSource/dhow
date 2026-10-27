package installer

import (
	"context"
	"errors"
)

const PurgePhrase = "delete dhow data"

type UninstallOptions struct {
	Purge bool
	Yes   bool
	Lang  string
}

func (app App) Uninstall(ctx context.Context, opts UninstallOptions) error {
	a, found := app.loadInstalledAnswers()
	m := NewMessages(ChooseLang(opts.Lang, a.Lang, app.Locale))
	if !app.Sys.IsRoot() {
		return errors.New(m.T(KeyNeedRoot))
	}
	det := Detect(ctx, app.Sys)
	if !found {
		if !det.Existing && !exists(app.Sys, BinaryPath) {
			app.say("%s\n", m.T(KeyNothingInstalled))
			return nil
		}
		a = DefaultAnswers()
		a.Database.Mode = DatabaseExisting
		a.Proxy.Kind = ProxyNone
		if exists(app.Sys, ComposeEnvPath) {
			a.Mode = ModeCompose
		}
	}
	u := uninstaller{sys: app.Sys, det: det, a: a, m: m}
	steps := u.steps(opts.Purge)
	app.say("%s\n", m.T(KeyUninstallPlan))
	for i, s := range steps {
		app.say("  %2d. %s\n", i+1, s.Title)
		for _, f := range s.Files {
			app.say("        %s %s\n", m.T(KeyPlanRemove), f)
		}
	}
	p := NewPrompter(app.In, app.Out, app.InFD, m)
	if opts.Purge {
		dbNote := m.T(KeyPurgeExternalDatabase)
		if a.Mode == ModeCompose || a.Database.Mode == DatabaseInstall {
			dbNote = m.T(KeyPurgeLocalDatabase)
		}
		app.say("\n%s\n%s\n", m.T(KeyPurgeWarning, ConfigDir, StateDir), dbNote)
		match, err := p.Typed(m.T(KeyPurgeType), PurgePhrase)
		if err != nil {
			return err
		}
		if !match {
			return errors.New(m.T(KeyAborted))
		}
	} else {
		app.say("\n%s\n", m.T(KeyUninstallKeeps, CredentialsDir, StateDir))
		if !opts.Yes {
			yes, err := p.Confirm(m.T(KeyConfirmUninstall), false)
			if err != nil {
				return err
			}
			if !yes {
				return errors.New(m.T(KeyAborted))
			}
		}
	}
	if err := (Runner{Out: app.Out, M: m}).Execute(ctx, steps); err != nil {
		return err
	}
	app.say("\n%s\n", m.T(KeyUninstalled))
	return nil
}

type uninstaller struct {
	sys System
	det Detection
	a   Answers
	m   Messages
}

func (u uninstaller) removeStep(title string, paths ...string) Step {
	return Step{
		Title: title,
		Files: paths,
		Done: func(context.Context) (bool, error) {
			for _, p := range paths {
				if exists(u.sys, p) {
					return false, nil
				}
			}
			return true, nil
		},
		Apply: func(context.Context) error {
			var errs []error
			for _, p := range paths {
				errs = append(errs, u.sys.RemoveAll(p))
			}
			return errors.Join(errs...)
		},
	}
}

func (u uninstaller) steps(purge bool) []Step {
	var steps []Step
	if u.a.Mode == ModeCompose {
		steps = append(steps, u.composeSteps(purge)...)
	} else {
		steps = append(steps, u.nativeSteps()...)
	}
	steps = append(steps, u.proxySteps()...)
	if purge {
		steps = append(steps, u.purgeSteps()...)
	}
	return steps
}

func (u uninstaller) nativeSteps() []Step {
	stop := Step{
		Title:    u.m.T(KeyStepStop),
		Commands: []string{"systemctl disable --now " + ServiceName},
		Done:     func(context.Context) (bool, error) { return !exists(u.sys, UnitPath), nil },
		Apply: func(ctx context.Context) error {
			_, err := run(ctx, u.sys, Cmd("systemctl", "disable", "--now", ServiceName))
			return err
		},
	}
	unit := u.removeStep(u.m.T(KeyStepRemoveUnit), UnitPath)
	remove := unit.Apply
	unit.Apply = func(ctx context.Context) error {
		if err := remove(ctx); err != nil {
			return err
		}
		_, err := run(ctx, u.sys, Cmd("systemctl", "daemon-reload"))
		return err
	}
	return []Step{
		stop,
		unit,
		u.removeStep(u.m.T(KeyStepRemoveBinary), BinaryPath),
		u.removeStep(u.m.T(KeyStepRemoveConfig), EnvFilePath, AnswersPath),
	}
}

func (u uninstaller) composeSteps(purge bool) []Step {
	down := composeCmd("down")
	if purge {
		down = composeCmd("down", "--volumes")
	}
	stop := Step{
		Title:    u.m.T(KeyStepComposeDown),
		Commands: []string{down.String()},
		Done:     func(context.Context) (bool, error) { return !exists(u.sys, ComposeFilePath), nil },
		Apply: func(ctx context.Context) error {
			_, err := run(ctx, u.sys, down)
			return err
		},
	}
	return []Step{stop, u.removeStep(u.m.T(KeyStepRemoveConfig), ComposeFilePath, AnswersPath)}
}

func (u uninstaller) proxySteps() []Step {
	var steps []Step
	if exists(u.sys, CaddySitePath) {
		s := u.removeStep(u.m.T(KeyStepRemoveProxy, "Caddy"), CaddySitePath)
		remove := s.Apply
		s.Apply = func(ctx context.Context) error {
			if data, err := u.sys.ReadFile(CaddyfilePath); err == nil {
				if err := u.sys.WriteFile(CaddyfilePath, WithoutCaddyImport(data), modeRootPublic); err != nil {
					return err
				}
			}
			if err := remove(ctx); err != nil {
				return err
			}
			_, err := run(ctx, u.sys, Cmd("systemctl", "reload-or-restart", "caddy"))
			return err
		}
		steps = append(steps, s)
	}
	if exists(u.sys, NginxSitePath) {
		s := u.removeStep(u.m.T(KeyStepRemoveProxy, "nginx"), NginxSitePath, CertbotHookPath)
		remove := s.Apply
		s.Apply = func(ctx context.Context) error {
			if err := remove(ctx); err != nil {
				return err
			}
			_, err := run(ctx, u.sys, Cmd("systemctl", "reload", "nginx"))
			return err
		}
		steps = append(steps, s)
	}
	return steps
}

func (u uninstaller) purgeSteps() []Step {
	var steps []Step
	if u.a.Mode == ModeNative && u.a.Database.Mode == DatabaseInstall {
		d := u.det.Distro
		steps = append(steps, Step{
			Title:    u.m.T(KeyStepDropDatabase),
			Commands: []string{"DROP DATABASE dhow; DROP ROLE dhow"},
			Apply:    func(ctx context.Context) error { return dropLocalDatabase(ctx, u.sys, d) },
		})
	}
	steps = append(steps, u.removeStep(u.m.T(KeyStepRemoveData), ConfigDir, StateDir))
	if u.a.Mode == ModeNative {
		check := Cmd("getent", "passwd", ServiceUser)
		steps = append(steps, Step{
			Title:    u.m.T(KeyStepRemoveUser, ServiceUser),
			Commands: []string{"userdel " + ServiceUser},
			Done:     func(ctx context.Context) (bool, error) { return !succeeds(ctx, u.sys, check), nil },
			Apply: func(ctx context.Context) error {
				_, err := run(ctx, u.sys, Cmd("userdel", ServiceUser))
				return err
			},
		})
	}
	return steps
}
