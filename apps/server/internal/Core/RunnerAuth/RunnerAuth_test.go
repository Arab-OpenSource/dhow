package runnerauth_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	runnerauth "github.com/Khaymah/dhow/apps/server/internal/Core/RunnerAuth"
)

func secret(b byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = b + byte(i)
	}
	return out
}

func TestKeysAreDeterministicAndSeparated(t *testing.T) {
	a, err := runnerauth.DeriveKeys(secret(1))
	require.NoError(t, err)
	again, err := runnerauth.DeriveKeys(secret(1))
	require.NoError(t, err)
	other, err := runnerauth.DeriveKeys(secret(2))
	require.NoError(t, err)

	assert.Equal(t, a.SigningPublicKey(), again.SigningPublicKey())
	assert.NotEqual(t, a.SigningPublicKey(), other.SigningPublicKey())
	assert.Len(t, a.SigningPublicKey(), ed25519.PublicKeySize)
	msg := []byte("grant")
	assert.True(t, ed25519.Verify(a.SigningPublicKey(), msg, a.Sign(msg)))
	assert.False(t, ed25519.Verify(other.SigningPublicKey(), msg, a.Sign(msg)))

	raw := []byte("token bytes")
	assert.Equal(t, a.RegistrationTokenHash(raw), again.RegistrationTokenHash(raw))
	assert.NotEqual(t, a.RegistrationTokenHash(raw), other.RegistrationTokenHash(raw))
	assert.NotEqual(t, a.RegistrationTokenHash(raw), a.ChallengeMAC(raw), "token and challenge keys differ")

	mac := a.ChallengeMAC(raw)
	assert.True(t, a.VerifyChallengeMAC(raw, mac))
	assert.False(t, a.VerifyChallengeMAC(append(raw, 0), mac))
	assert.False(t, a.VerifyChallengeMAC(raw, mac[:31]))
	assert.False(t, other.VerifyChallengeMAC(raw, mac))
	assert.Equal(t, int32(1), a.SigningKeyVersion())

	_, err = runnerauth.DeriveKeys(make([]byte, 16))
	assert.Error(t, err)
}

func TestTokens(t *testing.T) {
	reg, raw, err := runnerauth.NewRegistrationToken()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(reg, config.RunnerRegistrationTokenPrefix))
	parsed, ok := runnerauth.ParseRegistrationToken(reg)
	assert.True(t, ok)
	assert.Equal(t, raw, parsed)

	access, hash, err := runnerauth.NewAccessToken()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(access, config.RunnerAccessTokenPrefix))
	got, ok := runnerauth.HashAccessToken(access)
	assert.True(t, ok)
	assert.Equal(t, hash, got)

	for _, bad := range []string{"", reg[:len(reg)-1], strings.Replace(reg, "rrt", "rat", 1), reg + "A", "dhow_rrt_!!!!"} {
		_, ok := runnerauth.ParseRegistrationToken(bad)
		assert.False(t, ok, bad)
	}
	for _, spelling := range []string{reg[:15] + "\n" + reg[15:], reg[:15] + "\r" + reg[15:], reg + "\n"} {
		_, ok := runnerauth.ParseRegistrationToken(spelling)
		assert.False(t, ok, "a line break is not another spelling of the token: %q", spelling)
	}
	_, ok = runnerauth.HashAccessToken(access[:15] + "\n" + access[15:])
	assert.False(t, ok, "the same holds for access tokens")
	_, ok = runnerauth.HashAccessToken(reg)
	assert.False(t, ok, "a registration token is not an access token")
	_, ok = runnerauth.ParseRegistrationToken(access)
	assert.False(t, ok, "an access token is not a registration token")
}

func TestBearerToken(t *testing.T) {
	tests := map[string]string{"Bearer abc": "abc", "bearer abc": "abc", "Basic abc": "", "Bearer": "", "": "", "Bearer  abc ": "abc"}
	for in, want := range tests {
		h := http.Header{}
		h.Set("Authorization", in)
		assert.Equal(t, want, runnerauth.BearerToken(h), in)
	}
}

type fakeVerifier struct {
	p   runnerauth.Principal
	err error
	got []byte
}

func (f *fakeVerifier) Resolve(_ context.Context, hash []byte, _ time.Time) (runnerauth.Principal, error) {
	f.got = hash
	return f.p, f.err
}

func gateCheck(t *testing.T, v runnerauth.Verifier, procedure string, header http.Header) (context.Context, error) {
	t.Helper()
	table := policy.NewTable(
		policy.Runner("/svc/Register", policy.RunnerRegistration),
		policy.Runner("/svc/Session", policy.RunnerSession),
		policy.Organization(http.MethodPost, "/svc/Wrong", policy.OrgRead, "id"),
	)
	g := runnerauth.NewGate(runnerauth.GateOptions{Verifier: v, Log: slog.New(slog.DiscardHandler)})
	return runnerauth.CheckForTest(context.Background(), g, table, procedure, header)
}

func header(kv ...string) http.Header {
	h := http.Header{}
	h.Set(config.RunnerProtocolHeader, config.RunnerProtocolVersion)
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] == "" {
			h.Del(kv[i])
			continue
		}
		h.Set(kv[i], kv[i+1])
	}
	return h
}

