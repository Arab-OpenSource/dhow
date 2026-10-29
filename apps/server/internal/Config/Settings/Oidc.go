package settingspec

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	OIDCKindOIDC            = "oidc"
	OIDCKindGitHub          = "github"
	OIDCDefaultGitHubIssuer = "https://github.com"
	OIDCGoogleIssuer        = "https://accounts.google.com"

	OIDCMaxProviders      = 10
	OIDCMaxNameRunes      = 40
	OIDCMaxClientIDBytes  = 512
	OIDCMaxSecretBytes    = 1024
	OIDCMaxIssuerBytes    = 2048
	OIDCMaxAllowedDomains = 50
	OIDCMaxDomainLength   = 253
)

const (
	OIDCFieldID             = "Id"
	OIDCFieldType           = "Type"
	OIDCFieldName           = "Name"
	OIDCFieldIssuer         = "Issuer"
	OIDCFieldClientID       = "ClientId"
	OIDCFieldClientSecret   = "ClientSecret"
	OIDCFieldAllowedDomains = "AllowedDomains"
)

const (
	RuleOIDCID          = "oidcId"
	RuleOIDCType        = "oidcType"
	RuleOIDCName        = "oidcName"
	RuleOIDCIssuer      = "oidcIssuer"
	RuleOIDCIssuerHTTPS = "oidcIssuerHttps"
	RuleOIDCIssuerParts = "oidcIssuerParts"
	RuleOIDCIssuerHost  = "oidcIssuerBlocked"
	RuleOIDCIssuerTaken = "oidcIssuerTaken"
	RuleOIDCClientID    = "oidcClientId"
	RuleOIDCSecret      = "oidcSecret"
	RuleOIDCSecretAgain = "oidcSecretAgain"
	RuleOIDCPlaceholder = "oidcSecretPlaceholder"
	RuleOIDCDomain      = "oidcDomain"
	RuleOIDCDomainCount = "oidcDomainCount"
)

var (
	oidcProviderID = regexp.MustCompile(`^[a-z][a-z0-9]{0,31}$`)
	oidcDomain     = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
)

var placeholderMarkers = []string{
	"change", "replace", "example", "placeholder", "secret", "password", "default", "dummy",
}

type OIDCProvider struct {
	ID             string
	Kind           string
	Name           string
	Issuer         string
	ClientID       string
	ClientSecret   string
	AllowedDomains []string
}

func (p OIDCProvider) Equal(o OIDCProvider) bool {
	return p.ID == o.ID && p.Kind == o.Kind && p.Name == o.Name && p.Issuer == o.Issuer && p.ClientID == o.ClientID &&
		p.ClientSecret == o.ClientSecret && slices.Equal(p.AllowedDomains, o.AllowedDomains)
}

type OIDCError struct {
	Field   string
	Rule    string
	Message string
}

func (e *OIDCError) Error() string { return e.Message }

func oidcError(field, rule, message string) *OIDCError {
	return &OIDCError{Field: field, Rule: rule, Message: message}
}

func LooksLikePlaceholder(v string) bool {
	lower := strings.ToLower(v)
	for _, m := range placeholderMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func ValidOIDCProviderID(id string) bool { return oidcProviderID.MatchString(id) }

func ParseOIDCKind(v string) (string, *OIDCError) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", OIDCKindOIDC:
		return OIDCKindOIDC, nil
	case OIDCKindGitHub:
		return OIDCKindGitHub, nil
	default:
		return "", oidcError(OIDCFieldType, RuleOIDCType, "must be oidc or github")
	}
}

func ParseOIDCName(v, id, kind string) (string, *OIDCError) {
	v = strings.TrimSpace(v)
	if v == "" {
		if kind == OIDCKindGitHub {
			return "GitHub", nil
		}
		return id, nil
	}
	if utf8.RuneCountInString(v) > OIDCMaxNameRunes || strings.ContainsFunc(v, unicode.IsControl) {
		return "", oidcError(OIDCFieldName, RuleOIDCName, fmt.Sprintf("must be at most %d characters without control characters", OIDCMaxNameRunes))
	}
	return v, nil
}

