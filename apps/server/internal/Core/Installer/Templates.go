package installer

import (
	"bufio"
	"bytes"
	"net"
	"slices"
	"strconv"
	"strings"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

const (
	envLogLevel = "DHOW_LOG_LEVEL"
	envImage    = "DHOW_IMAGE"
	envPublish  = "DHOW_PUBLISH"
	envPGPass   = "POSTGRES_PASSWORD" //nolint:gosec // The name of a Compose variable, not a credential.
)

type EnvEntry struct {
	Key   string
	Value string
}

func ParseEnv(data []byte) []EnvEntry {
	var out []EnvEntry
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out = append(out, EnvEntry{Key: strings.TrimSpace(k), Value: unquote(strings.TrimSpace(v))})
	}
	return out
}

func EnvValue(entries []EnvEntry, key string) string {
	for _, e := range entries {
		if e.Key == key {
			return e.Value
		}
	}
	return ""
}

func RenderEnv(managed, existing []EnvEntry) []byte {
	var b bytes.Buffer
	seen := map[string]bool{}
	for _, e := range managed {
		seen[e.Key] = true
		b.WriteString(e.Key + "=" + e.Value + "\n")
	}
	for _, e := range existing {
		if seen[e.Key] {
			continue
		}
		seen[e.Key] = true
		b.WriteString(e.Key + "=" + e.Value + "\n")
	}
	return b.Bytes()
}

func nativeEnv(a Answers, existing []EnvEntry) []byte {
	managed := []EnvEntry{
		{config.EnvPublicURL, a.EffectivePublicURL()},
		{config.EnvListenAddr, a.ListenAddr},
		{config.EnvTrustedProxies, a.TrustedProxies()},
		{config.EnvBreachedPasswordCheck, strconv.FormatBool(a.BreachedCheck())},
	}
	if EnvValue(existing, envLogLevel) == "" {
		managed = append(managed, EnvEntry{envLogLevel, "info"})
	}
	return RenderEnv(managed, existing)
}

func composeEnv(a Answers, existing []EnvEntry, secretKey, pgPassword, image, trusted string) []byte {
	if v := EnvValue(existing, envImage); v != "" {
		image = v
	}
	managed := []EnvEntry{
		{envPGPass, pgPassword},
		{config.EnvSecretKey, secretKey},
		{config.EnvPublicURL, a.EffectivePublicURL()},
		{envImage, image},
		{envPublish, a.ListenAddr},
		{config.EnvTrustedProxies, trusted},
		{config.EnvBreachedPasswordCheck, strconv.FormatBool(a.BreachedCheck())},
	}
	if EnvValue(existing, envLogLevel) == "" {
		managed = append(managed, EnvEntry{envLogLevel, "info"})
	}
	return RenderEnv(managed, existing)
}

const (
	fixtureDomain   = "dhow.example.com"
	fixtureUpstream = "127.0.0.1:8080"
	nginxHTTP2Line  = "    http2 on;\n"
)

func upstream(a Answers) string {
	host, port, err := net.SplitHostPort(a.ListenAddr)
	if err != nil || host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func fromFixture(fixture []byte, a Answers) string {
	return strings.NewReplacer(fixtureDomain, a.Domain, fixtureUpstream, upstream(a)).Replace(string(fixture))
}

func ProxyNginxConfig(a Answers) string { return fromFixture(NginxFixture, a) }

func ProxyCaddyConfig(a Answers) string { return fromFixture(CaddyFixture, a) }

func CaddySite(a Answers) []byte { return []byte(ProxyCaddyConfig(a)) }

func caddyImportLine() string { return "import " + CaddySitePath }

func WithCaddyImport(current []byte) []byte {
	line := caddyImportLine()
	if slices.Contains(strings.Split(string(current), "\n"), line) {
		return current
	}
	return append([]byte(line+"\n\n"), current...)
}

func WithoutCaddyImport(current []byte) []byte {
	lines := strings.Split(string(current), "\n")
	var out []string
	skipBlank := false
	for _, l := range lines {
		if l == caddyImportLine() {
			skipBlank = true
			continue
		}
		if skipBlank && l == "" {
			skipBlank = false
			continue
		}
		skipBlank = false
		out = append(out, l)
	}
	return []byte(strings.Join(out, "\n"))
}

func fixtureRedirect(a Answers) string {
	return "server {\n    listen 80;\n    server_name " + a.Domain + ";\n\n    return 301 https://$host$request_uri;\n}\n"
}

func nginxChallengeServer(a Answers, redirect bool) string {
	last := "        return 404;\n"
	if redirect {
		last = "        return 301 https://$host$request_uri;\n"
	}
	return "server {\n    listen 80;\n    listen [::]:80;\n    server_name " + a.Domain + ";\n\n" +
		"    location /.well-known/acme-challenge/ {\n        root " + AcmeWebroot + ";\n    }\n\n" +
		"    location / {\n" + last + "    }\n}\n"
}

func NginxHTTPOnly(a Answers) []byte { return []byte(nginxChallengeServer(a, false)) }

func NginxTLS(a Answers) []byte {
	conf := ProxyNginxConfig(a)
	conf = strings.Replace(conf, fixtureRedirect(a), nginxChallengeServer(a, true), 1)
	conf = strings.Replace(conf, nginxHTTP2Line, "", 1)
	return []byte(conf)
}

func CertbotReloadHook() []byte {
	return []byte("#!/bin/sh\nexec systemctl reload nginx\n")
}

func ExistingProxySnippets(a Answers) string {
	return "nginx (" + NginxSitePath + "):\n\n" + ProxyNginxConfig(a) +
		"\nCaddy (Caddyfile):\n\n" + ProxyCaddyConfig(a)
}
