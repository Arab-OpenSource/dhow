package e2e_test

import (
	"context"
	"crypto/ed25519"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	"github.com/Khaymah/dhow/internal/runnersig"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func revokeSelf(session *apptest.RunnerClient, sig []byte) error {
	_, err := session.Auth.Revoke(context.Background(), connect.NewRequest(&runnerv1.RevokeRequest{Signature: sig}))
	return err
}

func TestARunnerRevokesItselfWithASignedRequest(t *testing.T) {
	w := newWorld(t)
	id, reg := w.registerOK(t, apptest.RegistrationTokenSpec{})
	other, _ := w.registerOK(t, apptest.RegistrationTokenSpec{})
	access, err := w.env.Authenticate(w.env.RunnerClient(apptest.RunnerClientOptions{}), id, reg.GetRunnerId())
	require.NoError(t, err)
	session := w.env.RunnerClient(apptest.RunnerClientOptions{AccessToken: access})
	runnerID := reg.GetRunnerId()

	requireRPCError(t, revokeSelf(session, nil), connect.CodeInvalidArgument, "VALIDATION_FAILED: Signature")
	forged := map[string][]byte{
		"another key":         ed25519.Sign(other.Signing, runnersig.RevokeMessage(apptest.PublicOrigin, runnerID, 1)),
		"another origin":      ed25519.Sign(id.Signing, runnersig.RevokeMessage("https://evil.example", runnerID, 1)),
		"another key version": ed25519.Sign(id.Signing, runnersig.RevokeMessage(apptest.PublicOrigin, runnerID, 2)),
		"a rotation message":  ed25519.Sign(id.Signing, runnersig.RotateMessage(apptest.PublicOrigin, runnerID, 1, nil, nil)),
	}
	for name, sig := range forged {
		requireRPCError(t, revokeSelf(session, sig), connect.CodeUnauthenticated, "INVALID_RUNNER_CREDENTIALS")
		_, runner := w.runnerStatus(t, runnerID)
		assert.NotEqual(t, "revoked", runner["State"], name)
	}

	require.NoError(t, revokeSelf(session, ed25519.Sign(id.Signing, runnersig.RevokeMessage(apptest.PublicOrigin, runnerID, 1))))
	_, runner := w.runnerStatus(t, runnerID)
	assert.Equal(t, "revoked", runner["State"])
	_, err = w.env.Authenticate(w.env.RunnerClient(apptest.RunnerClientOptions{}), id, runnerID)
	requireRPCError(t, err, connect.CodePermissionDenied, "RUNNER_REVOKED")
	requireRPCError(t, revokeSelf(session, ed25519.Sign(id.Signing, runnersig.RevokeMessage(apptest.PublicOrigin, runnerID, 1))),
		connect.CodePermissionDenied, "RUNNER_REVOKED")

	events := w.owner.Get(w.path("/audit-events?Limit=100"))
	var list []struct {
		Action, TargetId string
		ActorUserId      *string
		Data             map[string]string
	}
	events.Decode(t, &list)
	found := 0
	for _, e := range list {
		if e.Action == "runner.revoked" && e.TargetId == runnerID {
			found++
			assert.Nil(t, e.ActorUserId, "the runner revoked itself, so no user is named")
			assert.Equal(t, "revoked_by_runner", e.Data["Reason"])
		}
	}
	assert.Equal(t, 1, found)
}
