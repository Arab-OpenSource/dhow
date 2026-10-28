package jobspec_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/Arab-OpenSource/dhow/internal/jobspec"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const image = "registry.example.com/team/build:1.2@sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func valid() *runnerv1.JobSpec {
	return &runnerv1.JobSpec{
		Image: image,
		Steps: []*runnerv1.Step{{Name: "build", Run: "make build", Env: map[string]string{"GOFLAGS": "-mod=mod"}}},
		Env:   map[string]string{"CI": "true"},
		Secrets: []*runnerv1.SecretRef{
			{Name: "DEPLOY_TOKEN", Target: &runnerv1.SecretRef_AsEnv{AsEnv: "TOKEN"}, StepIndexes: []int32{0}},
			{Name: "KUBECONFIG", Target: &runnerv1.SecretRef_AsFile{AsFile: "kubeconfig"}},
		},
		Network: &runnerv1.Network{Egress: []string{"proxy.golang.org", "*.github.com:443"}},
		Labels:  []string{"linux", "dhow.isolation=container"},
	}
}

func violations(t *testing.T, err error) []jobspec.Violation {
	t.Helper()
	var e *jobspec.Error
	require.True(t, errors.As(err, &e), "want *jobspec.Error, got %v", err)
	return e.Violations
}

func TestPrepareAppliesDefaults(t *testing.T) {
	out, err := jobspec.Prepare(valid())
	require.NoError(t, err)
	assert.Equal(t, runnerv1.Isolation_ISOLATION_CONTAINER, out.GetIsolation())
	assert.Equal(t, jobspec.DefaultTimeoutSeconds, out.GetTimeoutSeconds())
	assert.EqualValues(t, jobspec.DefaultCPUMillis, out.GetResources().GetCpuMillis())
	assert.EqualValues(t, jobspec.DefaultMemoryBytes, out.GetResources().GetMemoryBytes())
	assert.EqualValues(t, jobspec.DefaultPids, out.GetResources().GetPids())
	assert.EqualValues(t, jobspec.DefaultDiskBytes, out.GetResources().GetDiskBytes())
	assert.Equal(t, []string{"dhow.isolation=container", "linux"}, out.GetLabels())
	assert.Equal(t, []string{"*.github.com:443", "proxy.golang.org"}, out.GetNetwork().GetEgress())
	assert.Equal(t, []string{"DEPLOY_TOKEN", "KUBECONFIG"}, jobspec.SecretNames(out))
}

func TestPrepareDoesNotChangeItsInput(t *testing.T) {
	in := valid()
	before := proto.Clone(in)
	_, err := jobspec.Prepare(in)
	require.NoError(t, err)
	assert.True(t, proto.Equal(before, in))
	_, err = jobspec.Prepare(nil)
	assert.ErrorIs(t, err, jobspec.ErrNilSpec)
}

