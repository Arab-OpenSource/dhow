package app_test

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/Khaymah/dhow/apps/server/internal/App"
	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	sources "github.com/Khaymah/dhow/apps/server/internal/Modules/Sources"
)

func webhookRoutes(t *testing.T, h http.Handler) []route {
	t.Helper()
	mux, ok := h.(chi.Routes)
	require.True(t, ok)
	var out []route
	require.NoError(t, chi.Walk(mux, func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(pattern, config.WebhookPrefix+"/") {
			out = append(out, route{Method: method, Pattern: pattern})
		}
		return nil
	}))
	sort.Slice(out, func(i, j int) bool { return out[i].key() < out[j].key() })
	return out
}

func TestEveryWebhookRouteDeclaresAWebhookRule(t *testing.T) {
	env := apptest.New(t)
	routes := webhookRoutes(t, env.Handler)
	require.NotEmpty(t, routes)
	undeclared, unrouted := missingPolicies(routes, sources.WebhookPolicies())
	assert.Empty(t, undeclared)
	assert.Empty(t, unrouted)
	for _, r := range sources.WebhookPolicies() {
		assert.True(t, r.Valid(), r.Key())
		assert.Equal(t, policy.ScopeWebhook, r.Scope)
	}
	for _, r := range app.Policies() {
		assert.NotEqual(t, policy.ScopeWebhook, r.Scope, "%s: webhook rules never go through the session guard", r.Key())
	}
}

func TestRouteTableWebhooksAcceptNoSessionTokenOrRole(t *testing.T) {
	env := apptest.New(t)
	fx := buildFixture(t, env)
	token := env.SeedAPIToken(mustUUID(fx.byRole[policy.RoleOwner]), mustUUID(fx.orgA), allScopes()...)
	for _, rule := range sources.WebhookPolicies() {
		path := strings.NewReplacer("{hookId}", strings.Repeat("a", 32)).Replace(rule.Path)
		for _, p := range principals(fx) {
			t.Run(rule.Key()+"/"+p.name, func(t *testing.T) {
				c := fx.client(env, p)
				headers := map[string]string{"Origin": apptest.PublicOrigin}
				if c.Cookie != "" {
					headers["Cookie"] = config.SessionCookieName + "=" + c.Cookie
					headers[config.CSRFHeaderName] = c.CSRFToken
				}
				res := env.Webhook(rule.Method, path, headers, []byte(`{}`))
				assert.Equal(t, http.StatusUnauthorized, res.Status, string(res.Raw))
				require.NotNil(t, res.Error)
				assert.Equal(t, "WEBHOOK_REJECTED", res.Error.Code)
			})
		}
		t.Run(rule.Key()+"/api token", func(t *testing.T) {
			res := env.Webhook(rule.Method, path, map[string]string{"Authorization": "Bearer " + token.Value}, []byte(`{}`))
			assert.Equal(t, http.StatusUnauthorized, res.Status, string(res.Raw))
		})
	}
}
