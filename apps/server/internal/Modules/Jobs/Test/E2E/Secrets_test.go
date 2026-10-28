package e2e_test

import (
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	gen "github.com/Khaymah/dhow/apps/server/internal/Modules/Jobs/Gen"
	"github.com/Khaymah/dhow/internal/runnersig"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const (
	tokenValue  = "s3cr3t-deploy-token-value"
	kubeconfig  = "apiVersion: v1\nkind: Config\nusers: [{token: kube-0123456789}]\n"
	secretsSpec = "secrets"
)

func secretJob() apptest.JobSpec {
	spec := apptest.SimpleSpec()
	spec["Secrets"] = []map[string]any{
		{"Name": "DEPLOY_TOKEN", "AsEnv": "TOKEN", "Steps": []int{1}},
		{"Name": "KUBECONFIG", "AsFile": "kubeconfig"},
	}
	return spec
}

func openBundle(t *testing.T, r *apptest.TestRunner, g *apptest.Grant, sealed *runnerv1.SealedSecrets, aad []byte) (*runnerv1.SecretBundle, error) {
	t.Helper()
	plain, err := runnersig.OpenSecrets(r.Identity.Encryption, sealed.GetEnc(), aad, sealed.GetCiphertext())
	if err != nil {
		return nil, err
	}
	var b runnerv1.SecretBundle
	require.NoError(t, proto.Unmarshal(plain, &b))
	return &b, nil
}

func TestSecretsReachOnlyTheRunnerAndAttemptTheyWereSealedFor(t *testing.T) {
	w := newWorldWithSecrets(t, map[string]string{"DEPLOY_TOKEN": tokenValue, "KUBECONFIG": kubeconfig})
	id := w.job(t, secretJob(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	start, err := r.Start(g)
	require.NoError(t, err)
	sealed := start.GetSecrets()
	require.NotNil(t, sealed)
	assert.Equal(t, runnerv1.HpkeKem_HPKE_KEM_MLKEM768_X25519, sealed.GetKem())

	aad := runnersig.SecretsAAD(g.Ref.GetLeaseId(), g.Ref.GetFencingToken(), g.Grant.GetAttemptId(), r.RunnerID)
	bundle, err := openBundle(t, r, g, sealed, aad)
	require.NoError(t, err)
	got := map[string]string{}
	for _, s := range bundle.GetSecrets() {
		got[s.GetName()] = string(s.GetValue())
	}
	assert.Equal(t, map[string]string{"DEPLOY_TOKEN": tokenValue, "KUBECONFIG": kubeconfig}, got)

	other := w.runner(apptest.SeedRunnerSpec{})
	_, err = openBundle(t, other, g, sealed, aad)
	require.ErrorIs(t, err, runnersig.ErrSecretsOpen, "another runner key cannot open it")
	for name, wrong := range map[string][]byte{
		"another attempt":  runnersig.SecretsAAD(g.Ref.GetLeaseId(), g.Ref.GetFencingToken(), "00000000-0000-7000-8000-000000000009", r.RunnerID),
		"an older fencing": runnersig.SecretsAAD(g.Ref.GetLeaseId(), g.Ref.GetFencingToken()-1, g.Grant.GetAttemptId(), r.RunnerID),
		"another runner":   runnersig.SecretsAAD(g.Ref.GetLeaseId(), g.Ref.GetFencingToken(), g.Grant.GetAttemptId(), other.RunnerID),
	} {
		_, err = openBundle(t, r, g, sealed, wrong)
		require.ErrorIs(t, err, runnersig.ErrSecretsOpen, name)
	}
	_, err = r.Start(g)
	requireRPC(t, err, connect.CodeFailedPrecondition, "ATTEMPT_ALREADY_STARTED")
	assert.Equal(t, 1, w.env.Secrets.Resolved(), "the values are resolved once per attempt")

	bodies := []string{}
	for _, path := range []string{"/jobs", "/jobs/" + id.String(), "/audit-events?Limit=100"} {
		res := w.owner.Get(w.path(path))
		require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
		bodies = append(bodies, string(res.Raw))
	}
	grant, err := proto.Marshal(g.Grant)
	require.NoError(t, err)
	bodies = append(bodies, w.env.Logs.String(), string(grant))
	var stored string
	require.NoError(t, w.env.Pool.QueryRow(t.Context(),
		`SELECT concat_ws('|', j."Spec"::text, a::text, l::text) FROM "Jobs" j JOIN "JobAttempts" a ON a."JobId" = j."Id" JOIN "Leases" l ON l."AttemptId" = a."Id"`).Scan(&stored))
	bodies = append(bodies, stored)
	for _, value := range []string{tokenValue, "kube-0123456789"} {
		forms := []string{value, base64.StdEncoding.EncodeToString([]byte(value)), hex.EncodeToString([]byte(value))}
		for _, body := range bodies {
			for _, f := range forms {
				assert.NotContains(t, body, f, "a secret value leaked")
			}
		}
	}
	assert.Contains(t, bodies[1], "DEPLOY_TOKEN", "the reference names are not secret")
}

func TestASecretThatDisappearedFailsTheAttemptWithoutRetry(t *testing.T) {
	w := newWorldWithSecrets(t, map[string]string{"DEPLOY_TOKEN": tokenValue, "KUBECONFIG": kubeconfig})
	id := w.job(t, secretJob(), nil)
	w.env.Secrets.Forget("KUBECONFIG")
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	requireRPC(t, err, connect.CodeFailedPrecondition, "SECRETS_UNAVAILABLE")
	v := w.get(t, id)
	assert.Equal(t, "failed", v.Job.State)
	assert.Equal(t, "SECRETS_UNAVAILABLE", v.Job.FailureReason)
	assert.NotContains(t, w.env.Logs.String(), tokenValue)
	assert.Contains(t, w.env.Logs.String(), "KUBECONFIG", "the log names the missing secret, never a value")
}

func TestJobDTOsCarryNoSecretValues(t *testing.T) {
	allowed := map[string]bool{
		"JobSpec.Secrets": true, "JobDetail.Spec": true, "JobSecretRef.Name": true, "JobAttempt.FencingToken": true,
	}
	suspicious := []string{"value", "token", "key", "password", "ciphertext", "signature"}
	for _, v := range []any{
		gen.Job{},
		gen.JobDetail{},
		gen.JobAttempt{},
		gen.JobAttemptStep{},
		gen.JobSpec{},
		gen.JobStep{},
		gen.JobSecretRef{},
		gen.JobResources{},
		gen.JobNetwork{},
		gen.CreateJobRequest{},
	} {
		typ := reflect.TypeOf(v)
		for i := range typ.NumField() {
			name := typ.Name() + "." + typ.Field(i).Name
			if allowed[name] {
				continue
			}
			for _, s := range suspicious {
				assert.NotContains(t, strings.ToLower(typ.Field(i).Name), s, "%s looks like it carries a credential", name)
			}
		}
	}
	assert.NotEmpty(t, secretsSpec)
}
