import re
import subprocess

from . import common

ADR = "Plan/ADR/0064-arabic-first-documentation.md"
DOCS = "docs/"
ENGLISH = "docs/en/"
ROOT_PAIRS = {
    "README.md": "README.en.md",
    "CONTRIBUTING.md": "CONTRIBUTING.en.md",
    "SECURITY.md": "SECURITY.en.md",
}
ROOT_PAIRS.update({english: arabic for arabic, english in list(ROOT_PAIRS.items())})

GIT_COMMIT = common.git_subcommand("commit")
STRINGS = re.compile(r"\"(?:[^\"\\]|\\.)*\"|'[^']*'")
ALL_FLAG = re.compile(r"(?:^|\s)(?:--all|-[a-zA-Z]*a[a-zA-Z]*)(?=\s|$)")


def counterpart(path):
    if path in ROOT_PAIRS:
        return ROOT_PAIRS[path]
    if not path.endswith(".md"):
        return None
    if path.startswith(ENGLISH):
        return DOCS + path[len(ENGLISH):]
    if path.startswith(DOCS):
        return ENGLISH + path[len(DOCS):]
    return None


def changed_paths(cwd, args):
    try:
        out = subprocess.run(["git", "-C", cwd, "diff", "--name-status", "-M", "-z", *args],
                             capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=5, check=False)
    except (OSError, subprocess.SubprocessError):
        return None
    if out.returncode != 0:
        return None
    fields = out.stdout.split("\0")
    paths, i = set(), 0
    while i < len(fields) and fields[i]:
        status = fields[i]
        count = 2 if status[:1] in ("R", "C") else 1
        paths.update(fields[i + 1:i + 1 + count])
        i += 1 + count
    return paths


def missing_pairs(paths):
    found = []
    for path in sorted(paths):
        other = counterpart(path)
        if other and other not in paths:
            found.append((path, other))
    return found


def check_docs_pairs(command, cwd, workdir):
    m = common.find_git(GIT_COMMIT, command)
    if not m:
        return None
    root = workdir(command, cwd)
    paths = changed_paths(root, ["--cached"])
    if paths is None:
        return None
    rest = STRINGS.sub(" ", command[m.end():].split("<<")[0])
    if ALL_FLAG.search(rest):
        paths |= changed_paths(root, []) or set()
    missing = missing_pairs(paths)
    if not missing:
        return None
    shown = "; ".join(f"{path} without {other}" for path, other in missing[:5])
    more = f" (and {len(missing) - 5} more)" if len(missing) > 5 else ""
    return (f"Docs change in only one language: {shown}{more}. The Arabic page in docs/ and its English mirror in "
            f"docs/en/ (and README, CONTRIBUTING and SECURITY with their .en.md) change in the same commit ({ADR}). "
            "Stage the missing file too; a pure rename from docs/X to docs/en/X counts as both.")
