import re

from .common import GENERATED_PATH, is_test, new_lines

SERVER_INTERNAL = "apps/server/internal/"
SOURCES_ROUTES = SERVER_INTERNAL + "Modules/Sources/Sources.Routes.go"
POLICY_PACKAGE = SERVER_INTERNAL + "Core/Policy/"
WEBHOOK_RULE = re.compile(r"\bpolicy\.Webhook\(")
ROUTE_CALL = re.compile(r"\b\w+\.(Get|Post|Put|Patch|Delete|Head|Options|Connect|Trace|Handle|HandleFunc|Method|MethodFunc|Mount)\(")
RULE_ROUTE = re.compile(r"\.Method\(\s*(?:\w+\.)?[Rr]ule\.Method\s*,\s*(?:\w+\.)?[Rr]ule\.Path\s*,")
POLICY_TABLE = re.compile(r"\bpolicy\.NewTable\(")
ADR = "Plan/ADR/0072-webhooks-and-git-credentials.md"


def check_webhook_rule_location(rel, added, old, **_):
    if not rel.endswith(".go") or not rel.startswith(SERVER_INTERNAL) or is_test(rel) or "/Test/" in rel:
        return None
    if GENERATED_PATH.search(rel) or rel == SOURCES_ROUTES or rel.startswith(POLICY_PACKAGE):
        return None
    if WEBHOOK_RULE.search("\n".join(new_lines(added, old))):
        return (f"{rel}: webhook routes are declared with policy.Webhook in {SOURCES_ROUTES} only, next to the "
                f"handlers that verify their signatures ({ADR}).")
    return None


def check_webhook_routes(rel, added, old, final_content, **_):
    if rel != SOURCES_ROUTES:
        return None
    text = final_content if final_content is not None else added
    for line in new_lines(added, old):
        m = ROUTE_CALL.search(line)
        if m and not RULE_ROUTE.search(line):
            return (f"{rel}: register a webhook route only from its rule, as `r.Method(route.Rule.Method, route.Rule.Path, h)`, "
                    f"where the rule comes from policy.Webhook in WebhookPolicies(). Offending: `{m.group(0)}`.")
    if RULE_ROUTE.search(text) and not (WEBHOOK_RULE.search(text) and POLICY_TABLE.search(text)):
        return (f"{rel}: every webhook route needs a declared rule. Declare it with policy.Webhook, build "
                "policy.NewTable from the rules and check every request against it.")
    return None
