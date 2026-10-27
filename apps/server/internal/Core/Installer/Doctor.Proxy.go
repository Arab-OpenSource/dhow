package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

const (
	certWarnDays  = 21
	certFailDays  = 7
	foreignOrigin = "https://doctor.invalid"
	logWait       = 2 * time.Second
)

func (d Doctor) public(path string) string { return d.A.EffectivePublicURL() + path }

func (d Doctor) publicChecks(ctx context.Context) []Finding {
	if d.A.Proxy.Kind == ProxyNone {
		return []Finding{{Severity: SeverityInfo, Title: d.M.T(KeyCheckProxy), Detail: d.M.T(KeyProxyNoneSkipped)}}
	}
	var out []Finding
	if d.A.Proxy.Kind == ProxyNginx {
		out = append(out, d.nginxConfig())
	}
	res, err := d.Sys.HTTP(ctx, Probe{URL: d.public("/api/v1/health/ready")})
	if err != nil {
		sev := SeverityWarn
		if d.A.ManagedProxy() {
			sev = SeverityFail
		}
		f := Finding{Severity: sev, Title: d.M.T(KeyCheckPublic, d.A.EffectivePublicURL()), Detail: err.Error(), Advice: d.M.T(KeyAdvicePublic)}
		if fix := d.proxyReload(); fix != nil {
			f.Fix = fix
		}
		return append(out, f)
	}
	if res.Status != http.StatusOK {
		out = append(out, Finding{Severity: SeverityFail, Title: d.M.T(KeyCheckPublic, d.A.EffectivePublicURL()), Detail: d.M.T(KeyPublicStatus, res.Status)})
	} else {
		out = append(out, ok(d.M.T(KeyCheckPublic, d.A.EffectivePublicURL())))
	}
	out = append(out, d.certificate(res.CertDays), d.origin(ctx), d.forwarded(ctx))
	return out
}

func (d Doctor) proxyReload() *Step {
	var s Step
	switch d.A.Proxy.Kind {
	case ProxyCaddy:
		s = CommandStep(d.Sys, d.M.T(KeyFixProxyReload, "caddy"), nil, []Command{Cmd("systemctl", "reload-or-restart", "caddy")}, nil)
	case ProxyNginx:
		s = CommandStep(d.Sys, d.M.T(KeyFixProxyReload, "nginx"), nil, []Command{Cmd("nginx", "-t"), Cmd("systemctl", "reload-or-restart", "nginx")}, nil)
	default:
		return nil
	}
	return &s
}

func (d Doctor) certificate(days int) Finding {
	title := d.M.T(KeyCheckCertificate)
	if days < 0 {
		return Finding{Severity: SeverityInfo, Title: title, Detail: d.M.T(KeyUnknown)}
	}
	if days >= certWarnDays {
		return Finding{Severity: SeverityOK, Title: title, Detail: d.M.T(KeyCertDays, days)}
	}
	f := Finding{Severity: SeverityWarn, Title: title, Detail: d.M.T(KeyCertDays, days)}
	if days < certFailDays {
		f.Severity = SeverityFail
	}
	switch d.A.Proxy.Kind {
	case ProxyNginx:
		s := CommandStep(d.Sys, d.M.T(KeyFixRenew), nil, []Command{Cmd("certbot", "renew"), Cmd("systemctl", "reload", "nginx")}, nil)
		f.Fix = &s
	case ProxyCaddy:
		f.Fix = d.proxyReload()
	default:
		f.Advice = d.M.T(KeyAdviceRenew)
	}
	return f
}

type errorEnvelope struct {
	Error struct {
		Code string
	}
}

func csrfRejected(res ProbeResult) bool {
	var e errorEnvelope
	return res.Status == http.StatusForbidden && json.Unmarshal(res.Body, &e) == nil && e.Error.Code == "CSRF_FAILED"
}

