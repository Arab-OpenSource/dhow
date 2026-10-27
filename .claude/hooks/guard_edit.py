import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from dhow_hooks import edit_rules


def project_root(payload):
    return os.environ.get("CLAUDE_PROJECT_DIR") or payload.get("cwd") or os.getcwd()


def segments_of(tool_input):
    out = []
    if isinstance(tool_input.get("content"), str):
        out.append((tool_input["content"], ""))
    if isinstance(tool_input.get("new_string"), str):
        out.append((tool_input["new_string"], tool_input.get("old_string") or ""))
    edits = tool_input.get("edits")
    if isinstance(edits, list):
        for e in edits:
            if isinstance(e, dict) and isinstance(e.get("new_string"), str):
                out.append((e["new_string"], e.get("old_string") or ""))
    return out


def final_content(path, tool_input, exists):
    if isinstance(tool_input.get("content"), str):
        return tool_input["content"]
    if not exists:
        return None
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as fh:
            current = fh.read()
    except OSError:
        return None
    edits = tool_input.get("edits") if isinstance(tool_input.get("edits"), list) else [tool_input]
    for e in edits:
        if not isinstance(e, dict) or not isinstance(e.get("old_string"), str) or not isinstance(e.get("new_string"), str):
            continue
        if e.get("replace_all"):
            current = current.replace(e["old_string"], e["new_string"])
        else:
            current = current.replace(e["old_string"], e["new_string"], 1)
    return current


def deny(reason):
    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": "[dhow-rules] " + reason,
        },
    }))


def main():
    try:
        payload = json.load(sys.stdin)
    except (ValueError, OSError):
        return 0
    tool_input = payload.get("tool_input") or {}
    path = tool_input.get("file_path") if isinstance(tool_input, dict) else None
    if not isinstance(path, str) or not path:
        return 0
    root = os.path.abspath(project_root(payload))
    abs_path = path if os.path.isabs(path) else os.path.join(root, path)
    rel = os.path.relpath(abs_path, root).replace(os.sep, "/")
    if rel.startswith("../") or rel.startswith(".claude/worktrees/"):
        return 0
    exists = os.path.exists(abs_path)
    reason = edit_rules.evaluate(
        rel=rel,
        root=root,
        segments=segments_of(tool_input),
        exists=exists,
        final_content=final_content(abs_path, tool_input, exists),
    )
    if reason:
        deny(reason)
    return 0


if __name__ == "__main__":
    sys.exit(main())
