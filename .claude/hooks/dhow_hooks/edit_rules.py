import os
import re
import subprocess

from . import go_comments, js_comments

TS_EXT = (".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs")
CSS_EXT = (".css",)
MAX_LINES = 500

ROOT_MARKDOWN = {"README.md", "README.ar.md", "CLAUDE.md", "CONTRIBUTING.md", "SECURITY.md", "LICENSE.md"}
MARKDOWN_DIRS = ("docs/", "Plan/", ".github/", ".claude/")

GENERATED_PATH = re.compile(
    r"(^|/)(gen|sqlc)/|\.gen\.go$|_gen\.go$|(^|/)apps/web/src/api/schema\.d\.ts$|"
    r"(^|/)(go\.sum|pnpm-lock\.yaml)$")

SECRET_PATTERNS = (
    (re.compile(r"-----BEGIN (RSA |EC |OPENSSH |DSA |ENCRYPTED )?PRIVATE KEY-----"), "a private key"),
    (re.compile(r"\bAKIA[0-9A-Z]{16}\b"), "an AWS access key"),
    (re.compile(r"\bgh[pousr]_[A-Za-z0-9]{36,}\b"), "a GitHub token"),
    (re.compile(r"\bglpat-[A-Za-z0-9_-]{20,}\b"), "a GitLab token"),
    (re.compile(r"\bxox[abprs]-[A-Za-z0-9-]{10,}\b"), "a Slack token"),
    (re.compile(r"\bsk-(live|test|proj|ant)-[A-Za-z0-9_-]{16,}\b"), "an API secret key"),
)

GO_SHELL = re.compile(
    r"exec\.Command(Context)?\(\s*(\w+\s*,\s*)?\"(/bin/|/usr/bin/)?(sh|bash|zsh|dash|ash)\"\s*,\s*\"-c\"")
GO_JSON_TAG = re.compile(r"json:\"([^\",]*)")
GO_PANIC = re.compile(r"(^|[^\w.])panic\(")
GO_SQL_FMT = re.compile(
    r"fmt\.Sprintf\(\s*[\"`]\s*(?i:select|insert|update|delete|with|create|alter|drop)\b")
GO_SQL_CONCAT = re.compile(
    r"[\"`]\s*(?i:select|insert\s+into|update|delete\s+from)\b[^\"`]*[\"`]\s*\+")
GO_ENV = re.compile(r"\bos\.(Getenv|LookupEnv|Environ)\(")
GO_PRINT = re.compile(r"\b(fmt\.Print(ln|f)?|log\.(Print|Fatal|Panic)(ln|f)?)\(")

TS_ANY = re.compile(r"(:\s*any\b|\bas\s+any\b|<any>|\bany\[\]|Array<any>|Record<[^>]*\bany\b)")
TS_CONSOLE = re.compile(r"\bconsole\.(log|debug|info|warn|error|trace)\(")
TS_EVAL = re.compile(r"(\beval\(|\bnew\s+Function\()")
TS_INNER_HTML = re.compile(r"dangerouslySetInnerHTML|\.innerHTML\s*=")
TS_INTERFACE = re.compile(r"\binterface\s+([A-Za-z_]\w*)")
TS_TYPE = re.compile(r"(?m)^\s*(?:export\s+)?(?:declare\s+)?type\s+([A-Za-z_]\w*)\s*(?:<[^=]*>)?\s*=")
TS_ENUM = re.compile(r"\benum\s+([A-Za-z_]\w*)")
TS_FUNCTION_COMPONENT = re.compile(r"(?m)^\s*(?:export\s+(?:default\s+)?)?function\s+([A-Z]\w*)\s*\(")

PHYSICAL_CLASS = re.compile(
    r"(?<![\w-])-?(ml|mr|pl|pr|left|right|border-l|border-r|rounded-l|rounded-r|rounded-tl|"
    r"rounded-tr|rounded-bl|rounded-br|scroll-ml|scroll-mr|scroll-pl|scroll-pr)-[\w\[./]+|"
    r"(?<![\w-])(text-left|text-right|float-left|float-right|clear-left|clear-right)(?![\w-])")
PHYSICAL_CSS = re.compile(
    r"(?<![\w-])(margin|padding|border|scroll-margin|scroll-padding)-(left|right)\s*:|"
    r"(?<![\w-])(left|right)\s*:\s*[^;{]|text-align\s*:\s*(left|right)|float\s*:\s*(left|right)")
