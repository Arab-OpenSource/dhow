package executor_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	executor "github.com/Arab-OpenSource/dhow/apps/runner/internal/Executor"
	identity "github.com/Arab-OpenSource/dhow/apps/runner/internal/Identity"
	"github.com/Arab-OpenSource/dhow/internal/runnersig"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

type fixture struct {
	id     *identity.Identity
	server ed25519.PrivateKey
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	keys, err := identity.GenerateKeys()
	require.NoError(t, err)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return fixture{server: priv, id: &identity.Identity{
		Keys: keys, RunnerID: uuid.NewString(), OrganizationID: uuid.NewString(), ServerURL: "https://d", ServerOrigin: "https://d",
		ServerSigningKey: pub, ServerKeyVersion: 1,
	}}
}

func (f fixture) grant(edit func(*runnerv1.LeaseGrant)) *runnerv1.LeaseGrant {
	g := &runnerv1.LeaseGrant{
		LeaseId: uuid.NewString(), AttemptId: uuid.NewString(), JobId: uuid.NewString(), OrganizationId: f.id.OrganizationID,
		RunnerId: f.id.RunnerID, FencingToken: 7, LeaseTtlSeconds: 45, Spec: &runnerv1.JobSpec{Image: "x"},
	}
	if edit != nil {
		edit(g)
	}
	return g
}

func (f fixture) sign(t *testing.T, g *runnerv1.LeaseGrant, key ed25519.PrivateKey, version int32) *runnerv1.SignedLeaseGrant {
	t.Helper()
	raw, err := proto.Marshal(g)
	require.NoError(t, err)
	return &runnerv1.SignedLeaseGrant{GrantBytes: raw, Signature: ed25519.Sign(key, runnersig.GrantMessage(raw)), KeyVersion: version}
}

func TestAProperGrantVerifies(t *testing.T) {
	f := newFixture(t)
	g, err := executor.VerifyGrant(f.id, f.sign(t, f.grant(nil), f.server, 1))
	require.NoError(t, err)
	assert.EqualValues(t, 7, g.GetFencingToken())
}

func TestForgedOrMisaddressedGrantsAreRefused(t *testing.T) {
	f := newFixture(t)
	_, other, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	tampered := f.sign(t, f.grant(nil), f.server, 1)
	tampered.GrantBytes = append([]byte(nil), tampered.GrantBytes...)
	tampered.GrantBytes[len(tampered.GrantBytes)-1] ^= 1
	cases := []struct {
		name   string
		signed *runnerv1.SignedLeaseGrant
		want   error
	}{
		{"nil", nil, executor.ErrGrantMissing},
		{"another key", f.sign(t, f.grant(nil), other, 1), executor.ErrGrantSignature},
		{"tampered bytes", tampered, executor.ErrGrantSignature},
		{"another key version", f.sign(t, f.grant(nil), f.server, 2), executor.ErrGrantKeyVersion},
		{"another runner", f.sign(t, f.grant(func(g *runnerv1.LeaseGrant) { g.RunnerId = uuid.NewString() }), f.server, 1), executor.ErrGrantRunner},
		{"another org", f.sign(t, f.grant(func(g *runnerv1.LeaseGrant) { g.OrganizationId = uuid.NewString() }), f.server, 1), executor.ErrGrantOrganization},
		{"bad lease id", f.sign(t, f.grant(func(g *runnerv1.LeaseGrant) { g.LeaseId = "../x" }), f.server, 1), executor.ErrGrantFields},
		{"zero fencing", f.sign(t, f.grant(func(g *runnerv1.LeaseGrant) { g.FencingToken = 0 }), f.server, 1), executor.ErrGrantFields},
		{"no spec", f.sign(t, f.grant(func(g *runnerv1.LeaseGrant) { g.Spec = nil }), f.server, 1), executor.ErrGrantFields},
		{"ttl too short to fence", f.sign(t, f.grant(func(g *runnerv1.LeaseGrant) { g.LeaseTtlSeconds = 15 }), f.server, 1), executor.ErrGrantTTL},
		{"ttl too long", f.sign(t, f.grant(func(g *runnerv1.LeaseGrant) { g.LeaseTtlSeconds = 3600 }), f.server, 1), executor.ErrGrantTTL},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := executor.VerifyGrant(f.id, c.signed)
			assert.ErrorIs(t, err, c.want)
		})
	}
}

func FuzzVerifyGrant(f *testing.F) {
	f.Add([]byte{1, 2, 3}, []byte{4, 5}, int32(1))
	f.Fuzz(func(t *testing.T, grant, sig []byte, version int32) {
		keys, err := identity.GenerateKeys()
		if err != nil {
			t.Fatal(err)
		}
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		id := &identity.Identity{Keys: keys, RunnerID: uuid.NewString(), ServerSigningKey: pub, ServerKeyVersion: 1}
		if _, err := executor.VerifyGrant(id, &runnerv1.SignedLeaseGrant{GrantBytes: grant, Signature: sig, KeyVersion: version}); err == nil {
			t.Fatal("an unsigned grant verified")
		}
	})
}
