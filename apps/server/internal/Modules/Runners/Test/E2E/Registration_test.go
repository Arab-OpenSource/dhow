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

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	"github.com/Arab-OpenSource/dhow/internal/runnersig"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func TestRegisterThenAuthenticate(t *testing.T) {
	w := newWorld(t)
	token, tokenID := w.env.CreateRegistrationToken(w.owner, w.org, apptest.RegistrationTokenSpec{Labels: []string{"env=staging", "gpu"}})
	id, reg := w.env.RegisterRunner(token)

	assert.Equal(t, w.org.String(), reg.GetOrganizationId())
	assert.Equal(t, []string{"env=staging", "gpu", "dhow.os=linux", "dhow.arch=amd64", "dhow.isolation=container"}, reg.GetLabels())
	assert.Len(t, reg.GetServerSigningPublicKey(), ed25519.PublicKeySize)
	assert.Equal(t, int32(1), reg.GetServerKeyVersion())

	access, err := w.env.Authenticate(w.env.RunnerClient(apptest.RunnerClientOptions{}), id, reg.GetRunnerId())
	require.NoError(t, err)
	assert.Regexp(t, `^dhow_rat_[A-Za-z0-9_-]{43}$`, access)

	status, runner := w.runnerStatus(t, reg.GetRunnerId())
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "online", runner["Status"])
	assert.Equal(t, "active", runner["State"])
	assert.Equal(t, "runner-1", runner["Name"])
	assert.Equal(t, []any{"env=staging", "gpu"}, runner["Labels"])

	res := w.owner.Get(w.path("/runner-registration-tokens"))
	require.Equal(t, http.StatusOK, res.Status)
	var tokens []map[string]any
	res.Decode(t, &tokens)
	require.Len(t, tokens, 1)
	assert.Equal(t, tokenID, tokens[0]["Id"])
	assert.Equal(t, "used_up", tokens[0]["Status"])
	assert.InDelta(t, 1, tokens[0]["Uses"], 0)
}

func TestARunnerGoesOfflineAfterTwoMinutes(t *testing.T) {
	w := newWorld(t)
	_, reg := w.registerOK(t, apptest.RegistrationTokenSpec{})
	w.env.Clock.Advance(2*time.Minute + time.Second)
	_, runner := w.runnerStatus(t, reg.GetRunnerId())
	assert.Equal(t, "offline", runner["Status"])
}

func TestTheRunnerCannotChooseItsLabels(t *testing.T) {
	w := newWorld(t)
	token, _ := w.env.CreateRegistrationToken(w.owner, w.org, apptest.RegistrationTokenSpec{Labels: []string{"pool=a"}})
	id := apptest.NewRunnerIdentity(t)
	req := id.RegisterRequest(token)
	req.Os, req.Arch = "linux", "arm64"
	req.IsolationLevels = []runnerv1.Isolation{runnerv1.Isolation_ISOLATION_GVISOR, runnerv1.Isolation_ISOLATION_CONTAINER}
	res, err := w.env.RunnerClient(apptest.RunnerClientOptions{}).Registration.Register(context.Background(), connect.NewRequest(req))
	require.NoError(t, err)
	assert.Equal(t, []string{"pool=a", "dhow.os=linux", "dhow.arch=arm64", "dhow.isolation=container", "dhow.isolation=gvisor"}, res.Msg.GetLabels())

	for _, labels := range [][]string{{"dhow.os=windows"}, {"Env=Prod"}, {"a b"}, {""}, {"x="}} {
		r := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/runner-registration-tokens"), Body: map[string]any{"Labels": labels}})
		assert.Equal(t, http.StatusUnprocessableEntity, r.Status, "%v: %s", labels, r.Raw)
	}
	r := w.owner.Do(apptest.Request{Method: http.MethodPatch, Path: w.path("/runners/" + res.Msg.GetRunnerId()), Body: map[string]any{"Labels": []string{"dhow.isolation=gvisor"}}})
	assert.Equal(t, http.StatusUnprocessableEntity, r.Status, string(r.Raw))
}

