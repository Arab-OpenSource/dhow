package config

import (
	"errors"
	"fmt"
	"strings"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
)

const (
	EnvOIDCProviders = "DHOW_OIDC_PROVIDERS"

	OIDCKindOIDC   = settingspec.OIDCKindOIDC
	OIDCKindGitHub = settingspec.OIDCKindGitHub

	OIDCDefaultGitHubIssuer = settingspec.OIDCDefaultGitHubIssuer
	OIDCCredentialPrefix    = "dhow-oidc-" //nolint:gosec // The prefix of a systemd credential name, not a credential.
	OIDCCredentialSuffix    = "-client-secret"
)

const (
	oidcSuffixType           = "_TYPE"
	oidcSuffixName           = "_NAME"
	oidcSuffixIssuer         = "_ISSUER"
	oidcSuffixClientID       = "_CLIENT_ID"
	oidcSuffixClientSecret   = "_CLIENT_SECRET"
	oidcSuffixAllowedDomains = "_ALLOWED_DOMAINS"
)

type OIDCProvider = settingspec.OIDCProvider

func OIDCEnvName(id, suffix string) string {
	return strings.TrimSuffix(EnvOIDCProviders, "PROVIDERS") + strings.ToUpper(id) + suffix
}

func loadOIDCProviders(getenv func(string) string, dev bool) ([]OIDCProvider, error) {
	list := strings.TrimSpace(getenv(EnvOIDCProviders))
	if list == "" {
		return nil, nil
	}
	ids := strings.Split(list, ",")
	if len(ids) > settingspec.OIDCMaxProviders {
		return nil, fmt.Errorf("%s: lists more than %d providers", EnvOIDCProviders, settingspec.OIDCMaxProviders)
	}
	var errs []error
	out := make([]OIDCProvider, 0, len(ids))
	seen := map[string]bool{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if !settingspec.ValidOIDCProviderID(id) {
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
	seen := []OIDCProvider{}
	for _, p := range providers {
		if first, _, found := settingspec.SharedIssuer(append(seen, p)); found {
			errs = append(errs, fmt.Errorf("%s and %s: share the issuer %s, and identities are keyed by issuer, so each provider needs its own",
				OIDCEnvName(first.ID, oidcSuffixIssuer), OIDCEnvName(p.ID, oidcSuffixIssuer), p.Issuer))
			continue
		}
		seen = append(seen, p)
	}
	return errs
}

func loadOIDCProvider(getenv func(string) string, id string, dev bool) (OIDCProvider, error) {
	p := OIDCProvider{ID: id}
	var errs []error
	check := func(suffix string, e *settingspec.OIDCError) {
		if e != nil {
			errs = append(errs, fmt.Errorf("%s: %s", OIDCEnvName(id, suffix), e.Message))
		}
	}
	var e *settingspec.OIDCError
	p.Kind, e = settingspec.ParseOIDCKind(getenv(OIDCEnvName(id, oidcSuffixType)))
	check(oidcSuffixType, e)
	p.Name, e = settingspec.ParseOIDCName(getenv(OIDCEnvName(id, oidcSuffixName)), id, p.Kind)
	check(oidcSuffixName, e)
	p.Issuer, e = settingspec.ParseOIDCIssuer(getenv(OIDCEnvName(id, oidcSuffixIssuer)), p.Kind, dev)
	check(oidcSuffixIssuer, e)
	p.ClientID, e = settingspec.ParseOIDCClientID(getenv(OIDCEnvName(id, oidcSuffixClientID)))
	check(oidcSuffixClientID, e)
	secret, err := secretValue(getenv, OIDCEnvName(id, oidcSuffixClientSecret), OIDCCredentialPrefix+id+OIDCCredentialSuffix)
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", OIDCEnvName(id, oidcSuffixClientSecret), err))
	} else {
		p.ClientSecret, e = settingspec.ParseOIDCSecret(secret)
		check(oidcSuffixClientSecret, e)
	}
	domains, e := settingspec.ParseAllowedDomains(strings.Split(getenv(OIDCEnvName(id, oidcSuffixAllowedDomains)), ","))
	check(oidcSuffixAllowedDomains, e)
	if len(domains) > 0 {
		p.AllowedDomains = domains
	}
	if len(errs) > 0 {
		return OIDCProvider{}, errors.Join(errs...)
	}
	return p, nil
}
