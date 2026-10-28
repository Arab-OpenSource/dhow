package oidcauth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	oidcauth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Oidc"
	oidcmock "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Oidc/Mock"
)

func devRegistry(t *testing.T, idp *oidcmock.IdP, c *clock) oidcauth.Provider {
	t.Helper()
	r, err := oidcauth.NewRegistry([]config.OIDCProvider{{
		ID: "corp", Kind: config.OIDCKindOIDC, Name: "Corp", Issuer: idp.Issuer(), ClientID: oidcmock.ClientID,
		ClientSecret: oidcmock.ClientSecret,
	}}, oidcauth.Options{Transport: idp.Transport(), Now: c.now, AllowHTTP: true})
	require.NoError(t, err)
	p, err := r.Get("corp")
	require.NoError(t, err)
	return p
}

func TestOIDCDiscoveryRunsOnceAndNeverHoldsACallerPastItsDeadline(t *testing.T) {
	c := newClock()
	idp := oidcmock.New(t, false, c.now)
	idp.HoldDiscovery = make(chan struct{})
	p := registry(t, idp, config.OIDCKindOIDC, c)

	short, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := p.AuthorizeURL(short, flow())
	assert.ErrorIs(t, err, oidcauth.ErrUnavailable)
	assert.Less(t, time.Since(started), 2*time.Second, "a slow provider does not hold the caller")

	const callers = 5
	results := make(chan error, callers)
	for range callers {
		go func() {
			_, err := p.AuthorizeURL(context.Background(), flow())
			results <- err
		}()
	}
	close(idp.HoldDiscovery)
	for range callers {
		assert.NoError(t, <-results)
	}
	assert.Equal(t, 1, idp.Count("discovery"), "every caller shares the one discovery in flight")
}

func TestOIDCRefusesPlainHTTPEndpointsFromDiscovery(t *testing.T) {
	for _, endpoint := range []string{"authorization_endpoint", "token_endpoint", "jwks_uri"} {
		t.Run(endpoint, func(t *testing.T) {
			c := newClock()
			idp := oidcmock.New(t, false, c.now)
			idp.PlainEndpoint = endpoint
			_, err := registry(t, idp, config.OIDCKindOIDC, c).AuthorizeURL(context.Background(), flow())
			assert.ErrorIs(t, err, oidcauth.ErrUnavailable)
			_, err = devRegistry(t, idp, c).AuthorizeURL(context.Background(), flow())
			assert.NoError(t, err, "DHOW_DEV allows plain http endpoints")
		})
	}
}
