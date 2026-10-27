package apptest

import (
	"encoding/base32"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/stretchr/testify/require"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
)

func SetCookie(h http.Header, name string) (value, raw string, ok bool) {
	for _, line := range h.Values("Set-Cookie") {
		n, rest, found := strings.Cut(line, "=")
		if !found || n != name {
			continue
		}
		v, _, _ := strings.Cut(rest, ";")
		return v, line, true
	}
	return "", "", false
}

func (e *Env) PasswordStep(email, password string, headers map[string]string) Response {
	e.T.Helper()
	h := map[string]string{"Origin": PublicOrigin}
	for k, v := range headers {
		h[k] = v
	}
	return e.Do(Request{Method: http.MethodPost, Path: "/sessions", Headers: h, Body: map[string]string{"Email": email, "Password": password}})
}

func (e *Env) StartMfa(email, password string) string {
	e.T.Helper()
	res := e.PasswordStep(email, password, nil)
	require.Equal(e.T, http.StatusAccepted, res.Status, string(res.Raw))
	cookie, _, ok := SetCookie(res.Header, config.MfaCookieName)
	require.True(e.T, ok, "the password step sets the challenge cookie")
	return cookie
}

func (e *Env) CompleteMfa(cookie string, body any, headers map[string]string) (*Client, Response) {
	e.T.Helper()
	h := map[string]string{"Origin": PublicOrigin}
	if cookie != "" {
		h["Cookie"] = config.MfaCookieName + "=" + cookie
	}
	for k, v := range headers {
		h[k] = v
	}
	res := e.Do(Request{Method: http.MethodPost, Path: "/sessions/mfa", Headers: h, Body: body})
	return e.clientFrom(res), res
}

func (e *Env) clientFrom(res Response) *Client {
	e.T.Helper()
	c := &Client{Env: e}
	if res.Status != http.StatusOK {
		return c
	}
	var data struct {
		CsrfToken string
		User      struct{ Id string }
	}
	res.Decode(e.T, &data)
	c.CSRFToken, c.UserID = data.CsrfToken, data.User.Id
	c.Cookie, _, _ = SetCookie(res.Header, config.SessionCookieName)
	return c
}

func (e *Env) TotpCode(secret []byte, offsetSteps int64) string {
	return auth.TotpCodeAt(secret, auth.TotpStep(e.Clock.Now())+offsetSteps)
}

func PasswordFactor(password string) map[string]any {
	return map[string]any{"Method": "password", "Password": password}
}

func CodeFactor(method, code string) map[string]any {
	return map[string]any{"Method": method, "Code": code}
}

type TotpEnrollment struct {
	Secret        []byte
	RecoveryCodes []string
	Response      Response
}

func (c *Client) StartTotp() []byte {
	c.Env.T.Helper()
	res := c.Do(Request{Method: http.MethodPost, Path: "/me/totp"})
	require.Equal(c.Env.T, http.StatusOK, res.Status, string(res.Raw))
	var data struct{ Secret, Uri string }
	res.Decode(c.Env.T, &data)
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(data.Secret)
	require.NoError(c.Env.T, err)
	require.Contains(c.Env.T, data.Uri, "secret="+data.Secret)
	return secret
}

func (c *Client) ConfirmTotp(code string, factor map[string]any) Response {
	c.Env.T.Helper()
	return c.Do(Request{Method: http.MethodPost, Path: "/me/totp/confirm", Body: map[string]any{"Code": code, "CurrentFactor": factor}})
}

func (c *Client) EnableTotp(factor map[string]any) TotpEnrollment {
	c.Env.T.Helper()
	secret := c.StartTotp()
	res := c.ConfirmTotp(c.Env.TotpCode(secret, 0), factor)
	require.Equal(c.Env.T, http.StatusOK, res.Status, string(res.Raw))
	var data struct{ RecoveryCodes []string }
	res.Decode(c.Env.T, &data)
	return TotpEnrollment{Secret: secret, RecoveryCodes: data.RecoveryCodes, Response: res}
}

type PasskeyChallenge struct {
	ChallengeId string
	Options     json.RawMessage
}

func (c *Client) PasskeyChallenge(path string) PasskeyChallenge {
	c.Env.T.Helper()
	res := c.Do(Request{Method: http.MethodPost, Path: path})
	require.Equal(c.Env.T, http.StatusOK, res.Status, string(res.Raw))
	var ch PasskeyChallenge
	res.Decode(c.Env.T, &ch)
	return ch
}

func (c *Client) AddPasskey(a *Authenticator, name string, factor map[string]any, o Override) Response {
	c.Env.T.Helper()
	ch := c.PasskeyChallenge("/me/passkeys/options")
	credential, err := a.Register(ch.Options, PublicOrigin, o)
	require.NoError(c.Env.T, err)
	return c.Do(Request{Method: http.MethodPost, Path: "/me/passkeys", Body: map[string]any{
		"ChallengeId": ch.ChallengeId, "Credential": credential, "Name": name, "CurrentFactor": factor,
	}})
}

func (c *Client) PasskeyFactor(a *Authenticator) map[string]any {
	c.Env.T.Helper()
	ch := c.PasskeyChallenge("/me/mfa/passkey-options")
	credential, err := a.Assert(ch.Options, PublicOrigin, Override{})
	require.NoError(c.Env.T, err)
	return map[string]any{"Method": "passkey", "ChallengeId": ch.ChallengeId, "Credential": credential}
}

func (e *Env) PasskeyLogin(a *Authenticator, o Override) (*Client, Response) {
	e.T.Helper()
	anonymous := &Client{Env: e}
	ch := anonymous.PasskeyChallenge("/sessions/passkey-options")
	credential, err := a.Assert(ch.Options, PublicOrigin, o)
	require.NoError(e.T, err)
	res := e.Do(Request{
		Method: http.MethodPost, Path: "/sessions/passkey", Headers: map[string]string{"Origin": PublicOrigin},
		Body: map[string]any{"ChallengeId": ch.ChallengeId, "Credential": credential},
	})
	return e.clientFrom(res), res
}

func (e *Env) MfaPasskeyAssertion(cookie string, a *Authenticator, o Override) map[string]any {
	e.T.Helper()
	res := e.Do(Request{Method: http.MethodPost, Path: "/sessions/mfa/passkey-options", Headers: map[string]string{
		"Origin": PublicOrigin, "Cookie": config.MfaCookieName + "=" + cookie,
	}})
	require.Equal(e.T, http.StatusOK, res.Status, string(res.Raw))
	var data struct{ Options json.RawMessage }
	res.Decode(e.T, &data)
	credential, err := a.Assert(data.Options, PublicOrigin, o)
	require.NoError(e.T, err)
	return map[string]any{"Method": "passkey", "Credential": credential}
}

func (e *Env) AuditCount(action string) int {
	e.T.Helper()
	var n int
	require.NoError(e.T, e.Pool.QueryRow(e.T.Context(), `SELECT count(*) FROM "AuditEvents" WHERE "Action" = $1`, action).Scan(&n))
	return n
}
