import os
import sys
import unittest

HOOKS = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, HOOKS)

from dhow_hooks import edit_rules


def edit(rel, added, old="", exists=False, final=None):
    return edit_rules.evaluate(rel=rel, root="/nonexistent", segments=[(added, old)], exists=exists,
                               final_content=final if final is not None else added)


SERVER = "apps/server/internal/"
SOURCES_ROUTES = SERVER + "Modules/Sources/Sources.Routes.go"
GOOD = (
    "package sources\n\n"
    "func WebhookPolicies() []policy.Rule {\n"
    "\treturn []policy.Rule{policy.Webhook(http.MethodPost, gitHookPath)}\n"
    "}\n\n"
    "func WebhookRoutes(r chi.Router, d WebhookDeps) {\n"
    "\ttable := policy.NewTable(WebhookPolicies()...)\n"
    "\tfor _, route := range webhookRoutes(d) {\n"
    "\t\tr.With(gate(table)).Method(route.Rule.Method, route.Rule.Path, route.Handler)\n"
    "\t}\n"
    "}\n"
)


class WebhookRules(unittest.TestCase):
    def test_rule_derived_webhook_routes_allowed(self):
        self.assertIsNone(edit(SOURCES_ROUTES, GOOD))

    def test_hand_registered_webhook_route_blocked(self):
        for call in ("r.Post(gitHookPath, h)", "r.Method(http.MethodPost, gitHookPath, h)", "r.Handle(path, h)",
                     "r.Mount(\"/webhooks\", h)", "r.HandleFunc(p, h)"):
            src = GOOD + "\nfunc More(r chi.Router) {\n\t" + call + "\n}\n"
            self.assertIsNotNone(edit(SOURCES_ROUTES, src), call)

    def test_rule_routes_without_a_declared_webhook_rule_blocked(self):
        src = GOOD.replace("policy.Webhook(http.MethodPost, gitHookPath)", "policy.Rule{}")
        self.assertIn("declared rule", edit(SOURCES_ROUTES, src))
        src = GOOD.replace("policy.NewTable(WebhookPolicies()...)", "nil")
        self.assertIn("declared rule", edit(SOURCES_ROUTES, src))

    def test_webhook_rules_outside_the_sources_routes_blocked(self):
        for rel in (SERVER + "Modules/Projects/Projects.Routes.go", SERVER + "App/App.go",
                    SERVER + "Modules/Sources/Sources.Service.go", SERVER + "Core/Auth/Guard.go"):
            self.assertIn("policy.Webhook", edit(rel, "package x\n\nvar r = policy.Webhook(http.MethodPost, \"/webhooks/x\")\n"), rel)

    def test_the_policy_package_and_tests_may_name_webhook_rules(self):
        src = "package policy\n\nfunc Webhook(method, path string) Rule { return Rule{} }\nvar x = policy.Webhook(a, b)\n"
        self.assertIsNone(edit(SERVER + "Core/Policy/Webhook.go", src))
        self.assertIsNone(edit(SERVER + "App/Webhooks_test.go", "package app_test\n\nvar r = policy.Webhook(a, b)\n"))
        self.assertIsNone(edit(SERVER + "Modules/Sources/Test/E2E/Webhook_test.go", "package e2e\n\nvar r = policy.Webhook(a, b)\n"))

    def test_webhook_rules_count_as_declared_policies_for_generated_routes(self):
        src = GOOD + (
            "\nfunc Routes(r chi.Router, d Deps) {\n"
            "\tguard := d.Guard.Middleware(policy.NewTable(Policies()...))\n"
            "\tgen.HandlerWithOptions(strict, gen.ChiServerOptions{BaseRouter: r, Middlewares: []gen.MiddlewareFunc{guard}})\n"
            "}\n")
        self.assertIsNone(edit(SOURCES_ROUTES, src))


if __name__ == "__main__":
    unittest.main()
