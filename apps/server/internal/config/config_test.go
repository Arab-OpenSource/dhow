package config

import (
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"strings"
	"testing"
)

const (
	goodKeyHex = "9f1c4a7e2b8d06f35a1e9c7b4d2f80a6c3e5b7091d4f6a8e2c0b5d7f9a1e3c6b"
	goodDB     = "postgres://dhow:pw@db.internal:5432/dhow?sslmode=verify-full"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func valid() map[string]string {
	return map[string]string{
		EnvDatabaseURL: goodDB,
		EnvSecretKey:   goodKeyHex,
		EnvPublicURL:   "https://dhow.example.com/",
	}
}

func TestLoadValid(t *testing.T) {
	cfg, err := Load(env(valid()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenAddr != DefaultListenAddr {
		t.Errorf("ListenAddr = %q, want default %q", cfg.ListenAddr, DefaultListenAddr)
	}
	if got := cfg.PublicURL.String(); got != "https://dhow.example.com" {
		t.Errorf("PublicURL = %q, want trailing slash trimmed", got)
	}
	if len(cfg.SecretKey) != 32 {
		t.Errorf("SecretKey length = %d, want 32", len(cfg.SecretKey))
	}
	if cfg.LogLevel != slog.LevelInfo || cfg.Dev {
		t.Errorf("defaults: level=%v dev=%v", cfg.LogLevel, cfg.Dev)
	}
}

func TestLoadAcceptsBase64Key(t *testing.T) {
	raw, err := hex.DecodeString(goodKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawURLEncoding} {
		m := valid()
		m[EnvSecretKey] = enc.EncodeToString(raw)
		cfg, err := Load(env(m))
		if err != nil {
			t.Fatalf("Load with base64 key: %v", err)
		}
		if string(cfg.SecretKey) != string(raw) {
			t.Fatal("decoded key differs")
		}
	}
}

func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name    string
		set     map[string]string
		wantErr string
	}{
		{"missing secret", map[string]string{EnvSecretKey: ""}, EnvSecretKey + ": is required"},
		{"placeholder secret", map[string]string{EnvSecretKey: "change_this_secret_in_production_please_123456"}, "placeholder"},
		{"short secret", map[string]string{EnvSecretKey: "9f1c4a7e2b8d06f35a1e9c7b4d2f80a6"}, "at least 32 bytes"},
		{"repeated secret", map[string]string{EnvSecretKey: strings.Repeat("ab", 32)}, "too little variety"},
		{"not encoded secret", map[string]string{EnvSecretKey: "this is not hex or base64!!"}, "hex or base64"},
		{"missing database", map[string]string{EnvDatabaseURL: ""}, EnvDatabaseURL + ": is required"},
		{"dsn instead of url", map[string]string{EnvDatabaseURL: "host=db user=dhow"}, "postgres:// URL"},
		{"plaintext database", map[string]string{EnvDatabaseURL: "postgres://dhow:pw@db/dhow?sslmode=disable"}, "sslmode"},
		{"default sslmode", map[string]string{EnvDatabaseURL: "postgres://dhow:pw@db/dhow"}, "sslmode"},
		{"missing public url", map[string]string{EnvPublicURL: ""}, EnvPublicURL + ": is required"},
		{"http public url", map[string]string{EnvPublicURL: "http://dhow.example.com"}, "must use https"},
		{"relative public url", map[string]string{EnvPublicURL: "dhow.example.com"}, "absolute URL"},
		{"public url with query", map[string]string{EnvPublicURL: "https://dhow.example.com/?x=1"}, "query"},
		{"bad listen addr", map[string]string{EnvListenAddr: "8080"}, "host:port"},
		{"bad proxy", map[string]string{EnvTrustedProxies: "10.0.0.0/8, nope"}, `invalid IP "nope"`},
		{"bad cidr", map[string]string{EnvTrustedProxies: "10.0.0.0/99"}, "invalid CIDR"},
		{"bad level", map[string]string{EnvLogLevel: "loud"}, EnvLogLevel},
		{"bad dev flag", map[string]string{EnvDev: "maybe"}, EnvDev},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := valid()
			for k, v := range tt.set {
				m[k] = v
			}
			_, err := Load(env(m))
			if err == nil {
				t.Fatal("Load succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestErrorsNeverEchoSecrets(t *testing.T) {
	m := map[string]string{
		EnvSecretKey:   "change_me_" + goodKeyHex,
		EnvDatabaseURL: "postgres://dhow:SuperSecretPw@db/dhow?sslmode=disable",
		EnvPublicURL:   "ftp://x",
	}
	_, err := Load(env(m))
	if err == nil {
		t.Fatal("want error")
	}
	for _, leak := range []string{goodKeyHex, "SuperSecretPw"} {
		if strings.Contains(err.Error(), leak) {
			t.Fatalf("error leaks %q: %v", leak, err)
		}
	}
}

func TestDevRelaxesTransportRules(t *testing.T) {
	m := valid()
	m[EnvDev] = "true"
	m[EnvPublicURL] = "http://localhost:8080"
	m[EnvDatabaseURL] = "postgres://dhow@localhost/dhow?sslmode=disable"
	cfg, err := Load(env(m))
	if err != nil {
		t.Fatalf("Load in dev: %v", err)
	}
	if !cfg.Dev {
		t.Fatal("Dev = false")
	}
	m[EnvSecretKey] = "dev"
	if _, err := Load(env(m)); err == nil {
		t.Fatal("weak secret accepted in dev mode")
	}
}

func TestTrustedProxiesParsing(t *testing.T) {
	m := valid()
	m[EnvTrustedProxies] = " 10.0.0.0/8 , 192.168.1.10,::1 "
	cfg, err := Load(env(m))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"10.0.0.0/8", "192.168.1.10/32", "::1/128"}
	if len(cfg.TrustedProxies) != len(want) {
		t.Fatalf("got %v", cfg.TrustedProxies)
	}
	for i, p := range cfg.TrustedProxies {
		if p.String() != want[i] {
			t.Errorf("proxy %d = %s, want %s", i, p, want[i])
		}
	}
}
