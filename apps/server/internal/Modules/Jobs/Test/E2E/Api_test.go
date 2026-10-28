package e2e_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func TestCreateNormalizesTheSpec(t *testing.T) {
	w := newWorld(t)
	spec := apptest.SimpleSpec()
	spec["Labels"] = []string{"linux", "gpu", "gpu"}
	res := w.env.CreateJob(w.owner, w.org, "Build the app", spec, map[string]any{"Priority": 7})
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	assert.Equal(t, "jobs.job.created", res.Message)
	var v jobView
	res.Decode(t, &v)
	assert.Equal(t, "queued", v.Job.State)
	assert.Len(t, v.Job.SpecDigest, 64)
	assert.Equal(t, []any{"gpu", "linux"}, v.Spec["Labels"])
	assert.Equal(t, "container", v.Spec["Isolation"])
	assert.EqualValues(t, 3600, v.Spec["TimeoutSeconds"])
	assert.EqualValues(t, 1000, v.Spec["Resources"].(map[string]any)["CpuMillis"])
	assert.Empty(t, v.Attempts)

	again := w.env.CreateJob(w.owner, w.org, "Build the app", spec, nil)
	var other jobView
	again.Decode(t, &other)
	assert.Equal(t, v.Job.SpecDigest, other.Job.SpecDigest, "the same spec has the same digest")
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	w := newWorld(t)
	tests := []struct {
		name  string
		job   string
		spec  apptest.JobSpec
		extra map[string]any
		field string
	}{
		{"an unpinned image", "x", apptest.JobSpec{"Image": "alpine:3", "Steps": apptest.Steps(map[string]any{"Name": "a", "Run": "true"})}, nil, "Spec.Image"},
		{"no steps", "x", apptest.JobSpec{"Image": apptest.TestImage, "Steps": []any{}}, nil, "Spec.Steps"},
		{"a reserved env name", "x", apptest.JobSpec{
			"Image": apptest.TestImage, "Env": map[string]string{"DHOW_FENCING_TOKEN": "1"},
			"Steps": apptest.Steps(map[string]any{"Name": "a", "Run": "true"}),
		}, nil, "Spec.Env.DHOW_FENCING_TOKEN"},
		{"a secret with two targets", "x", apptest.JobSpec{
			"Image": apptest.TestImage, "Steps": apptest.Steps(map[string]any{"Name": "a", "Run": "true"}),
			"Secrets": []map[string]any{{"Name": "T", "AsEnv": "T", "AsFile": "t"}},
		}, nil, "Spec.Secrets[0]"},
		{"a huge timeout", "x", apptest.JobSpec{
			"Image": apptest.TestImage, "TimeoutSeconds": 1 << 40,
			"Steps": apptest.Steps(map[string]any{"Name": "a", "Run": "true"}),
		}, nil, "Spec.TimeoutSeconds"},
		{"an empty name", "", apptest.SimpleSpec(), nil, "Name"},
		{"a long name", strings.Repeat("n", 101), apptest.SimpleSpec(), nil, "Name"},
		{"a priority above 100", "x", apptest.SimpleSpec(), map[string]any{"Priority": 101}, "Priority"},
		{"too many retries", "x", apptest.SimpleSpec(), map[string]any{"MaxInfraRetries": 6}, "MaxInfraRetries"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := w.env.CreateJob(w.owner, w.org, tt.job, tt.spec, tt.extra)
			require.Equal(t, http.StatusUnprocessableEntity, res.Status, string(res.Raw))
			require.NotNil(t, res.Error)
			assert.Equal(t, "VALIDATION_FAILED", res.Error.Code)
			fields := []string{}
			for _, d := range res.Error.Details {
				fields = append(fields, d.Field)
			}
			assert.Contains(t, fields, tt.field)
		})
	}
	res := w.owner.Get(w.path("/jobs"))
	assert.Equal(t, 0, res.Meta.Total, "nothing was created")
}

func TestASpecWithSecretsIsRefusedWithoutASecretStore(t *testing.T) {
	w := newWorld(t)
	spec := apptest.SimpleSpec()
	spec["Secrets"] = []map[string]any{{"Name": "DEPLOY_TOKEN", "AsEnv": "TOKEN"}}
	res := w.env.CreateJob(w.owner, w.org, "x", spec, nil)
	assert.Equal(t, http.StatusUnprocessableEntity, res.Status, string(res.Raw))
	require.NotNil(t, res.Error)
	assert.Equal(t, "SECRETS_UNAVAILABLE", res.Error.Code)
	var jobs int
	require.NoError(t, w.env.Pool.QueryRow(t.Context(), `SELECT count(*) FROM "Jobs"`).Scan(&jobs))
	assert.Zero(t, jobs)
}

func TestListFiltersByState(t *testing.T) {
	w := newWorld(t)
	first := w.job(t, apptest.SimpleSpec(), nil)
	w.job(t, apptest.SimpleSpec(), nil)
	res := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/jobs/" + first.String() + "/cancel")})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	all := w.owner.Get(w.path("/jobs"))
	assert.Equal(t, 2, all.Meta.Total)
	queued := w.owner.Get(w.path("/jobs?State=queued"))
	assert.Equal(t, 1, queued.Meta.Total)
	canceledRes := w.owner.Get(w.path("/jobs?State=canceled"))
	var items []struct{ Id, State, Image string }
	canceledRes.Decode(t, &items)
	require.Len(t, items, 1)
	assert.Equal(t, first.String(), items[0].Id)
	assert.Equal(t, apptest.TestImage, items[0].Image, "the list reads the image from its own column")
	bad := w.owner.Get(w.path("/jobs?State=paused"))
	assert.Equal(t, http.StatusUnprocessableEntity, bad.Status, string(bad.Raw))
}

func TestQueuedReasonShowsWhenNoRunnerCanTakeTheJob(t *testing.T) {
	w := newWorld(t)
	spec := apptest.SimpleSpec()
	spec["Labels"] = []string{"gpu"}
	id := w.job(t, spec, nil)
	assert.Equal(t, "NO_MATCHING_RUNNER", w.get(t, id).Job.QueuedReason)

	w.runner(apptest.SeedRunnerSpec{})
	assert.Equal(t, "NO_MATCHING_RUNNER", w.get(t, id).Job.QueuedReason, "a runner without the label does not count")
	w.runner(apptest.SeedRunnerSpec{Labels: []string{"gpu"}})
	assert.Empty(t, w.get(t, id).Job.QueuedReason)
}

func TestMembersOfAnotherOrganizationSeeNothing(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	other := w.env.SeedOrganization("Other", "other")
	outsider := w.env.SeedSession(w.env.SeedUser(other, "o@example.com", "a long password for o", "owner"))
	res := outsider.Get("/organizations/" + other.String() + "/jobs/" + id.String())
	assert.Equal(t, http.StatusNotFound, res.Status, "a job id of another organization is not found")
	list := outsider.Get("/organizations/" + other.String() + "/jobs")
	assert.Equal(t, 0, list.Meta.Total)
	cancel := outsider.Do(apptest.Request{Method: http.MethodPost, Path: "/organizations/" + other.String() + "/jobs/" + id.String() + "/cancel"})
	assert.Equal(t, http.StatusNotFound, cancel.Status)
	assert.Equal(t, "queued", w.get(t, id).Job.State)
}