func connectMessage(t *testing.T, err error) string {
	t.Helper()
	var ce *connect.Error
	require.True(t, errors.As(err, &ce), "%v", err)
	return ce.Message()
}

func TestGate(t *testing.T) {
	token, hash, err := runnerauth.NewAccessToken()
	require.NoError(t, err)
	runner := uuid.New()
	tests := []struct {
		name      string
		verifier  *fakeVerifier
		procedure string
		header    http.Header
		want      string
	}{
		{"unknown procedure", &fakeVerifier{}, "/svc/Missing", header(), "PERMISSION_DENIED"},
		{"rule from another scope", &fakeVerifier{}, "/svc/Wrong", header(), "PERMISSION_DENIED"},
		{"missing protocol", &fakeVerifier{}, "/svc/Register", header(config.RunnerProtocolHeader, ""), "RUNNER_UPGRADE_REQUIRED"},
		{"newer protocol", &fakeVerifier{}, "/svc/Register", header(config.RunnerProtocolHeader, "2"), "RUNNER_UPGRADE_REQUIRED"},
		{"browser session", &fakeVerifier{}, "/svc/Register", header("Cookie", config.SessionCookieName+"=x"), "UNAUTHENTICATED"},
		{"mfa cookie", &fakeVerifier{}, "/svc/Session", header("Cookie", config.MfaCookieName+"=x", "Authorization", "Bearer "+token), "UNAUTHENTICATED"},
		{"registration needs no token", &fakeVerifier{}, "/svc/Register", header(), ""},
		{"session without token", &fakeVerifier{}, "/svc/Session", header(), "UNAUTHENTICATED"},
		{"session with a registration token", &fakeVerifier{}, "/svc/Session", header("Authorization", "Bearer dhow_rrt_"+token[9:]), "UNAUTHENTICATED"},
		{"stale session", &fakeVerifier{err: runnerauth.ErrNoSession}, "/svc/Session", header("Authorization", "Bearer "+token), "UNAUTHENTICATED"},
		{"revoked runner", &fakeVerifier{err: runnerauth.ErrRevoked}, "/svc/Session", header("Authorization", "Bearer "+token), "RUNNER_REVOKED"},
		{"database failure", &fakeVerifier{err: errors.New("db down")}, "/svc/Session", header("Authorization", "Bearer "+token), "INTERNAL"},
		{"valid session", &fakeVerifier{p: runnerauth.Principal{RunnerID: runner}}, "/svc/Session", header("Authorization", "Bearer "+token), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := gateCheck(t, tt.verifier, tt.procedure, tt.header)
			if tt.want != "" {
				assert.Equal(t, tt.want, connectMessage(t, err))
				_, ok := runnerauth.PrincipalFrom(ctx)
				assert.False(t, ok)
				return
			}
			require.NoError(t, err)
			if tt.procedure == "/svc/Session" {
				p, ok := runnerauth.PrincipalFrom(ctx)
				require.True(t, ok)
				assert.Equal(t, runner, p.RunnerID)
				assert.True(t, bytes.Equal(hash, tt.verifier.got), "the gate resolves the SHA-256 of the token")
			}
		})
	}
}

func TestErrorsCarryStableCodes(t *testing.T) {
	tests := []struct {
		err  *connect.Error
		code connect.Code
		msg  string
	}{
		{runnerauth.InvalidRegistrationToken(0), connect.CodeUnauthenticated, "INVALID_REGISTRATION_TOKEN"},
		{runnerauth.InvalidRunnerCredentials(0), connect.CodeUnauthenticated, "INVALID_RUNNER_CREDENTIALS"},
		{runnerauth.RunnerRevoked(), connect.CodePermissionDenied, "RUNNER_REVOKED"},
		{runnerauth.RunnerDisabled(), connect.CodePermissionDenied, "RUNNER_DISABLED"},
		{runnerauth.UpgradeRequired(), connect.CodeFailedPrecondition, "RUNNER_UPGRADE_REQUIRED"},
		{runnerauth.ValidationFailed("Name"), connect.CodeInvalidArgument, "VALIDATION_FAILED: Name"},
		{runnerauth.KeyInUse(), connect.CodeAlreadyExists, "RUNNER_KEY_IN_USE"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.code, tt.err.Code(), tt.msg)
		assert.Equal(t, tt.msg, tt.err.Message())
	}
	assert.Equal(t, "31", runnerauth.InvalidRegistrationToken(30500*time.Millisecond).Meta().Get(runnerauth.RetryAfterHeader))
	assert.Empty(t, runnerauth.InvalidRegistrationToken(0).Meta().Get(runnerauth.RetryAfterHeader))
}

func TestInterceptorHidesInternalErrors(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	call := func(err error) error {
		next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) { return nil, err })
		_, out := runnerauth.Interceptor(log).WrapUnary(next)(context.Background(), connect.NewRequest(&struct{}{}))
		return out
	}
	out := call(errors.New("pq: relation \"Runners\" secret detail"))
	assert.Equal(t, "INTERNAL", connectMessage(t, out))
	assert.Contains(t, logs.String(), "secret detail", "the cause is logged for operators")
	assert.Equal(t, "RUNNER_REVOKED", connectMessage(t, call(runnerauth.RunnerRevoked())))
	assert.Equal(t, connect.CodeCanceled, connect.CodeOf(call(context.Canceled)))
}
