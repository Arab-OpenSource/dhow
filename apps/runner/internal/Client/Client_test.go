package client_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	client "github.com/Arab-OpenSource/dhow/apps/runner/internal/Client"
	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	identity "github.com/Arab-OpenSource/dhow/apps/runner/internal/Identity"
	"github.com/Arab-OpenSource/dhow/internal/runnersig"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
	"github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1/runnerv1connect"
)

const origin = "https://dhow.example.com"

type fakeAuth struct {
	runnerv1connect.UnimplementedAuthServiceHandler
	runnerv1connect.UnimplementedLeaseServiceHandler
	id         *identity.Identity
	auths      atomic.Int32
	heartbeats atomic.Int32
	mu         sync.Mutex
	valid      map[string]bool
	headers    http.Header
	revoked    bool
	rejectNext atomic.Bool
	failAuth   atomic.Bool
	blockAuth  chan struct{}
}

func (f *fakeAuth) Challenge(_ context.Context, req *connect.Request[runnerv1.ChallengeRequest]) (*connect.Response[runnerv1.ChallengeResponse], error) {
	raw, _ := proto.Marshal(&runnerv1.AuthChallenge{Nonce: []byte(uuid.NewString()), RunnerId: req.Msg.GetRunner().GetRunnerId()})
	return connect.NewResponse(&runnerv1.ChallengeResponse{Challenge: &runnerv1.IssuedChallenge{ChallengeBytes: raw}}), nil
}

func (f *fakeAuth) Authenticate(_ context.Context, req *connect.Request[runnerv1.AuthenticateRequest]) (*connect.Response[runnerv1.AuthenticateResponse], error) {
	if f.revoked {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("RUNNER_REVOKED"))
	}
	if f.blockAuth != nil {
		<-f.blockAuth
	}
	if f.failAuth.Load() {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("UNAVAILABLE"))
	}
	msg := runnersig.AuthMessage(origin, 1, req.Msg.GetChallenge().GetChallengeBytes())
	if !ed25519.Verify(f.id.SigningPublicKey(), msg, req.Msg.GetSignature()) {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("INVALID_RUNNER_CREDENTIALS"))
	}
	f.auths.Add(1)
	token := config.AccessTokenPrefix + uuid.NewString()
	f.mu.Lock()
	f.valid[token] = true
	f.mu.Unlock()
	return connect.NewResponse(&runnerv1.AuthenticateResponse{AccessToken: token, ExpiresInSeconds: 900}), nil
}

func (f *fakeAuth) Heartbeat(_ context.Context, req *connect.Request[runnerv1.HeartbeatRequest]) (*connect.Response[runnerv1.HeartbeatResponse], error) {
	f.mu.Lock()
	f.headers = req.Header().Clone()
	ok := f.valid[req.Header().Get("Authorization")[len("Bearer "):]]
	f.mu.Unlock()
	if !ok || f.rejectNext.Swap(false) {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("UNAUTHENTICATED"))
	}
	f.heartbeats.Add(1)
	return connect.NewResponse(&runnerv1.HeartbeatResponse{Control: &runnerv1.LeaseControl{LeaseTtlSeconds: 45}}), nil
}

func testIdentity(t *testing.T) *identity.Identity {
	t.Helper()
	keys, err := identity.GenerateKeys()
	require.NoError(t, err)
	server, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return &identity.Identity{
		Keys: keys, RunnerID: uuid.NewString(), OrganizationID: uuid.NewString(), ServerURL: origin,
		ServerOrigin: origin, ServerSigningKey: server, ServerKeyVersion: 1,
	}
}

func serve(t *testing.T, f *fakeAuth, extra func(*http.ServeMux)) (*httptest.Server, string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(runnerv1connect.NewAuthServiceHandler(f))
	mux.Handle(runnerv1connect.NewLeaseServiceHandler(f))
	if extra != nil {
		extra(mux)
	}
	root := http.NewServeMux()
	root.Handle(config.APIPrefix+"/", http.StripPrefix(config.APIPrefix, mux))
	srv := httptest.NewUnstartedServer(root)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	ca := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0o600))
	return srv, ca
}

