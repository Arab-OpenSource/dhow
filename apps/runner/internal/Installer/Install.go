package installer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	engine "github.com/Khaymah/dhow/internal/installer"
)

const (
	groupOtherWrite = 0o022
	groupOtherRead  = 0o044
)

var (
	tokenPattern   = regexp.MustCompile(`^` + config.RegistrationTokenPrefix + `[A-Za-z0-9_-]{16,200}$`)
	ErrTokenFormat = errors.New("the registration token does not look like dhow_rrt_...; copy it again from the Runners page")
)

func (app App) Install(ctx context.Context, opts InstallOptions) error {
	det := Detect(ctx, app.Sys)
	a, err := app.initialAnswers(opts, opts.Lang)
	if err != nil {
		return err
	}
	m := newMessages(engine.ChooseLang(opts.Lang, a.Lang, app.Locale))
	if !opts.DryRun && !det.Root {
		return errors.New(m.T(KeyNeedRoot))
	}
	app.say("%s\n\n", m.T(KeyWelcome, app.Version))
	app.printDetection(det, m)
	prompter := engine.NewPrompter(app.In, app.Out, app.InFD, m)
	token := ""
	if opts.Answers == "" {
		iv := Interview{P: prompter, M: m, Det: det, Out: app.Out}
		if a, token, err = iv.Run(a); err != nil {
			return err
		}
	} else if !det.Registered() && !opts.DryRun {
		if token, err = app.readToken(a.TokenFile, m); err != nil {
			return err
		}
	}
	exe, err := app.Sys.Executable()
	if err != nil {
		return fmt.Errorf("find the running binary: %w", err)
	}
	b := &Builder{
		Sys: app.Sys, Det: det, A: a, M: m, Exe: exe, Version: app.Version, Token: token, Protocol: app.Protocol,
		Target: app.target, Sleep: app.Sleep,
	}
	plan := b.Build()
	engine.PrintPlan(ctx, app.Out, plan, m)
	if len(plan.Blockers) > 0 {
		return ErrBlocked
	}
	if opts.DryRun {
		app.say("\n%s\n", m.T(KeyDryRunDone))
		return nil
	}
	if !opts.Yes && opts.Answers == "" {
		ok, err := prompter.Confirm(m.T(KeyConfirmPlan), false)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New(m.T(KeyAborted))
		}
	}
	app.say("\n")
	if err := (engine.Runner{Out: app.Out, M: m}).Execute(ctx, plan.Steps); err != nil {
		return fmt.Errorf("%w\n%s", err, m.T(KeyRerunHint))
	}
	app.finish(b, m)
	return nil
}

func (app App) initialAnswers(opts InstallOptions, lang string) (Answers, error) {
	if opts.Answers != "" {
		data, err := app.readAnswersFile(opts.Answers, newMessages(engine.ChooseLang(lang, "", app.Locale)))
		if err != nil {
			return Answers{}, err
		}
		return ParseAnswers(bytes.NewReader(data))
	}
	a, _ := app.installedAnswers()
	return a, nil
}

func (app App) readAnswersFile(path string, m engine.Messages) ([]byte, error) {
	data, info, err := app.Sys.ReadFileNoFollow(path, engine.MaxAnswersBytes)
	if err != nil {
		return nil, fmt.Errorf("read answers: %w", err)
	}
	if !app.Sys.IsRoot() {
		return data, nil
	}
	if info.UID != 0 || info.Mode.Perm()&groupOtherWrite != 0 {
		return nil, errors.New(m.T(KeyAnswersUnsafe, path, info.UID, info.Mode.Perm()))
	}
	dir, err := app.Sys.Stat(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("read answers: %w", err)
	}
	if dir.Mode.Perm()&groupOtherWrite != 0 && dir.Mode&fs.ModeSticky == 0 {
		return nil, errors.New(m.T(KeyAnswersDirUnsafe, filepath.Dir(path)))
	}
	return data, nil
}

func (app App) readToken(path string, m engine.Messages) (string, error) {
	var r io.Reader
	switch path {
	case "":
		return "", errors.New(m.T(KeyNeedToken))
	case TokenStdin:
		r = app.In
	default:
		data, info, err := app.Sys.ReadFileNoFollow(path, config.MaxRegistrationToken+2)
		if err != nil {
			return "", fmt.Errorf("token_file: %w", err)
		}
		if info.Mode.Perm()&groupOtherWrite != 0 {
			return "", errors.New(m.T(KeyTokenFileUnsafe, path, info.Mode.Perm()))
		}
		if info.Mode.Perm()&groupOtherRead != 0 {
			app.say("%s\n", m.T(KeyTokenFileReadable, path, info.Mode.Perm()))
		}
		r = bytes.NewReader(data)
	}
	line, err := bufio.NewReader(io.LimitReader(r, config.MaxRegistrationToken+2)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read the token: %w", err)
	}
	return ValidToken(line)
}

func ValidToken(raw string) (string, error) {
	token := strings.TrimSpace(raw)
	if !tokenPattern.MatchString(token) {
		return "", ErrTokenFormat
	}
	return token, nil
}

func (app App) printDetection(det Detection, m engine.Messages) {
	app.say("%s\n", m.T(KeyDetected))
	name := det.Distro.PrettyName
	if name == "" {
		name = "?"
	}
	podman := det.Podman
	if podman == "" {
		podman = m.T(engine.KeyNone)
	}
	rows := [][2]string{
		{m.T(KeyRowSystem), name},
		{"systemd", m.YesNo(det.Systemd)},
		{m.T(KeyRowRoot), m.YesNo(det.Root)},
		{"Podman", podman},
		{"crun / newuidmap", m.YesNo(det.Crun) + " / " + m.YesNo(det.Newuidmap)},
		{"cgroup v2", m.YesNo(det.CgroupV2)},
		{m.T(KeyRowUserNS), m.YesNo(det.UserNS && !det.AppArmorNS)},
		{"systemd-creds", credsLabel(det.Creds, m)},
		{m.T(KeyRowRegistered), m.YesNo(det.Registered())},
	}
	for _, r := range rows {
		app.say("  %-26s %s\n", r[0], r[1])
	}
}

func credsLabel(c Creds, m engine.Messages) string {
	switch c {
	case CredsTPM2:
		return m.T(KeyCredsTPM2)
	case CredsHost:
		return m.T(KeyCredsHost)
	default:
		return m.T(KeyCredsNone)
	}
}

func (app App) finish(b *Builder, m engine.Messages) {
	if b.Registered != nil {
		app.say("\n%s\n", m.T(KeyRegistered, b.Registered.RunnerID, strings.Join(b.Registered.Labels, ", ")))
	}
	if b.Encrypted() {
		app.say("%s\n", m.T(KeyIdentitySealed, SealedPath))
	} else {
		app.say("%s\n", m.T(KeyIdentityPlain, IdentityPath))
	}
	app.say("\n%s\n%s\n", m.T(KeyInstalled), m.T(KeyAfterInstall, AnswersPath))
}
