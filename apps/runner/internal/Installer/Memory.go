package installer

import (
	"maps"
	"strconv"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	engine "github.com/Khaymah/dhow/internal/installer"
)

const (
	mib             = int64(1) << 20
	gib             = int64(1) << 30
	fitStep         = 64 * mib
	MinHeadroom     = gib
	HeadroomPercent = 25
	MinFitMemory    = 512 * mib
	MinFitDisk      = 512 * mib
	MinFitHost      = MinFitMemory + MinFitDisk + MinHeadroom
)

type MemoryCheck struct {
	Machine  int64
	Current  config.JobMemory
	Explicit bool
	Err      error
	Fits     bool
	Fitted   config.JobMemory
	CanFit   bool
}

func Headroom(machine int64) int64 { return max(MinHeadroom, machine/100*HeadroomPercent) }

func FitJobMemory(machine int64) (config.JobMemory, bool) {
	def := config.DefaultJobMemory()
	budget := machine - Headroom(machine)
	share := def.MemoryBytes / mib * (max(budget, 0) / mib) / (def.Total() / mib) * mib
	memory := roundDown(max(share, MinFitMemory))
	fit := config.JobMemory{MemoryBytes: memory, DiskBytes: roundDown(budget - memory), Tmpfs: def.Tmpfs}
	if fit.DiskBytes < MinFitDisk {
		return fit, false
	}
	return fit, config.CheckJobMemory(fit, machine) == nil
}

func roundDown(n int64) int64 { return n / fitStep * fitStep }

func CheckMemory(env []byte, machine int64) MemoryCheck {
	get := EnvGetter(env)
	c := MemoryCheck{Machine: machine, Explicit: get(config.EnvMaxMemory) != "" || get(config.EnvMaxDisk) != ""}
	c.Current, c.Err = config.ReadJobMemory(get)
	c.Fits = c.Err == nil && config.CheckJobMemory(c.Current, machine) == nil
	c.Fitted, c.CanFit = FitJobMemory(machine)
	return c
}

func FormatSize(n int64) string {
	if n%gib == 0 {
		return strconv.FormatInt(n/gib, 10) + "GiB"
	}
	if n%mib == 0 {
		return strconv.FormatInt(n/mib, 10) + "MiB"
	}
	return strconv.FormatInt(n, 10)
}

func hostSize(n int64) string { return FormatSize(n / mib * mib) }

func (c MemoryCheck) TooHigh(m engine.Messages) string {
	return m.T(KeyMemoryTooHigh, FormatSize(c.Current.Total()), config.EnvMaxMemory, FormatSize(c.Current.MemoryBytes),
		config.EnvMaxDisk, FormatSize(c.Current.DiskBytes), hostSize(c.Machine))
}

func (c MemoryCheck) Advice(m engine.Messages) string {
	if !c.CanFit {
		return m.T(KeyAdviceMemorySmall, FormatSize(MinFitHost))
	}
	return m.T(KeyAdviceMemory, EnvFilePath, config.EnvMaxMemory, FormatSize(c.Fitted.MemoryBytes),
		config.EnvMaxDisk, FormatSize(c.Fitted.DiskBytes))
}

func (c MemoryCheck) fittedNote(m engine.Messages) string {
	def := config.DefaultJobMemory()
	return m.T(KeyNoteMemoryFitted, hostSize(c.Machine), FormatSize(def.Total()), FormatSize(def.MemoryBytes),
		FormatSize(def.DiskBytes), hostSize(Headroom(c.Machine)), EnvFilePath,
		config.EnvMaxMemory, FormatSize(c.Fitted.MemoryBytes), config.EnvMaxDisk, FormatSize(c.Fitted.DiskBytes))
}

func (c MemoryCheck) tooSmall(m engine.Messages) string {
	def := config.DefaultJobMemory()
	return m.T(KeyBlockMemorySmall, hostSize(c.Machine), FormatSize(def.Total()), FormatSize(MinFitMemory),
		FormatSize(MinFitDisk), FormatSize(MinHeadroom), FormatSize(MinFitHost), config.EnvMaxMemory, config.EnvMaxDisk)
}

func (b *Builder) fitMemory() (blockers, notes []string) {
	c := CheckMemory(RenderEnv(b.A, b.existing(EnvFilePath)), b.Det.MemoryBytes)
	switch {
	case c.Err != nil:
		return []string{b.M.T(KeyBlockLimitsInvalid, c.Err.Error())}, nil
	case c.Fits:
		return nil, nil
	case c.Explicit:
		return []string{c.TooHigh(b.M) + " " + c.Advice(b.M)}, nil
	case !c.CanFit:
		return []string{c.tooSmall(b.M)}, nil
	}
	env := maps.Clone(b.A.Env)
	if env == nil {
		env = map[string]string{}
	}
	env[config.EnvMaxMemory] = FormatSize(c.Fitted.MemoryBytes)
	env[config.EnvMaxDisk] = FormatSize(c.Fitted.DiskBytes)
	b.A.Env = env
	return nil, []string{c.fittedNote(b.M)}
}
