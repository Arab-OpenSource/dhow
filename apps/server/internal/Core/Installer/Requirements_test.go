package installer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMemTotal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want uint64
	}{
		{"typical", "MemTotal:        2027872 kB\nMemFree:  1 kB\n", 2027872 << 10},
		{"not first", "MemFree: 1 kB\nMemTotal: 1024 kB\n", 1 << 20},
		{"missing", "MemFree: 1 kB\n", 0},
		{"garbage", "MemTotal: lots kB\n", 0},
		{"absurd", "MemTotal: 99999999999999999999 kB\n", 0},
		{"empty", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ParseMemTotal([]byte(tt.in)))
		})
	}
}

func TestParseCPUList(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"0\n", 1},
		{"0-3\n", 4},
		{"0-1,4-5,7", 5},
		{"", 0},
		{"3-1", 0},
		{"a-b", 0},
		{"-1", 0},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, ParseCPUList(tt.in))
		})
	}
}

func TestCheckResourcesLevels(t *testing.T) {
	const kib = 1 << 10
	tests := []struct {
		name                string
		host                HostResources
		cpus, memory, disk  Level
		messageHasMinimum   bool
		messageHasRecommend bool
	}{
		{"recommended host", HostResources{CPUs: 2, Memory: 4 * gib, DiskFree: 40 * gib}, LevelOK, LevelOK, LevelOK, false, false},
		{"a 4 GB VM reports a little less", HostResources{CPUs: 4, Memory: 3_900_000 * kib, DiskFree: 20 * gib}, LevelOK, LevelOK, LevelOK, false, false},
		{"minimum host", HostResources{CPUs: 1, Memory: 2 * gib, DiskFree: 5 * gib}, LevelBelowRecommended, LevelBelowRecommended, LevelBelowRecommended, false, true},
		{"a 2 GB VM reports a little less", HostResources{CPUs: 1, Memory: 1_950_000 * kib, DiskFree: 10 * gib}, LevelBelowRecommended, LevelBelowRecommended, LevelBelowRecommended, false, true},
		{"a 1 GB VPS", HostResources{CPUs: 1, Memory: 980_000 * kib, DiskFree: 25 * gib}, LevelBelowRecommended, LevelBelowMinimum, LevelOK, true, false},
		{"a full disk", HostResources{CPUs: 8, Memory: 16 * gib, DiskFree: 3 * gib}, LevelOK, LevelOK, LevelBelowMinimum, true, false},
		{"nothing readable", HostResources{}, LevelUnknown, LevelUnknown, LevelUnknown, false, false},
	}
	m := NewMessages(LangEn)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckResources(tt.host, m)
			assert.Equal(t, tt.cpus, got.CPUs.Level, "cpus")
			assert.Equal(t, tt.memory, got.Memory.Level, "memory")
			assert.Equal(t, tt.disk, got.Disk.Level, "disk")
			for _, c := range got.All() {
				msg := c.Message(m)
				if c.Level == LevelBelowMinimum {
					assert.Contains(t, msg, "below the minimum of "+c.Minimum)
					assert.Contains(t, msg, m.T(KeyRequirementsURL))
				}
				if c.Level == LevelBelowRecommended {
					assert.Contains(t, msg, "below the recommended "+c.Recommended)
				}
			}
		})
	}
}

func TestInstallRefusesAHostBelowTheMinimum(t *testing.T) {
	tests := []struct {
		name      string
		memKiB    string
		cpus      string
		free      uint64
		existing  bool
		blocked   bool
		wantInOut string
	}{
		{"1 GB of memory", "980000", "0", 50 * gib, false, true, "Memory (RAM): this host has 957 MiB, below the minimum of 2.0 GiB"},
		{"too little disk", "8000000", "0-1", 3 * gib, false, true, "Free disk space in /var/lib: this host has 3.0 GiB, below the minimum of 5.0 GiB"},
		{"minimum host warns", "2000000", "0", 10 * gib, false, false, "below the recommended 4.0 GiB"},
		{"an upgrade on a small host continues", "980000", "0", 50 * gib, true, false, "the upgrade continues"},
		{"a recommended host is quiet", "8000000", "0-3", 50 * gib, false, false, "Memory (RAM)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := liveHost(debianHost())
			f.resources(tt.memKiB, tt.cpus, tt.free)
			if tt.existing {
				f.put(AnswersPath, ciAnswers, 0o600)
			}
			f.put("/root/answers.yaml", ciAnswers, 0o600)
			app, out := newApp(f, "")
			err := app.Install(context.Background(), InstallOptions{Answers: "/root/answers.yaml", Yes: true})
			if tt.blocked {
				require.ErrorIs(t, err, ErrBlocked)
				assert.False(t, f.has(UnitPath), "nothing is installed on a host below the minimum")
			} else {
				require.NoError(t, err, out.String())
			}
			assert.Contains(t, out.String(), tt.wantInOut)
		})
	}
}

func TestDoctorChecksMemoryAndCPUs(t *testing.T) {
	tests := []struct {
		name   string
		memKiB string
		cpus   string
		want   map[string]Severity
	}{
		{"recommended", "8000000", "0-3", map[string]Severity{"Memory (RAM)": SeverityOK, "CPU cores": SeverityOK}},
		{"below recommended", "2000000", "0", map[string]Severity{"Memory (RAM)": SeverityWarn, "CPU cores": SeverityWarn}},
		{"below minimum", "900000", "0-1", map[string]Severity{"Memory (RAM)": SeverityFail, "CPU cores": SeverityOK}},
		{"unreadable", "", "", map[string]Severity{"Memory (RAM)": SeverityInfo, "CPU cores": SeverityInfo}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := debianHost()
			f.resources(tt.memKiB, tt.cpus, 100*gib)
			got := map[string]Severity{}
			for _, finding := range doctorFor(f, Answers{Mode: ModeNative}).resources() {
				got[finding.Title] = finding.Severity
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDoctorFailsBelowTheMinimum(t *testing.T) {
	f, _ := installedHost(t)
	f.resources("900000", "0", 100*gib)
	app, out := newApp(f, "")
	require.ErrorIs(t, app.Doctor(context.Background(), DoctorOptions{}), ErrUnhealthy)
	assert.Contains(t, out.String(), "below the minimum of 2.0 GiB")
	assert.Contains(t, out.String(), NewMessages(LangEn).T(KeyRequirementsURL))
}

func TestResourceMessagesInArabic(t *testing.T) {
	m := NewMessages(LangAr)
	c := CheckResources(HostResources{CPUs: 1, Memory: 900 << 20, DiskFree: 50 * gib}, m).Memory
	msg := c.Message(m)
	assert.Contains(t, msg, "الحد الأدنى")
	assert.Contains(t, msg, "2.0 GiB")
	assert.Contains(t, msg, m.T(KeyRequirementsURL))
}

func TestRequirementsLinkFollowsTheLanguage(t *testing.T) {
	assert.Equal(t, "https://github.com/Khaymah/dhow/blob/main/docs/requirements.md", NewMessages(LangAr).T(KeyRequirementsURL))
	assert.Equal(t, "https://github.com/Khaymah/dhow/blob/main/docs/en/requirements.md", NewMessages(LangEn).T(KeyRequirementsURL))
}