func (d Doctor) origin(ctx context.Context) Finding {
	title := d.M.T(KeyCheckOrigin)
	target := d.public("/api/v1/sessions/current")
	own, err := d.Sys.HTTP(ctx, Probe{Method: http.MethodDelete, URL: target, Headers: map[string]string{"Origin": d.A.EffectivePublicURL()}})
	if err != nil {
		return Finding{Severity: SeverityWarn, Title: title, Detail: err.Error()}
	}
	foreign, err := d.Sys.HTTP(ctx, Probe{Method: http.MethodDelete, URL: target, Headers: map[string]string{"Origin": foreignOrigin}})
	if err != nil {
		return Finding{Severity: SeverityWarn, Title: title, Detail: err.Error()}
	}
	switch {
	case csrfRejected(own):
		return Finding{Severity: SeverityFail, Title: title, Detail: d.M.T(KeyOriginDropped), Advice: d.M.T(KeyAdviceOrigin)}
	case !csrfRejected(foreign):
		return Finding{Severity: SeverityFail, Title: title, Detail: d.M.T(KeyOriginNotChecked, foreign.Status), Advice: d.M.T(KeyAdviceOrigin)}
	}
	return ok(title)
}

func (d Doctor) forwarded(ctx context.Context) Finding {
	title := d.M.T(KeyCheckForwarded)
	nonce, err := d.Probe()
	if err != nil {
		return Finding{Severity: SeverityInfo, Title: title, Detail: err.Error()}
	}
	path := "/api/v1/doctor-probe-" + nonce
	if _, err := d.Sys.HTTP(ctx, Probe{URL: d.public(path)}); err != nil {
		return Finding{Severity: SeverityWarn, Title: title, Detail: err.Error()}
	}
	d.b.sleep(logWait)
	ip, found := d.clientIPFor(ctx, path)
	if !found {
		return Finding{Severity: SeverityInfo, Title: title, Detail: d.M.T(KeyForwardedUnknown)}
	}
	addr, err := netip.ParseAddr(ip)
	if err == nil && (addr.IsLoopback() || d.isGateway(ctx, addr)) {
		return Finding{Severity: SeverityFail, Title: title, Detail: d.M.T(KeyForwardedLoopback, ip), Advice: d.M.T(KeyAdviceForwarded)}
	}
	return Finding{Severity: SeverityOK, Title: title, Detail: d.M.T(KeyForwardedSeen, ip)}
}

func (d Doctor) isGateway(ctx context.Context, addr netip.Addr) bool {
	if d.A.Mode != ModeCompose {
		return false
	}
	gw, err := d.b.gateway(ctx)
	return err == nil && gw == netip.PrefixFrom(addr, addr.BitLen()).String()
}

func (d Doctor) clientIPFor(ctx context.Context, path string) (string, bool) {
	c := Cmd("journalctl", "--unit", ServiceName, "--output", "cat", "--no-pager", "--since", "-5min")
	if d.A.Mode == ModeCompose {
		c = composeCmd("logs", "--no-color", "--no-log-prefix", "--since", "5m", "dhow")
	}
	out, err := d.Sys.Run(ctx, c)
	if err != nil {
		return "", false
	}
	for _, line := range bytes.Split(out.Stdout, []byte("\n")) {
		i := bytes.IndexByte(line, '{')
		if i < 0 || !bytes.Contains(line, []byte(path)) {
			continue
		}
		var rec map[string]any
		if json.Unmarshal(line[i:], &rec) != nil {
			continue
		}
		if p, _ := rec["path"].(string); p != path {
			continue
		}
		if ip, ok := rec["client_ip"].(string); ok {
			return ip, true
		}
	}
	return "", false
}

func (d Doctor) nginxConfig() Finding {
	title := d.M.T(KeyCheckNginxConfig)
	data, err := d.Sys.ReadFile(NginxSitePath)
	if err != nil {
		return Finding{Severity: SeverityFail, Title: title, Detail: err.Error(), Advice: d.M.T(KeyAdviceReinstall)}
	}
	var missing []string
	for _, want := range []string{"proxy_set_header Host ", "proxy_set_header X-Forwarded-For ", "proxy_set_header X-Forwarded-Proto "} {
		if !strings.Contains(string(data), want) {
			missing = append(missing, want)
		}
	}
	if len(missing) == 0 {
		return ok(title)
	}
	f := Finding{Severity: SeverityFail, Title: title, Detail: strings.Join(missing, " ")}
	if exists(d.Sys, d.b.certPath()) {
		fix := d.b.ConfigStep(d.M.T(KeyStepNginxTLS, d.A.Domain), []fileSpec{{Path: NginxSitePath, Data: NginxTLS(d.A), Mode: modeRootPublic}},
			[]Command{Cmd("nginx", "-t"), Cmd("systemctl", "reload", "nginx")}, nil)
		f.Fix = &fix
	}
	return f
}