PHYSICAL_STYLE = re.compile(
    r"\b(marginLeft|marginRight|paddingLeft|paddingRight|borderLeft\w*|borderRight\w*|"
    r"left|right)\s*:\s*[^,}\n]")

SQL_CREATE_TABLE = re.compile(r"(?i)\bcreate\s+table\s+(?:if\s+not\s+exists\s+)?([^\s(]+)")
SQL_COLUMN = re.compile(
    r"(?im)^\s*([A-Za-z_][\w]*)\s+(uuid|text|varchar|char|int|integer|bigint|smallint|boolean|bool|"
    r"timestamptz|timestamp|date|jsonb|json|bytea|numeric|real|double|serial|bigserial)\b")
QUOTED_PASCAL = re.compile(r'^(?:"[A-Z][A-Za-z0-9]*"\.)?"[A-Z][A-Za-z0-9]*"$')


def is_go(path):
    return path.endswith(".go")


def is_ts(path):
    return path.lower().endswith(TS_EXT) and not path.endswith(".d.ts")


def is_test(path):
    return path.endswith("_test.go") or re.search(r"\.(test|spec)\.[cm]?[jt]sx?$", path) is not None


def new_lines(added, old):
    old_set = {ln.strip() for ln in old.splitlines()}
    return [ln for ln in added.splitlines() if ln.strip() and ln.strip() not in old_set]


def check_generated(rel, **_):
    if GENERATED_PATH.search(rel):
        return (f"{rel} is generated or a lockfile. Do not edit it by hand: change the source "
                "(packages/openapi/openapi.yaml, SQL queries, go.mod, package.json) and run `make gen`, "
                "`go mod tidy` or `pnpm install`.")
    return None


def check_secrets(rel, added, **_):
    for pattern, what in SECRET_PATTERNS:
        if pattern.search(added):
            return f"This edit writes {what} into {rel}. Secrets never go into the repository."
    return None


def check_env_file(rel, **_):
    base = os.path.basename(rel)
    if base.startswith(".env") and base != ".env.example":
        return f"{rel}: environment files with real values are never written to the repository. Use .env.example with placeholders."
    return None


def check_markdown_location(rel, exists, **_):
    if not rel.endswith(".md") or exists:
        return None
    if rel in ROOT_MARKDOWN or rel.startswith(MARKDOWN_DIRS):
        return None
    return (f"{rel}: documentation lives in docs/ (public, English) or Plan/ (internal, Arabic). "
            "Do not create Markdown files elsewhere.")


def check_comments(rel, added, old, **_):
    if is_go(rel):
        scanner = go_comments
    elif is_ts(rel):
        scanner = js_comments
    else:
        return None
    for start, end, kind in scanner.find_comments(added):
        text = added[start:end]
        if scanner.is_allowed(kind, text):
            continue
        stripped = text.strip()
        if stripped and stripped in old:
            continue
        snippet = " ".join(stripped.split())[:90]
        return (f"{rel}: code carries no comments (no line comments, block comments, JSDoc or Go doc comments). "
                f"Offending comment: {snippet}. Rationale belongs in Plan/ADR. The only exceptions are build "
                "directives (//go:, //nolint:, eslint-, ts-expect-error, ...) and a comment that starts with `IMPORTANT:`.")
    return None


def check_go(rel, added, old, **_):
    if not is_go(rel):
        return None
    lines = new_lines(added, old)
    text = "\n".join(lines)
    if GO_SHELL.search(text):
        return (f"{rel}: never run a shell with a command string (`sh -c`). Build an argv slice: "
                "exec.CommandContext(ctx, \"git\", \"checkout\", \"--\", ref).")
    for m in GO_JSON_TAG.finditer(text):
        name = m.group(1)
        if name and name != "-" and not name[0].isupper():
            return f"{rel}: JSON field names are PascalCase (`json:\"{name[:1].upper()}{name[1:]}\"`), not `{name}`."
    if GO_SQL_FMT.search(text) or GO_SQL_CONCAT.search(text):
        return f"{rel}: SQL is never built with strings. Write the query in a .sql file and use the sqlc-generated function."
    if is_test(rel):
        return None
    if GO_PANIC.search(text):
        return f"{rel}: no panic in production code. Return a wrapped error (`fmt.Errorf(\"...: %w\", err)`)."
    if GO_PRINT.search(text) and "/cmd/" not in rel:
        return f"{rel}: use the structured logger (log/slog), not fmt.Print or log.Print."
    if GO_ENV.search(text) and "/internal/config/" not in rel and not rel.startswith("internal/config/"):
        return f"{rel}: read environment variables only in the config package, which validates them and fails closed."
    return None