func TestDigestIsStableAndSensitive(t *testing.T) {
	a, err := jobspec.Prepare(valid())
	require.NoError(t, err)
	b, err := jobspec.Prepare(valid())
	require.NoError(t, err)
	da, err := jobspec.Digest(a)
	require.NoError(t, err)
	db, err := jobspec.Digest(b)
	require.NoError(t, err)
	assert.Len(t, da, 32)
	assert.Equal(t, da, db)

	reordered := valid()
	reordered.Labels = []string{"dhow.isolation=container", "linux", "linux"}
	c, err := jobspec.Prepare(reordered)
	require.NoError(t, err)
	dc, err := jobspec.Digest(c)
	require.NoError(t, err)
	assert.Equal(t, da, dc, "normalized specs share a digest")

	b.Steps[0].Run = "make test"
	db, err = jobspec.Digest(b)
	require.NoError(t, err)
	assert.NotEqual(t, da, db)
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*runnerv1.JobSpec)
		field  string
		reason string
	}{
		{"no image", func(s *runnerv1.JobSpec) { s.Image = "" }, "Image", jobspec.ReasonRequired},
		{"an image without a digest", func(s *runnerv1.JobSpec) { s.Image = "alpine:3.22" }, "Image", jobspec.ReasonNotPinned},
		{"a short digest", func(s *runnerv1.JobSpec) { s.Image = "alpine@sha256:abc" }, "Image", jobspec.ReasonInvalid},
		{"an image with spaces", func(s *runnerv1.JobSpec) { s.Image = "alp ine@sha256:" + strings.Repeat("a", 64) }, "Image", jobspec.ReasonInvalid},
		{"a long image", func(s *runnerv1.JobSpec) { s.Image = strings.Repeat("a", 600) + "@sha256:" + strings.Repeat("a", 64) }, "Image", jobspec.ReasonTooLong},
		{"no steps", func(s *runnerv1.JobSpec) { s.Steps = nil }, "Steps", jobspec.ReasonRequired},
		{"too many steps", func(s *runnerv1.JobSpec) {
			for range jobspec.MaxSteps {
				s.Steps = append(s.Steps, &runnerv1.Step{Name: "x", Run: "true"})
			}
		}, "Steps", jobspec.ReasonTooMany},
		{"a step without a name", func(s *runnerv1.JobSpec) { s.Steps[0].Name = " " }, "Steps[0].Name", jobspec.ReasonRequired},
		{"a step name with a control character", func(s *runnerv1.JobSpec) { s.Steps[0].Name = "a\x1bb" }, "Steps[0].Name", jobspec.ReasonInvalid},
		{"a long step name", func(s *runnerv1.JobSpec) { s.Steps[0].Name = strings.Repeat("n", 101) }, "Steps[0].Name", jobspec.ReasonTooLong},
		{"a step without a script", func(s *runnerv1.JobSpec) { s.Steps[0].Run = "" }, "Steps[0].Run", jobspec.ReasonRequired},
		{"a script with NUL", func(s *runnerv1.JobSpec) { s.Steps[0].Run = "echo\x00" }, "Steps[0].Run", jobspec.ReasonInvalid},
		{"a huge script", func(s *runnerv1.JobSpec) { s.Steps[0].Run = strings.Repeat("x", jobspec.MaxRunBytes+1) }, "Steps[0].Run", jobspec.ReasonTooLong},
		{"a step timeout above the job timeout", func(s *runnerv1.JobSpec) { s.TimeoutSeconds = 60; s.Steps[0].TimeoutSeconds = 61 }, "Steps[0].TimeoutSeconds", jobspec.ReasonOutOfRange},
		{"a negative step timeout", func(s *runnerv1.JobSpec) { s.Steps[0].TimeoutSeconds = -1 }, "Steps[0].TimeoutSeconds", jobspec.ReasonOutOfRange},
		{"a reserved env name", func(s *runnerv1.JobSpec) { s.Env["DHOW_FENCING_TOKEN"] = "1" }, "Env.DHOW_FENCING_TOKEN", jobspec.ReasonReserved},
		{"a reserved env name in lower case", func(s *runnerv1.JobSpec) { s.Env["dhow_x"] = "1" }, "Env.dhow_x", jobspec.ReasonReserved},
		{"an invalid env name", func(s *runnerv1.JobSpec) { s.Env["1BAD"] = "1" }, "Env.1BAD", jobspec.ReasonInvalid},
		{"an env value with NUL", func(s *runnerv1.JobSpec) { s.Env["CI"] = "a\x00" }, "Env.CI", jobspec.ReasonInvalid},
		{"a huge env value", func(s *runnerv1.JobSpec) { s.Env["CI"] = strings.Repeat("v", jobspec.MaxEnvValueBytes+1) }, "Env.CI", jobspec.ReasonTooLong},
		{"an invalid step env name", func(s *runnerv1.JobSpec) { s.Steps[0].Env["A-B"] = "1" }, "Steps[0].Env.A-B", jobspec.ReasonInvalid},
		{"a secret without a target", func(s *runnerv1.JobSpec) { s.Secrets[0].Target = nil }, "Secrets[0].AsEnv", jobspec.ReasonRequired},
		{"a secret with an invalid name", func(s *runnerv1.JobSpec) { s.Secrets[0].Name = "a b" }, "Secrets[0].Name", jobspec.ReasonInvalid},
		{"a secret env that shadows env", func(s *runnerv1.JobSpec) { s.Secrets[0].Target = &runnerv1.SecretRef_AsEnv{AsEnv: "CI"} }, "Secrets[0].AsEnv", jobspec.ReasonDuplicate},
		{"a secret env that shadows step env", func(s *runnerv1.JobSpec) {
			s.Secrets[0].Target = &runnerv1.SecretRef_AsEnv{AsEnv: "GOFLAGS"}
		}, "Secrets[0].AsEnv", jobspec.ReasonDuplicate},
		{"a reserved secret env", func(s *runnerv1.JobSpec) { s.Secrets[0].Target = &runnerv1.SecretRef_AsEnv{AsEnv: "DHOW_TOKEN"} }, "Secrets[0].AsEnv", jobspec.ReasonReserved},
		{"a secret file outside the secrets folder", func(s *runnerv1.JobSpec) {
			s.Secrets[1].Target = &runnerv1.SecretRef_AsFile{AsFile: "../etc/passwd"}
		}, "Secrets[1].AsFile", jobspec.ReasonInvalid},
		{"a secret file named dot dot", func(s *runnerv1.JobSpec) { s.Secrets[1].Target = &runnerv1.SecretRef_AsFile{AsFile: ".."} }, "Secrets[1].AsFile", jobspec.ReasonInvalid},
		{"two secrets on one target", func(s *runnerv1.JobSpec) {
			s.Secrets[1].Target = &runnerv1.SecretRef_AsEnv{AsEnv: "TOKEN"}
		}, "Secrets[1]", jobspec.ReasonDuplicate},
		{"a secret for a missing step", func(s *runnerv1.JobSpec) { s.Secrets[0].StepIndexes = []int32{3} }, "Secrets[0].Steps", jobspec.ReasonOutOfRange},
		{"too little CPU", func(s *runnerv1.JobSpec) { s.Resources = &runnerv1.Resources{CpuMillis: 10} }, "Resources.CpuMillis", jobspec.ReasonOutOfRange},
		{"too much memory", func(s *runnerv1.JobSpec) { s.Resources = &runnerv1.Resources{MemoryBytes: 1 << 40} }, "Resources.MemoryBytes", jobspec.ReasonOutOfRange},
		{"negative pids", func(s *runnerv1.JobSpec) { s.Resources = &runnerv1.Resources{Pids: -1} }, "Resources.Pids", jobspec.ReasonOutOfRange},
		{"too much disk", func(s *runnerv1.JobSpec) { s.Resources = &runnerv1.Resources{DiskBytes: 1 << 40} }, "Resources.DiskBytes", jobspec.ReasonOutOfRange},
		{"a timeout above a day", func(s *runnerv1.JobSpec) { s.TimeoutSeconds = jobspec.MaxTimeoutSeconds + 1 }, "TimeoutSeconds", jobspec.ReasonOutOfRange},
		{"a negative timeout", func(s *runnerv1.JobSpec) { s.TimeoutSeconds = -5 }, "TimeoutSeconds", jobspec.ReasonOutOfRange},
		{"an egress URL", func(s *runnerv1.JobSpec) { s.Network.Egress = []string{"https://example.com"} }, "Network.Egress[0]", jobspec.ReasonInvalid},
		{"an egress port out of range", func(s *runnerv1.JobSpec) { s.Network.Egress = []string{"example.com:70000"} }, "Network.Egress[0]", jobspec.ReasonInvalid},
		{"an egress port with a leading zero", func(s *runnerv1.JobSpec) { s.Network.Egress = []string{"example.com:0443"} }, "Network.Egress[0]", jobspec.ReasonInvalid},
		{"an egress wildcard in the middle", func(s *runnerv1.JobSpec) { s.Network.Egress = []string{"a.*.com"} }, "Network.Egress[0]", jobspec.ReasonInvalid},
		{"an unknown isolation", func(s *runnerv1.JobSpec) { s.Isolation = runnerv1.Isolation(9) }, "Isolation", jobspec.ReasonInvalid},
		{"an invalid label", func(s *runnerv1.JobSpec) { s.Labels = []string{"Prod"} }, "Labels[0]", jobspec.ReasonInvalid},
		{"too many labels", func(s *runnerv1.JobSpec) {
			s.Labels = nil
			for i := range jobspec.MaxLabels + 1 {
				s.Labels = append(s.Labels, "l"+strings.Repeat("x", i))
			}
		}, "Labels", jobspec.ReasonTooMany},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := valid()
			tt.mutate(s)
			_, err := jobspec.Prepare(s)
			require.Error(t, err)
			assert.Contains(t, violations(t, err), jobspec.Violation{Field: tt.field, Reason: tt.reason})
		})
	}
}

