package e2e_test

import (
	"context"
	"crypto/ed25519"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	"github.com/Khaymah/dhow/internal/runnersig"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func challenge(t *testing.T, rc *apptest.RunnerClient, runnerID string) *runnerv1.IssuedChallenge {
	t.Helper()
	res, err := rc.Auth.Challenge(context.Background(), connect.NewRequest(&runnerv1.ChallengeRequest{Runner: &runnerv1.RunnerRef{RunnerId: runnerID}}))
	require.NoError(t, err)
	return res.Msg.GetChallenge()
}

func authenticate(rc *apptest.RunnerClient, runnerID string, c *runnerv1.IssuedChallenge, sig []byte) error {
	_, err := rc.Auth.Authenticate(context.Background(), connect.NewRequest(&runnerv1.AuthenticateRequest{
		Runner: &runnerv1.RunnerRef{RunnerId: runnerID}, Challenge: c, Signature: sig,
	}))
	return err
}

func TestAuthenticateRefusesEveryForgery(t *testing.T) {
	w := newWorld(t)
	id, reg := w.registerOK(t, apptest.RegistrationTokenSpec{})
	otherID, other := w.registerOK(t, apptest.RegistrationTokenSpec{})
	rc := w.env.RunnerClient(apptest.RunnerClientOptions{})
	runnerID := reg.GetRunnerId()

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{"signature by another key", func(t *testing.T) error {
			c := challenge(t, rc, runnerID)
			return authenticate(rc, runnerID, c, otherID.Sign(c))
		}},
		{"no signature", func(t *testing.T) error { return authenticate(rc, runnerID, challenge(t, rc, runnerID), nil) }},
		{"signature over another origin", func(t *testing.T) error {
			c := challenge(t, rc, runnerID)
			return authenticate(rc, runnerID, c, ed25519.Sign(id.Signing, runnersig.AuthMessage("https://evil.example", 1, c.GetChallengeBytes())))
		}},
		{"signature over another server key version", func(t *testing.T) error {
			c := challenge(t, rc, runnerID)
			return authenticate(rc, runnerID, c, ed25519.Sign(id.Signing, runnersig.AuthMessage(apptest.PublicOrigin, 2, c.GetChallengeBytes())))
		}},
		{"tampered MAC", func(t *testing.T) error {
			c := challenge(t, rc, runnerID)
			c.Mac[0] ^= 1
			return authenticate(rc, runnerID, c, id.Sign(c))
		}},
		{"forged challenge with a far expiry", func(t *testing.T) error {
			c := challenge(t, rc, runnerID)
			var inner runnerv1.AuthChallenge
			require.NoError(t, proto.Unmarshal(c.GetChallengeBytes(), &inner))
			inner.ExpiresAt = timestamppb.New(time.Now().Add(time.Hour))
			c.ChallengeBytes, _ = proto.Marshal(&inner)
			return authenticate(rc, runnerID, c, id.Sign(c))
		}},
		{"challenge issued for another runner", func(t *testing.T) error {
			c := challenge(t, rc, other.GetRunnerId())
			return authenticate(rc, runnerID, c, id.Sign(c))
		}},
		{"unknown runner", func(t *testing.T) error {
			ghost := "00000000-0000-7000-8000-00000000abcd"
			c := challenge(t, rc, ghost)
			return authenticate(rc, ghost, c, id.Sign(c))
		}},
		{"replayed challenge and signature", func(t *testing.T) error {
			c := challenge(t, rc, runnerID)
			require.NoError(t, authenticate(rc, runnerID, c, id.Sign(c)))
			return authenticate(rc, runnerID, c, id.Sign(c))
		}},
		{"challenge older than 60 seconds", func(t *testing.T) error {
			c := challenge(t, rc, runnerID)
			w.env.Clock.Advance(61 * time.Second)
			return authenticate(rc, runnerID, c, id.Sign(c))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireRPCError(t, tt.run(t), connect.CodeUnauthenticated, "INVALID_RUNNER_CREDENTIALS")
		})
	}
	_, err := w.env.Authenticate(rc, id, runnerID)
	assert.NoError(t, err, "the genuine runner still signs in")
}

