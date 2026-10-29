package oidcauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	gooidc "github.com/coreos/go-oidc/v3/oidc"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
)

var errInsecureEndpoint = errors.New("discovery names an endpoint that is not https")

func (p *oidcProvider) provider(ctx context.Context) (*gooidc.Provider, error) {
	p.mu.Lock()
	if p.discovered != nil {
		discovered := p.discovered
		p.mu.Unlock()
		return discovered, nil
	}
	if p.inflight == nil {
		if !p.failedAt.IsZero() && p.now().Sub(p.failedAt) < DiscoveryRetryAfter {
			p.mu.Unlock()
			return nil, ErrUnavailable
		}
		p.inflight = make(chan struct{})
		go p.discover(context.WithoutCancel(ctx), p.inflight)
	}
	done := p.inflight
	p.mu.Unlock()
	select {
	case <-done:
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: discovery for %s: %w", ErrUnavailable, p.cfg.ID, ctx.Err())
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.discovered != nil {
		return p.discovered, nil
	}
	return nil, p.failure
}

func (p *oidcProvider) discover(ctx context.Context, done chan struct{}) {
	defer close(done)
	dctx, cancel := context.WithTimeout(gooidc.ClientContext(ctx, p.client), RequestTimeout)
	defer cancel()
	discovered, err := gooidc.NewProvider(dctx, p.cfg.Issuer)
	if err == nil {
		err = p.checkEndpoints(discovered)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inflight = nil
	if err != nil {
		p.failedAt = p.now()
		p.failure = fmt.Errorf("%w: discovery for %s: %w", ErrUnavailable, p.cfg.ID, err)
		return
	}
	p.discovered = discovered
}

func (p *oidcProvider) checkEndpoints(d *gooidc.Provider) error {
	_, err := secureEndpoints(d, p.allowHTTP)
	return err
}

func secureEndpoints(d *gooidc.Provider, allowHTTP bool) (string, error) {
	var raw map[string]json.RawMessage
	if err := d.Claims(&raw); err != nil {
		return "", err
	}
	var jwks string
	if err := json.Unmarshal(raw["jwks_uri"], &jwks); err != nil {
		return "", fmt.Errorf("jwks_uri: %w", err)
	}
	endpoints := []struct{ name, value string }{
		{"authorization_endpoint", d.Endpoint().AuthURL},
		{"token_endpoint", d.Endpoint().TokenURL},
		{"jwks_uri", jwks},
	}
	for _, e := range endpoints {
		if !secureEndpoint(e.value, allowHTTP) {
			return "", fmt.Errorf("%w: %s", errInsecureEndpoint, e.name)
		}
	}
	return jwks, nil
}

func secureEndpoint(raw string, allowHTTP bool) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || settingspec.BlockedHost(u.Hostname()) {
		return false
	}
	return u.Scheme == "https" || (allowHTTP && u.Scheme == "http")
}
