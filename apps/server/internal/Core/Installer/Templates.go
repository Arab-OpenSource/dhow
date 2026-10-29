package installer

import (
	"net"
	"slices"
	"strconv"
	"strings"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	engine "github.com/Khaymah/dhow/internal/installer"
)

const (
	envLogLevel = "DHOW_LOG_LEVEL"
	envImage    = "DHOW_IMAGE"
	envPublish  = "DHOW_PUBLISH"
	envPGPass   = "POSTGRES_PASSWORD" //nolint:gosec // The name of a Compose variable, not a credential.
)

type EnvEntry = engine.EnvEntry

func ParseEnv(data []byte) []EnvEntry { return engine.ParseEnv(data) }

func EnvValue(entries []EnvEntry, key string) string { return engine.EnvValue(entries, key) }

func RenderEnv(managed, existing []EnvEntry) []byte { return engine.RenderEnv(managed, existing) }

func breachedEntry(a Answers, managed, existing []EnvEntry) ([]EnvEntry, []EnvEntry) {
	kept := make([]EnvEntry, 0, len(existing))
	for _, e := range existing {
		if e.Key != config.EnvBreachedPasswordCheck {
			kept = append(kept, e)
		}
	}
	if a.BreachedCheck() {
		return managed, kept
	}
	return append(managed, EnvEntry{Key: config.EnvBreachedPasswordCheck, Value: strconv.FormatBool(false)}), kept
}

func nativeEnv(a Answers, existing []EnvEntry) []byte {
	managed := []EnvEntry{
		{Key: config.EnvPublicURL, Value: a.EffectivePublicURL()},
		{Key: config.EnvListenAddr, Value: a.ListenAddr},
		{Key: config.EnvTrustedProxies, Value: a.TrustedProxies()},
	}
	managed, existing = breachedEntry(a, managed, existing)
	if EnvValue(existing, envLogLevel) == "" {
		managed = append(managed, EnvEntry{Key: envLogLevel, Value: "info"})
	}
	return RenderEnv(managed, existing)
}

func composeEnv(a Answers, existing []EnvEntry, secretKey, pgPassword, image, trusted string) []byte {
	if v := EnvValue(existing, envImage); v != "" {
		image = v
	}
	managed := []EnvEntry{
		{Key: envPGPass, Value: pgPassword},
		{Key: config.EnvSecretKey, Value: secretKey},
		{Key: config.EnvPublicURL, Value: a.EffectivePublicURL()},
		{Key: envImage, Value: image},
		{Key: envPublish, Value: a.ListenAddr},
		{Key: config.EnvTrustedProxies, Value: trusted},
	}
	managed, existing = breachedEntry(a, managed, existing)
	if EnvValue(existing, envLogLevel) == "" {
		managed = append(managed, EnvEntry{Key: envLogLevel, Value: "info"})
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
