package oidcmock

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	ClientID     = "dhow-test-client"
	ClientSecret = "Vq8rT2mZx4Lp7Nc1Bw9Ks3Hd6Fg0Jy5A" //nolint:gosec // A fixture of the mock identity provider used only by tests.
	keyID        = "test-key"
)

type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Issuer        string
	Audience      []string
	Azp           string
	ExpiresIn     time.Duration
	Nonce         *string
	SignWithOther bool
	GitHubEmails  []GitHubEmail
}

type GitHubEmail struct {
	Email    string
	Primary  bool
	Verified bool
}

type grant struct {
	challenge string
	nonce     string
	redirect  string
	claims    Claims
	used      bool
}

type IdP struct {
	Server           *httptest.Server
	GitHub           bool
	DiscoveredIssuer string
	HoldDiscovery    chan struct{}
	PlainEndpoint    string

	t     testing.TB
	now   func() time.Time
	key   *rsa.PrivateKey
	other *rsa.PrivateKey

	mu       sync.Mutex
	grants   map[string]*grant
	accesses map[string]Claims
	Requests map[string]int
}

func New(t testing.TB, github bool, now func() time.Time) *IdP {
	t.Helper()
	if now == nil {
		now = time.Now
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	idp := &IdP{
		GitHub: github, t: t, now: now, key: key, other: other,
		grants: map[string]*grant{}, accesses: map[string]Claims{}, Requests: map[string]int{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", idp.discovery)
	mux.HandleFunc("GET /jwks", idp.jwks)
	mux.HandleFunc("POST /token", idp.token)
	mux.HandleFunc("POST /login/oauth/access_token", idp.token)
	mux.HandleFunc("GET /api/v3/user", idp.githubUser)
	mux.HandleFunc("GET /api/v3/user/emails", idp.githubEmails)
	mux.HandleFunc("GET /api/v3/meta", idp.githubMeta)
	idp.Server = httptest.NewTLSServer(mux)
	t.Cleanup(idp.Server.Close)
	return idp
}

func (i *IdP) Issuer() string { return i.Server.URL }

func (i *IdP) Transport() http.RoundTripper { return i.Server.Client().Transport }

func (i *IdP) count(name string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Requests[name]++
}

func (i *IdP) Count(name string) int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.Requests[name]
}

func (i *IdP) discovery(w http.ResponseWriter, r *http.Request) {
	i.count("discovery")
	if i.HoldDiscovery != nil {
		select {
		case <-i.HoldDiscovery:
		case <-r.Context().Done():
			return
		}
	}
	issuer := i.Issuer()
	if i.DiscoveredIssuer != "" {
		issuer = i.DiscoveredIssuer
	}
	doc := map[string]any{
		"issuer": issuer, "authorization_endpoint": i.Issuer() + "/authorize", "token_endpoint": i.Issuer() + "/token",
		"jwks_uri": i.Issuer() + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"},
		"response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"},
	}
	if endpoint, ok := doc[i.PlainEndpoint].(string); ok {
		doc[i.PlainEndpoint] = "http://" + strings.TrimPrefix(endpoint, "https://")
	}
	i.writeJSON(w, http.StatusOK, doc)
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (i *IdP) jwks(w http.ResponseWriter, _ *http.Request) {
	i.count("jwks")
	pub := i.key.PublicKey
	i.writeJSON(w, http.StatusOK, map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "alg": "RS256", "use": "sig", "kid": keyID,
		"n": b64(pub.N.Bytes()), "e": b64(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

func (i *IdP) Authorize(authURL string, c Claims) (code, state string) {
	i.t.Helper()
	u, err := url.Parse(authURL)
	require.NoError(i.t, err)
	q := u.Query()
	require.Equal(i.t, ClientID, q.Get("client_id"))
	require.Equal(i.t, "code", q.Get("response_type"))
	require.Equal(i.t, "S256", q.Get("code_challenge_method"))
	require.NotEmpty(i.t, q.Get("code_challenge"))
	require.NotEmpty(i.t, q.Get("state"))
	if !i.GitHub {
		require.NotEmpty(i.t, q.Get("nonce"))
	}
	raw := make([]byte, 16)
	_, err = rand.Read(raw)
	require.NoError(i.t, err)
	code = b64(raw)
	i.mu.Lock()
	defer i.mu.Unlock()
	i.grants[code] = &grant{challenge: q.Get("code_challenge"), nonce: q.Get("nonce"), redirect: q.Get("redirect_uri"), claims: c}
	return code, q.Get("state")
}

func (i *IdP) token(w http.ResponseWriter, r *http.Request) {
	i.count("token")
	if err := r.ParseForm(); err != nil {
		i.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id != ClientID || subtle.ConstantTimeCompare([]byte(secret), []byte(ClientSecret)) != 1 {
		i.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}
	g, ok := i.take(r.PostForm.Get("code"))
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !ok || g.redirect != r.PostForm.Get("redirect_uri") || b64(sum[:]) != g.challenge {
		i.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
		return
	}
	access := make([]byte, 16)
	if _, err := rand.Read(access); err != nil {
		i.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}
	body := map[string]any{"access_token": b64(access), "token_type": "Bearer", "expires_in": 300}
	i.mu.Lock()
	i.accesses[b64(access)] = g.claims
	i.mu.Unlock()
	if !i.GitHub {
		body["id_token"] = i.idToken(g)
	}
	i.writeJSON(w, http.StatusOK, body)
}

func (i *IdP) take(code string) (grant, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	g, ok := i.grants[code]
	if !ok || g.used {
		return grant{}, false
	}
	g.used = true
	return *g, true
}

func (i *IdP) idToken(g grant) string {
	c := g.claims
	now := i.now()
	expires := c.ExpiresIn
	if expires == 0 {
		expires = 5 * time.Minute
	}
	issuer := c.Issuer
	if issuer == "" {
		issuer = i.Issuer()
	}
	aud := c.Audience
	if aud == nil {
		aud = []string{ClientID}
	}
	nonce := g.nonce
	if c.Nonce != nil {
		nonce = *c.Nonce
	}
	payload := map[string]any{
		"iss": issuer, "sub": c.Subject, "aud": aud, "iat": now.Add(-time.Second).Unix(), "exp": now.Add(expires).Unix(),
		"nonce": nonce, "email": c.Email, "email_verified": c.EmailVerified, "name": c.Name,
	}
	if c.Azp != "" {
		payload["azp"] = c.Azp
	}
	key := i.key
	if c.SignWithOther {
		key = i.other
	}
	return sign(i.t, key, payload)
}

func sign(t testing.TB, key *rsa.PrivateKey, payload map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": keyID})
	require.NoError(t, err)
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	signing := b64(header) + "." + b64(body)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return signing + "." + b64(sig)
}

func (i *IdP) claimsFor(r *http.Request) (Claims, bool) {
	const prefix = "Bearer "
	v := r.Header.Get("Authorization")
	if len(v) <= len(prefix) {
		return Claims{}, false
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	c, ok := i.accesses[v[len(prefix):]]
	return c, ok
}

func (i *IdP) githubUser(w http.ResponseWriter, r *http.Request) {
	i.count("github_user")
	c, ok := i.claimsFor(r)
	if !ok {
		i.writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials"})
		return
	}
	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil {
		id = 0
	}
	i.writeJSON(w, http.StatusOK, map[string]any{"id": id, "login": "octo-" + c.Subject, "name": c.Name})
}

func (i *IdP) githubMeta(w http.ResponseWriter, r *http.Request) {
	i.count("github_meta")
	if r.Header.Get("Authorization") != "" {
		i.writeJSON(w, http.StatusBadRequest, map[string]string{"message": "the meta check sends no credentials"})
		return
	}
	i.writeJSON(w, http.StatusOK, map[string]any{"verifiable_password_authentication": false})
}

func (i *IdP) githubEmails(w http.ResponseWriter, r *http.Request) {
	i.count("github_emails")
	c, ok := i.claimsFor(r)
	if !ok {
		i.writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials"})
		return
	}
	emails := make([]map[string]any, 0, len(c.GitHubEmails))
	for _, e := range c.GitHubEmails {
		emails = append(emails, map[string]any{"email": e.Email, "primary": e.Primary, "verified": e.Verified})
	}
	i.writeJSON(w, http.StatusOK, emails)
}

func (i *IdP) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		i.t.Errorf("encode mock response: %v", err)
	}
}
