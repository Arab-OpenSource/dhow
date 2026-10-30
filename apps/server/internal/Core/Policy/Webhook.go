package policy

import (
	"net/http"
	"strings"
)

const WebhookPrefix = "/webhooks/"

func Webhook(method, path string) Rule {
	return Rule{Method: method, Path: path, Scope: ScopeWebhook}
}

func (r Rule) validWebhook() bool {
	return r.Method == http.MethodPost && strings.HasPrefix(r.Path, WebhookPrefix) && len(r.Path) > len(WebhookPrefix) &&
		r.Action == "" && r.OrgParam == "" && r.ProjectParam == "" && r.Runner == 0 && !r.Tokens
}
