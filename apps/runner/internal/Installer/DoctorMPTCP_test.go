package installer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	installer "github.com/Khaymah/dhow/apps/runner/internal/Installer"
	engine "github.com/Khaymah/dhow/internal/installer"
)

func TestDoctorWarnsAboutMPTCPButStillPasses(t *testing.T) {
	f, p := installed(t, debian13)
	f.Put(engine.MPTCPEnabledPath, "1\n", 0o644)
	f.Put(engine.KernelReleasePath, "6.12.48+deb13-amd64\n", 0o444)
	out, err := doctor(f, p, installer.DoctorOptions{Fix: true})
	require.NoError(t, err, out)
	assert.Contains(t, out, "WARN  Kernel multipath TCP (MPTCP, CVE-2026-98432)")
	assert.Contains(t, out, "MPTCP is on on this host (net.mptcp.enabled=1). Kernel 6.12.48+deb13-amd64.")
	assert.Contains(t, out, "sysctl -w net.mptcp.enabled=0")
	assert.Contains(t, out, "Nothing blocks Dhow, but read the warnings above.")
	assert.NotContains(t, out, "Everything looks good.")
	assert.False(t, f.Ran("sysctl"), "neither install nor doctor --fix changes the host sysctl")
	for path := range f.Files {
		assert.NotContains(t, path, "sysctl.")
	}
	assert.Equal(t, "1\n", f.Content(engine.MPTCPEnabledPath))
}

func TestDoctorPassesWhenMPTCPIsOffOrMissing(t *testing.T) {
	for _, enabled := range []string{"0\n", ""} {
		f, p := installed(t, debian13)
		if enabled != "" {
			f.Put(engine.MPTCPEnabledPath, enabled, 0o644)
		}
		out, err := doctor(f, p, installer.DoctorOptions{})
		require.NoError(t, err, out)
		assert.Contains(t, out, "ok    Kernel multipath TCP (MPTCP, CVE-2026-98432)")
		assert.Contains(t, out, "Everything looks good.")
	}
}

func TestTheMPTCPWarningReadsInArabic(t *testing.T) {
	f, p := installed(t, debian13)
	f.Put(engine.MPTCPEnabledPath, "1\n", 0o644)
	out, err := doctor(f, p, installer.DoctorOptions{Lang: engine.LangAr})
	require.NoError(t, err, out)
	assert.Contains(t, out, "تحذير MPTCP في نواة النظام (CVE-2026-98432)")
	assert.Contains(t, out, "MPTCP مفعّل على هذا الجهاز (net.mptcp.enabled=1). النواة غير معروفة.")
	assert.Contains(t, out, "لا شيء يمنع داو من العمل، لكن اقرأ التحذيرات أعلاه.")
}