func ParseOIDCIssuer(v, kind string, dev bool) (string, *OIDCError) {
	v = strings.TrimSpace(v)
	if v == "" {
		if kind == OIDCKindGitHub {
			return OIDCDefaultGitHubIssuer, nil
		}
		return "", oidcError(OIDCFieldIssuer, RuleOIDCIssuer, "is required, for example "+OIDCGoogleIssuer)
	}
	u, err := url.Parse(v)
	if err != nil || !u.IsAbs() || u.Host == "" || len(v) > OIDCMaxIssuerBytes {
		return "", oidcError(OIDCFieldIssuer, RuleOIDCIssuer, "must be an absolute URL")
	}
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && dev:
	default:
		return "", oidcError(OIDCFieldIssuer, RuleOIDCIssuerHTTPS, "must use https (http is allowed only with DHOW_DEV=true)")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return "", oidcError(OIDCFieldIssuer, RuleOIDCIssuerParts, "must not contain credentials, a query or a fragment")
	}
	if BlockedHost(u.Hostname()) {
		return "", oidcError(OIDCFieldIssuer, RuleOIDCIssuerHost, "must not point at a link-local or cloud metadata address")
	}
	if kind == OIDCKindGitHub {
		return strings.TrimSuffix(v, "/"), nil
	}
	return v, nil
}

func ParseOIDCClientID(v string) (string, *OIDCError) {
	v = strings.TrimSpace(v)
	if v == "" || len(v) > OIDCMaxClientIDBytes || strings.ContainsFunc(v, unicode.IsSpace) {
		return "", oidcError(OIDCFieldClientID, RuleOIDCClientID, fmt.Sprintf("is required, without spaces, at most %d bytes", OIDCMaxClientIDBytes))
	}
	return v, nil
}

func ParseOIDCSecret(v string) (string, *OIDCError) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", oidcError(OIDCFieldClientSecret, RuleOIDCSecret, "is required (or set the _FILE variant or the systemd credential)")
	}
	if len(v) > OIDCMaxSecretBytes || strings.ContainsFunc(v, unicode.IsSpace) {
		return "", oidcError(OIDCFieldClientSecret, RuleOIDCSecret, fmt.Sprintf("must be at most %d bytes without spaces", OIDCMaxSecretBytes))
	}
	if LooksLikePlaceholder(v) {
		return "", oidcError(OIDCFieldClientSecret, RuleOIDCPlaceholder, "looks like a placeholder; copy the secret from the identity provider")
	}
	return v, nil
}

func ParseAllowedDomains(list []string) ([]string, *OIDCError) {
	out := []string{}
	for _, part := range list {
		d := strings.ToLower(strings.TrimSpace(part))
		if d == "" {
			continue
		}
		if len(d) > OIDCMaxDomainLength || !oidcDomain.MatchString(d) {
			return nil, oidcError(OIDCFieldAllowedDomains, RuleOIDCDomain, fmt.Sprintf("%q is not a domain name", d))
		}
		if !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	if len(out) > OIDCMaxAllowedDomains {
		return nil, oidcError(OIDCFieldAllowedDomains, RuleOIDCDomainCount, fmt.Sprintf("lists more than %d domains", OIDCMaxAllowedDomains))
	}
	return out, nil
}

func NormalizedIssuer(issuer string) string {
	u, err := url.Parse(issuer)
	if err != nil {
		return issuer
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + strings.TrimSuffix(u.EscapedPath(), "/")
}

func SharedIssuer(providers []OIDCProvider) (first, second OIDCProvider, found bool) {
	owner := map[string]OIDCProvider{}
	for _, p := range providers {
		key := NormalizedIssuer(p.Issuer)
		if prev, taken := owner[key]; taken {
			return prev, p, true
		}
		owner[key] = p
	}
	return OIDCProvider{}, OIDCProvider{}, false
}

func ValidateOIDCProvider(p OIDCProvider, dev, needSecret bool) (OIDCProvider, []OIDCError) {
	var errs []OIDCError
	add := func(e *OIDCError) {
		if e != nil {
			errs = append(errs, *e)
		}
	}
	out := OIDCProvider{ID: p.ID}
	if !ValidOIDCProviderID(p.ID) {
		add(oidcError(OIDCFieldID, RuleOIDCID, "is not a provider id (lowercase letters and digits, starting with a letter, at most 32)"))
	}
	var e *OIDCError
	out.Kind, e = ParseOIDCKind(p.Kind)
	add(e)
	out.Name, e = ParseOIDCName(p.Name, p.ID, out.Kind)
	add(e)
	out.Issuer, e = ParseOIDCIssuer(p.Issuer, out.Kind, dev)
	add(e)
	out.ClientID, e = ParseOIDCClientID(p.ClientID)
	add(e)
	if p.ClientSecret != "" || needSecret {
		out.ClientSecret, e = ParseOIDCSecret(p.ClientSecret)
		add(e)
	}
	out.AllowedDomains, e = ParseAllowedDomains(p.AllowedDomains)
	add(e)
	if len(errs) > 0 {
		return OIDCProvider{}, errs
	}
	return out, nil
}
