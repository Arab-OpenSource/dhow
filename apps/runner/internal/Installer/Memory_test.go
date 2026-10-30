package installer_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	installer "github.com/Khaymah/dhow/apps/runner/internal/Installer"
	engine "github.com/Khaymah/dhow/internal/installer"
	"github.com/Khaymah/dhow/internal/installer/fakesys"
)

const (
	mib        = int64(1) << 20
	gib        = int64(1) << 30
	smallVPS   = int64(4112945152)
	fittedMem  = "DHOW_RUNNER_MAX_MEMORY=960MiB"
	fittedDisk = "DHOW_RUNNER_MAX_DISK=1920MiB"
)

func hostWithMemory(bytes int64) *fakesys.Sys {
	f := runnerHost(debian13)
	f.Put(engine.MemInfoPath, "MemTotal:       "+strconv.FormatInt(bytes>>10, 10)+" kB\nMemFree:         1024 kB\n", 0o444)
	return f
}

func installOn(t *testing.T, f *fakesys.Sys, bytes int64, yaml, lang string) (string, *fakeProtocol, error) {
	t.Helper()
	writeAnswers(f, yaml)
	p := &fakeProtocol{}
	app, out := newApp(f, p, "")
	app.Machine = func() (config.Machine, error) { return config.Machine{MemoryBytes: bytes}, nil }
	err := app.Install(context.Background(), installer.InstallOptions{Answers: answersPath, Lang: lang})
	return out.String(), p, err
}

func assertNothingChanged(t *testing.T, f *fakesys.Sys, p *fakeProtocol) {
	t.Helper()
	assert.False(t, f.Has(installer.EnvFilePath))
	assert.False(t, f.Has(installer.UnitPath))
	assert.False(t, f.Has(installer.AnswersPath))
	assert.False(t, f.Ran("useradd"))
	assert.False(t, f.Ran("systemctl"))
	assert.Empty(t, p.tokens)
}

func TestASmallHostGetsLowerJobLimitsAndThePlanSaysSo(t *testing.T) {
	f := hostWithMemory(smallVPS)
	out, p, err := installOn(t, f, smallVPS, basicAnswers, "")
	require.NoError(t, err, out)

	env := f.Content(installer.EnvFilePath)
	assert.Contains(t, env, fittedMem+"\n")
	assert.Contains(t, env, fittedDisk+"\n")
	assert.Contains(t, out, "writes lower limits to /etc/dhow-runner/runner.env: "+fittedMem+" and "+fittedDisk)
	assert.Contains(t, out, "keeps 980MiB for the runner and the system")
	assert.Less(t, strings.Index(out, "writes lower limits"), strings.Index(out, "Create the system user"))
	require.Len(t, p.targets, 1)
	assert.Equal(t, 960*mib, p.targets[0].Config.Limits.MemoryBytes)
	assert.Equal(t, 1920*mib, p.targets[0].Config.Limits.DiskBytes)
	assert.Contains(t, f.Content(installer.AnswersPath), "DHOW_RUNNER_MAX_MEMORY: 960MiB")

	again, _, err := installOn(t, f, smallVPS, basicAnswers, "")
	require.NoError(t, err, again)
	assert.Equal(t, env, f.Content(installer.EnvFilePath))
}

func TestANominalTwoGiBVPSPassesTheFit(t *testing.T) {
	f := hostWithMemory(1945 * mib)
	out, p, err := installOn(t, f, 1945*mib, basicAnswers, "")
	require.NoError(t, err, out)
	assert.Contains(t, f.Content(installer.EnvFilePath), "DHOW_RUNNER_MAX_DISK=640MiB\nDHOW_RUNNER_MAX_MEMORY=512MiB\n")
	require.Len(t, p.targets, 1)
	assert.Equal(t, 640*mib, p.targets[0].Config.Limits.DiskBytes)
}

func TestTheLowerLimitsNoteIsInArabicToo(t *testing.T) {
	f := hostWithMemory(smallVPS)
	out, _, err := installOn(t, f, smallVPS, basicAnswers, engine.LangAr)
	require.NoError(t, err, out)
	assert.Contains(t, out, "وتكتب حدودًا أقل في /etc/dhow-runner/runner.env: "+fittedMem+" و"+fittedDisk)
}

func TestABigEnoughHostKeepsTheDefaultLimits(t *testing.T) {
	f := hostWithMemory(16 * gib)
	out, p, err := installOn(t, f, 16*gib, basicAnswers, "")
	require.NoError(t, err, out)
	assert.NotContains(t, f.Content(installer.EnvFilePath), "MAX_MEMORY")
	assert.NotContains(t, f.Content(installer.EnvFilePath), "MAX_DISK")
	assert.NotContains(t, out, "lower limits")
	require.Len(t, p.targets, 1)
	assert.Equal(t, config.DefaultJobMemory().MemoryBytes, p.targets[0].Config.Limits.MemoryBytes)
}

