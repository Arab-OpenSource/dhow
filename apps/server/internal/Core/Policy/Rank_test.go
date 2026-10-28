package policy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
)

func TestRankOrdersTheRoles(t *testing.T) {
	roles := policy.Roles()
	for i := 1; i < len(roles); i++ {
		assert.Greater(t, policy.Rank(roles[i-1]), policy.Rank(roles[i]), "%s above %s", roles[i-1], roles[i])
	}
	assert.Zero(t, policy.Rank("root"))
	assert.False(t, policy.AtLeast("root", policy.RoleViewer))
	assert.True(t, policy.AtLeast(policy.RoleAdmin, policy.RoleDeveloper))
}

func TestEffectiveIsTheHighestRoleButNeverOwnerFromABinding(t *testing.T) {
	tests := []struct {
		name     string
		direct   policy.Role
		bindings []policy.Role
		want     policy.Role
	}{
		{"direct only", policy.RoleViewer, nil, policy.RoleViewer},
		{"a team raises it", policy.RoleViewer, []policy.Role{policy.RoleDeveloper, policy.RoleMaintainer}, policy.RoleMaintainer},
		{"a team never lowers it", policy.RoleAdmin, []policy.Role{policy.RoleViewer}, policy.RoleAdmin},
		{"a binding never grants owner", policy.RoleViewer, []policy.Role{policy.RoleOwner}, policy.RoleViewer},
		{"no direct membership grants nothing", "", []policy.Role{policy.RoleAdmin}, ""},
		{"an unknown direct role grants nothing", "root", []policy.Role{policy.RoleAdmin}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, policy.Effective(tt.direct, tt.bindings...))
		})
	}
	role, ok := policy.EffectiveOf("developer", []string{"admin", "bogus"})
	assert.True(t, ok)
	assert.Equal(t, policy.RoleAdmin, role)
	_, ok = policy.EffectiveOf("bogus", []string{"admin"})
	assert.False(t, ok)
}

func TestGrantAndManageRules(t *testing.T) {
	assert.True(t, policy.CanGrant(policy.RoleAdmin, policy.RoleAdmin))
	assert.False(t, policy.CanGrant(policy.RoleAdmin, policy.RoleOwner))
	assert.False(t, policy.CanGrant(policy.RoleMaintainer, policy.RoleAdmin))
	assert.False(t, policy.CanGrant(policy.RoleOwner, "root"))
	assert.True(t, policy.CanManage(policy.RoleOwner, policy.RoleOwner))
	assert.False(t, policy.CanManage(policy.RoleAdmin, policy.RoleOwner))
	assert.True(t, policy.CanManage(policy.RoleAdmin, policy.RoleAdmin))
	assert.False(t, policy.CanManage(policy.RoleMaintainer, policy.RoleViewer))
	assert.False(t, policy.Bindable(policy.RoleOwner))
	assert.True(t, policy.Bindable(policy.RoleViewer))
}

func TestMfaResetNeedsAStrictlyHigherRole(t *testing.T) {
	cases := []struct {
		actor, target policy.Role
		instanceOwner bool
		want          bool
	}{
		{policy.RoleOwner, policy.RoleAdmin, false, true},
		{policy.RoleOwner, policy.RoleOwner, false, false},
		{policy.RoleAdmin, policy.RoleMaintainer, false, true},
		{policy.RoleAdmin, policy.RoleViewer, false, true},
		{policy.RoleAdmin, policy.RoleAdmin, false, false},
		{policy.RoleAdmin, policy.RoleOwner, false, false},
		{policy.RoleMaintainer, policy.RoleViewer, false, false},
		{policy.RoleOwner, policy.RoleViewer, true, false},
		{policy.RoleOwner, policy.RoleAdmin, true, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, policy.CanResetMfa(c.actor, c.target, c.instanceOwner), "%s resets %s (instance owner %v)", c.actor, c.target, c.instanceOwner)
	}
}
