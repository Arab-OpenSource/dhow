import re

from .common import GENERATED_PATH, is_test, new_lines

RUNNER_INTERNAL = "apps/runner/internal/"
RUNNER_IMPORT_PREFIX = "github.com/Khaymah/dhow/apps/runner/internal/"
RUNNER_LAYERS = ("App", "Agent", "Config", "Identity", "Client", "Egress", "Executor", "Journal", "Masking", "Logs", "Sandbox")
RUNNER_ALLOWED_IMPORTS = {
    "App": set(RUNNER_LAYERS),
    "Agent": {"Sandbox"},
    "Executor": {"Client", "Egress", "Identity", "Journal", "Logs", "Masking", "Sandbox", "Config"},
    "Egress": set(),
    "Client": {"Identity", "Config"},
    "Logs": {"Masking", "Config"},
    "Identity": {"Config"},
    "Sandbox": {"Config"},
    "Journal": {"Config"},
    "Masking": set(),
    "Config": set(),
}
PASCAL_DIR = re.compile(r"^[A-Z][A-Za-z0-9]*$")
PASCAL_GO_FILE = re.compile(r"^[A-Z][A-Za-z0-9]*(\.[A-Z][A-Za-z0-9]*)*\.go$")
GO_IMPORT_PATH = re.compile(r'"' + re.escape(RUNNER_IMPORT_PREFIX) + r'([^"]+)"')
SERVER_IMPORT = re.compile(r'"github\.com/Khaymah/dhow/apps/server/')


def is_runner_go(rel):
    return rel.endswith(".go") and rel.startswith(RUNNER_INTERNAL) and not GENERATED_PATH.search(rel)


def check_runner_layout(rel, **_):
    if not is_runner_go(rel):
        return None
    parts = rel[len(RUNNER_INTERNAL):].split("/")
    layers = ", ".join(RUNNER_LAYERS)
    if len(parts) < 2 or parts[0] not in RUNNER_LAYERS:
        return (f"{rel}: Go code under apps/runner/internal lives in one of {layers} (ADR-0053, ADR-0055), "
                "each in its own folder (Executor/Executor.go).")
    for folder in parts[:-1]:
        if not PASCAL_DIR.match(folder):
            return f"{rel}: folder `{folder}` must be PascalCase (Sandbox/Fake)."
    name = parts[-1]
    if not name.endswith("_test.go") and not PASCAL_GO_FILE.match(name):
        return f"{rel}: Go file names are PascalCase with dot roles (Executor.Heartbeat.go), not `{name}`."
    return None


def runner_import_violation(src, dst):
    if dst == src:
        return None
    if dst == "App":
        return "App is the runner's composition root, so nothing imports it."
    if dst not in RUNNER_ALLOWED_IMPORTS.get(src, set()):
        allowed = ", ".join(sorted(RUNNER_ALLOWED_IMPORTS.get(src, set()))) or "no other runner layer"
        return f"{src} may import {allowed}. Pass what it needs from App or Executor instead (ADR-0053)."
    return None


def check_runner_imports(rel, added, old, **_):
    if not is_runner_go(rel) or is_test(rel):
        return None
    parts = rel[len(RUNNER_INTERNAL):].split("/")
    if parts[0] not in RUNNER_LAYERS:
        return None
    lines = new_lines(added, old)
    for line in lines:
        if SERVER_IMPORT.search(line):
            return (f"{rel}: the runner never imports server code. Shared contracts live in internal/ "
                    "(jobspec, runnersig) and packages/proto.")
        for m in GO_IMPORT_PATH.finditer(line):
            target = m.group(1).split("/")[0]
            if target not in RUNNER_LAYERS:
                return f"{rel}: internal/{m.group(1)} is not a runner layer ({', '.join(RUNNER_LAYERS)})."
            reason = runner_import_violation(parts[0], target)
            if reason:
                return f"{rel}: forbidden import of internal/{m.group(1)}. {reason}"
    return None