func newClient(t *testing.T, srv *httptest.Server, ca string, id *identity.Identity, now func() time.Time) *client.Client {
	t.Helper()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.New(client.Options{BaseURL: u, CAFile: ca, Version: "test", Identity: id, Now: now})
	require.NoError(t, err)
	return c
}

var ref = &runnerv1.LeaseRef{LeaseId: uuid.NewString(), FencingToken: 1}

func TestTLSIsVerifiedStrictly(t *testing.T) {
	id := testIdentity(t)
	f := &fakeAuth{id: id, valid: map[string]bool{}}
	srv, ca := serve(t, f, nil)

	_, err := newClient(t, srv, "", id, nil).Heartbeat(context.Background(), ref)
	require.Error(t, err, "the system roots do not trust a self-signed server")
	assert.Contains(t, err.Error(), "certificate")
	assert.Zero(t, f.auths.Load())

	_, err = newClient(t, srv, ca, id, nil).Heartbeat(context.Background(), ref)
	require.NoError(t, err)
	assert.Equal(t, config.ProtocolVersion, f.headers.Get(config.ProtocolHeader))
	assert.Equal(t, "test", f.headers.Get(config.VersionHeader))
	assert.Contains(t, f.headers.Get("Authorization"), "Bearer "+config.AccessTokenPrefix)
}

func TestPlainHTTPNeedsDevAndLoopback(t *testing.T) {
	u, _ := url.Parse("http://127.0.0.1:1")
	_, err := client.New(client.Options{BaseURL: u})
	assert.ErrorIs(t, err, client.ErrPlainHTTP)
	_, err = client.New(client.Options{BaseURL: u, Dev: true})
	assert.NoError(t, err)
	remote, _ := url.Parse("http://dhow.example.com")
	_, err = client.New(client.Options{BaseURL: remote, Dev: true})
	assert.ErrorIs(t, err, client.ErrPlainHTTP)
}

func TestRedirectsAreNeverFollowed(t *testing.T) {
	id := testIdentity(t)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://elsewhere.example.com"+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)
	ca := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0o600))
	_, err := newClient(t, srv, ca, id, nil).Heartbeat(context.Background(), ref)
	require.Error(t, err)
	assert.ErrorIs(t, err, client.ErrRedirect)
}

func TestTheTokenIsRenewedAtTwoThirdsOfItsLife(t *testing.T) {
	id := testIdentity(t)
	f := &fakeAuth{id: id, valid: map[string]bool{}}
	srv, ca := serve(t, f, nil)
	var clock atomic.Int64
	start := time.Now()
	now := func() time.Time { return start.Add(time.Duration(clock.Load())) }
	c := newClient(t, srv, ca, id, now)
	for _, step := range []time.Duration{0, 5 * time.Minute, 9*time.Minute + 59*time.Second} {
		clock.Store(int64(step))
		_, err := c.Heartbeat(context.Background(), ref)
		require.NoError(t, err)
	}
	assert.EqualValues(t, 1, f.auths.Load(), "one sign-in covers the first two thirds")
	clock.Store(int64(10 * time.Minute))
	_, err := c.Heartbeat(context.Background(), ref)
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return f.auths.Load() == 2 }, 5*time.Second, 10*time.Millisecond,
		"the token is renewed at ten of fifteen minutes")
}

func TestARejectedTokenIsReplacedOnce(t *testing.T) {
	id := testIdentity(t)
	f := &fakeAuth{id: id, valid: map[string]bool{}}
	srv, ca := serve(t, f, nil)
	c := newClient(t, srv, ca, id, nil)
	_, err := c.Heartbeat(context.Background(), ref)
	require.NoError(t, err)
	f.rejectNext.Store(true)
	_, err = c.Heartbeat(context.Background(), ref)
	require.NoError(t, err)
	assert.EqualValues(t, 2, f.auths.Load())
	assert.EqualValues(t, 2, f.heartbeats.Load())
}

func TestARevokedRunnerStops(t *testing.T) {
	id := testIdentity(t)
	f := &fakeAuth{id: id, valid: map[string]bool{}, revoked: true}
	srv, ca := serve(t, f, nil)
	_, err := newClient(t, srv, ca, id, nil).Heartbeat(context.Background(), ref)
	assert.ErrorIs(t, err, client.ErrRevoked)
	assert.True(t, client.Fatal(err))
	assert.False(t, client.Transient(err))
}

