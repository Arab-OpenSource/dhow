package oidcauth

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"syscall"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
)

type Check string

const (
	CheckDiscovery Check = "discovery"
	CheckEndpoints Check = "endpoints"
	CheckKeys      Check = "keys"
	CheckAPI       Check = "api"

	dialTimeout   = 10 * time.Second
	dialKeepAlive = 30 * time.Second
)

var (
	errBlockedAddress = errors.New("the address is link-local or a cloud metadata service")
	errNoKeys         = errors.New("the key set has no keys")
)

type CheckResult struct {
	Check  Check
	Passed bool
}

type ProbeResult struct {
	Checks []CheckResult
	Err    error
}

func (r ProbeResult) Passed() bool {
	for _, c := range r.Checks {
		if !c.Passed {
			return false
		}
	}
	return len(r.Checks) > 0
}

func refuseBlocked(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if settingspec.BlockedHost(host) {
		return errBlockedAddress
	}
	return nil
}

func guardedTransport() http.RoundTripper {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}
	t := base.Clone()
	t.DialContext = (&net.Dialer{Timeout: dialTimeout, KeepAlive: dialKeepAlive, Control: refuseBlocked}).DialContext
	return t
}

func (r *Registry) Probe(ctx context.Context, kind, issuer string) ProbeResult {
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	if kind == config.OIDCKindGitHub {
		return r.probeGitHub(ctx, issuer)
	}
	return r.probeOIDC(ctx, issuer)
}

func (r *Registry) probeOIDC(ctx context.Context, issuer string) ProbeResult {
	out := ProbeResult{Checks: []CheckResult{{Check: CheckDiscovery}, {Check: CheckEndpoints}, {Check: CheckKeys}}}
	discovered, err := gooidc.NewProvider(gooidc.ClientContext(ctx, r.client), issuer)
	if err != nil {
		out.Err = err
		return out
	}
	out.Checks[0].Passed = true
	jwks, err := secureEndpoints(discovered, r.allowHTTP)
	if err != nil {
		out.Err = err
		return out
	}
	out.Checks[1].Passed = true
	if err := r.fetchKeys(ctx, jwks); err != nil {
		out.Err = err
		return out
	}
	out.Checks[2].Passed = true
	return out
}

func (r *Registry) fetchKeys(ctx context.Context, jwks string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwks, http.NoBody)
	if err != nil {
		return err
	}
	res, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	body, err := readJSON(res)
	if err != nil {
		return err
	}
	var set map[string]json.RawMessage
	if err := json.Unmarshal(body, &set); err != nil {
		return err
	}
	var keys []json.RawMessage
	if err := json.Unmarshal(set["keys"], &keys); err != nil || len(keys) == 0 {
		return errNoKeys
	}
	return nil
}

func (r *Registry) probeGitHub(ctx context.Context, issuer string) ProbeResult {
	out := ProbeResult{Checks: []CheckResult{{Check: CheckAPI}}}
	p := newGitHub(config.OIDCProvider{Issuer: issuer}, r.client)
	var meta map[string]json.RawMessage
	if err := p.get(ctx, "", "/meta", &meta); err != nil {
		out.Err = err
		return out
	}
	out.Checks[0].Passed = true
	return out
}