func TestRegistrationValidatesTheRequestBeforeConsumingTheToken(t *testing.T) {
	w := newWorld(t)
	token, _ := w.env.CreateRegistrationToken(w.owner, w.org, apptest.RegistrationTokenSpec{})
	other := apptest.NewRunnerIdentity(t)
	tests := []struct {
		name   string
		mutate func(*runnerv1.RegisterRequest)
		field  string
	}{
		{"empty name", func(r *runnerv1.RegisterRequest) { r.Name = "" }, "Name"},
		{"control characters in the name", func(r *runnerv1.RegisterRequest) { r.Name = "a\nb" }, "Name"},
		{"unsupported os", func(r *runnerv1.RegisterRequest) { r.Os = "windows" }, "Os"},
		{"unsupported arch", func(r *runnerv1.RegisterRequest) { r.Arch = "mips" }, "Arch"},
		{"bad version", func(r *runnerv1.RegisterRequest) { r.Version = "1 2" }, "Version"},
		{"zero capacity", func(r *runnerv1.RegisterRequest) { r.Capacity = 0 }, "Capacity"},
		{"too much capacity", func(r *runnerv1.RegisterRequest) { r.Capacity = 17 }, "Capacity"},
		{"no isolation", func(r *runnerv1.RegisterRequest) { r.IsolationLevels = nil }, "IsolationLevels"},
		{"unspecified isolation", func(r *runnerv1.RegisterRequest) {
			r.IsolationLevels = []runnerv1.Isolation{runnerv1.Isolation_ISOLATION_UNSPECIFIED}
		}, "IsolationLevels"},
		{"duplicate isolation", func(r *runnerv1.RegisterRequest) {
			r.IsolationLevels = []runnerv1.Isolation{runnerv1.Isolation_ISOLATION_CONTAINER, runnerv1.Isolation_ISOLATION_CONTAINER}
		}, "IsolationLevels"},
		{"short signing key", func(r *runnerv1.RegisterRequest) { r.SigningPublicKey = r.SigningPublicKey[:31] }, "SigningPublicKey"},
		{"wrong encryption key", func(r *runnerv1.RegisterRequest) { r.EncryptionPublicKey = make([]byte, 32) }, "EncryptionPublicKey"},
		{"no proof", func(r *runnerv1.RegisterRequest) { r.Proof = nil }, "Proof"},
		{"proof for another server", func(r *runnerv1.RegisterRequest) {
			r.Proof = ed25519.Sign(other.Signing, runnersig.RegisterMessage("https://other.example", r.SigningPublicKey, r.EncryptionPublicKey))
		}, "Proof"},
		{"someone else's key", func(r *runnerv1.RegisterRequest) {
			r.SigningPublicKey = other.SigningPublicKey()
		}, "Proof"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := apptest.NewRunnerIdentity(t).RegisterRequest(token)
			tt.mutate(req)
			_, err := w.env.RunnerClient(apptest.RunnerClientOptions{Remote: "198.51.100.99:1"}).Registration.Register(context.Background(), connect.NewRequest(req))
			requireRPCError(t, err, connect.CodeInvalidArgument, "VALIDATION_FAILED: "+tt.field)
		})
	}
	_, reg := w.env.RegisterRunner(token)
	assert.NotEmpty(t, reg.GetRunnerId(), "the token is still unused after every rejected request")
}

func TestAKeyIsRegisteredOnceAndAFailedInsertKeepsTheTokenUse(t *testing.T) {
	w := newWorld(t)
	token, _ := w.env.CreateRegistrationToken(w.owner, w.org, apptest.RegistrationTokenSpec{MaxUses: 2})
	id, _ := w.env.RegisterRunner(token)
	_, err := w.register(token, id)
	requireRPCError(t, err, connect.CodeAlreadyExists, "RUNNER_KEY_IN_USE")
	_, reg := w.env.RegisterRunner(token)
	assert.NotEmpty(t, reg.GetRunnerId(), "the second use is still available")
}

func TestRegistrationIsAudited(t *testing.T) {
	w := newWorld(t)
	_, reg := w.registerOK(t, apptest.RegistrationTokenSpec{Labels: []string{"pool=b"}})
	res := w.owner.Do(apptest.Request{Method: http.MethodPatch, Path: w.path("/runners/" + reg.GetRunnerId()), Body: map[string]any{"Name": "renamed", "Disabled": true}})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	res = w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/runners/" + reg.GetRunnerId() + "/revoke")})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	events := w.owner.Get(w.path("/audit-events?Limit=100"))
	require.Equal(t, http.StatusOK, events.Status)
	var list []struct {
		Action, TargetType, TargetId string
		ActorUserId                  *string
		Data                         map[string]string
	}
	events.Decode(t, &list)
	byAction := map[string]int{}
	for _, e := range list {
		byAction[e.Action]++
		if e.Action == "runner.registered" {
			assert.Equal(t, reg.GetRunnerId(), e.TargetId)
			assert.Nil(t, e.ActorUserId, "the runner registers itself")
			assert.Equal(t, "pool=b", e.Data["Labels"])
			assert.NotEmpty(t, e.Data["RegisteredWith"])
		}
	}
	for _, a := range []string{"runner.registration_token_created", "runner.registered", "runner.updated", "runner.revoked"} {
		assert.Equal(t, 1, byAction[a], a)
	}
}
