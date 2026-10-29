import os
import re
import subprocess

from . import docs_rules

ARABIC = re.compile(r"[؀-ۿݐ-ݿﭐ-﷿ﹰ-﻿]")

BANNED = (
    "co-authored-by: claude",
    "co-authored-by: anthropic",
    "generated with [claude code]",
    "generated with claude code",
    "\U0001F916",
    "noreply@anthropic.com",
    "claude-session:",
)

CMD_START = r"(?:^|[\n;&|(]|&&|\|\||\bthen\b|\bdo\b|\belse\b)\s*"

WRITES_MESSAGE = re.compile(
    CMD_START + r"(?:\w+=\S+\s+)*"
    r"(?:git\s+(?:-C\s+\S+\s+)?(?:commit|tag\b[^\n]*-[am])|"
    r"gh\s+(?:pr|issue|release)\s+(?:create|edit|comment))",
    re.IGNORECASE,
)

IS_GIT_COMMIT = re.compile(CMD_START + r"(?:\w+=\S+\s+)*git\s+(?:-C\s+\S+\s+)?commit\b", re.IGNORECASE)
IS_GIT_PUSH = re.compile(CMD_START + r"(?:\w+=\S+\s+)*git\s+(?:-C\s+\S+\s+)?push\b", re.IGNORECASE)
NO_VERIFY = re.compile(r"(?:^|\s)--no-verify(?=\s|$)")
COMMIT_SHORT_N = re.compile(r"(?:^|\s)-[a-zA-Z]*n[a-zA-Z]*(?=\s|$)")
STRINGS = re.compile(r"\"(?:[^\"\\]|\\.)*\"|'[^']*'")
FORCE = re.compile(r"(?:^|\s)(--force(?!-with-lease)|-f)(?=\s|$)|(?:^|\s)\+\S+")
PROTECTED = re.compile(r"(?:^|[\s:+])(main|master)(?=\s|$)")

CONVENTIONAL = re.compile(
    r"^(feat|fix|docs|chore|refactor|test|perf|ci|build|style|revert)(\([\w./-]+\))?!?: \S")

QUOTED = r"(\"(?:[^\"\\]|\\.)*\"|'[^']*'|\S+)"


def unquote(s):
    return s[1:-1] if len(s) > 1 and s[0] in "\"'" and s[-1] == s[0] else s


def workdir(command, cwd):
    m = re.search(r"\bgit\s+-C\s+" + QUOTED, command)
    if m:
        d = unquote(m.group(1))
        return d if os.path.isabs(d) else os.path.join(cwd, d)
    m = re.match(r"\s*cd\s+" + QUOTED + r"\s*&&", command)
    if m:
        d = unquote(m.group(1))
        return d if os.path.isabs(d) else os.path.join(cwd, d)
    return cwd


def inherited_message(command, cwd):
    ref = None
    m = re.search(r"(?:--reuse-message|--reedit-message)[= ]+" + QUOTED, command)
    if m:
        ref = unquote(m.group(1))
    else:
        m = re.search(r"\bcommit\b.*?\s-[cC]\s*" + QUOTED, command, re.DOTALL)
        if m:
            ref = unquote(m.group(1))
    if ref is None and re.search(r"--amend\b", command):
        ref = "HEAD"
    if ref is None:
        return ""
    try:
        out = subprocess.run(
            ["git", "-C", workdir(command, cwd), "log", "-1", "--format=%B", ref],
            capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=5, check=False)
    except (OSError, subprocess.SubprocessError):
        return ""
    return out.stdout if out.returncode == 0 else ""


def message_parts(command, cwd):
    parts = []
    for m in re.finditer(r"(?:-m|--message|-b|--body|--notes|-t|--title)[= ]+" + QUOTED, command):
        parts.append(unquote(m.group(1)))
    for m in re.finditer(r"<<-?\s*(['\"]?)(\w+)\1\s*\n(.*?)\n\s*\2\b", command, re.DOTALL):
        parts.append(m.group(3))
    for m in re.finditer(r"(?:-F|--file|--body-file)[= ]+" + QUOTED, command):
        path = unquote(m.group(1))
        if path == "-":
            continue
        if not os.path.isabs(path):
            path = os.path.join(workdir(command, cwd), path)
        try:
            with open(path, "r", encoding="utf-8", errors="replace") as fh:
                parts.append(fh.read())
        except OSError:
            pass
    return parts


def subject_of(command, parts):
    m = re.search(r"(?:-m|--message)[= ]+" + QUOTED, command)
    if m:
        return unquote(m.group(1)).strip().splitlines()[0] if unquote(m.group(1)).strip() else ""
    for p in parts:
        stripped = p.strip()
        if stripped:
            return stripped.splitlines()[0]
    return ""


def check_push(command):
    if not IS_GIT_PUSH.search(command):
        return None
    if NO_VERIFY.search(command):
        return "Never skip git hooks (--no-verify)."
    if FORCE.search(command) and PROTECTED.search(command):
        return "Never force-push main. Work on a branch and open a pull request."
    return None


def check_message(command, cwd):
    if not WRITES_MESSAGE.search(command):
        return None
    is_commit = IS_GIT_COMMIT.search(command) is not None
    head = STRINGS.sub(" ", command.split("<<")[0])
    if is_commit and (NO_VERIFY.search(head) or COMMIT_SHORT_N.search(head.split("commit", 1)[-1])):
        return "Never skip git hooks (--no-verify)."
    parts = message_parts(command, cwd)
    text = "\n".join(parts)
    inherited = ""
    if not text.strip() and is_commit:
        inherited = inherited_message(command, cwd)
        text = inherited
    haystack = (command + "\n" + text).lower()
    for token in BANNED:
        if token in haystack:
            shown = "the robot emoji" if token == "\U0001F916" else repr(token)
            return (f"{shown} appears in this commit or pull request. Dhow history carries no AI attribution "
                    "(no Co-Authored-By: Claude, no 'Generated with Claude Code', no session links). "
                    "End the message at its last real line and re-run.")
    if text and ARABIC.search(text):
        sample = next((ln.strip() for ln in text.splitlines() if ARABIC.search(ln)), "")
        return (f"Commit messages and pull requests are written in English. Offending line: {sample[:80]}. "
                "Arabic belongs in Plan/ and the UI copy, not in git history.")
    if is_commit and not inherited:
        subject = subject_of(command, parts)
        if subject and not CONVENTIONAL.match(subject):
            return (f"Commit subject `{subject[:72]}` is not a Conventional Commit. "
                    "Use `type(scope): summary` with type in feat|fix|docs|chore|refactor|test|perf|ci|build|style|revert.")
    return None


def evaluate(command, cwd):
    return check_push(command) or check_message(command, cwd) or docs_rules.check_docs_pairs(command, cwd, workdir)
