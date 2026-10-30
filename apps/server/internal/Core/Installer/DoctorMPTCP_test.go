package installer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engine "github.com/Khaymah/dhow/internal/installer"
)

func TestDoctorWarnsAboutMPTCPButStillPasses(t *testing.T) {
	f, _ := installedHost(t)
	f.put(engine.MPTCPEnabledPath, "1\n", 0o644)
	f.put(engine.KernelReleasePath, "6.8.0-45-generic\n", 0o444)
	app, out := newApp(f, "")
	require.NoError(t, app.Doctor(context.Background(), DoctorOptions{Fix: true}), out.String())
	assert.Contains(t, out.String(), "WARN  Kernel multipath TCP (MPTCP, CVE-2026-98432)")
	assert.Contains(t, out.String(), "MPTCP is on on this host (net.mptcp.enabled=1). Kernel 6.8.0-45-generic.")
	assert.Contains(t, out.String(), "sysctl -w net.mptcp.enabled=0")
	assert.Contains(t, out.String(), "Nothing blocks Dhow, but read the warnings above.")
	assert.NotContains(t, out.String(), "Everything looks good.")
	assert.False(t, f.ran("sysctl"), "neither install nor doctor --fix changes the host sysctl")
	for p := range f.files {
		assert.NotContains(t, p, "sysctl.")
	}
	assert.Equal(t, "1\n", f.content(engine.MPTCPEnabledPath))
}

func TestDoctorPassesWhenMPTCPIsOffOrMissing(t *testing.T) {
	for _, enabled := range []string{"0\n", ""} {
		f, _ := installedHost(t)
		if enabled != "" {
			f.put(engine.MPTCPEnabledPath, enabled, 0o644)
		}
		app, out := newApp(f, "")
		require.NoError(t, app.Doctor(context.Background(), DoctorOptions{}), out.String())
		assert.Contains(t, out.String(), "ok    Kernel multipath TCP (MPTCP, CVE-2026-98432)")
		assert.Contains(t, out.String(), "Everything looks good.")
	}
}
