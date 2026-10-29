package installer

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

const (
	MemInfoPath   = "/proc/meminfo"
	CPUOnlinePath = "/sys/devices/system/cpu/online"
	DataRoot      = "/var/lib"

	MinimumCPUs         = 1
	RecommendedCPUs     = 2
	MinimumMemory       = 2 * gib
	RecommendedMemory   = 4 * gib
	MinimumDiskFree     = 5 * gib
	RecommendedDiskFree = 20 * gib

	diskFailFree = gib
	diskWarnFree = MinimumDiskFree

	memoryVisiblePercent = 90
)

type Level int

const (
	LevelUnknown Level = iota
	LevelOK
	LevelBelowRecommended
	LevelBelowMinimum
)

type HostResources struct {
	CPUs     int
	Memory   uint64
	DiskFree uint64
	DiskPath string
}

type ResourceCheck struct {
	Title       string
	Have        string
	Minimum     string
	Recommended string
	Level       Level
}

func ReadHostResources(sys System) HostResources {
	h := HostResources{DiskPath: DataRoot}
	if data, err := sys.ReadFile(MemInfoPath); err == nil {
		h.Memory = ParseMemTotal(data)
	}
	if data, err := sys.ReadFile(CPUOnlinePath); err == nil {
		h.CPUs = ParseCPUList(string(data))
	}
	if free, err := sys.FreeBytes(DataRoot); err == nil {
		h.DiskFree = free
	}
	return h
}

func ParseMemTotal(data []byte) uint64 {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || fields[0] != "MemTotal:" {
			continue
		}
		kib, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || kib > (1<<54) {
			return 0
		}
		return kib << 10
	}
	return 0
}

func ParseCPUList(list string) int {
	total := 0
	for _, part := range strings.Split(strings.TrimSpace(list), ",") {
		if part == "" {
			continue
		}
		lo, hi, isRange := strings.Cut(part, "-")
		a, err := strconv.Atoi(lo)
		if err != nil || a < 0 {
			return 0
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(hi); err != nil || b < a {
				return 0
			}
		}
		total += b - a + 1
	}
	return total
}

func memoryLevel(have uint64) Level {
	visible := func(nominal uint64) uint64 { return nominal / 100 * memoryVisiblePercent }
	switch {
	case have == 0:
		return LevelUnknown
	case have < visible(MinimumMemory):
		return LevelBelowMinimum
	case have < visible(RecommendedMemory):
		return LevelBelowRecommended
	default:
		return LevelOK
	}
}

func countLevel(have, minimum, recommended uint64) Level {
	switch {
	case have == 0:
		return LevelUnknown
	case have < minimum:
		return LevelBelowMinimum
	case have < recommended:
		return LevelBelowRecommended
	default:
		return LevelOK
	}
}

func cpuLevel(have int) Level {
	switch {
	case have <= 0:
		return LevelUnknown
	case have < MinimumCPUs:
		return LevelBelowMinimum
	case have < RecommendedCPUs:
		return LevelBelowRecommended
	default:
		return LevelOK
	}
}

func FormatBytes(b uint64) string {
	if b >= gib {
		return strconv.FormatFloat(float64(b)/gib, 'f', 1, 64) + " GiB"
	}
	return strconv.FormatUint(b>>20, 10) + " MiB"
}

type ResourceChecks struct {
	CPUs   ResourceCheck
	Memory ResourceCheck
	Disk   ResourceCheck
}

func (r ResourceChecks) All() []ResourceCheck { return []ResourceCheck{r.CPUs, r.Memory, r.Disk} }

func CheckResources(h HostResources, m Messages) ResourceChecks {
	return ResourceChecks{
		CPUs: ResourceCheck{
			Title: m.T(KeyCheckCPUs), Have: strconv.Itoa(h.CPUs),
			Minimum: strconv.Itoa(MinimumCPUs), Recommended: strconv.Itoa(RecommendedCPUs),
			Level: cpuLevel(h.CPUs),
		},
		Memory: ResourceCheck{
			Title: m.T(KeyCheckMemory), Have: FormatBytes(h.Memory),
			Minimum: FormatBytes(MinimumMemory), Recommended: FormatBytes(RecommendedMemory),
			Level: memoryLevel(h.Memory),
		},
		Disk: ResourceCheck{
			Title: m.T(KeyCheckDisk, h.DiskPath), Have: FormatBytes(h.DiskFree),
			Minimum: FormatBytes(MinimumDiskFree), Recommended: FormatBytes(RecommendedDiskFree),
			Level: countLevel(h.DiskFree, MinimumDiskFree, RecommendedDiskFree),
		},
	}
}

func (c ResourceCheck) Message(m Messages) string {
	switch c.Level {
	case LevelBelowMinimum:
		return m.T(KeyResourceTooLow, c.Title, c.Have, c.Minimum, m.T(KeyRequirementsURL))
	case LevelBelowRecommended:
		return m.T(KeyResourceLow, c.Title, c.Have, c.Recommended, m.T(KeyRequirementsURL))
	case LevelUnknown:
		return m.T(KeyResourceUnknown, c.Title, m.T(KeyRequirementsURL))
	default:
		return m.T(KeyResourceEnough, c.Have, c.Minimum, c.Recommended)
	}
}

func (b *Builder) resourceBlockers() (blockers, notes []string) {
	for _, c := range CheckResources(b.Det.Resources, b.M).All() {
		switch {
		case c.Level == LevelBelowMinimum && !b.Det.Existing:
			blockers = append(blockers, c.Message(b.M))
		case c.Level == LevelBelowMinimum:
			notes = append(notes, c.Message(b.M)+"\n"+b.M.T(KeyResourceUpgrade))
		case c.Level != LevelOK:
			notes = append(notes, c.Message(b.M))
		}
	}
	return blockers, notes
}

func (d Doctor) resources() []Finding {
	checks := CheckResources(ReadHostResources(d.Sys), d.M)
	var out []Finding
	for _, c := range []ResourceCheck{checks.CPUs, checks.Memory} {
		f := Finding{Title: c.Title, Detail: c.Message(d.M)}
		switch c.Level {
		case LevelBelowMinimum:
			f.Severity = SeverityFail
		case LevelBelowRecommended:
			f.Severity = SeverityWarn
		case LevelUnknown:
			f.Severity = SeverityInfo
		default:
			f.Severity = SeverityOK
		}
		out = append(out, f)
	}
	return out
}
