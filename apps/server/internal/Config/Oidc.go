package config

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	EnvOIDCProviders = "DHOW_OIDC_PROVIDERS"

	OIDCKindOIDC   = "oidc"
	OIDCKindGitHub = "github"

	OIDCDefaultGitHubIssuer = "https://github.com"
	OIDCCredentialPrefix    = "dhow-oidc-" //nolint:gosec // The prefix of a systemd credential name, not a credential.
	OIDCCredentialSuffix    = "-client-secret"

	oidcMaxProviders      = 10
	oidcMaxNameRunes      = 40
	oidcMaxClientIDBytes  = 512
	oidcMaxSecretBytes    = 1024
	oidcMaxAllowedDomains = 50
	oidcMaxDomainLength   = 253
)

const (
	oidcSuffixType           = "_TYPE"
	oidcSuffixName           = "_NAME"
	oidcSuffixIssuer         = "_ISSUER"
	oidcSuffixClientID       = "_CLIENT_ID"
	oidcSuffixClientSecret   = "_CLIENT_SECRET"
	oidcSuffixAllowedDomains = "_ALLOWED_DOMAINS"
)

var (
	oidcProviderID = regexp.MustCompile(`^[a-z][a-z0-9]{0,31}$`)
	oidcDomain     = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
)

type OIDCProvider struct {
	ID             string
	Kind           string
	Name           string
	Issuer         string
	ClientID       string
	ClientSecret   string
	AllowedDomains []string
}

func OIDCEnvName(id, suffix string) string {
	return strings.TrimSuffix(EnvOIDCProviders, "PROVIDERS") + strings.ToUpper(id) + suffix
}

func loadOIDCProviders(getenv func(string) string, dev bool) ([]OIDCProvider, error) {
	list := strings.TrimSpace(getenv(EnvOIDCProviders))
	if list == "" {
		return nil, nil
	}
	ids := strings.Split(list, ",")
	if len(ids) > oidcMaxProviders {
		return nil, fmt.Errorf("%s: lists more than %d providers", EnvOIDCProviders, oidcMaxProviders)
	}
	var errs []error
	out := make([]OIDCProvider, 0, len(ids))
	seen := map[string]bool{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if !oidcProviderID.MatchString(id) {
			errs = append(errs, fmt.Errorf("%s: %q is not a provider id (lowercase letters and digits, starting with a letter, at most 32)", EnvOIDCProviders, id))
			continue
		}
		if seen[id] {
			errs = append(errs, fmt.Errorf("%s: %q is listed twice", EnvOIDCProviders, id))
			continue
		}
		seen[id] = true
		p, err := loadOIDCProvider(getenv, id, dev)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, p)
	}
	errs = append(errs, sharedIssuers(out)...)
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}

func sharedIssuers(providers []OIDCProvider) []error {
	var errs []error
	owner := map[string]string{}
	for _, p := range providers {
		key := normalizedIssuer(p.Issuer)
		if first, taken := owner[key]; taken {
			errs = append(errs, fmt.Errorf("%s and %s: share the issuer %s, and identities are keyed by issuer, so each provider needs its own",
				OIDCEnvName(first, oidcSuffixIssuer), OIDCEnvName(p.ID, oidcSuffixIssuer), p.Issuer))
			continue
		}
		owner[key] = p.ID
	}
	return errs
}

