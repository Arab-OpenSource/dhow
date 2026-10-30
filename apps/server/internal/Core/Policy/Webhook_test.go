package policy

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebhookRules(t *testing.T) {
	assert.True(t, Webhook(http.MethodPost, "/webhooks/git/{hookId}").Valid())
	invalid := []Rule{
		Webhook(http.MethodGet, "/webhooks/git/{hookId}"),
		Webhook(http.MethodPost, "/api/v1/webhooks/git"),
		Webhook(http.MethodPost, "/webhooks/"),
		Webhook(http.MethodPost, "/webhooks/git").AllowTokens(),
		{Method: http.MethodPost, Path: "/webhooks/git", Scope: ScopeWebhook, Action: ProjectManage},
		{Method: http.MethodPost, Path: "/webhooks/git", Scope: ScopeWebhook, OrgParam: "id"},
		{Method: http.MethodPost, Path: "/webhooks/git", Scope: ScopeWebhook, Runner: RunnerSigned},
	}
	for _, r := range invalid {
		assert.False(t, r.Valid(), "%+v", r)
	}
	table := NewTable(Webhook(http.MethodPost, "/webhooks/git/{hookId}"))
	_, ok := table.Lookup(http.MethodPost, "/webhooks/git/{hookId}")
	assert.True(t, ok)
	_, ok = table.Lookup(http.MethodPost, "/webhooks/github")
	assert.False(t, ok)
}
