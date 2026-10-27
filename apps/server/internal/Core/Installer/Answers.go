package installer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
)

const (
	ModeNative  = "native"
	ModeCompose = "compose"

	DatabaseInstall  = "install"
	DatabaseExisting = "existing"

	ProxyCaddy    = "caddy"
	ProxyNginx    = "nginx"
	ProxyExisting = "existing"
	ProxyNone     = "none"

	LangEn = "en"
	LangAr = "ar"

	maxAnswersBytes = 64 << 10
	loopbackProxies = "127.0.0.1/32,::1/128"
)

var (
	ErrInvalidAnswers = errors.New("invalid answers")
	hostnameRe        = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)
	emailRe           = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

type DatabaseAnswers struct {
	Mode    string `yaml:"mode"`
	URLFile string `yaml:"url_file,omitempty"`
	url     string
	kept    bool
}

type ProxyAnswers struct {
	Kind           string `yaml:"kind"`
	Email          string `yaml:"email,omitempty"`
	TrustedProxies string `yaml:"trusted_proxies,omitempty"`
}

type Answers struct {
	Lang                  string          `yaml:"lang,omitempty"`
	Mode                  string          `yaml:"mode"`
	Domain                string          `yaml:"domain"`
	PublicURL             string          `yaml:"public_url,omitempty"`
	ListenAddr            string          `yaml:"listen_addr,omitempty"`
	Database              DatabaseAnswers `yaml:"database"`
	Proxy                 ProxyAnswers    `yaml:"proxy"`
	Firewall              bool            `yaml:"firewall"`
	InstallPackages       bool            `yaml:"install_packages"`
	BreachedPasswordCheck *bool           `yaml:"breached_password_check,omitempty"`
}

func DefaultAnswers() Answers {
	on := true
	return Answers{
		Mode:                  ModeNative,
		ListenAddr:            DefaultListen,
		Database:              DatabaseAnswers{Mode: DatabaseInstall},
		Proxy:                 ProxyAnswers{Kind: ProxyCaddy},
		BreachedPasswordCheck: &on,
	}
}

func ParseAnswers(r io.Reader) (Answers, error) { return parseAnswers(r, false) }

func ParseInstalledAnswers(r io.Reader, urlStored bool) (Answers, error) {
	return parseAnswers(r, urlStored)
}

func parseAnswers(r io.Reader, urlStored bool) (Answers, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxAnswersBytes+1))
	if err != nil {
		return Answers{}, fmt.Errorf("read answers: %w", err)
	}
	if len(data) > maxAnswersBytes {
		return Answers{}, fmt.Errorf("%w: the file is larger than %d bytes", ErrInvalidAnswers, maxAnswersBytes)
	}
	a := DefaultAnswers()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&a); err != nil {
		if errors.Is(err, io.EOF) {
			return Answers{}, fmt.Errorf("%w: the file is empty", ErrInvalidAnswers)
		}
		return Answers{}, fmt.Errorf("%w: %w", ErrInvalidAnswers, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return Answers{}, fmt.Errorf("%w: only one YAML document is allowed", ErrInvalidAnswers)
	}
	a.normalize()
	a.Database.kept = urlStored
	if err := a.Validate(); err != nil {
		return Answers{}, err
	}
	return a, nil
}

func (a *Answers) normalize() {
	a.Lang = strings.ToLower(strings.TrimSpace(a.Lang))
	a.Mode = strings.ToLower(strings.TrimSpace(a.Mode))
	a.Domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(a.Domain), "."))
	a.PublicURL = strings.TrimSuffix(strings.TrimSpace(a.PublicURL), "/")
	a.ListenAddr = strings.TrimSpace(a.ListenAddr)
	if a.ListenAddr == "" {
		a.ListenAddr = DefaultListen
	}
	a.Database.Mode = strings.ToLower(strings.TrimSpace(a.Database.Mode))
	a.Proxy.Kind = strings.ToLower(strings.TrimSpace(a.Proxy.Kind))
	a.Proxy.Email = strings.TrimSpace(a.Proxy.Email)
	a.Proxy.TrustedProxies = strings.TrimSpace(a.Proxy.TrustedProxies)
	if a.BreachedPasswordCheck == nil {
		on := true
		a.BreachedPasswordCheck = &on
	}
}

