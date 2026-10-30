package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	engine "github.com/Khaymah/dhow/internal/installer"
)

type uninstaller struct {
	app App
	det Detection
	a   Answers
	m   engine.Messages
	out io.Writer
}

func (app App) Uninstall(ctx context.Context, opts UninstallOptions) error {
	a, _ := app.installedAnswers()
	m := newMessages(engine.ChooseLang(opts.Lang, a.Lang, app.Locale))
	if !app.Sys.IsRoot() {
		return errors.New(m.T(KeyNeedRoot))
	}
	det := Detect(ctx, app.Sys)
	if !app.anythingInstalled(det, opts.RemoveUser) {
		app.say("%s\n", m.T(KeyNothingInstalled))
		return nil
	}
	u := uninstaller{app: app, det: det, a: a, m: m, out: app.Out}
	steps := u.steps(opts.RemoveUser)
	app.say("%s\n", m.T(KeyUninstallPlan))
	for i, s := range steps {
		app.say("  %2d. %s\n", i+1, s.Title)
		for _, f := range s.Files {
			app.say("        %s %s\n", m.T(engine.KeyPlanRemove), f)
		}
	}
	if !opts.RemoveUser && det.User {
		app.say("\n%s\n", m.T(KeyUninstallKeepsUser, ServiceUser))
	}
	if !opts.Yes {
		ok, err := engine.NewPrompter(app.In, app.Out, app.InFD, m).Confirm(m.T(KeyConfirmUninstall), false)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New(m.T(KeyAborted))
		}
	}
	app.say("\n")
	if err := (engine.Runner{Out: app.Out, M: m}).Execute(ctx, steps); err != nil {
		return err
	}
	app.say("\n%s\n", m.T(KeyUninstalled))
	return nil
}

func (app App) anythingInstalled(det Detection, removeUser bool) bool {
	for _, p := range []string{BinaryPath, UnitPath, DropInDir, ConfigDir, StateDir} {
		if engine.Exists(app.Sys, p) {
			return true
		}
	}
	return det.Installed || det.Registered() || (removeUser && det.UserCreated)
}

func (u uninstaller) say(format string, args ...any) { _, _ = fmt.Fprintf(u.out, format, args...) }

func (u uninstaller) steps(removeUser bool) []engine.Step {
	sys := u.app.Sys
	stop := engine.Cmd("systemctl", "disable", "--now", ServiceName)
	steps := []engine.Step{
		{
			Title:    u.m.T(KeyStepStop),
			Commands: []string{stop.String()},
			Done:     func(context.Context) (bool, error) { return !engine.Exists(sys, UnitPath), nil },
			Apply: func(ctx context.Context) error {
				_, err := engine.Run(ctx, sys, stop)
				return err
			},
		},
		u.revokeStep(),
		u.unitStep(),
		u.stateStep(),
		engine.RemoveStep(sys, u.m.T(KeyStepRemoveConfig), ConfigDir),
		engine.RemoveStep(sys, u.m.T(KeyStepRemoveBinary), BinaryPath),
	}
	if removeUser {
		steps = append(steps, u.userStep())
	}
	return steps
}

func (u uninstaller) revokeStep() engine.Step {
	sys := u.app.Sys
	return engine.Step{
		Title: u.m.T(KeyStepRevoke),
		Done: func(context.Context) (bool, error) {
			return !engine.Exists(sys, IdentityPath) && !engine.Exists(sys, SealedPath), nil
		},
		Apply: func(ctx context.Context) error {
			env, _ := sys.ReadFile(EnvFilePath)
			t, err := u.app.target(env, u.a.Dev, engine.Exists(sys, SealedPath) && !engine.Exists(sys, IdentityPath))
			if err != nil {
				u.say("      %s\n      %s\n", err.Error(), u.m.T(KeyRevokeByHand, "?"))
				return nil
			}
			id, result, err := u.app.Protocol.Revoke(ctx, t)
			if id == "" {
				id = "?"
			}
			switch {
			case err != nil:
				u.say("      %s\n      %s\n", err.Error(), u.m.T(KeyRevokeByHand, id))
			case result == RevokeAlready:
				u.say("      %s\n", u.m.T(KeyRevokedBefore, id))
			default:
				u.say("      %s\n", u.m.T(KeyRevokedNow, id))
			}
			return nil
		},
	}
}

func (u uninstaller) unitStep() engine.Step {
	sys := u.app.Sys
	s := engine.RemoveStep(sys, u.m.T(KeyStepRemoveUnit), UnitPath, DropInDir)
	remove := s.Apply
	s.Apply = func(ctx context.Context) error {
		if err := remove(ctx); err != nil {
			return err
		}
		_, _ = sys.Run(ctx, engine.Cmd("systemctl", "reset-failed", ServiceName))
		_, err := engine.Run(ctx, sys, engine.Cmd("systemctl", "daemon-reload"))
		return err
	}
	return s
}

func (u uninstaller) stateStep() engine.Step {
	sys := u.app.Sys
	s := engine.RemoveStep(sys, u.m.T(KeyStepRemoveState), StateDir)
	remove := s.Apply
	s.Apply = func(ctx context.Context) error {
		for _, p := range []string{IdentityPath, PendingPath} {
			if err := sys.ShredNoFollow(p); err != nil {
				return err
			}
		}
		return remove(ctx)
	}
	return s
}

func (u uninstaller) userStep() engine.Step {
	sys := u.app.Sys
	if u.det.User && !u.det.UserCreated {
		return engine.Step{
			Title: u.m.T(KeyStepKeepUser, ServiceUser),
			Done:  func(context.Context) (bool, error) { return true, nil },
			Apply: func(context.Context) error { return nil },
		}
	}
	check := engine.Cmd("getent", "passwd", ServiceUser)
	return engine.Step{
		Title:    u.m.T(KeyStepRemoveUser, ServiceUser),
		Files:    []string{SubUIDPath, SubGIDPath},
		Commands: []string{"userdel " + ServiceUser},
		Done: func(ctx context.Context) (bool, error) {
			return !engine.Succeeds(ctx, sys, check) && subIDCount(sys, SubUIDPath) == 0 &&
				subIDCount(sys, SubGIDPath) == 0, nil
		},
		Apply: func(ctx context.Context) error {
			if engine.Succeeds(ctx, sys, check) {
				if _, err := engine.Run(ctx, sys, engine.Cmd("userdel", ServiceUser)); err != nil {
					return err
				}
			}
			for _, path := range []string{SubUIDPath, SubGIDPath} {
				if err := dropSubIDs(sys, path, ServiceUser); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func dropSubIDs(sys engine.System, path, owner string) error {
	data, err := sys.ReadFile(path)
	if err != nil {
		return nil
	}
	var kept bytes.Buffer
	changed := false
	for _, line := range strings.SplitAfter(string(data), "\n") {
		if strings.HasPrefix(line, owner+":") {
			changed = true
			continue
		}
		kept.WriteString(line)
	}
	if !changed {
		return nil
	}
	return sys.WriteFile(path, kept.Bytes(), modeRootPublic)
}
