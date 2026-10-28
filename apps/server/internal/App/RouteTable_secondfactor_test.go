package app_test

import (
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/Khaymah/dhow/apps/server/internal/App"
	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
)

var secondFactorRequired = outcome{http.StatusForbidden, "SECOND_FACTOR_REQUIRED"}

type secondFactorVariant struct {
	name    string
	elapsed time.Duration
	factor  bool
	blocks  bool
}

func secondFactorVariants() []secondFactorVariant {
	return []secondFactorVariant{
		{name: "past the grace period without a factor", elapsed: config.SecondFactorGrace + time.Hour, blocks: true},
		{name: "past the grace period with a passkey", elapsed: config.SecondFactorGrace + time.Hour, factor: true},
		{name: "inside the grace period", elapsed: config.SecondFactorGrace - time.Hour},
	}
}

func secondFactorFixture(t *testing.T, v secondFactorVariant) (*apptest.Env, fixture) {
	t.Helper()
	env := apptest.New(t)
	fx := buildFixture(t, env)
	start := env.Clock.Now().Add(-v.elapsed)
	env.SetSecondFactorRule(mustUUID(fx.orgA), string(policy.SecondFactorEveryone), start)
	env.SetJoinedAt(mustUUID(fx.orgA), start)
	if v.factor {
		for _, id := range fx.byRole {
			env.SeedPasskey(mustUUID(id))
		}
	}
	return env, fx
}

func dropSeededFactors(env *apptest.Env, v secondFactorVariant) {
	if v.factor {
		return
	}
	_, err := env.Pool.Exec(env.T.Context(), `UPDATE "Passkeys" SET "IsDeleted" = true WHERE NOT "IsDeleted"`)
	require.NoError(env.T, err)
}

func withSecondFactor(base outcome, rule policy.Rule, p principal, v secondFactorVariant) outcome {
	if v.blocks && base == allowed && p.kind == "member" && rule.Scope == policy.ScopeOrganization && policy.IsSensitive(rule.Action) {
		return secondFactorRequired
	}
	return base
}

func checkOutcome(t *testing.T, want outcome, res apptest.Response) {
	t.Helper()
	if want == allowed {
		assert.False(t, policyFailure(res) || (res.Error != nil && res.Error.Code == "SECOND_FACTOR_REQUIRED"),
			"must pass authorization, got %d %s", res.Status, res.Raw)
		assert.Less(t, res.Status, 500, string(res.Raw))
		return
	}
	assert.Equal(t, want.status, res.Status, string(res.Raw))
	require.NotNil(t, res.Error, string(res.Raw))
	assert.Equal(t, want.code, res.Error.Code)
}

func TestRouteTableWithARequiredSecondFactor(t *testing.T) {
	for _, v := range secondFactorVariants() {
		t.Run(v.name, func(t *testing.T) {
			env, fx := secondFactorFixture(t, v)
			rules := app.Policies()
			sort.Slice(rules, func(i, j int) bool { return rules[i].Key() < rules[j].Key() })
			for _, rule := range rules {
				for _, p := range principals(fx) {
					t.Run(rule.Key()+"/"+p.name, func(t *testing.T) {
						c := fx.client(env, p)
						if rule.Path == "/organizations/{id}/leave" && p.kind == "member" {
							c = env.SeedSession(env.SeedUser(mustUUID(fx.orgA), randomEmail(t), "a long password for leaving", string(p.role)))
						}
						dropSeededFactors(env, v)
						checkOutcome(t, withSecondFactor(expected(rule, p), rule, p, v), c.Do(requestFor(rule, fx, c)))
					})
				}
			}
		})
	}
}

func TestRouteTableWithAPITokensAndARequiredSecondFactor(t *testing.T) {
	for _, v := range secondFactorVariants() {
		t.Run(v.name, func(t *testing.T) {
			env, fx := secondFactorFixture(t, v)
			rules := app.Policies()
			sort.Slice(rules, func(i, j int) bool { return rules[i].Key() < rules[j].Key() })
			for _, p := range tokenPrincipals() {
				token := env.SeedAPIToken(mustUUID(fx.userOf(p)), mustUUID(fx.orgA), allScopes()...)
				for _, rule := range rules {
					t.Run(rule.Key()+"/"+p.name, func(t *testing.T) {
						dropSeededFactors(env, v)
						req := requestFor(rule, fx, &apptest.Client{Env: env})
						req.Headers = map[string]string{"Origin": "", "Authorization": "Bearer " + token.Value}
						want := withSecondFactor(expectedWithToken(rule, p, allScopes()), rule, p, v)
						checkOutcome(t, want, env.Do(req))
					})
				}
			}
		})
	}
}
