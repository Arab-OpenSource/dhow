package e2e_test

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	"github.com/Khaymah/dhow/internal/netguard"
)

const sha = "0123456789abcdef0123456789abcdef01234567"

type world struct {
	env      *apptest.Env
	owner    *apptest.Client
	org      uuid.UUID
	project  uuid.UUID
	triggers *apptest.FakeTriggers
}

type repoView struct {
	Id, ProjectId, Provider, CloneUrl, Transport, FullName, DefaultBranch, WebhookUrl, State string
	DeployKeyPublic, PreviousDeployKeyPublic, HostKeyFingerprint, PendingHostKeyFingerprint  *string
	LastDeliveryAt                                                                           *string
}

type linkedView struct {
	Repository    repoView
	WebhookSecret *string
}

func lookup(_ context.Context, host string) ([]netip.Addr, error) {
	answers := map[string][]string{
		"git.example.test":     {"10.255.0.10"},
		"meta.example.test":    {"169.254.169.254"},
		"private.example.test": {"10.1.2.3"},
		"mixed.example.test":   {"203.0.113.10", "127.0.0.1"},
		"mapped.example.test":  {"::ffff:10.255.0.10"},
	}
	var out []netip.Addr
	for _, a := range answers[host] {
		out = append(out, netip.MustParseAddr(a))
	}
	if len(out) == 0 {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return out, nil
}

func toLoopback(ctx context.Context, network, address string) (net.Conn, error) {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	var d net.Dialer
	return d.DialContext(ctx, network, net.JoinHostPort("127.0.0.1", port))
}

func newWorld(t *testing.T, allowLAN bool) world {
	t.Helper()
	fake := &apptest.FakeTriggers{}
	env := apptest.NewWithOptions(t, apptest.Options{
		Triggers: fake, GitLookup: lookup, GitDial: toLoopback,
		Profile: func(c *config.Config) {
			if allowLAN {
				allowed, err := netguard.ParseCIDRs("10.255.0.0/16")
				require.NoError(t, err)
				c.Sources.AllowPrivate = allowed
			}
		},
	})
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	return world{env: env, owner: owner, org: org, project: env.SeedProject(org, uuid.MustParse(owner.UserID)), triggers: fake}
}

func (w world) path(rest string) string {
	return apptest.OrgPath(w.org, "/projects/"+w.project.String()+"/repository"+rest)
}

func (w world) link(t *testing.T, c *apptest.Client, cloneURL string) apptest.Response {
	t.Helper()
	return c.Do(apptest.Request{Method: http.MethodPut, Path: w.path(""), Body: map[string]any{"CloneUrl": cloneURL}})
}

func (w world) post(c *apptest.Client, action string, body any) apptest.Response {
	return c.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/" + action), Body: body})
}

func linked(t *testing.T, res apptest.Response, status int) linkedView {
	t.Helper()
	require.Equal(t, status, res.Status, string(res.Raw))
	var l linkedView
	res.Decode(t, &l)
	return l
}

func repo(t *testing.T, res apptest.Response) repoView {
	t.Helper()
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var r repoView
	res.Decode(t, &r)
	return r
}

func requireError(t *testing.T, res apptest.Response, status int, code string) {
	t.Helper()
	require.Equal(t, status, res.Status, string(res.Raw))
	require.NotNil(t, res.Error, string(res.Raw))
	assert.Equal(t, code, res.Error.Code)
}

func hookPath(r repoView) string {
	_, rest, found := strings.Cut(r.WebhookUrl, apptest.PublicOrigin)
	if !found {
		return r.WebhookUrl
	}
	return rest
}

func mac(key string, parts ...string) string {
	h := hmac.New(sha256.New, []byte(key))
	for _, p := range parts {
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func giteaPush(ref, after string) []byte {
	return []byte(`{"ref":"` + ref + `","before":"` + strings.Repeat("1", 40) + `","after":"` + after +
		`","pusher":{"login":"sara","email":"sara@example.com"}}`)
}

func giteaHeaders(secret, delivery string, body []byte) map[string]string {
	return map[string]string{"X-Gitea-Signature": mac(secret, string(body)), "X-Gitea-Delivery": delivery, "X-Gitea-Event": "push"}
}

func dhowHeaders(secret, delivery string, unix int64, body []byte) map[string]string {
	t := strconv.FormatInt(unix, 10)
	return map[string]string{"X-Dhow-Signature": "t=" + t + ",v1=" + mac(secret, t, ".", string(body)), "X-Dhow-Delivery": delivery}
}

type sshServer struct {
	port        string
	fingerprint string
	authorized  string
}

func startSSH(t *testing.T) sshServer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				_, _, _, _ = ssh.NewServerConn(conn, cfg)
			}()
		}
	}()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return sshServer{
		port: port, fingerprint: ssh.FingerprintSHA256(signer.PublicKey()),
		authorized: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))),
	}
}

func (w world) deliveries(t *testing.T) int {
	t.Helper()
	return w.env.CountRows(`SELECT count(*) FROM "WebhookDeliveries"`)
}
