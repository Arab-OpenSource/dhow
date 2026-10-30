package installer_test

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Khaymah/dhow/internal/installer"
	"github.com/Khaymah/dhow/internal/installer/fakesys"
)

var verbRe = regexp.MustCompile(`%[sdqvt]`)

func TestEngineCatalogsHaveTheSameKeysAndVerbs(t *testing.T) {
	en, ar := installer.EngineCatalog(installer.LangEn), installer.EngineCatalog(installer.LangAr)
	assert.Equal(t, installer.Keys(en), installer.Keys(ar))
	for k, v := range en {
		assert.Equal(t, verbRe.FindAllString(v, -1), verbRe.FindAllString(ar[k], -1), string(k))
		assert.NotEmpty(t, ar[k], string(k))
	}
}

func TestMessagesMergeTheAppCatalogOverTheEngine(t *testing.T) {
	en := map[installer.Key]string{"app.hello": "Hello %s", installer.KeyNone: "nothing"}
	ar := map[installer.Key]string{"app.hello": "أهلًا %s"}
	m := installer.NewMessages(installer.LangAr, en, ar)
	assert.Equal(t, "أهلًا سارة", m.T("app.hello", "سارة"))
	assert.Equal(t, "لا يوجد", m.T(installer.KeyNone), "the engine's Arabic wins over the app's English")
	assert.Equal(t, "nothing", installer.NewMessages(installer.LangEn, en, ar).T(installer.KeyNone))
	assert.Equal(t, "missing.key", m.T("missing.key"))
	assert.Equal(t, "نعم", m.YesNo(true))
}

func TestChooseLang(t *testing.T) {
	assert.Equal(t, installer.LangAr, installer.ChooseLang("", "", "ar_EG.UTF-8"))
	assert.Equal(t, installer.LangEn, installer.ChooseLang("", "", "ar_EG"))
	assert.Equal(t, installer.LangEn, installer.ChooseLang("en", "ar", "ar_EG.UTF-8"))
	assert.Equal(t, installer.LangAr, installer.ChooseLang("", "ar", "C"))
}

func TestParseOSReleaseFamilies(t *testing.T) {
	tests := []struct {
		release string
		family  installer.Family
		el      bool
		fedora  bool
		major   int
	}{
		{"ID=debian\nVERSION_ID=\"13\"\nVERSION_CODENAME=trixie\n", installer.FamilyApt, false, false, 13},
		{"ID=ubuntu\nVERSION_ID=\"24.04\"\n", installer.FamilyApt, false, false, 24},
		{"ID=\"rocky\"\nVERSION_ID=\"10.0\"\n", installer.FamilyDnf, true, false, 10},
		{"ID=\"almalinux\"\nVERSION_ID=\"10.0\"\n", installer.FamilyDnf, true, false, 10},
		{"ID=fedora\nVERSION_ID=43\n", installer.FamilyDnf, false, true, 43},
		{"ID=arch\n", installer.FamilyUnknown, false, false, 0},
	}
	for _, tt := range tests {
		d := installer.ParseOSRelease([]byte(tt.release))
		assert.Equal(t, tt.family, d.Family, tt.release)
		assert.Equal(t, tt.el, d.EL, tt.release)
		assert.Equal(t, tt.fedora, d.Fedora, tt.release)
		assert.Equal(t, tt.major, d.Major(), tt.release)
		assert.NotEmpty(t, d.PrettyName, tt.release)
	}
}

type sample struct {
	Name  string `yaml:"name"`
	Count int    `yaml:"count"`
}

func TestDecodeAnswers(t *testing.T) {
	var s sample
	require.NoError(t, installer.DecodeAnswers(strings.NewReader("name: a\ncount: 2\n"), &s))
	assert.Equal(t, sample{Name: "a", Count: 2}, s)
	for in, want := range map[string]string{
		"":                            "the file is empty",
		"nme: a\n":                    "field nme not found",
		"name: a\n---\nname: b\n":     "only one YAML document",
		strings.Repeat("#", 64<<10+1): "larger than",
	} {
		err := installer.DecodeAnswers(strings.NewReader(in), &sample{})
		require.ErrorIs(t, err, installer.ErrInvalidAnswers, want)
		assert.Contains(t, err.Error(), want)
	}
	data, err := installer.EncodeAnswers("# header\n", sample{Name: "x"})
	require.NoError(t, err)
	assert.Equal(t, "# header\nname: x\ncount: 0\n", string(data))
}