func normalizedIssuer(issuer string) string {
	u, err := url.Parse(issuer)
	if err != nil {
		return issuer
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + strings.TrimSuffix(u.EscapedPath(), "/")
}

func loadOIDCProvider(getenv func(string) string, id string, dev bool) (OIDCProvider, error) {
	p := OIDCProvider{ID: id, ClientID: strings.TrimSpace(getenv(OIDCEnvName(id, oidcSuffixClientID)))}
	var errs []error
	var err error
	if p.Kind, err = parseOIDCKind(getenv(OIDCEnvName(id, oidcSuffixType))); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", OIDCEnvName(id, oidcSuffixType), err))
	}
	if p.Name, err = parseOIDCName(getenv(OIDCEnvName(id, oidcSuffixName)), id, p.Kind); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", OIDCEnvName(id, oidcSuffixName), err))
	}
	if p.Issuer, err = parseOIDCIssuer(getenv(OIDCEnvName(id, oidcSuffixIssuer)), p.Kind, dev); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", OIDCEnvName(id, oidcSuffixIssuer), err))
	}
	if p.ClientID == "" || len(p.ClientID) > oidcMaxClientIDBytes || strings.ContainsFunc(p.ClientID, unicode.IsSpace) {
		errs = append(errs, fmt.Errorf("%s: is required, without spaces, at most %d bytes", OIDCEnvName(id, oidcSuffixClientID), oidcMaxClientIDBytes))
	}
	if p.ClientSecret, err = loadOIDCSecret(getenv, id); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", OIDCEnvName(id, oidcSuffixClientSecret), err))
	}
	if p.AllowedDomains, err = parseAllowedDomains(getenv(OIDCEnvName(id, oidcSuffixAllowedDomains))); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", OIDCEnvName(id, oidcSuffixAllowedDomains), err))
	}
	if len(errs) > 0 {
		return OIDCProvider{}, errors.Join(errs...)
	}
	return p, nil
}

func parseOIDCKind(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", OIDCKindOIDC:
		return OIDCKindOIDC, nil
	case OIDCKindGitHub:
		return OIDCKindGitHub, nil
	default:
		return "", errors.New("must be oidc or github")
	}
}

func parseOIDCName(v, id, kind string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		if kind == OIDCKindGitHub {
			return "GitHub", nil
		}
		return id, nil
	}
	if utf8.RuneCountInString(v) > oidcMaxNameRunes || strings.ContainsFunc(v, unicode.IsControl) {
		return "", fmt.Errorf("must be at most %d characters without control characters", oidcMaxNameRunes)
	}
	return v, nil
}

func parseOIDCIssuer(v, kind string, dev bool) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		if kind == OIDCKindGitHub {
			return OIDCDefaultGitHubIssuer, nil
		}
		return "", errors.New("is required, for example https://accounts.google.com")
	}
	u, err := url.Parse(v)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return "", errors.New("must be an absolute URL")
	}
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && dev:
	default:
		return "", errors.New("must use https (http is allowed only with DHOW_DEV=true)")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("must not contain credentials, a query or a fragment")
	}
	if kind == OIDCKindGitHub {
		return strings.TrimSuffix(v, "/"), nil
	}
	return v, nil
}

func loadOIDCSecret(getenv func(string) string, id string) (string, error) {
	v, err := secretValue(getenv, OIDCEnvName(id, oidcSuffixClientSecret), OIDCCredentialPrefix+id+OIDCCredentialSuffix)
	if err != nil {
		return "", err
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return "", errors.New("is required (or set the _FILE variant or the systemd credential)")
	}
	if len(v) > oidcMaxSecretBytes || strings.ContainsFunc(v, unicode.IsSpace) {
		return "", fmt.Errorf("must be at most %d bytes without spaces", oidcMaxSecretBytes)
	}
	lower := strings.ToLower(v)
	for _, m := range placeholderMarkers {
		if strings.Contains(lower, m) {
			return "", errors.New("looks like a placeholder; copy the secret from the identity provider")
		}
	}
	return v, nil
}

func parseAllowedDomains(v string) ([]string, error) {
	var out []string
	for _, part := range strings.Split(v, ",") {
		d := strings.ToLower(strings.TrimSpace(part))
		if d == "" {
			continue
		}
		if len(d) > oidcMaxDomainLength || !oidcDomain.MatchString(d) {
			return nil, fmt.Errorf("%q is not a domain name", d)
		}
		if !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	if len(out) > oidcMaxAllowedDomains {
		return nil, fmt.Errorf("lists more than %d domains", oidcMaxAllowedDomains)
	}
	return out, nil
}
