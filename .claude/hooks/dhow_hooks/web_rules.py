import posixpath
import re

from .common import GENERATED_PATH, is_test, new_lines

WEB_SRC = "apps/web/Src/"
WEB_LEGACY_SRC = "apps/web/src/"
WEB_TOP_LEVEL = ("App", "Core", "I18n", "Styles", "Modules")
WEB_ROOT_FILES = ("Main.tsx",)
PASCAL_DIR = re.compile(r"^[A-Z][A-Za-z0-9]*$")
WEB_FILE = re.compile(r"^[A-Z][A-Za-z0-9]*(\.d)?\.(ts|tsx|css|json)$|^[A-Z][A-Za-z0-9]*\.test\.tsx?$")
WEB_HOOK_FILE = re.compile(r"^use[A-Z][A-Za-z0-9]*(\.test)?\.tsx?$")
WEB_LAYOUT = "App, Core/<Name>, I18n, Styles or Modules/<Module>/<Folder>/..."

TS_IMPORT = re.compile(r"""(?:\bfrom\s*|\bimport\s*\(\s*|\bimport\s+)(['"])([^'"\n]+)\1""")

LETTER_RUN = re.compile(r"[A-Za-z؀-ۿݐ-ݿﭐ-﷿ﹰ-﻿]{2,}")
ATTR_LITERAL = re.compile(
    r"""\b(aria-label|aria-description|title|placeholder|alt)\s*=\s*(?:"([^"]*)"|'([^']*)'|\{\s*(["'`])([^"'`]*)\4\s*\})""")
CHILD_LITERAL = re.compile(r"""(?<!=)>\s*\{\s*(["'`])([^"'`]*)\1\s*\}""")
EXPRESSION = re.compile(r"\{[^{}]*\}")
TAG = re.compile(r"</?[A-Za-z][^<>]*>")
TEXT_AFTER_TAG = re.compile(r"(?<!=)>([^<>\x00=;()]*)(?=<|\x00)")
TEXT_AFTER_EXPRESSION = re.compile(r"\x00([^<>\x00=;()]+)(?=<)")
TEXT_BEFORE_CLOSING_TAG = re.compile(r"^([^<>\x00=;()]+)(?=</)")
TEXT_TO_END_OF_LINE = re.compile(r"^<[^>]*>([^<>\x00=;()]+)$")
PLAIN_TEXT_LINE = re.compile(r"^[^<>{}()\[\]=;|&/\\@#$%^*+~\"`]+$")


def web_area(rel):
    parts = rel[len(WEB_SRC):].split("/")
    if parts[0] in ("Core", "Modules") and len(parts) > 1:
        return (parts[0], parts[1])
    return (parts[0], None)


def check_web_layout(rel, **_):
    if rel.startswith(WEB_LEGACY_SRC):
        return f"{rel}: the web source folder is `Src/` (PascalCase), not `src/`. Layout: {WEB_LAYOUT}."
    if not rel.startswith(WEB_SRC) or GENERATED_PATH.search(rel):
        return None
    parts = rel[len(WEB_SRC):].split("/")
    name = parts[-1]
    if len(parts) == 1:
        if name in WEB_ROOT_FILES:
            return None
        return f"{rel}: only Main.tsx sits directly in Src/. Put code in {WEB_LAYOUT}."
    if parts[0] not in WEB_TOP_LEVEL:
        return f"{rel}: web code under Src must live in {WEB_LAYOUT}. Cross-cutting code goes in Core, wiring in App."
    needed = {"Modules": 4, "Core": 3}.get(parts[0], 2)
    if len(parts) < needed:
        where = {"Core": "Core/<Name>/File.tsx", "Modules": "Modules/<Module>/<Folder>/File.tsx"}[parts[0]]
        return f"{rel}: files here sit in a named folder ({where}), not directly in {parts[0]}/."
    for folder in parts[:-1]:
        if not PASCAL_DIR.match(folder):
            return f"{rel}: folder `{folder}` must be PascalCase (Core/Ui, Modules/System/Components)."
    if re.match(r"^Use[A-Z]", name):
        return (f"{rel}: hooks are named `useX` with a lowercase `use`, or eslint-plugin-react-hooks stops recognizing "
                f"them (ADR-0031). Rename `{name}` to `use{name[3:]}`.")
    if not (WEB_FILE.match(name) or WEB_HOOK_FILE.match(name)):
        return (f"{rel}: file names are PascalCase (Button.tsx, Button.test.tsx), and only hooks are camelCase with "
                f"a `use` prefix (useMeta.ts). `{name}` is neither.")
    return None


