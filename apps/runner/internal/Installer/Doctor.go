package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	engine "github.com/Khaymah/dhow/internal/installer"
)

var exposureRe = regexp.MustCompile(`exposure level for ` + regexp.QuoteMeta(ServiceUnit) + `: ([0-9]+(?:\.[0-9]+)?) ([A-Z]+)`)

type doctor struct {
	app App
	det Detection
	env []byte
	m   engine.Messages
	b   *Builder
}

func (app App) Doctor(ctx context.Context, opts DoctorOptions) error {
	a, found := app.installedAnswers()
	m := newMessages(engine.ChooseLang(opts.Lang, a.Lang, app.Locale))
	if !app.Sys.IsRoot() {
		return errors.New(m.T(KeyNeedRoot))
	}
	det := Detect(ctx, app.Sys)
	if !det.Installed && !det.Registered() {
		app.say("%s\n", m.T(KeyDoctorNotInstalled))
		return ErrUnhealthy
	}
	env, _ := app.Sys.ReadFile(EnvFilePath)
	if !found {
		a = answersFromEnv(env)
	}
	d := doctor{app: app, det: det, env: env, m: m, b: &Builder{Sys: app.Sys, Det: det, A: a, M: m, Sleep: app.Sleep}}
	findings := d.run(ctx)
	engine.PrintFindings(app.Out, findings, m)
	interactive := engine.IsTerminal(app.InFD)
	remaining, err := engine.ApplyFixes(ctx, findings, engine.FixOptions{
		Fix: opts.Fix, Interactive: interactive, Prompter: engine.NewPrompter(app.In, app.Out, app.InFD, m), Out: app.Out, M: m,
	})
	if err != nil {
		return err
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

func answersFromEnv(env []byte) Answers {
	get := EnvGetter(env)
	a := DefaultAnswers()
	a.URL, a.ServerOrigin, a.CAFile = get(config.EnvURL), get(config.EnvServerOrigin), get(config.EnvCAFile)
	if v := get(config.EnvDriver); v != "" {
		a.Driver = v
	}
	if n, err := strconv.Atoi(get(config.EnvCapacity)); err == nil {
		a.Capacity = n
	}
	return a
}

func (d doctor) run(ctx context.Context) []engine.Finding {
	out := []engine.Finding{d.installed()}
	if out[0].Severity == engine.SeverityFail {
		return out
	}
	if d.b.Sandbox() {
		out = append(out, d.podman(), d.kernel(), d.subIDs())
	}
	out = append(out, d.unit(), d.service(ctx), d.exposure(ctx), d.permissions(), d.identity(), d.pending())
	return append(out, d.server(ctx)...)
}

func (d doctor) fail(title, detail, advice string, fix *engine.Step) engine.Finding {
	return engine.Finding{Severity: engine.SeverityFail, Title: title, Detail: detail, Advice: advice, Fix: fix}
}

func (d doctor) installed() engine.Finding {
	var missing []string
	for _, p := range []string{BinaryPath, UnitPath, EnvFilePath, StateDir} {
		if !engine.Exists(d.app.Sys, p) {
			missing = append(missing, p)
		}
	}
	if !d.det.User {
		missing = append(missing, "user "+ServiceUser)
	}
	if len(missing) > 0 {
		return d.fail(d.m.T(KeyCheckInstalled), strings.Join(missing, ", "), d.m.T(KeyAdviceReinstall), nil)
	}
	return engine.OK(d.m.T(KeyCheckInstalled))
}

func (d doctor) podman() engine.Finding {
	title := d.m.T(KeyCheckPodman, MinPodman)
	switch {
	case d.det.Podman == "":
		return d.fail(title, d.m.T(KeyPodmanMissing), d.m.T(KeyAdviceReinstall), nil)
	case !PodmanAtLeast(d.det.Podman, MinPodman):
		return d.fail(title, d.m.T(KeyBlockPodmanOld, d.det.Podman, MinPodman), d.m.T(KeyAdvicePodman), nil)
	case !d.det.Crun || !d.det.Newuidmap:
		return d.fail(title, d.m.T(KeyHelpersMissing), d.m.T(KeyAdviceReinstall), nil)
	}
	f := engine.OK(title)
	f.Detail = "Podman " + d.det.Podman
	return f
}

func (d doctor) kernel() engine.Finding {
	title := d.m.T(KeyCheckKernel)
	switch {
	case !d.det.CgroupV2:
		return d.fail(title, d.m.T(KeyBlockCgroup), "", nil)
	case !d.det.UserNS:
		return d.fail(title, d.m.T(KeyBlockUserNS), "", nil)
	case d.det.AppArmorNS:
		return d.fail(title, d.m.T(KeyBlockAppArmor), "", nil)
	}
	return engine.OK(title)
}

func (d doctor) subIDs() engine.Finding {
	need := d.b.A.Capacity * SubIDsPerJob
	title := d.m.T(KeyCheckSubIDs)
	if d.det.SubUIDs >= need && d.det.SubGIDs >= need {
		return engine.OK(title)
	}
	fix := d.b.subIDStep()
	fix.Done = nil
	detail := d.m.T(KeySubIDsShort, d.det.SubUIDs, d.det.SubGIDs, need)
	return d.fail(title, detail, "", &fix)
}

func (d doctor) unit() engine.Finding {
	title := d.m.T(KeyCheckUnit)
	current, err := d.app.Sys.ReadFile(UnitPath)
	if err == nil && bytes.Equal(current, UnitFile) {
		return engine.OK(title)
	}
	fix := d.b.unitStep()
	fix.Done = nil
	restart := engine.Cmd("systemctl", "restart", ServiceName)
	apply := fix.Apply
	fix.Apply = func(ctx context.Context) error {
		if err := apply(ctx); err != nil {
			return err
		}
		_, err := engine.Run(ctx, d.app.Sys, restart)
		return err
	}
	fix.Commands = append(fix.Commands, restart.String())
	return engine.Finding{Severity: engine.SeverityWarn, Title: title, Detail: d.m.T(KeyUnitDrift, UnitPath), Fix: &fix}
}

func (d doctor) service(ctx context.Context) engine.Finding {
	title := d.m.T(KeyCheckService)
	active := d.b.serviceActive(ctx)
	enabled := engine.Succeeds(ctx, d.app.Sys, engine.Cmd("systemctl", "is-enabled", "--quiet", ServiceName))
	if active && enabled {
		return engine.OK(title)
	}
	fix := engine.CommandStep(d.app.Sys, d.m.T(KeyFixService), nil, []engine.Command{
		engine.Cmd("systemctl", "enable", ServiceName), engine.Cmd("systemctl", "restart", ServiceName),
	}, nil)
	detail := d.m.T(KeyServiceState, d.m.YesNo(active), d.m.YesNo(enabled))
	return d.fail(title, detail, "journalctl -u "+ServiceName+" -n 50", &fix)
}

func (d doctor) exposure(ctx context.Context) engine.Finding {
	title := d.m.T(KeyCheckExposure, fmt.Sprintf("%.1f", MaxExposure))
	if !d.det.AnalyzeTools {
		return engine.Finding{Severity: engine.SeverityInfo, Title: title, Detail: d.m.T(KeyNoAnalyze)}
	}
	out, err := d.app.Sys.Run(ctx, engine.Cmd("systemd-analyze", "security", "--no-pager", ServiceUnit))
	score, level, ok := ParseExposure(out.Stdout)
	if err != nil && !ok {
		return d.fail(title, strings.TrimSpace(err.Error()), "", nil)
	}
	if !ok {
		return d.fail(title, d.m.T(KeyExposureUnknown), "", nil)
	}
	detail := fmt.Sprintf("%.1f %s", score, level)
	if score > MaxExposure {
		return d.fail(title, detail, d.m.T(KeyAdviceExposure), nil)
	}
	f := engine.OK(title)
	f.Detail = detail
	return f
}

func ParseExposure(out []byte) (float64, string, bool) {
	match := exposureRe.FindSubmatch(out)
	if match == nil {
		return 0, "", false
	}
	score, err := strconv.ParseFloat(string(match[1]), 64)
	if err != nil {
		return 0, "", false
	}
	return score, string(match[2]), true
}
