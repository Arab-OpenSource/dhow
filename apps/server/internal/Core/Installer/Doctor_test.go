package installer

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoctorIsCleanAfterAnInstall(t *testing.T) {
	f, _ := installedHost(t)
	app, out := newApp(f, "")
	require.NoError(t, app.Doctor(context.Background(), DoctorOptions{}), out.String())
	assert.Contains(t, out.String(), "Everything looks good.")
	assert.Contains(t, out.String(), "journalctl -u dhow | grep setup_url")
	assert.NotContains(t, out.String(), "FAIL")
	assert.NotContains(t, out.String(), "WARN")
}

func TestDoctorFindsAndFixesProblems(t *testing.T) {
	tests := []struct {
		name   string
		breaks func(f *fakeSys)
		want   string
		check  func(t *testing.T, f *fakeSys)
	}{
		{"world-readable secret", func(f *fakeSys) { f.files[SecretKeyPath].mode = FileMode{Perm: 0o644} }, "File ownership and permissions", func(t *testing.T, f *fakeSys) {
			assert.Equal(t, uint32(0o600), uint32(f.files[SecretKeyPath].mode.Perm))
		}},
		{"edited unit", func(f *fakeSys) { f.put(UnitPath, "[Service]\nExecStart=/bin/true\n", 0o644) }, "The systemd unit matches this release", func(t *testing.T, f *fakeSys) {
			assert.Equal(t, string(UnitFile), f.content(UnitPath))
		}},
		{"stopped service", func(f *fakeSys) { delete(f.ok, "systemctl is-active --quiet dhow") }, "The dhow service is enabled and running", func(t *testing.T, f *fakeSys) {
			assert.True(t, f.ran("systemctl restart dhow"))
		}},
		{"trusted proxies dropped", func(f *fakeSys) {
			f.put(EnvFilePath, strings.Replace(f.content(EnvFilePath), "DHOW_TRUSTED_PROXIES=", "DHOW_TRUSTED_PROXIES=0.0.0.0/0", 1), 0o600)
		}, "trusts every address", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := installedHost(t)
			tt.breaks(f)
			f.commands = nil
			app, out := newApp(f, "")
			err := app.Doctor(context.Background(), DoctorOptions{})
			require.ErrorIs(t, err, ErrUnhealthy)
			assert.Contains(t, out.String(), tt.want)
			assert.Contains(t, out.String(), "doctor --fix")
			if tt.check == nil {
				return
			}
			assert.False(t, f.ran("systemctl restart dhow") && tt.name != "stopped service", "nothing is fixed without --fix")
			app, out = newApp(f, "")
			require.NoError(t, app.Doctor(context.Background(), DoctorOptions{Fix: true}), out.String())
			tt.check(t, f)
		})
	}
}

func TestDoctorNeedsAnInstall(t *testing.T) {
	app, out := newApp(debianHost(), "")
	require.ErrorIs(t, app.Doctor(context.Background(), DoctorOptions{}), ErrUnhealthy)
	assert.Contains(t, out.String(), AnswersPath)
}

func TestDoctorSecretsAreValidatedButNeverRegenerated(t *testing.T) {
	f, _ := installedHost(t)
	f.put(SecretKeyPath, "changeme\n", 0o600)
	app, out := newApp(f, "")
	require.ErrorIs(t, app.Doctor(context.Background(), DoctorOptions{Fix: true}), ErrUnhealthy)
	assert.Contains(t, out.String(), "looks like a placeholder")
	assert.Equal(t, "changeme\n", f.content(SecretKeyPath))
}

func doctorFor(f *fakeSys, a Answers) Doctor {
	det := Detect(context.Background(), f)
	m := NewMessages(LangEn)
	return Doctor{
		Sys: f, Det: det, A: a, M: m, Probe: func() (string, error) { return "nonce", nil },
		b: &Builder{Sys: f, Det: det, A: a, M: m, Sleep: func(time.Duration) {}},
	}
}

func TestTrustedProxiesSanity(t *testing.T) {
	tests := []struct {
		kind, value string
		want        Severity
		detail      string
	}{
		{ProxyCaddy, "127.0.0.1/32,::1/128", SeverityOK, ""},
		{ProxyCaddy, "", SeverityFail, "without 127.0.0.1"},
		{ProxyNginx, "10.0.0.0/8", SeverityFail, "without 127.0.0.1"},
		{ProxyExisting, "", SeverityWarn, "DHOW_TRUSTED_PROXIES is empty"},
		{ProxyExisting, "10.1.2.3", SeverityOK, ""},
		{ProxyExisting, "0.0.0.0/0", SeverityFail, "trusts every address"},
		{ProxyExisting, "::/0", SeverityFail, "trusts every address"},
		{ProxyExisting, "8.0.0.0/8", SeverityWarn, "very large public range"},
		{ProxyExisting, "192.168.0.0/16", SeverityOK, ""},
		{ProxyNone, "", SeverityOK, ""},
		{ProxyNone, "nope", SeverityFail, "not an IP or CIDR"},
	}
	for _, tt := range tests {
		t.Run(tt.kind+" "+tt.value, func(t *testing.T) {
			f := debianHost()
			f.put(EnvFilePath, "DHOW_TRUSTED_PROXIES="+tt.value+"\n", 0o600)
			got := doctorFor(f, Answers{Mode: ModeNative, Domain: "dhow.example.com", ListenAddr: DefaultListen, Proxy: ProxyAnswers{Kind: tt.kind}}).trustedProxies()
			assert.Equal(t, tt.want, got.Severity, got.Detail)
			assert.Contains(t, got.Detail, tt.detail)
		})
	}
}