func (a Answers) Validate() error {
	var errs []error
	add := func(field, msg string) { errs = append(errs, fmt.Errorf("%s: %s", field, msg)) }
	if a.Lang != "" && a.Lang != LangEn && a.Lang != LangAr {
		add("lang", "must be en or ar")
	}
	if a.Mode != ModeNative && a.Mode != ModeCompose {
		add("mode", "must be native or compose")
	}
	if !ValidDomain(a.Domain) {
		add("domain", "must be a host name such as dhow.example.com, without a scheme, port or path")
	}
	if err := config.ValidatePublicURL(a.EffectivePublicURL()); err != nil {
		add("public_url", err.Error())
	}
	if err := validateListen(a.ListenAddr); err != nil {
		add("listen_addr", err.Error())
	}
	errs = append(errs, a.validateDatabase()...)
	errs = append(errs, a.validateProxy()...)
	if len(errs) > 0 {
		return fmt.Errorf("%w:\n%w", ErrInvalidAnswers, errors.Join(errs...))
	}
	return nil
}

func validateListen(v string) error {
	if err := config.ValidateListenAddr(v); err != nil {
		return err
	}
	if _, port, err := net.SplitHostPort(v); err != nil || port == "" {
		return errors.New("must be host:port")
	}
	return nil
}

func (a Answers) validateDatabase() []error {
	var errs []error
	switch a.Database.Mode {
	case DatabaseInstall:
		if a.Database.URLFile != "" {
			errs = append(errs, errors.New("database.url_file: only applies to database.mode existing"))
		}
	case DatabaseExisting:
		if a.Mode == ModeNative && a.Database.URLFile == "" && a.Database.url == "" && !a.Database.kept {
			errs = append(errs, errors.New("database.url_file: is required with database.mode existing; it names a file that holds the postgres:// URL"))
		}
	default:
		errs = append(errs, errors.New("database.mode: must be install or existing"))
	}
	if a.Mode == ModeCompose && a.Database.Mode == DatabaseExisting {
		errs = append(errs, errors.New("database.mode: the compose install always runs its own PostgreSQL; use install"))
	}
	return errs
}

func (a Answers) validateProxy() []error {
	var errs []error
	switch a.Proxy.Kind {
	case ProxyCaddy, ProxyExisting, ProxyNone:
	case ProxyNginx:
		if a.Proxy.Email == "" {
			errs = append(errs, errors.New("proxy.email: is required with nginx, for the Let's Encrypt account"))
		}
	default:
		errs = append(errs, errors.New("proxy.kind: must be caddy, nginx, existing or none"))
	}
	if a.Proxy.Email != "" && !emailRe.MatchString(a.Proxy.Email) {
		errs = append(errs, errors.New("proxy.email: is not an email address"))
	}
	if err := config.ValidateTrustedProxies(a.Proxy.TrustedProxies); err != nil {
		errs = append(errs, fmt.Errorf("proxy.trusted_proxies: %w", err))
	}
	if a.Firewall && !a.ManagedProxy() {
		errs = append(errs, errors.New("firewall: opening ports 80 and 443 only applies with proxy.kind caddy or nginx"))
	}
	return errs
}

func ValidDomain(d string) bool {
	return len(d) <= 253 && hostnameRe.MatchString(d)
}

func (a Answers) EffectivePublicURL() string {
	if a.PublicURL != "" {
		return a.PublicURL
	}
	return "https://" + a.Domain
}

func (a Answers) ManagedProxy() bool {
	return a.Proxy.Kind == ProxyCaddy || a.Proxy.Kind == ProxyNginx
}

func (a Answers) TrustedProxies() string {
	if a.ManagedProxy() {
		return loopbackProxies
	}
	return a.Proxy.TrustedProxies
}

func (a Answers) BreachedCheck() bool {
	return a.BreachedPasswordCheck == nil || *a.BreachedPasswordCheck
}

func (a Answers) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("# Written by dhow-server install. It holds no secrets.\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(a); err != nil {
		return nil, fmt.Errorf("encode answers: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode answers: %w", err)
	}
	return buf.Bytes(), nil
}

func (a *Answers) SetDatabaseURL(url string) { a.Database.url = url }

func (a Answers) DatabaseURL() string { return a.Database.url }

func (a *Answers) KeepDatabaseURL() { a.Database.kept = true }