def check_ts(rel, added, old, **_):
    if not is_ts(rel):
        return None
    text = "\n".join(new_lines(added, old))
    if TS_ANY.search(text):
        return f"{rel}: `any` is not allowed. Use a precise type, generics or `unknown` with a type guard."
    if TS_EVAL.search(text):
        return f"{rel}: eval and new Function are never allowed."
    if TS_INNER_HTML.search(text):
        return f"{rel}: do not inject HTML (dangerouslySetInnerHTML / innerHTML). Render React elements."
    if not is_test(rel) and TS_CONSOLE.search(text):
        return f"{rel}: no console.* in application code."
    for m in TS_INTERFACE.finditer(text):
        if not re.match(r"^I[A-Z]", m.group(1)):
            return f"{rel}: interface names start with `I` (`I{m.group(1)}`)."
    for m in TS_TYPE.finditer(text):
        if not re.match(r"^T[A-Z]", m.group(1)):
            return f"{rel}: type alias names start with `T` (`T{m.group(1)}`)."
    for m in TS_ENUM.finditer(text):
        if not re.match(r"^E[A-Z]", m.group(1)):
            return f"{rel}: enum names start with `E` (`E{m.group(1)}`)."
    if rel.endswith((".tsx", ".jsx")):
        m = TS_FUNCTION_COMPONENT.search(text)
        if m:
            return f"{rel}: components are arrow functions (`export const {m.group(1)} = (...) => ...`)."
    return None


def check_direction(rel, added, old, **_):
    if not rel.startswith("apps/web/"):
        return None
    if not (is_ts(rel) or rel.endswith(CSS_EXT)):
        return None
    for line in new_lines(added, old):
        if "IMPORTANT:" in line:
            continue
        hit = PHYSICAL_CLASS.search(line) if is_ts(rel) else None
        if rel.endswith(CSS_EXT):
            hit = PHYSICAL_CSS.search(line)
        elif hit is None and "style" in line:
            hit = PHYSICAL_STYLE.search(line)
        if hit:
            return (f"{rel}: the UI is RTL-first, so use logical properties only "
                    f"(ms-/me-/ps-/pe-/start-/end-/text-start/text-end, margin-inline-*, inset-inline-*). "
                    f"Offending: `{hit.group(0).strip()}`. A deliberate exception needs an `IMPORTANT:` comment on the line.")
    return None


def check_sql(rel, added, old, **_):
    if not rel.endswith(".sql"):
        return None
    text = "\n".join(new_lines(added, old))
    for m in SQL_CREATE_TABLE.finditer(text):
        if not QUOTED_PASCAL.match(m.group(1)):
            return f"{rel}: table names are quoted PascalCase (\"Projects\"), not {m.group(1)}."
    for m in SQL_COLUMN.finditer(text):
        return f"{rel}: column names are quoted PascalCase (\"CreatedAt\"), not {m.group(1)}."
    return None


def migration_on_main(root, rel):
    if not re.search(r"(^|/)migrations/[^/]+\.sql$", rel):
        return False
    for ref in ("origin/main", "main"):
        try:
            r = subprocess.run(["git", "-C", root, "cat-file", "-e", f"{ref}:{rel}"],
                               capture_output=True, timeout=5, check=False)
        except (OSError, subprocess.SubprocessError):
            return False
        if r.returncode == 0:
            return True
    return False


def check_migration_immutable(rel, root, exists, **_):
    if exists and migration_on_main(root, rel):
        return f"{rel} is already on main. Never edit a merged migration: add a new one."
    return None


def check_length(rel, final_content, **_):
    if final_content is None or GENERATED_PATH.search(rel):
        return None
    if not (is_go(rel) or is_ts(rel)):
        return None
    count = final_content.count("\n") + (0 if final_content.endswith("\n") or not final_content else 1)
    if count > MAX_LINES:
        return f"{rel} would be {count} lines. Files stay at or under {MAX_LINES} lines: split by responsibility."
    return None


CHECKS = (
    check_generated,
    check_secrets,
    check_env_file,
    check_markdown_location,
    check_migration_immutable,
    check_comments,
    check_go,
    check_ts,
    check_direction,
    check_sql,
    check_length,
)


def evaluate(rel, root, segments, exists, final_content):
    for added, old in segments:
        for check in CHECKS:
            reason = check(rel=rel, root=root, added=added, old=old, exists=exists, final_content=final_content)
            if reason:
                return reason
    if not segments:
        return check_generated(rel=rel)
    return None