func envelopeError(code string) []byte {
	return []byte(fmt.Sprintf(`{"Success":false,"Error":{"Code":"%s","Message":"x"}}`, code))
}

func TestPublicChecks(t *testing.T) {
	tests := []struct {
		name     string
		own      int
		ownCode  string
		foreign  int
		forCode  string
		certDays int
		clientIP string
		want     map[string]Severity
	}{
		{
			"healthy proxy", 401, "UNAUTHENTICATED", 403, "CSRF_FAILED", 60, "203.0.113.9",
			map[string]Severity{"Origin": SeverityOK, "TLS certificate": SeverityOK, "X-Forwarded-For": SeverityOK},
		},
		{
			"origin stripped", 403, "CSRF_FAILED", 403, "CSRF_FAILED", 60, "203.0.113.9",
			map[string]Severity{"Origin": SeverityFail},
		},
		{
			"origin not checked", 401, "UNAUTHENTICATED", 401, "UNAUTHENTICATED", 60, "203.0.113.9",
			map[string]Severity{"Origin": SeverityFail},
		},
		{
			"certificate expiring", 401, "UNAUTHENTICATED", 403, "CSRF_FAILED", 10, "203.0.113.9",
			map[string]Severity{"TLS certificate": SeverityWarn},
		},
		{
			"certificate almost expired", 401, "UNAUTHENTICATED", 403, "CSRF_FAILED", 2, "203.0.113.9",
			map[string]Severity{"TLS certificate": SeverityFail},
		},
		{
			"client address lost", 401, "UNAUTHENTICATED", 403, "CSRF_FAILED", 60, "127.0.0.1",
			map[string]Severity{"X-Forwarded-For": SeverityFail},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := debianHost()
			f.put(NginxSitePath, string(NginxTLS(Answers{Domain: "dhow.example.com", ListenAddr: DefaultListen})), 0o644)
			f.stdout["journalctl --unit dhow --output cat --no-pager --since -5min"] = `{"msg":"http request","path":"/api/v1/doctor-probe-nonce","client_ip":"` + tt.clientIP + `"}` + "\n"
			f.httpFn = func(p Probe) (ProbeResult, error) {
				switch {
				case strings.HasSuffix(p.URL, "/sessions/current") && p.Headers["Origin"] == "https://dhow.example.com":
					return ProbeResult{Status: tt.own, Body: envelopeError(tt.ownCode)}, nil
				case strings.HasSuffix(p.URL, "/sessions/current"):
					return ProbeResult{Status: tt.foreign, Body: envelopeError(tt.forCode)}, nil
				default:
					return ProbeResult{Status: 200, Body: []byte(readyBody), CertDays: tt.certDays}, nil
				}
			}
			a := Answers{Mode: ModeNative, Domain: "dhow.example.com", ListenAddr: DefaultListen, Proxy: ProxyAnswers{Kind: ProxyNginx, Email: "a@b.co"}}
			findings := doctorFor(f, a).publicChecks(context.Background())
			for part, sev := range tt.want {
				found := false
				for _, fd := range findings {
					if strings.Contains(fd.Title, part) {
						found = true
						assert.Equal(t, sev, fd.Severity, "%s: %s", fd.Title, fd.Detail)
					}
				}
				assert.True(t, found, part)
			}
		})
	}
}

func TestPublicChecksForAnUnreachableManagedProxy(t *testing.T) {
	f := debianHost()
	f.put(CaddySitePath, "x", 0o644)
	a := Answers{Mode: ModeNative, Domain: "dhow.example.com", ListenAddr: DefaultListen, Proxy: ProxyAnswers{Kind: ProxyCaddy}}
	findings := doctorFor(f, a).publicChecks(context.Background())
	require.Len(t, findings, 1)
	assert.Equal(t, SeverityFail, findings[0].Severity)
	require.NotNil(t, findings[0].Fix)
	assert.Contains(t, findings[0].Fix.Commands, "systemctl reload-or-restart caddy")
}