func TestEnvRoundTrip(t *testing.T) {
	entries := installer.ParseEnv([]byte("# note\nA=1\nB=\"two words\"\n\nbroken\nA=3\n"))
	assert.Equal(t, "1", installer.EnvValue(entries, "A"))
	assert.Equal(t, "two words", installer.EnvValue(entries, "B"))
	out := installer.RenderEnv([]installer.EnvEntry{{Key: "A", Value: "9"}}, entries)
	assert.Equal(t, "A=9\nB=two words\n", string(out))
}

func TestExecuteSkipsDoneStepsAndRollsBackAFailure(t *testing.T) {
	f := fakesys.New()
	f.Put("/etc/old.conf", "old", 0o644)
	m := installer.NewMessages(installer.LangEn, nil, nil)
	var out bytes.Buffer
	steps := []installer.Step{
		installer.FileStep(f, "same", "/etc/old.conf", []byte("old"), installer.FileMode{Perm: 0o644}),
		installer.FileStep(f, "new", "/etc/new.conf", []byte("new"), installer.FileMode{Perm: 0o600}),
		installer.DirStep(f, "dir", "/var/lib/x", installer.FileMode{Perm: 0o700}),
		installer.CommandStep(f, "boom", nil, []installer.Command{installer.Cmd("false")}, []installer.Command{installer.Cmd("undo")}),
	}
	f.Fail["false"] = true
	err := installer.Runner{Out: &out, M: m}.Execute(context.Background(), steps)
	require.ErrorIs(t, err, installer.ErrCommandFailed)
	assert.Contains(t, out.String(), "[1/4] same: already done")
	assert.Contains(t, out.String(), "rolled back")
	assert.True(t, f.Ran("undo"))
	assert.Equal(t, "new", f.Content("/etc/new.conf"), "earlier steps stay; a re-run skips them")

	out.Reset()
	f.Fail = map[string]bool{}
	require.NoError(t, installer.Runner{Out: &out, M: m}.Execute(context.Background(), steps))
	assert.Contains(t, out.String(), "[2/4] new: already done")
	assert.Contains(t, out.String(), "[3/4] dir: already done")
}

func TestFilesStepUndoRestoresEveryFile(t *testing.T) {
	f := fakesys.New()
	f.Put("/a", "before", 0o644)
	files := []installer.File{{Path: "/a", Data: []byte("after"), Mode: installer.FileMode{Perm: 0o644}}, {Path: "/b", Data: []byte("b"), Mode: installer.FileMode{Perm: 0o644}}}
	s := installer.FilesStep(f, "files", files, []installer.Command{installer.Cmd("reload")}, nil)
	f.Fail["reload"] = true
	require.Error(t, s.Apply(context.Background()))
	require.NoError(t, s.Undo(context.Background()))
	assert.Equal(t, "before", f.Content("/a"))
	assert.False(t, f.Has("/b"))
	same, err := installer.FilesMatch(f, files)
	require.NoError(t, err)
	assert.False(t, same)
}

func TestRemoveStepIsIdempotent(t *testing.T) {
	f := fakesys.New()
	f.PutDir("/x", installer.FileMode{Perm: 0o700})
	f.Put("/x/a", "1", 0o600)
	s := installer.RemoveStep(f, "remove", "/x", "/y")
	done, err := s.Done(context.Background())
	require.NoError(t, err)
	assert.False(t, done)
	require.NoError(t, s.Apply(context.Background()))
	assert.False(t, f.Has("/x/a"))
	done, err = s.Done(context.Background())
	require.NoError(t, err)
	assert.True(t, done)
}