func TestValidateAccepts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*runnerv1.JobSpec)
	}{
		{"an image without a registry", func(s *runnerv1.JobSpec) { s.Image = "alpine@sha256:" + strings.Repeat("a", 64) }},
		{"a registry with a port", func(s *runnerv1.JobSpec) { s.Image = "localhost:5000/a/b@sha256:" + strings.Repeat("b", 64) }},
		{"gvisor", func(s *runnerv1.JobSpec) { s.Isolation = runnerv1.Isolation_ISOLATION_GVISOR }},
		{"no secrets and no network", func(s *runnerv1.JobSpec) { s.Secrets, s.Network = nil, nil }},
		{"limits at their edges", func(s *runnerv1.JobSpec) {
			s.Resources = &runnerv1.Resources{CpuMillis: jobspec.MaxCPUMillis, MemoryBytes: jobspec.MinMemoryBytes, Pids: jobspec.MaxPids, DiskBytes: jobspec.MaxDiskBytes}
			s.TimeoutSeconds = jobspec.MaxTimeoutSeconds
		}},
		{"a mutating step", func(s *runnerv1.JobSpec) { s.Steps[0].Mutating = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := valid()
			tt.mutate(s)
			_, err := jobspec.Prepare(s)
			assert.NoError(t, err)
		})
	}
}

func TestOversizedSpecIsRejectedBeforeAnythingElse(t *testing.T) {
	s := valid()
	for range 5 {
		s.Steps = append(s.Steps, &runnerv1.Step{Name: "big", Run: strings.Repeat("x", jobspec.MaxRunBytes)})
	}
	_, err := jobspec.Prepare(s)
	assert.Equal(t, []jobspec.Violation{{Field: "Spec", Reason: jobspec.ReasonTooLong}}, violations(t, err))
	assert.Contains(t, err.Error(), "Spec: too_long")
}

func TestIsolationNames(t *testing.T) {
	for _, name := range []string{"container", "gvisor"} {
		v, ok := jobspec.ParseIsolation(name)
		require.True(t, ok)
		assert.Equal(t, name, jobspec.IsolationName(v))
	}
	v, ok := jobspec.ParseIsolation("")
	assert.True(t, ok)
	assert.Equal(t, runnerv1.Isolation_ISOLATION_CONTAINER, v)
	_, ok = jobspec.ParseIsolation("vm")
	assert.False(t, ok)
}
