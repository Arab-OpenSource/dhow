package app_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/Khaymah/dhow/apps/server/internal/App"
	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
)

type bindingCase struct {
	name    string
	org     policy.Role
	binding policy.Role
	viaTeam bool
	want    policy.Role
}

func bindingCases() []bindingCase {
	return []bindingCase{
		{"a developer raised to maintainer by a user binding", policy.RoleDeveloper, policy.RoleMaintainer, false, policy.RoleMaintainer},
		{"a developer raised to maintainer by a team binding", policy.RoleDeveloper, policy.RoleMaintainer, true, policy.RoleMaintainer},
		{"a viewer raised to developer", policy.RoleViewer, policy.RoleDeveloper, false, policy.RoleDeveloper},
		{"a maintainer lowered to viewer by a user binding", policy.RoleMaintainer, policy.RoleViewer, false, policy.RoleViewer},
		{"a maintainer lowered to developer by a team binding", policy.RoleMaintainer, policy.RoleDeveloper, true, policy.RoleDeveloper},
		{"an admin is not lowered by a viewer binding", policy.RoleAdmin, policy.RoleViewer, false, policy.RoleAdmin},
	}
}

func projectRules(scope policy.Scope) []policy.Rule {
	var out []policy.Rule
	for _, r := range app.Policies() {
		if r.Scope == scope && strings.Contains(r.Path, "/projects") {
			out = append(out, r)
		}
	}
	return out
}

func projectRequest(rule policy.Rule, org, project uuid.UUID, env *apptest.Env) apptest.Request {
	path := strings.NewReplacer("{id}", org.String(), "{projectId}", project.String()).Replace(rule.Path)
	req := apptest.Request{Method: rule.Method, Path: path}
	switch {
	case rule.Method == http.MethodPatch:
		req.Body = map[string]string{"Name": "Bound project " + randomSuffix(env.T)}
	case rule.Method == http.MethodPost && rule.Path == "/organizations/{id}/projects":
		req.Body = map[string]string{"Name": "Bound project", "Slug": "bp-" + randomSuffix(env.T)}
	}
	return req
}

func assertOutcome(t *testing.T, res apptest.Response, allowedNow bool) {
	t.Helper()
	if allowedNow {
		assert.False(t, policyFailure(res), "must pass authorization, got %d %s", res.Status, res.Raw)
		assert.Less(t, res.Status, 500, string(res.Raw))
		return
	}
	assert.Equal(t, http.StatusForbidden, res.Status, string(res.Raw))
	require.NotNil(t, res.Error, string(res.Raw))
	assert.Equal(t, "FORBIDDEN", res.Error.Code)
}

func boundMember(env *apptest.Env, org, owner, project uuid.UUID, c bindingCase) uuid.UUID {
	user := env.SeedUser(org, randomEmail(env.T), "a long password for a bound member", string(c.org))
	if c.viaTeam {
		env.SeedTeamProjectBinding(org, project, user, string(c.binding))
	} else {
		env.SeedUserProjectBinding(org, project, user, string(c.binding))
	}
	return user
}

func TestRouteTableProjectBindingsRaiseAndLowerTheRole(t *testing.T) {
	env := apptest.New(t)
	fx := buildFixture(t, env)
	org, owner := mustUUID(fx.orgA), mustUUID(fx.byRole[policy.RoleOwner])
	for _, c := range bindingCases() {
		for _, rule := range projectRules(policy.ScopeProject) {
			t.Run(c.name+"/"+rule.Key(), func(t *testing.T) {
				bound, other := env.SeedProject(org, owner), env.SeedProject(org, owner)
				client := env.SeedSession(boundMember(env, org, owner, bound, c))
				assertOutcome(t, client.Do(projectRequest(rule, org, bound, env)), policy.Allows(c.want, rule.Action))
				assertOutcome(t, client.Do(projectRequest(rule, org, other, env)), policy.Allows(c.org, rule.Action))
			})
		}
		for _, rule := range projectRules(policy.ScopeOrganization) {
			t.Run(c.name+"/"+rule.Key(), func(t *testing.T) {
				client := env.SeedSession(boundMember(env, org, owner, env.SeedProject(org, owner), c))
				assertOutcome(t, client.Do(projectRequest(rule, org, uuid.Nil, env)), policy.Allows(c.org, rule.Action))
			})
		}
	}
}

func TestRouteTableProjectBindingsApplyToAPITokens(t *testing.T) {
	env := apptest.New(t)
	fx := buildFixture(t, env)
	org, owner := mustUUID(fx.orgA), mustUUID(fx.byRole[policy.RoleOwner])
	scopes := [][]string{allScopes(), {string(policy.ProjectRead)}}
	for _, c := range bindingCases() {
		for _, granted := range scopes {
			for _, rule := range projectRules(policy.ScopeProject) {
				t.Run(c.name+"/"+strings.Join(granted, "+")+"/"+rule.Key(), func(t *testing.T) {
					bound, other := env.SeedProject(org, owner), env.SeedProject(org, owner)
					token := env.SeedAPIToken(boundMember(env, org, owner, bound, c), org, granted...)
					scoped := false
					for _, s := range granted {
						scoped = scoped || policy.Action(s) == rule.Action
					}
					for project, role := range map[uuid.UUID]policy.Role{bound: c.want, other: c.org} {
						req := projectRequest(rule, org, project, env)
						req.Headers = map[string]string{"Origin": ""}
						assertOutcome(t, env.Bearer(token.Value, req), scoped && policy.Allows(role, rule.Action))
					}
				})
			}
		}
	}
}

func TestRouteTableProjectsOfAnotherOrganizationAreNotFound(t *testing.T) {
	env := apptest.New(t)
	fx := buildFixture(t, env)
	foreign := env.SeedProject(mustUUID(fx.orgB), mustUUID(fx.outsiderID))
	for _, rule := range projectRules(policy.ScopeProject) {
		for _, role := range policy.Roles() {
			t.Run(rule.Key()+"/"+string(role), func(t *testing.T) {
				res := env.SeedSession(mustUUID(fx.byRole[role])).Do(projectRequest(rule, mustUUID(fx.orgA), foreign, env))
				assert.Equal(t, http.StatusNotFound, res.Status, string(res.Raw))
				require.NotNil(t, res.Error)
				assert.Equal(t, "NOT_FOUND", res.Error.Code)
			})
		}
	}
}