func TestAWrongIdentityIsRefused(t *testing.T) {
	id := testIdentity(t)
	f := &fakeAuth{id: testIdentity(t), valid: map[string]bool{}}
	srv, ca := serve(t, f, nil)
	_, err := newClient(t, srv, ca, id, nil).Heartbeat(context.Background(), ref)
	assert.ErrorIs(t, err, client.ErrCredentials)
}

func TestBackoffHasJitterWithinBounds(t *testing.T) {
	b := client.Backoff{Min: time.Second, Max: 30 * time.Second}
	seen := map[time.Duration]bool{}
	for n := range 40 {
		d := b.Delay(n, nil)
		assert.GreaterOrEqual(t, d, time.Second)
		assert.LessOrEqual(t, d, min(30*time.Second, time.Second<<min(n, 10)))
		seen[d] = true
	}
	assert.Greater(t, len(seen), 10, "delays are jittered")
	e := connect.NewError(connect.CodeResourceExhausted, errors.New("LEASE_RATE_LIMITED"))
	e.Meta().Set("Retry-After", "5")
	assert.Equal(t, 5*time.Second, b.Delay(0, e))
	assert.True(t, client.Transient(e))
}

func TestErrorClassification(t *testing.T) {
	lost := connect.NewError(connect.CodeFailedPrecondition, errors.New("LEASE_LOST"))
	assert.True(t, client.IsLeaseLost(lost))
	assert.False(t, client.Transient(lost))
	assert.True(t, client.Transient(connect.NewError(connect.CodeUnavailable, errors.New("x"))))
	assert.True(t, client.Transient(errors.New("dial tcp: refused")))
	assert.False(t, client.Transient(context.Canceled))
	assert.True(t, client.Is(connect.NewError(connect.CodeFailedPrecondition, errors.New("CANCEL_REQUESTED")), client.CodeCancelRequested))
}

func TestAFailedRenewalKeepsTheValidToken(t *testing.T) {
	id := testIdentity(t)
	f := &fakeAuth{id: id, valid: map[string]bool{}}
	srv, ca := serve(t, f, nil)
	var clock atomic.Int64
	start := time.Now()
	c := newClient(t, srv, ca, id, func() time.Time { return start.Add(time.Duration(clock.Load())) })
	_, err := c.Heartbeat(context.Background(), ref)
	require.NoError(t, err)
	f.failAuth.Store(true)
	clock.Store(int64(11 * time.Minute))
	_, err = c.Heartbeat(context.Background(), ref)
	require.NoError(t, err, "the token lives 15 minutes, so a failed renewal at 11 must not fail the call")
	clock.Store(int64(14*time.Minute + 45*time.Second))
	_, err = c.Heartbeat(context.Background(), ref)
	assert.Error(t, err, "close to the real expiry the old token is no longer used")
	f.failAuth.Store(false)
	_, err = c.Heartbeat(context.Background(), ref)
	require.NoError(t, err)
}

func TestARenewalInFlightDoesNotBlockOtherCalls(t *testing.T) {
	id := testIdentity(t)
	f := &fakeAuth{id: id, valid: map[string]bool{}}
	srv, ca := serve(t, f, nil)
	var clock atomic.Int64
	start := time.Now()
	c := newClient(t, srv, ca, id, func() time.Time { return start.Add(time.Duration(clock.Load())) })
	_, err := c.Heartbeat(context.Background(), ref)
	require.NoError(t, err)
	f.blockAuth = make(chan struct{})
	clock.Store(int64(11 * time.Minute))
	leader := make(chan error, 1)
	go func() {
		_, err := c.Heartbeat(context.Background(), ref)
		leader <- err
	}()
	assert.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := c.Heartbeat(ctx, ref)
		return err == nil
	}, 5*time.Second, 50*time.Millisecond, "another call goes through with the current token while the renewal waits")
	require.NoError(t, <-leader)
	close(f.blockAuth)
	assert.Eventually(t, func() bool { return f.auths.Load() == 2 }, 5*time.Second, 10*time.Millisecond)
}
