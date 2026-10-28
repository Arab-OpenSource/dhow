package oidcauth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	oidcauth "github.com/Khaymah/dhow/apps/server/internal/Core/Oidc"
	oidcmock "github.com/Khaymah/dhow/apps/server/internal/Core/Oidc/Mock"
)

const redirect = "https://dhow.example.com/oidc/callback/corp"

type clock struct{ nanos atomic.Int64 }

func newClock() *clock {
	c := &clock{}
	c.nanos.Store(time.Now().UnixNano())
	return c
}

func (c *clock) now() time.Time { return time.Unix(0, c.nanos.Load()) }

func (c *clock) advance(d time.Duration) { c.nanos.Add(int64(d)) }

func registry(t *testing.T, idp *oidcmock.IdP, kind string, c *clock, domains ...string) oidcauth.Provider {
	t.Helper()
	r, err := oidcauth.NewRegistry([]config.OIDCProvider{{
		ID: "corp", Kind: kind, Name: "Corp", Issuer: idp.Issuer(), ClientID: oidcmock.ClientID,
		ClientSecret: oidcmock.ClientSecret, AllowedDomains: domains,
	}}, oidcauth.Options{Transport: idp.Transport(), Now: c.now})
	require.NoError(t, err)
	p, err := r.Get("corp")
	require.NoError(t, err)
	return p
}

func flow() oidcauth.Flow {
	return oidcauth.Flow{
		State: "state-value", Nonce: "nonce-value", RedirectURL: redirect,
		Verifier: "verifier-value-that-is-long-enough-for-pkce-0123456789",
	}
}

func signIn(t *testing.T, idp *oidcmock.IdP, p oidcauth.Provider, f oidcauth.Flow, claims oidcmock.Claims) (oidcauth.Identity, error) {
	t.Helper()
	authURL, err := p.AuthorizeURL(context.Background(), f)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(authURL, idp.Issuer()+"/"))
	code, state := idp.Authorize(authURL, claims)
	assert.Equal(t, f.State, state)
	return p.Exchange(context.Background(), code, f)
}

func TestOIDCSignInReturnsTheVerifiedIdentity(t *testing.T) {
	c := newClock()
	idp := oidcmock.New(t, false, c.now)
	p := registry(t, idp, config.OIDCKindOIDC, c)
	id, err := signIn(t, idp, p, flow(), oidcmock.Claims{Subject: "user-1", Email: "Sara@Example.com", EmailVerified: true, Name: "Sara"})
	require.NoError(t, err)
	assert.Equal(t, idp.Issuer(), id.Issuer)
	assert.Equal(t, "user-1", id.Subject)
	assert.Equal(t, "Sara", id.Name)
	email, ok := oidcauth.AcceptedEmail(id, nil)
	assert.True(t, ok)
	assert.Equal(t, "sara@example.com", email)
	assert.Equal(t, 1, idp.Count("discovery"))
	assert.Equal(t, oidcauth.Info{ID: "corp", Name: "Corp", Kind: config.OIDCKindOIDC}, p.Info())
}