func TestPackageStepsPerFamily(t *testing.T) {
	f := fakesys.New()
	apt := installer.InstallStep(f, installer.FamilyApt, "pkgs", []string{"podman", "uidmap"})
	dnf := installer.InstallStep(f, installer.FamilyDnf, "pkgs", []string{"podman"})
	assert.Equal(t, []string{"apt-get update", "apt-get install -y --no-install-recommends podman uidmap"}, apt.Commands)
	assert.Equal(t, []string{"dnf install -y podman"}, dnf.Commands)
	f.Stdout["dpkg-query -W -f=${Status} podman"] = "install ok installed"
	f.OK["dpkg-query -W -f=${Status} podman"] = true
	assert.True(t, installer.PackagesInstalled(context.Background(), f, installer.FamilyApt, []string{"podman"}))
	assert.False(t, installer.PackagesInstalled(context.Background(), f, installer.FamilyApt, []string{"uidmap"}))
	require.NoError(t, apt.Apply(context.Background()))
	cmds := f.Commands()
	assert.Equal(t, []string{installer.AptEnv}, cmds[len(cmds)-1].Env)
}

func TestPrintPlanAndBlockers(t *testing.T) {
	m := installer.NewMessages(installer.LangEn, nil, nil)
	var out bytes.Buffer
	plan := installer.Plan{Notes: []string{"a note"}, Steps: []installer.Step{{Title: "one", Files: []string{"/f"}, Packages: []string{"p"}, Commands: []string{"c"}}}}
	installer.PrintPlan(context.Background(), &out, plan, m)
	for _, want := range []string{"a note", "Plan:", " 1. one", "file /f", "package p", "run c", "Packages to install: p"} {
		assert.Contains(t, out.String(), want)
	}
	out.Reset()
	installer.PrintPlan(context.Background(), &out, installer.Plan{Blockers: []string{"no podman"}}, m)
	assert.Contains(t, out.String(), "The install cannot continue:\n  - no podman")
}

func TestApplyFixesCountsWhatIsLeft(t *testing.T) {
	m := installer.NewMessages(installer.LangEn, nil, nil)
	fixed := false
	fix := installer.Step{Title: "fix it", Apply: func(context.Context) error { fixed = true; return nil }}
	broken := installer.Step{Title: "cannot", Apply: func(context.Context) error { return errors.New("nope") }}
	findings := []installer.Finding{
		installer.OK("fine"),
		{Severity: installer.SeverityWarn, Title: "fixable", Fix: &fix},
		{Severity: installer.SeverityFail, Title: "manual", Advice: "do this"},
		{Severity: installer.SeverityFail, Title: "fix fails", Fix: &broken},
	}
	var out bytes.Buffer
	installer.PrintFindings(&out, findings, m)
	assert.Contains(t, out.String(), "FAIL  manual")
	assert.Contains(t, out.String(), "do this")
	assert.Equal(t, installer.SeverityFail, installer.Worst(findings))

	left, err := installer.ApplyFixes(context.Background(), findings, installer.FixOptions{Out: &out, M: m})
	require.NoError(t, err)
	assert.Equal(t, 3, left, "without --fix nothing is applied")
	assert.False(t, fixed)
	left, err = installer.ApplyFixes(context.Background(), findings, installer.FixOptions{Fix: true, Out: &out, M: m})
	require.NoError(t, err)
	assert.Equal(t, 2, left)
	assert.True(t, fixed)
}

func TestPrompterAsksAgainAndHidesSecrets(t *testing.T) {
	m := installer.NewMessages(installer.LangEn, nil, nil)
	var out bytes.Buffer
	p := installer.NewPrompter(strings.NewReader("bad\ngood\n3\n2\nmaybe\ny\nhunter2\n"), &out, -1, m)
	v, err := p.Ask("Name", "", func(s string) error {
		if s != "good" {
			return errors.New("try again")
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "good", v)
	choice, err := p.Choose("Pick", []installer.Option{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}}, "a")
	require.NoError(t, err)
	assert.Equal(t, "b", choice)
	yes, err := p.Confirm("Sure?", false)
	require.NoError(t, err)
	assert.True(t, yes)
	secret, err := p.Secret("Token", func(string) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, "hunter2", secret)
	assert.NotContains(t, out.String(), "hunter2")
	_, err = p.Ask("More", "", nil)
	require.ErrorIs(t, err, installer.ErrNoInput)
}
