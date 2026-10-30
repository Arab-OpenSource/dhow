package installer_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Khaymah/dhow/internal/installer"
	"github.com/Khaymah/dhow/internal/installer/fakesys"
)

type unreadable struct {
	*fakesys.Sys
	path string
}

func (u unreadable) ReadFile(p string) ([]byte, error) {
	if p == u.path {
		return nil, errors.New("permission denied")
	}
	return u.Sys.ReadFile(p)
}

func TestKernelMPTCP(t *testing.T) {
	en := installer.NewMessages(installer.LangEn, nil, nil)
	tests := []struct {
		name     string
		enabled  *string
		release  string
		severity installer.Severity
		advisory bool
		detail   string
	}{
		{"not built in", nil, "6.1.0-28-amd64\n", installer.SeverityOK, false, "This kernel has no MPTCP. Kernel 6.1.0-28-amd64."},
		{"turned off", new("0\n"), "6.12.48+deb13-amd64\n", installer.SeverityOK, false, "MPTCP is off on this host (net.mptcp.enabled=0). Kernel 6.12.48+deb13-amd64."},
		{"turned on", new("1\n"), "6.8.0-45-generic\n", installer.SeverityWarn, true, "MPTCP is on on this host (net.mptcp.enabled=1). Kernel 6.8.0-45-generic."},
		{"a patched version string still warns", new("1\n"), "7.3.0\n", installer.SeverityWarn, true, "Kernel 7.3.0."},
		{"no release file", new("1\n"), "", installer.SeverityWarn, true, "Kernel unknown."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := fakesys.New()
			if tt.enabled != nil {
				f.Put(installer.MPTCPEnabledPath, *tt.enabled, 0o644)
			}
			if tt.release != "" {
				f.Put(installer.KernelReleasePath, tt.release, 0o444)
			}
			got := installer.KernelMPTCP(f, en)
			assert.Equal(t, tt.severity, got.Severity)
			assert.Equal(t, tt.advisory, got.Advisory)
			assert.Contains(t, got.Detail, tt.detail)
			assert.Equal(t, "Kernel multipath TCP (MPTCP, CVE-2026-98432)", got.Title)
			assert.Nil(t, got.Fix, "doctor never changes the host sysctl")
			assert.Empty(t, f.CommandLines())
			if tt.advisory {
				assert.Contains(t, got.Advice, "sysctl -w net.mptcp.enabled=0")
				assert.Contains(t, got.Advice, "CVE-2026-98432")
				assert.Contains(t, got.Advice, "other services on this host")
				assert.Contains(t, got.Advice, "docs/en/security/kernel-advisories.md")
			}
		})
	}
}

func TestKernelMPTCPThatCannotBeReadIsOnlyInfo(t *testing.T) {
	f := fakesys.New()
	f.Put(installer.KernelReleasePath, "6.18.44\n", 0o444)
	got := installer.KernelMPTCP(unreadable{Sys: f, path: installer.MPTCPEnabledPath}, installer.NewMessages(installer.LangEn, nil, nil))
	assert.Equal(t, installer.SeverityInfo, got.Severity)
	assert.Contains(t, got.Detail, installer.MPTCPEnabledPath)
}

func TestKernelMPTCPReadsInArabic(t *testing.T) {
	f := fakesys.New()
	f.Put(installer.MPTCPEnabledPath, "1\n", 0o644)
	f.Put(installer.KernelReleasePath, "6.8.0-45-generic\n", 0o444)
	got := installer.KernelMPTCP(f, installer.NewMessages(installer.LangAr, nil, nil))
	assert.Contains(t, got.Detail, "MPTCP مفعّل على هذا الجهاز")
	assert.Contains(t, got.Detail, "6.8.0-45-generic")
	assert.Contains(t, got.Advice, "sysctl -w net.mptcp.enabled=0")
	assert.Contains(t, got.Advice, "https://github.com/Khaymah/dhow/blob/main/docs/security/kernel-advisories.md")
}

func TestAnAdvisoryWarningIsShownButNeverLeftOver(t *testing.T) {
	m := installer.NewMessages(installer.LangEn, nil, nil)
	findings := []installer.Finding{
		installer.OK("fine"),
		{Severity: installer.SeverityWarn, Advisory: true, Title: "kernel", Detail: "on", Advice: "update the kernel"},
	}
	var out bytes.Buffer
	installer.PrintFindings(&out, findings, m)
	assert.Contains(t, out.String(), "WARN  kernel")
	assert.Contains(t, out.String(), "update the kernel")
	assert.True(t, installer.HasAdvisory(findings))

	left, err := installer.ApplyFixes(context.Background(), findings, installer.FixOptions{Fix: true, Out: &out, M: m})
	require.NoError(t, err)
	assert.Zero(t, left)
	assert.False(t, installer.HasAdvisory(findings[:1]))
}