func TestOIDCRejectsTokensThatDoNotMatchTheFlow(t *testing.T) {
	other := "another-nonce"
	tests := []struct {
		name   string
		claims oidcmock.Claims
		flow   func(oidcauth.Flow) oidcauth.Flow
	}{
		{"wrong nonce", oidcmock.Claims{Subject: "u", Nonce: &other}, nil},
		{"missing nonce", oidcmock.Claims{Subject: "u", Nonce: new(string)}, nil},
		{"issuer mismatch", oidcmock.Claims{Subject: "u", Issuer: "https://evil.example"}, nil},
		{"audience mismatch", oidcmock.Claims{Subject: "u", Audience: []string{"someone-else"}}, nil},
		{"issued to another party", oidcmock.Claims{Subject: "u", Audience: []string{oidcmock.ClientID, "other"}, Azp: "other"}, nil},
		{"expired id_token", oidcmock.Claims{Subject: "u", ExpiresIn: -time.Minute}, nil},
		{"signed with an unknown key", oidcmock.Claims{Subject: "u", SignWithOther: true}, nil},
		{"empty subject", oidcmock.Claims{}, nil},
		{"subject longer than 255", oidcmock.Claims{Subject: strings.Repeat("s", 256)}, nil},
		{"wrong PKCE verifier", oidcmock.Claims{Subject: "u"}, func(f oidcauth.Flow) oidcauth.Flow {
			f.Verifier = "a-different-verifier-that-is-long-enough-0123456789abc"
			return f
		}},
		{"other redirect URI", oidcmock.Claims{Subject: "u"}, func(f oidcauth.Flow) oidcauth.Flow {
			f.RedirectURL = "https://dhow.example.com/oidc/callback/other"
			return f
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClock()
			idp := oidcmock.New(t, false, c.now)
			p := registry(t, idp, config.OIDCKindOIDC, c)
			f := flow()
			authURL, err := p.AuthorizeURL(context.Background(), f)
			require.NoError(t, err)
			code, _ := idp.Authorize(authURL, tt.claims)
			if tt.flow != nil {
				f = tt.flow(f)
			}
			_, err = p.Exchange(context.Background(), code, f)
			require.Error(t, err)
			assert.ErrorIs(t, err, oidcauth.ErrRejected)
		})
	}
}

func TestOIDCCodeWorksOnce(t *testing.T) {
	c := newClock()
	idp := oidcmock.New(t, false, c.now)
	p := registry(t, idp, config.OIDCKindOIDC, c)
	f := flow()
	authURL, err := p.AuthorizeURL(context.Background(), f)
	require.NoError(t, err)
	code, _ := idp.Authorize(authURL, oidcmock.Claims{Subject: "u"})
	_, err = p.Exchange(context.Background(), code, f)
	require.NoError(t, err)
	_, err = p.Exchange(context.Background(), code, f)
	assert.ErrorIs(t, err, oidcauth.ErrRejected)
}

func TestOIDCDiscoveryIssuerMismatchIsUnavailableAndRetriedLater(t *testing.T) {
	c := newClock()
	idp := oidcmock.New(t, false, c.now)
	idp.DiscoveredIssuer = "https://other.example"
	p := registry(t, idp, config.OIDCKindOIDC, c)
	_, err := p.AuthorizeURL(context.Background(), flow())
	assert.ErrorIs(t, err, oidcauth.ErrUnavailable)
	_, err = p.AuthorizeURL(context.Background(), flow())
	assert.ErrorIs(t, err, oidcauth.ErrUnavailable)
	assert.Equal(t, 1, idp.Count("discovery"), "a failed discovery is not retried at once")
	idp.DiscoveredIssuer = ""
	c.advance(oidcauth.DiscoveryRetryAfter)
	_, err = p.AuthorizeURL(context.Background(), flow())
	require.NoError(t, err)
	assert.Equal(t, 2, idp.Count("discovery"))
}

func TestGitHubUsesTheNumericIDAsTheSubject(t *testing.T) {
	c := newClock()
	idp := oidcmock.New(t, true, c.now)
	p := registry(t, idp, config.OIDCKindGitHub, c, "example.com")
	id, err := signIn(t, idp, p, flow(), oidcmock.Claims{Subject: "583231", Name: "Octo", GitHubEmails: []oidcmock.GitHubEmail{
		{Email: "octo@personal.dev", Primary: true, Verified: true},
		{Email: "octo@example.com", Verified: false},
		{Email: "octo.work@example.com", Verified: true},
	}})
	require.NoError(t, err)
	assert.Equal(t, "583231", id.Subject)
	assert.Equal(t, idp.Issuer(), id.Issuer)
	email, ok := oidcauth.AcceptedEmail(id, p.AllowedDomains())
	assert.True(t, ok)
	assert.Equal(t, "octo.work@example.com", email)
	assert.Zero(t, idp.Count("discovery"))
}

