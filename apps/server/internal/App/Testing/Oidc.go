package apptest

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	oidcmock "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Oidc/Mock"
)

const OidcProvider = "corp"

type OidcOptions struct {
	GitHub  bool
	Domains []string
	Pool    Options
}

func NewWithIdP(t *testing.T, o OidcOptions) (*Env, *oidcmock.IdP) {
	t.Helper()
	clock := NewClock(time.Now().UTC().Truncate(time.Second))
	idp := oidcmock.New(t, o.GitHub, clock.Now)
	kind := config.OIDCKindOIDC
	if o.GitHub {
		kind = config.OIDCKindGitHub
	}
	opts := o.Pool
	opts.Clock, opts.OidcTransport = clock, idp.Transport()
	profile := opts.Profile
	opts.Profile = func(c *config.Config) {
		c.OIDCProviders = []config.OIDCProvider{{
			ID: OidcProvider, Kind: kind, Name: "Corp SSO", Issuer: idp.Issuer(), ClientID: oidcmock.ClientID,
			ClientSecret: oidcmock.ClientSecret, AllowedDomains: o.Domains,
		}}
		if profile != nil {
			profile(c)
		}
	}
	return NewWithOptions(t, opts), idp
}

type OidcStart struct {
	URL    string
	Cookie string
	Res    Response
}

func cookies(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] != "" {
			parts = append(parts, pairs[i]+"="+pairs[i+1])
		}
	}
	return strings.Join(parts, "; ")
}

func (c *Client) oidcHeaders(flowCookie string) map[string]string {
	h := map[string]string{}
	if v := cookies(config.SessionCookieName, c.Cookie, config.OidcCookieName, flowCookie); v != "" {
		h["Cookie"] = v
	}
	return h
}

func (c *Client) BeginOidc(path string, body any) OidcStart {
	c.Env.T.Helper()
	res := c.Do(Request{Method: http.MethodPost, Path: path, Body: body})
	out := OidcStart{Res: res}
	if res.Status != http.StatusOK {
		return out
	}
	var data struct{ AuthorizationUrl string }
	res.Decode(c.Env.T, &data)
	cookie, raw, ok := SetCookie(res.Header, config.OidcCookieName)
	require.True(c.Env.T, ok, "the flow sets its cookie")
	require.Contains(c.Env.T, raw, "HttpOnly")
	require.Contains(c.Env.T, raw, "SameSite=Strict")
	out.URL, out.Cookie = data.AuthorizationUrl, cookie
	return out
}

func (c *Client) FinishOidc(path, flowCookie, code, state string, headers map[string]string) Response {
	c.Env.T.Helper()
	h := c.oidcHeaders(flowCookie)
	for k, v := range headers {
		h[k] = v
	}
	return c.Do(Request{Method: http.MethodPost, Path: path, Headers: h, Body: map[string]string{
		"Provider": OidcProvider, "Code": code, "State": state,
	}})
}

func (e *Env) OidcLogin(idp *oidcmock.IdP, claims oidcmock.Claims) (*Client, Response) {
	e.T.Helper()
	anonymous := &Client{Env: e}
	start := anonymous.BeginOidc("/oidc/authorize", map[string]string{"Provider": OidcProvider})
	require.Equal(e.T, http.StatusOK, start.Res.Status, string(start.Res.Raw))
	code, state := idp.Authorize(start.URL, claims)
	res := anonymous.FinishOidc("/oidc/callback", start.Cookie, code, state, nil)
	return e.clientFrom(res), res
}

func (c *Client) LinkOidc(idp *oidcmock.IdP, factor map[string]any, claims oidcmock.Claims) Response {
	c.Env.T.Helper()
	start := c.BeginOidc("/me/identities/authorize", map[string]any{"Provider": OidcProvider, "CurrentFactor": factor})
	require.Equal(c.Env.T, http.StatusOK, start.Res.Status, string(start.Res.Raw))
	code, state := idp.Authorize(start.URL, claims)
	return c.FinishOidc("/me/identities/callback", start.Cookie, code, state, nil)
}
