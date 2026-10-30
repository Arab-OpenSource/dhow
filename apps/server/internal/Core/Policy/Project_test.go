package policy_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
)

func TestProjectRoleRaisesOrLowersButNeverTouchesAdmins(t *testing.T) {
	tests := []struct {
		name     string
		org      policy.Role
		bindings []policy.Role
		want     policy.Role
	}{
		{"no binding keeps the organization role", policy.RoleDeveloper, nil, policy.RoleDeveloper},
		{"a binding raises a developer", policy.RoleDeveloper, []policy.Role{policy.RoleMaintainer}, policy.RoleMaintainer},
		{"a binding lowers a maintainer", policy.RoleMaintainer, []policy.Role{policy.RoleViewer}, policy.RoleViewer},
		{"the highest of several bindings wins", policy.RoleViewer, []policy.Role{policy.RoleDeveloper, policy.RoleMaintainer}, policy.RoleMaintainer},
		{"a binding never grants owner", policy.RoleDeveloper, []policy.Role{policy.RoleOwner}, policy.RoleDeveloper},
		{"an unknown binding is ignored", policy.RoleDeveloper, []policy.Role{"root"}, policy.RoleDeveloper},
		{"an admin is never lowered", policy.RoleAdmin, []policy.Role{policy.RoleViewer}, policy.RoleAdmin},
		{"an owner is never lowered", policy.RoleOwner, []policy.Role{policy.RoleViewer}, policy.RoleOwner},
		{"no organization role means no project role", "", []policy.Role{policy.RoleAdmin}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, policy.ProjectRole(tt.org, tt.bindings))
		})
	}
}

func TestCanOnAProjectUsesTheProjectRole(t *testing.T) {
	org, project := uuid.New(), uuid.New()
	member := func(role policy.Role) policy.Subject {
		return policy.Subject{UserID: uuid.New(), Memberships: []policy.Membership{{OrganizationID: org, Role: role}}}
	}
	on := func(bindings ...policy.Role) policy.Resource {
		return policy.Resource{OrganizationID: org, ProjectID: project, ProjectBindings: bindings}
	}
	tests := []struct {
		name     string
		subject  policy.Subject
		action   policy.Action
		resource policy.Resource
		want     bool
	}{
		{"a developer cannot manage a project", member(policy.RoleDeveloper), policy.ProjectManage, on(), false},
		{"a developer with a maintainer binding manages it", member(policy.RoleDeveloper), policy.ProjectManage, on(policy.RoleMaintainer), true},
		{
			"a developer with a maintainer binding still cannot create projects", member(policy.RoleDeveloper), policy.ProjectManage,
			policy.Resource{OrganizationID: org},
			false,
		},
		{"a maintainer lowered to viewer cannot manage it", member(policy.RoleMaintainer), policy.ProjectManage, on(policy.RoleViewer), false},
		{"a maintainer lowered to viewer still reads it", member(policy.RoleMaintainer), policy.ProjectRead, on(policy.RoleViewer), true},
		{"a viewer reads a project", member(policy.RoleViewer), policy.ProjectRead, on(), true},
		{"a project resource never grants an organization action", member(policy.RoleOwner), policy.MemberManage, on(), false},
		{"another organization's member gets nothing", policy.Subject{UserID: uuid.New(), Memberships: []policy.Membership{
			{OrganizationID: uuid.New(), Role: policy.RoleOwner},
		}}, policy.ProjectRead, on(policy.RoleAdmin), false},
		{"a token needs the scope as well", policy.Subject{
			UserID: uuid.New(), TokenID: uuid.New(), TokenScopes: []policy.Action{policy.ProjectRead},
			Memberships: []policy.Membership{{OrganizationID: org, Role: policy.RoleDeveloper}},
		}, policy.ProjectManage, on(policy.RoleMaintainer), false},
		{"a token with the scope follows the binding", policy.Subject{
			UserID: uuid.New(), TokenID: uuid.New(), TokenScopes: []policy.Action{policy.ProjectManage},
			Memberships: []policy.Membership{{OrganizationID: org, Role: policy.RoleDeveloper}},
		}, policy.ProjectManage, on(policy.RoleMaintainer), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, policy.Can(tt.subject, tt.action, tt.resource))
		})
	}
}

func TestProjectRules(t *testing.T) {
	rule := policy.Project(http.MethodPatch, "/organizations/{id}/projects/{projectId}", policy.ProjectManage, "id", "projectId")
	assert.True(t, rule.Valid())
	assert.True(t, rule.AllowTokens().Valid())
	assert.Equal(t, policy.ScopeProject, rule.Scope)
	tests := []struct {
		name string
		rule policy.Rule
	}{
		{"an organization action", policy.Project(http.MethodGet, "/x/{id}/{p}", policy.MemberRead, "id", "p")},
		{"no project parameter", policy.Project(http.MethodGet, "/x/{id}", policy.ProjectRead, "id", "")},
		{"no organization parameter", policy.Project(http.MethodGet, "/x/{p}", policy.ProjectRead, "", "p")},
		{"one parameter for both", policy.Project(http.MethodGet, "/x/{id}", policy.ProjectRead, "id", "id")},
		{"a project parameter on an organization rule", policy.Rule{
			Method: http.MethodGet, Path: "/x/{id}", Scope: policy.ScopeOrganization, Action: policy.ProjectRead, OrgParam: "id", ProjectParam: "p",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, tt.rule.Valid())
		})
	}
	assert.True(t, policy.IsTokenAction(policy.ProjectRead))
	assert.True(t, policy.IsTokenAction(policy.ProjectManage))
	for _, a := range policy.ProjectActions() {
		assert.Contains(t, policy.Actions(), a)
	}
}