def web_import_target(rel, spec):
    if spec.startswith("@/"):
        target = spec[2:]
    elif spec.startswith("."):
        target = posixpath.normpath(posixpath.join(posixpath.dirname(rel[len(WEB_SRC):]), spec))
    else:
        return None
    if target.startswith(".."):
        return None
    return target.split("/")


def web_import_violation(src, dst):
    s_top, s_name = src
    d_top, d_name = dst
    if d_top == "Modules":
        if s_top == "App":
            return None
        if s_top == "Modules":
            if s_name == d_name:
                return None
            return (f"Modules/{s_name} must not import Modules/{d_name}. Share through Core (hooks, stores, UI) "
                    "or let App compose the two.")
        return f"{s_top} must not import Modules. Only App imports modules."
    if d_top == "App" and s_top != "App":
        return f"{s_top} must not import App. App is the composition root and depends on everything else."
    return None


def check_web_imports(rel, added, old, **_):
    if not rel.startswith(WEB_SRC) or is_test(rel) or GENERATED_PATH.search(rel):
        return None
    src = web_area(rel)
    if src[0] not in WEB_TOP_LEVEL:
        return None
    known = {ln.strip() for ln in old.splitlines()}
    for m in TS_IMPORT.finditer(added):
        line_start = added.rfind("\n", 0, m.start(2)) + 1
        line_end = added.find("\n", m.end(2))
        if added[line_start:line_end if line_end != -1 else None].strip() in known:
            continue
        target = web_import_target(rel, m.group(2))
        if not target or target[0] not in WEB_TOP_LEVEL:
            continue
        dst = (target[0], target[1] if target[0] in ("Core", "Modules") and len(target) > 1 else None)
        reason = web_import_violation(src, dst)
        if reason:
            return f"{rel}: forbidden import of `{m.group(2)}`. {reason}"
    return None


def hardcoded_exempt(rel):
    return (not rel.startswith(WEB_SRC) or not rel.endswith(".tsx") or is_test(rel)
            or "/I18n/Locales/" in rel or "/Core/Testing/" in rel or GENERATED_PATH.search(rel) is not None)


def literal_hits(lines):
    for i, raw in enumerate(lines):
        line = raw.strip()
        for m in ATTR_LITERAL.finditer(line):
            value = m.group(2) or m.group(3) or m.group(5) or ""
            if LETTER_RUN.search(value):
                yield i, f'{m.group(1)}="{value[:40]}"'
        child = CHILD_LITERAL.search(line)
        if child and LETTER_RUN.search(child.group(2)):
            yield i, child.group(2)[:40]
        text = jsx_text(lines, i, line)
        if text:
            yield i, text


def jsx_text(lines, i, line):
    stripped = line
    for _ in range(4):
        stripped = EXPRESSION.sub("\x00", stripped)
    candidates = [m.group(1) for m in TEXT_AFTER_TAG.finditer(stripped)]
    if TAG.search(stripped):
        candidates += [m.group(1) for m in TEXT_AFTER_EXPRESSION.finditer(stripped)]
        candidates += [m.group(1) for m in TEXT_BEFORE_CLOSING_TAG.finditer(stripped)]
    candidates += [m.group(1) for m in TEXT_TO_END_OF_LINE.finditer(stripped)]
    if PLAIN_TEXT_LINE.match(stripped.replace("\x00", "")) and neighbours_are_tags(lines, i):
        candidates.append(stripped)
    for text in candidates:
        if LETTER_RUN.search(text):
            return " ".join(text.split())[:60]
    return None


def neighbours_are_tags(lines, i):
    previous = next((ln.strip() for ln in reversed(lines[:i]) if ln.strip()), "")
    following = next((ln.strip() for ln in lines[i + 1:] if ln.strip()), "")
    return previous.endswith(">") and not previous.endswith("=>") and following.startswith(("<", "{"))


def check_web_text(rel, added, old, **_):
    if hardcoded_exempt(rel):
        return None
    fresh = {ln.strip() for ln in new_lines(added, old)}
    lines = added.splitlines()
    for index, sample in literal_hits(lines):
        if lines[index].strip() in fresh:
            return (f"{rel}: no hardcoded user-facing text in the UI (`{sample}`). Add a key to I18n/Locales/Ar.ts "
                    "and En.ts and render it with t('your.key'). Only the locale files and tests may contain literal text.")
    return None
