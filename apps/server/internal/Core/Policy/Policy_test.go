package policy_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
)

func TestMatrix(t *testing.T) {
	org := uuid.New()
	want := map[policy.Role]map[policy.Action]bool{
		policy.RoleOwner:      {policy.OrgRead: true, policy.OrgUpdate: true, policy.MemberRead: true, policy.MemberManage: true, policy.AuditRead: true},
		policy.RoleAdmin:      {policy.OrgRead: true, policy.OrgUpdate: true, policy.MemberRead: true, policy.MemberManage: true, policy.AuditRead: true},
		policy.RoleMaintainer: {policy.OrgRead: true, policy.MemberRead: true},
		policy.RoleDeveloper:  {policy.OrgRead: true, policy.MemberRead: true},
		policy.RoleViewer:     {policy.OrgRead: true},
	}
	for _, role := range policy.Roles() {
		subject := policy.Subject{UserID: uuid.New(), Memberships: []policy.Membership{{OrganizationID: org, Role: role}}}
		for _, action := range policy.Actions() {
			if action == policy.SessionManageSelf {
				continue
			}
			t.Run(string(role)+"/"+string(action), func(t *testing.T) {
				assert.Equal(t, want[role][action], policy.Can(subject, action, policy.Resource{OrganizationID: org}))
			})
		}
	}
}

func TestSessionManageSelf(t *testing.T) {
	org := uuid.New()
	tests := []struct {
		name     string
		subject  policy.Subject
		resource policy.Resource
		want     bool
	}{
		{"member", policy.Subject{UserID: uuid.New(), Memberships: []policy.Membership{{OrganizationID: org, Role: policy.RoleViewer}}}, policy.Resource{}, true},
		{"no membership at all", policy.Subject{UserID: uuid.New()}, policy.Resource{}, true},
		{"anonymous", policy.Subject{}, policy.Resource{}, false},
		{"self action never carries an organization", policy.Subject{UserID: uuid.New()}, policy.Resource{OrganizationID: org}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, policy.Can(tt.subject, policy.SessionManageSelf, tt.resource))
		})
	}
}

func TestCanDenies(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	owner := policy.Subject{UserID: uuid.New(), Memberships: []policy.Membership{{OrganizationID: orgA, Role: policy.RoleOwner}}}
	tests := []struct {
		name     string
		subject  policy.Subject
		action   policy.Action
		resource policy.Resource
	}{
		{"anonymous", policy.Subject{}, policy.OrgRead, policy.Resource{OrganizationID: orgA}},
		{"member of another organization", owner, policy.OrgRead, policy.Resource{OrganizationID: orgB}},
		{"organization action without an organization", owner, policy.OrgRead, policy.Resource{}},
		{"unknown action", owner, policy.Action("org.delete"), policy.Resource{OrganizationID: orgA}},
		{"unknown role", policy.Subject{UserID: uuid.New(), Memberships: []policy.Membership{{OrganizationID: orgA, Role: "root"}}}, policy.OrgRead, policy.Resource{OrganizationID: orgA}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, policy.Can(tt.subject, tt.action, tt.resource))
		})
	}
}

func TestParseRole(t *testing.T) {
	for _, r := range policy.Roles() {
		got, ok := policy.ParseRole(string(r))
		assert.True(t, ok)
		assert.Equal(t, r, got)
	}
	_, ok := policy.ParseRole("superuser")
	assert.False(t, ok)
}

func TestTable(t *testing.T) {
	table := policy.NewTable(
		policy.Public(http.MethodPost, "/sessions"),
		policy.Authenticated(http.MethodGet, "/me", policy.SessionManageSelf),
		policy.Organization(http.MethodGet, "/organizations/{id}", policy.OrgRead, "id"),
		policy.Organization(http.MethodGet, "/broken", policy.OrgRead, ""),
		policy.Authenticated(http.MethodGet, "/wrong-action", policy.OrgRead),
		policy.Rule{Method: http.MethodGet, Path: "/no-scope"},
	)
	tests := []struct {
		name   string
		method string
		path   string
		found  bool
		scope  policy.Scope
	}{
		{"public", http.MethodPost, "/sessions", true, policy.ScopePublic},
		{"authenticated", http.MethodGet, "/me", true, policy.ScopeAuthenticated},
		{"organization", http.MethodGet, "/organizations/{id}", true, policy.ScopeOrganization},
		{"wrong method", http.MethodDelete, "/sessions", false, 0},
		{"unknown path", http.MethodGet, "/unknown", false, 0},
		{"organization rule without a parameter is invalid", http.MethodGet, "/broken", false, 0},
		{"authenticated rule with an organization action is invalid", http.MethodGet, "/wrong-action", false, 0},
		{"rule without a scope is invalid", http.MethodGet, "/no-scope", false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, ok := table.Lookup(tt.method, tt.path)
			assert.Equal(t, tt.found, ok)
			assert.Equal(t, tt.scope, rule.Scope)
		})
	}
}

func TestRuleUnsafe(t *testing.T) {
	tests := map[string]bool{
		http.MethodGet: false, http.MethodHead: false, http.MethodOptions: false,
		http.MethodPost: true, http.MethodPut: true, http.MethodPatch: true, http.MethodDelete: true,
	}
	for method, want := range tests {
		assert.Equal(t, want, policy.Public(method, "/x").Unsafe(), method)
	}
}