func TestChallengeNeedsARunnerID(t *testing.T) {
	w := newWorld(t)
	_, err := w.env.RunnerClient(apptest.RunnerClientOptions{}).Auth.Challenge(context.Background(), connect.NewRequest(&runnerv1.ChallengeRequest{}))
	requireRPCError(t, err, connect.CodeInvalidArgument, "VALIDATION_FAILED: Runner")
}

func rotateRequest(id *apptest.RunnerIdentity, runnerID string, version int32, next *apptest.RunnerIdentity) *runnerv1.RotateKeysRequest {
	sk, ek := next.SigningPublicKey(), next.EncryptionPublicKey()
	msg := runnersig.RotateMessage(apptest.PublicOrigin, runnerID, version, sk, ek)
	return &runnerv1.RotateKeysRequest{
		NewSigningPublicKey: sk, NewEncryptionPublicKey: ek,
		OldKeySignature: ed25519.Sign(id.Signing, msg), NewKeySignature: ed25519.Sign(next.Signing, msg),
	}
}

func TestRevocationStopsTheVeryNextCall(t *testing.T) {
	w := newWorld(t)
	id, reg := w.registerOK(t, apptest.RegistrationTokenSpec{})
	access, err := w.env.Authenticate(w.env.RunnerClient(apptest.RunnerClientOptions{}), id, reg.GetRunnerId())
	require.NoError(t, err)
	session := w.env.RunnerClient(apptest.RunnerClientOptions{AccessToken: access})

	res := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/runners/" + reg.GetRunnerId() + "/revoke")})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	_, err = session.Auth.RotateKeys(context.Background(), connect.NewRequest(rotateRequest(id, reg.GetRunnerId(), 1, apptest.NewRunnerIdentity(t))))
	requireRPCError(t, err, connect.CodePermissionDenied, "RUNNER_REVOKED")
	_, err = w.env.Authenticate(w.env.RunnerClient(apptest.RunnerClientOptions{}), id, reg.GetRunnerId())
	requireRPCError(t, err, connect.CodePermissionDenied, "RUNNER_REVOKED")

	again := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/runners/" + reg.GetRunnerId() + "/revoke")})
	assert.Equal(t, http.StatusOK, again.Status, "revoking twice is idempotent")
	patch := w.owner.Do(apptest.Request{Method: http.MethodPatch, Path: w.path("/runners/" + reg.GetRunnerId()), Body: map[string]any{"Disabled": false}})
	assert.Equal(t, http.StatusConflict, patch.Status)
	require.NotNil(t, patch.Error)
	assert.Equal(t, "RUNNER_REVOKED", patch.Error.Code)
}

func TestStaleAccessTokensAreRefused(t *testing.T) {
	w := newWorld(t)
	id, reg := w.registerOK(t, apptest.RegistrationTokenSpec{})
	rc := w.env.RunnerClient(apptest.RunnerClientOptions{})
	first, err := w.env.Authenticate(rc, id, reg.GetRunnerId())
	require.NoError(t, err)
	second, err := w.env.Authenticate(rc, id, reg.GetRunnerId())
	require.NoError(t, err)
	rotate := func(token string) error {
		_, err := w.env.RunnerClient(apptest.RunnerClientOptions{AccessToken: token}).Auth.RotateKeys(context.Background(),
			connect.NewRequest(&runnerv1.RotateKeysRequest{}))
		return err
	}
	requireRPCError(t, rotate(first), connect.CodeUnauthenticated, "UNAUTHENTICATED")
	requireRPCError(t, rotate(second), connect.CodeInvalidArgument, "VALIDATION_FAILED: NewSigningPublicKey")

	w.env.Clock.Advance(15*time.Minute + time.Second)
	requireRPCError(t, rotate(second), connect.CodeUnauthenticated, "UNAUTHENTICATED")
}

