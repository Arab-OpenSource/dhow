package installer

import (
	"errors"
	"io/fs"
	"strings"
)

const (
	MPTCPEnabledPath  = "/proc/sys/net/mptcp/enabled"
	KernelReleasePath = "/proc/sys/kernel/osrelease"
)

func KernelRelease(sys System, m Messages) string {
	data, err := sys.ReadFile(KernelReleasePath)
	release := strings.TrimSpace(string(data))
	if err != nil || release == "" {
		return m.T(KeyKernelUnknown)
	}
	return release
}

func KernelMPTCP(sys System, m Messages) Finding {
	title := m.T(KeyCheckMPTCP)
	release := KernelRelease(sys, m)
	data, err := sys.ReadFile(MPTCPEnabledPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Finding{Severity: SeverityOK, Title: title, Detail: m.T(KeyMPTCPAbsent, release)}
	case err != nil:
		return Finding{Severity: SeverityInfo, Title: title, Detail: m.T(KeyMPTCPUnreadable, MPTCPEnabledPath, release)}
	}
	value := strings.TrimSpace(string(data))
	if value == "0" {
		return Finding{Severity: SeverityOK, Title: title, Detail: m.T(KeyMPTCPOff, release)}
	}
	return Finding{
		Severity: SeverityWarn,
		Advisory: true,
		Title:    title,
		Detail:   m.T(KeyMPTCPOn, value, release),
		Advice:   m.T(KeyAdviceMPTCP, m.T(KeyKernelAdvisoriesURL)),
	}
}
