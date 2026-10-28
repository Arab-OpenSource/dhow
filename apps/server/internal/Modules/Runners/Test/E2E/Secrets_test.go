package e2e_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	gen "github.com/Khaymah/dhow/apps/server/internal/Modules/Runners/Gen"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func encodings(b []byte) []string {
	return []string{
		base64.StdEncoding.EncodeToString(b), base64.RawStdEncoding.EncodeToString(b),
		base64.URLEncoding.EncodeToString(b), base64.RawURLEncoding.EncodeToString(b), hex.EncodeToString(b),
	}
}

func TestNoKeyOrTokenReachesAnyResponseOrLogLine(t *testing.T) {
	w := newWorld(t)
	var bodies []string
	record := func(res apptest.Response) apptest.Response {
		bodies = append(bodies, string(res.Raw))
		for k, v := range res.Header {
			bodies = append(bodies, k+": "+strings.Join(v, ","))
		}
		return res
	}
	create := record(w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/runner-registration-tokens"), Body: map[string]any{"MaxUses": 2}}))
	require.Equal(t, http.StatusCreated, create.Status)
	var created struct {
		Token             string
		RegistrationToken struct{ Id string }
	}
	create.Decode(t, &created)
	bodies = bodies[1:]

	id := apptest.NewRunnerIdentity(t)
	reg, err := w.register(created.Token, id)
	require.NoError(t, err)
	_, err = w.register("dhow_rrt_"+created.Token[9:20], apptest.NewRunnerIdentity(t))
	require.Error(t, err)
	rc := w.env.RunnerClient(apptest.RunnerClientOptions{})
	ch, err := rc.Auth.Challenge(context.Background(), connect.NewRequest(&runnerv1.ChallengeRequest{Runner: &runnerv1.RunnerRef{RunnerId: reg.GetRunnerId()}}))
	require.NoError(t, err)
	auth, err := rc.Auth.Authenticate(context.Background(), connect.NewRequest(&runnerv1.AuthenticateRequest{
		Runner: &runnerv1.RunnerRef{RunnerId: reg.GetRunnerId()}, Challenge: ch.Msg.GetChallenge(), Signature: id.Sign(ch.Msg.GetChallenge()),
	}))
	require.NoError(t, err)
	access := auth.Msg.GetAccessToken()
	_, err = w.env.RunnerClient(apptest.RunnerClientOptions{AccessToken: access}).Auth.RotateKeys(context.Background(), connect.NewRequest(&runnerv1.RotateKeysRequest{}))
	require.Error(t, err)
	for _, m := range []proto.Message{reg} {
		raw, err := protojson.Marshal(m)
		require.NoError(t, err)
		bodies = append(bodies, string(raw))
	}

	runner := "/runners/" + reg.GetRunnerId()
	record(w.owner.Get(w.path("/runners")))
	record(w.owner.Get(w.path(runner)))
	record(w.owner.Do(apptest.Request{Method: http.MethodPatch, Path: w.path(runner), Body: map[string]any{"Name": "renamed"}}))
	record(w.owner.Get(w.path("/runner-registration-tokens")))
	record(w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/runner-registration-tokens/" + created.RegistrationToken.Id + "/revoke")}))
	record(w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path(runner + "/revoke")}))
	record(w.owner.Get(w.path("/audit-events?Limit=100")))

	rawToken := created.Token[len("dhow_rrt_"):]
	tokenBytes, err := base64.RawURLEncoding.DecodeString(rawToken)
	require.NoError(t, err)
	accessBytes, err := base64.RawURLEncoding.DecodeString(access[len("dhow_rat_"):])
	require.NoError(t, err)
	accessHash := sha256.Sum256(accessBytes)
	secrets := map[string][]string{
		"registration token": {created.Token, rawToken},
		"access token":       {access, access[len("dhow_rat_"):]},
		"token bytes":        encodings(tokenBytes),
		"access token hash":  encodings(accessHash[:]),
		"runner signing key": encodings(id.SigningPublicKey()),
		"challenge MAC":      encodings(ch.Msg.GetChallenge().GetMac()),
	}
	logs := w.env.Logs.String()
	require.NotEmpty(t, logs)
	for name, values := range secrets {
		for _, v := range values {
			assert.NotContains(t, logs, v, "%s must not appear in any log line", name)
			for _, body := range bodies {
				assert.NotContains(t, body, v, "%s must not appear in any response", name)
			}
		}
	}
	ek := encodings(id.EncryptionPublicKey())
	for _, body := range bodies {
		assert.NotContains(t, body, ek[0][:64], "the encryption key is never returned")
	}
	assert.NotContains(t, logs, "Authorization")
	assert.NotContains(t, logs, "Bearer")
}

func TestRestDTOsHaveNoKeyOrTokenField(t *testing.T) {
	allowed := map[string]bool{"CreatedRunnerRegistrationToken.Token": true}
	suspicious := []string{"key", "token", "hash", "secret", "signature", "proof", "mac", "nonce"}
	for _, v := range []any{
		gen.Runner{},
		gen.RunnerRegistrationToken{},
		gen.CreatedRunnerRegistrationToken{},
		gen.RunnerListResponse{},
		gen.RunnerResponse{},
		gen.RunnerRegistrationTokenListResponse{},
		gen.RunnerRegistrationTokenResponse{},
		gen.CreatedRunnerRegistrationTokenResponse{},
		gen.UpdateRunnerRequest{},
		gen.CreateRunnerRegistrationTokenRequest{},
	} {
		typ := reflect.TypeOf(v)
		for i := range typ.NumField() {
			f := typ.Field(i)
			name := typ.Name() + "." + f.Name
			if allowed[name] || f.Name == "KeyVersion" || f.Name == "KeysRotatedAt" || f.Name == "RegistrationToken" {
				continue
			}
			for _, s := range suspicious {
				assert.NotContains(t, strings.ToLower(f.Name), s, "%s looks like it carries a credential", name)
			}
		}
	}
}