func TestExplicitLimitsThatDoNotFitAreRefusedBeforeAnyStep(t *testing.T) {
	f := hostWithMemory(smallVPS)
	yaml := basicAnswers + "env:\n  DHOW_RUNNER_MAX_MEMORY: 3GiB\n  DHOW_RUNNER_MAX_DISK: 1GiB\n"
	out, p, err := installOn(t, f, smallVPS, yaml, "")
	require.ErrorIs(t, err, installer.ErrBlocked)
	assert.Contains(t, out, "Each job can use 4GiB of memory: DHOW_RUNNER_MAX_MEMORY=3GiB plus DHOW_RUNNER_MAX_DISK=1GiB")
	assert.Contains(t, out, "This host has only 3922MiB.")
	assert.Contains(t, out, "for example "+fittedMem+" and "+fittedDisk)
	assertNothingChanged(t, f, p)
}

func TestAHostBelowTheMinimumsIsRefused(t *testing.T) {
	f := hostWithMemory(1536 * mib)
	out, p, err := installOn(t, f, 1536*mib, basicAnswers, "")
	require.ErrorIs(t, err, installer.ErrBlocked)
	assert.Contains(t, out, "This host has 1536MiB of memory")
	assert.Contains(t, out, "so 1664MiB in total. Use a host with more memory")
	assertNothingChanged(t, f, p)
}

func TestInteractiveInstallOffersTheLowerLimits(t *testing.T) {
	answers := func(memory, disk string) string {
		return strings.Join([]string{"https://dhow.example.com", testToken, "build-02", "1", "1", memory, disk, "y"}, "\n") + "\n"
	}
	t.Run("accept", func(t *testing.T) {
		f := hostWithMemory(smallVPS)
		app, out := newApp(f, &fakeProtocol{}, answers("", ""))
		require.NoError(t, app.Install(context.Background(), installer.InstallOptions{}), out.String())
		assert.Contains(t, out.String(), "Memory per job [960MiB]")
		assert.Contains(t, out.String(), "[1920MiB]")
		assert.Contains(t, f.Content(installer.EnvFilePath), fittedMem)
	})
	t.Run("change", func(t *testing.T) {
		f := hostWithMemory(smallVPS)
		in := strings.Replace(answers("1GiB", "4GiB"), "4GiB\n", "4GiB\n1GiB\n", 1)
		app, out := newApp(f, &fakeProtocol{}, in)
		require.NoError(t, app.Install(context.Background(), installer.InstallOptions{}), out.String())
		assert.Contains(t, out.String(), "This host has only 3922MiB.")
		env := f.Content(installer.EnvFilePath)
		assert.Contains(t, env, "DHOW_RUNNER_MAX_MEMORY=1GiB\n")
		assert.Contains(t, env, "DHOW_RUNNER_MAX_DISK=1GiB\n")
		assert.NotContains(t, out.String(), "writes lower limits")
	})
}

func TestDoctorReportsTheSameMemoryCheck(t *testing.T) {
	f, p := installed(t, debian13)
	f.Put(engine.MemInfoPath, "MemTotal:       4016548 kB\n", 0o444)
	out, err := doctor(f, p, installer.DoctorOptions{})
	require.ErrorIs(t, err, installer.ErrUnhealthy)
	assert.Contains(t, out, "Each job can use 6GiB of memory: DHOW_RUNNER_MAX_MEMORY=2GiB plus DHOW_RUNNER_MAX_DISK=4GiB")
	assert.Contains(t, out, "for example "+fittedMem+" and "+fittedDisk)

	f.Put(installer.EnvFilePath, f.Content(installer.EnvFilePath)+fittedMem+"\n"+fittedDisk+"\n", 0o640)
	out, _ = doctor(f, p, installer.DoctorOptions{})
	assert.Contains(t, out, "Each job can use 2880MiB of memory (with the tmpfs disk), and this host has 3922MiB.")
}

func TestFitJobMemory(t *testing.T) {
	tests := []struct {
		name         string
		machine      int64
		memory, disk int64
		fits         bool
	}{
		{"2 GiB VPS", 1945 * mib, 512 * mib, 640 * mib, true},
		{"2 GiB host", 2 * gib, 512 * mib, 768 * mib, true},
		{"4 GiB VPS", smallVPS, 960 * mib, 1920 * mib, true},
		{"6 GiB host", 6 * gib, 1536 * mib, 3072 * mib, true},
		{"1.5 GiB host", 1536 * mib, 512 * mib, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fit, ok := installer.FitJobMemory(tt.machine)
			require.Equal(t, tt.fits, ok)
			assert.Equal(t, tt.memory, fit.MemoryBytes)
			if !ok {
				return
			}
			assert.Equal(t, tt.disk, fit.DiskBytes)
			assert.NoError(t, config.CheckJobMemory(fit, tt.machine))
			assert.LessOrEqual(t, fit.Total()+installer.Headroom(tt.machine), tt.machine)
			assert.GreaterOrEqual(t, fit.MemoryBytes, installer.MinFitMemory)
			assert.GreaterOrEqual(t, fit.DiskBytes, installer.MinFitDisk)
		})
	}
}
