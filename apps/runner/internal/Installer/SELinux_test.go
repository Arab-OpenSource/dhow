package installer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	installer "github.com/Khaymah/dhow/apps/runner/internal/Installer"
	engine "github.com/Khaymah/dhow/internal/installer"
)

const selinuxRefusal = "SELinux is enforcing on this host, and jobs cannot run under SELinux enforcing yet"

func TestAnSELinuxEnforcingHostIsRefusedBeforeAnyStep(t *testing.T) {
	f := runnerHost(fedora43)
	f.Put(installer.SELinuxEnforce, "1\n", 0o644)
	out, p, err := installOn(t, f, 16*gib, basicAnswers, "")
	require.ErrorIs(t, err, installer.ErrBlocked)
	assert.Contains(t, out, selinuxRefusal)
	assert.Contains(t, out, "docs/en/runner/install.md#supported-hosts")
	assert.Contains(t, out, "we do not recommend it")
	assertNothingChanged(t, f, p)

	ar, _, err := installOn(t, f, 16*gib, basicAnswers, engine.LangAr)
	require.ErrorIs(t, err, installer.ErrBlocked)
	assert.Contains(t, ar, "ولا تعمل التشغيلات في هذا الوضع بعد")
	assert.Contains(t, ar, "docs/runner/install.md#الأجهزة-المدعومة")
}

func TestAnSELinuxPermissiveHostInstalls(t *testing.T) {
	f := runnerHost(fedora43)
	f.Put(installer.SELinuxEnforce, "0\n", 0o644)
	out, p, err := installOn(t, f, 16*gib, basicAnswers, "")
	require.NoError(t, err, out)
	assert.NotContains(t, out, selinuxRefusal)
	assert.Len(t, p.targets, 1)
}

func TestDoctorReportsSELinuxEnforcing(t *testing.T) {
	f, p := installed(t, debian13)
	out, err := doctor(f, p, installer.DoctorOptions{})
	require.NoError(t, err, out)
	assert.NotContains(t, out, selinuxRefusal)

	f.Put(installer.SELinuxEnforce, "1\n", 0o644)
	out, err = doctor(f, p, installer.DoctorOptions{})
	require.ErrorIs(t, err, installer.ErrUnhealthy)
	assert.Contains(t, out, selinuxRefusal)
	assert.Contains(t, out, "docs/en/runner/install.md#supported-hosts")
}
