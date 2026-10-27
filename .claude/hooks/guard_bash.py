import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from dhow_hooks import bash_rules


def main():
    try:
        payload = json.load(sys.stdin)
    except (ValueError, OSError):
        return 0
    tool_input = payload.get("tool_input") or {}
    command = tool_input.get("command") if isinstance(tool_input, dict) else None
    if not isinstance(command, str) or not command:
        return 0
    cwd = payload.get("cwd") or os.environ.get("CLAUDE_PROJECT_DIR") or os.getcwd()
    reason = bash_rules.evaluate(command, cwd)
    if reason:
        print(json.dumps({
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": "[dhow-rules] " + reason,
            },
        }))
    return 0


if __name__ == "__main__":
    sys.exit(main())
