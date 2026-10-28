package e2e_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	"github.com/Arab-OpenSource/dhow/internal/jobspec"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func maxSizeSteps(t *testing.T) []*runnerv1.Step {
	t.Helper()
	const chunk = 60_000
	steps := []*runnerv1.Step{}
	size := func() int {
		return proto.Size(jobspec.Normalize(&runnerv1.JobSpec{Image: apptest.TestImage, Steps: steps}))
	}
	for size()+chunk+16 < jobspec.MaxSpecBytes {
		steps = append(steps, &runnerv1.Step{Name: "fill", Run: strings.Repeat("x", chunk)})
	}
	last := &runnerv1.Step{Name: "last", Run: "x"}
	steps = append(steps, last)
	for range 8 {
		missing := jobspec.MaxSpecBytes - size()
		if missing == 0 {
			break
		}
		last.Run = strings.Repeat("x", len(last.Run)+missing)
	}
	require.Equal(t, jobspec.MaxSpecBytes, size())
	return steps
}

func restSteps(steps []*runnerv1.Step) []map[string]any {
	out := make([]map[string]any, 0, len(steps))
	for _, s := range steps {
		out = append(out, map[string]any{"Name": s.GetName(), "Run": s.GetRun()})
	}
	return out
}

func TestTheLargestSpecStillFitsInTheGrant(t *testing.T) {
	assert.Equal(t, config.RunnerReadMaxBytes, jobspec.MaxGrantBytes, "the spec limit is derived from the runner plane read limit")
	w := newWorld(t)
	steps := maxSizeSteps(t)
	w.job(t, apptest.SimpleSpec(restSteps(steps)...), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	assert.LessOrEqual(t, proto.Size(&runnerv1.LeaseResponse{Grant: g.Signed}), config.RunnerReadMaxBytes)

	steps[len(steps)-1].Run += "x"
	res := w.env.CreateJob(w.owner, w.org, "too big", apptest.SimpleSpec(restSteps(steps)...), nil)
	require.Equal(t, http.StatusUnprocessableEntity, res.Status, string(res.Raw))
	assert.Equal(t, "Spec.Spec", res.Error.Details[0].Field)
}