func TestRotateKeys(t *testing.T) {
	w := newWorld(t)
	id, reg := w.registerOK(t, apptest.RegistrationTokenSpec{})
	rc := w.env.RunnerClient(apptest.RunnerClientOptions{})
	access, err := w.env.Authenticate(rc, id, reg.GetRunnerId())
	require.NoError(t, err)
	session := w.env.RunnerClient(apptest.RunnerClientOptions{AccessToken: access})
	next := apptest.NewRunnerIdentity(t)

	bad := rotateRequest(id, reg.GetRunnerId(), 1, next)
	bad.NewKeySignature = ed25519.Sign(id.Signing, []byte("not the message"))
	_, err = session.Auth.RotateKeys(context.Background(), connect.NewRequest(bad))
	requireRPCError(t, err, connect.CodeUnauthenticated, "INVALID_RUNNER_CREDENTIALS")
	stolen := rotateRequest(apptest.NewRunnerIdentity(t), reg.GetRunnerId(), 1, next)
	_, err = session.Auth.RotateKeys(context.Background(), connect.NewRequest(stolen))
	requireRPCError(t, err, connect.CodeUnauthenticated, "INVALID_RUNNER_CREDENTIALS")

	res, err := session.Auth.RotateKeys(context.Background(), connect.NewRequest(rotateRequest(id, reg.GetRunnerId(), 1, next)))
	require.NoError(t, err)
	assert.Equal(t, int32(2), res.Msg.GetKeyVersion())

	_, err = session.Auth.RotateKeys(context.Background(), connect.NewRequest(rotateRequest(next, reg.GetRunnerId(), 2, apptest.NewRunnerIdentity(t))))
	requireRPCError(t, err, connect.CodeUnauthenticated, "UNAUTHENTICATED")
	_, err = w.env.Authenticate(rc, id, reg.GetRunnerId())
	requireRPCError(t, err, connect.CodeUnauthenticated, "INVALID_RUNNER_CREDENTIALS")
	_, err = w.env.Authenticate(rc, next, reg.GetRunnerId())
	require.NoError(t, err)
	_, runner := w.runnerStatus(t, reg.GetRunnerId())
	assert.InDelta(t, 2, runner["KeyVersion"], 0)
}

func TestAManuallyDisabledRunnerStillSignsIn(t *testing.T) {
	w := newWorld(t)
	id, reg := w.registerOK(t, apptest.RegistrationTokenSpec{})
	res := w.owner.Do(apptest.Request{Method: http.MethodPatch, Path: w.path("/runners/" + reg.GetRunnerId()), Body: map[string]any{"Disabled": true}})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var runner map[string]any
	res.Decode(t, &runner)
	assert.Equal(t, "disabled", runner["State"])
	assert.Equal(t, "manual", runner["DisabledReason"])
	_, err := w.env.Authenticate(w.env.RunnerClient(apptest.RunnerClientOptions{}), id, reg.GetRunnerId())
	assert.NoError(t, err, "a disabled runner keeps its session to finish running jobs")

	res = w.owner.Do(apptest.Request{Method: http.MethodPatch, Path: w.path("/runners/" + reg.GetRunnerId()), Body: map[string]any{"Disabled": false}})
	require.Equal(t, http.StatusOK, res.Status)
	runner = nil
	res.Decode(t, &runner)
	assert.Equal(t, "active", runner["State"])
	assert.Nil(t, runner["DisabledReason"])
	empty := w.owner.Do(apptest.Request{Method: http.MethodPatch, Path: w.path("/runners/" + reg.GetRunnerId()), Body: map[string]any{}})
	assert.Equal(t, http.StatusUnprocessableEntity, empty.Status)
}