func TestGitHubRejectsAUserWithoutAnID(t *testing.T) {
	c := newClock()
	idp := oidcmock.New(t, true, c.now)
	p := registry(t, idp, config.OIDCKindGitHub, c)
	_, err := signIn(t, idp, p, flow(), oidcmock.Claims{Subject: "not-a-number"})
	assert.ErrorIs(t, err, oidcauth.ErrRejected)
}

func TestAcceptedEmail(t *testing.T) {
	id := func(emails ...oidcauth.Email) oidcauth.Identity { return oidcauth.Identity{Emails: emails} }
	tests := []struct {
		name    string
		id      oidcauth.Identity
		allowed []string
		email   string
		ok      bool
	}{
		{"no allowlist, verified", id(oidcauth.Email{Address: "A@X.io", Verified: true}), nil, "a@x.io", true},
		{"no allowlist, unverified is not shown", id(oidcauth.Email{Address: "a@x.io"}), nil, "", true},
		{"no allowlist, no email", id(), nil, "", true},
		{"allowlisted and verified", id(oidcauth.Email{Address: "a@corp.io", Verified: true}), []string{"corp.io"}, "a@corp.io", true},
		{"allowlisted but unverified", id(oidcauth.Email{Address: "a@corp.io"}), []string{"corp.io"}, "", false},
		{"other domain", id(oidcauth.Email{Address: "a@gmail.com", Verified: true}), []string{"corp.io"}, "", false},
		{"subdomain is not the domain", id(oidcauth.Email{Address: "a@evil.corp.io", Verified: true}), []string{"corp.io"}, "", false},
		{"suffix trick", id(oidcauth.Email{Address: "a@corp.io.evil.com", Verified: true}), []string{"corp.io"}, "", false},
		{"two at signs", id(oidcauth.Email{Address: "a@corp.io@evil.com", Verified: true}), []string{"corp.io"}, "", false},
		{"no email", id(), []string{"corp.io"}, "", false},
		{"prefers the primary", id(
			oidcauth.Email{Address: "b@corp.io", Verified: true},
			oidcauth.Email{Address: "a@corp.io", Verified: true, Primary: true},
		), []string{"corp.io"}, "a@corp.io", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, ok := oidcauth.AcceptedEmail(tt.id, tt.allowed)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.email, email)
		})
	}
}

func TestRegistry(t *testing.T) {
	r, err := oidcauth.NewRegistry(nil, oidcauth.Options{})
	require.NoError(t, err)
	assert.Empty(t, r.List())
	_, err = r.Get("google")
	assert.ErrorIs(t, err, oidcauth.ErrUnknown)
	var none *oidcauth.Registry
	assert.Empty(t, none.List())
	_, err = none.Get("google")
	assert.ErrorIs(t, err, oidcauth.ErrUnknown)
	_, err = oidcauth.NewRegistry([]config.OIDCProvider{{ID: "x", Kind: "saml"}}, oidcauth.Options{})
	assert.Error(t, err)
}

func TestResponsesAreCappedAndRedirectsAreNotFollowed(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"oversized discovery document", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issuer":"` + strings.Repeat("a", oidcauth.MaxResponseBytes) + `"}`))
		}},
		{"redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://elsewhere.example/.well-known/openid-configuration", http.StatusFound)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewTLSServer(tt.handler)
			t.Cleanup(srv.Close)
			r, err := oidcauth.NewRegistry([]config.OIDCProvider{{
				ID: "big", Kind: config.OIDCKindOIDC, Issuer: srv.URL, ClientID: "c", ClientSecret: "s",
			}}, oidcauth.Options{Transport: srv.Client().Transport})
			require.NoError(t, err)
			p, err := r.Get("big")
			require.NoError(t, err)
			_, err = p.AuthorizeURL(context.Background(), flow())
			require.Error(t, err)
			assert.True(t, errors.Is(err, oidcauth.ErrUnavailable))
		})
	}
}
