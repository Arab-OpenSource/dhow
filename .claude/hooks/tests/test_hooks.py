import json
import os
import subprocess
import sys
import tempfile
import unittest

HOOKS = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, HOOKS)

from dhow_hooks import bash_rules, edit_rules


def edit(rel, added, old="", exists=False, final=None, root="/nonexistent"):
    return edit_rules.evaluate(rel=rel, root=root, segments=[(added, old)], exists=exists,
                               final_content=final if final is not None else added)


class CommentRules(unittest.TestCase):
    def test_go_line_comment_blocked(self):
        self.assertIn("no comments", edit("apps/server/x.go", "package x\n\n// Get returns x\nfunc Get() {}\n"))

    def test_go_doc_block_blocked(self):
        self.assertIsNotNone(edit("internal/a/a.go", "package a\n/* doc */\n"))

    def test_go_directives_allowed(self):
        src = "//go:build linux\n\npackage a\n\n//go:embed x\nvar f string\n\nvar x = 1 //nolint:gosec // reason\n"
        self.assertIsNone(edit("internal/a/a.go", src))

    def test_go_important_allowed(self):
        self.assertIsNone(edit("internal/a/a.go", "package a\n// IMPORTANT: lock order matters\n"))

    def test_go_comment_markers_inside_strings_ignored(self):
        src = 'package a\nvar u = "https://x"\nvar r = `/* not */`\nvar c = \'/\'\n'
        self.assertIsNone(edit("internal/a/a.go", src))

    def test_existing_comment_not_flagged(self):
        self.assertIsNone(edit("internal/a/a.go", "// old\nx := 1\n", old="// old\n"))

    def test_ts_comment_blocked(self):
        self.assertIsNotNone(edit("apps/web/src/a.ts", "export const IX = 1;\n// hi\n"))

    def test_ts_directive_allowed(self):
        self.assertIsNone(edit("apps/web/src/a.ts", "// eslint-disable-next-line no-alert\nexport const A = 1;\n"))

    def test_markdown_not_checked(self):
        self.assertIsNone(edit("docs/a.md", "<!-- c -->\n", exists=True))


class GoRules(unittest.TestCase):
    def test_shell_exec_blocked(self):
        r = edit("apps/runner/x.go", 'package x\nfunc f(){ exec.CommandContext(ctx, "sh", "-c", s) }\n')
        self.assertIn("sh -c", r)

    def test_argv_exec_allowed(self):
        self.assertIsNone(edit("apps/runner/x.go", 'package x\nfunc f(){ exec.CommandContext(ctx, "git", "--", r) }\n'))

    def test_lowercase_json_tag_blocked(self):
        self.assertIn("PascalCase", edit("internal/m/m.go", 'package m\ntype A struct{ B int `json:"setupRequired"` }\n'))

    def test_pascal_json_tag_allowed(self):
        self.assertIsNone(edit("internal/m/m.go", 'package m\ntype A struct{ B int `json:"SetupRequired,omitempty"`; C int `json:"-"` }\n'))

    def test_sql_string_blocked(self):
        self.assertIn("sqlc", edit("internal/r/r.go", 'package r\nvar q = fmt.Sprintf("SELECT * FROM x WHERE id=%d", id)\n'))

    def test_panic_blocked_outside_tests(self):
        self.assertIn("panic", edit("internal/r/r.go", "package r\nfunc f(){ panic(err) }\n"))
        self.assertIsNone(edit("internal/r/r_test.go", "package r\nfunc f(){ panic(err) }\n"))

    def test_getenv_only_in_config(self):
        self.assertIn("config", edit("apps/server/internal/x/x.go", 'package x\nvar v = os.Getenv("A")\n'))
        self.assertIsNone(edit("apps/server/internal/config/c.go", 'package config\nvar v = os.Getenv("A")\n'))

    def test_fmt_print_blocked(self):
        self.assertIn("slog", edit("apps/server/internal/x/x.go", 'package x\nfunc f(){ fmt.Println("x") }\n'))


class TsRules(unittest.TestCase):
    def test_any_blocked(self):
        self.assertIn("any", edit("apps/web/src/a.ts", "export const X = (v: any) => v;\n"))

    def test_naming(self):
        self.assertIn("`I`", edit("apps/web/src/a.ts", "export interface Props { A: string }\n"))
        self.assertIn("`T`", edit("apps/web/src/a.ts", "export type Role = 'a' | 'b';\n"))
        self.assertIn("`E`", edit("apps/web/src/a.ts", "export enum Status { Live = 'live' }\n"))
        self.assertIsNone(edit("apps/web/src/a.ts", "export interface IProps { A: string }\nexport type TRole = 'a';\nexport enum EStatus { Live = 'live' }\n"))

    def test_console_blocked(self):
        self.assertIsNotNone(edit("apps/web/src/a.ts", "console.log(1);\n"))
        self.assertIsNone(edit("apps/web/src/a.test.ts", "console.log(1);\n"))

    def test_inner_html_and_eval_blocked(self):
        self.assertIsNotNone(edit("apps/web/src/a.tsx", "export const A = () => <div dangerouslySetInnerHTML={x} />;\n"))
        self.assertIsNotNone(edit("apps/web/src/a.ts", "const f = new Function('x');\n"))

    def test_function_component_blocked(self):
        self.assertIn("arrow", edit("apps/web/src/A.tsx", "export function Card() { return null; }\n"))

    def test_physical_direction_blocked(self):
        self.assertIn("logical", edit("apps/web/src/A.tsx", 'export const A = () => <div className="ml-2 text-left" />;\n'))
        self.assertIn("logical", edit("apps/web/src/a.css", ".x { margin-left: 4px; }\n"))
        self.assertIsNone(edit("apps/web/src/A.tsx", 'export const A = () => <div className="ms-2 text-start" />;\n'))
        self.assertIsNone(edit("apps/web/src/a.css", ".x { margin-inline-start: 4px; inset-inline-end: 0; }\n"))


class FileRules(unittest.TestCase):
    def test_generated_blocked(self):
        self.assertIn("generated", edit("apps/server/internal/api/gen/api.gen.go", "x", exists=True))
        self.assertIn("generated", edit("apps/web/pnpm-lock.yaml", "x", exists=True))

    def test_secret_blocked(self):
        self.assertIn("private key", edit("deploy/x.yaml", "-----BEGIN " + "OPENSSH PRIVATE KEY-----\n"))
        self.assertIn("GitHub token", edit("deploy/x.yaml", "token: ghp_" + "a" * 36 + "\n"))

    def test_env_file_blocked(self):
        self.assertIsNotNone(edit(".env", "A=1\n"))
        self.assertIsNone(edit(".env.example", "A=\n"))

    def test_markdown_location(self):
        self.assertIsNotNone(edit("apps/server/NOTES.md", "# x\n"))
        self.assertIsNone(edit("Plan/Progress/2026-10-28.md", "# x\n"))
        self.assertIsNone(edit("docs/new.md", "# x\n"))

    def test_length_limit(self):
        big = "package a\n" + "var x = 1\n" * 500
        self.assertIn("lines", edit("internal/a/a.go", "var y = 2\n", final=big))

    def test_sql_naming(self):
        self.assertIn("table names", edit("apps/server/migrations/00002_x.sql", "CREATE TABLE projects (\n"))
        self.assertIn("column names", edit("apps/server/migrations/00002_x.sql", 'CREATE TABLE "Projects" (\n  created_at timestamptz\n'))
        self.assertIsNone(edit("apps/server/migrations/00002_x.sql", 'CREATE TABLE "Projects" (\n  "CreatedAt" timestamptz NOT NULL\n);\n'))


class BashRules(unittest.TestCase):
    def test_ai_attribution_blocked(self):
        cmd = 'git commit -m "feat: x\n\nCo-Authored-By: Claude <noreply@anthropic.com>"'
        self.assertIn("AI attribution", bash_rules.evaluate(cmd, "/"))

    def test_heredoc_attribution_blocked(self):
        cmd = "git commit -F - <<'EOF'\nfix: y\n\nGenerated with Claude Code\nEOF"
        self.assertIsNotNone(bash_rules.evaluate(cmd, "/"))

    def test_arabic_message_blocked(self):
        self.assertIn("English", bash_rules.evaluate('git commit -m "fix: تصليح"', "/"))

    def test_conventional_subject(self):
        self.assertIn("Conventional", bash_rules.evaluate('git commit -m "update stuff"', "/"))
        self.assertIsNone(bash_rules.evaluate('git commit -s -m "feat(server): add health"', "/"))

    def test_no_verify_blocked(self):
        self.assertIsNotNone(bash_rules.evaluate('git commit --no-verify -m "fix: a"', "/"))
        self.assertIsNotNone(bash_rules.evaluate('git commit -nm "fix: a"', "/"))

    def test_force_push_main_blocked(self):
        self.assertIsNotNone(bash_rules.evaluate("git push --force origin main", "/"))
        self.assertIsNotNone(bash_rules.evaluate("git push origin +main", "/"))
        self.assertIsNone(bash_rules.evaluate("git push --force-with-lease origin m1/web", "/"))
        self.assertIsNone(bash_rules.evaluate("git push -n origin main", "/"))

    def test_prose_not_flagged(self):
        self.assertIsNone(bash_rules.evaluate('echo "never add Co-Authored-By: Claude"', "/"))

    def test_pr_body_checked(self):
        cmd = 'gh pr create --title "feat: x" --body "Generated with Claude Code"'
        self.assertIsNotNone(bash_rules.evaluate(cmd, "/"))


class EndToEnd(unittest.TestCase):
    def run_hook(self, script, payload):
        out = subprocess.run([sys.executable, os.path.join(HOOKS, script)], input=json.dumps(payload),
                             capture_output=True, text=True, check=True)
        return json.loads(out.stdout) if out.stdout.strip() else None

    def test_edit_hook_denies(self):
        with tempfile.TemporaryDirectory() as root:
            res = self.run_hook("guard_edit.py", {
                "cwd": root,
                "tool_input": {"file_path": os.path.join(root, "internal/a/a.go"), "content": "package a\n// x\n"},
            })
        self.assertEqual(res["hookSpecificOutput"]["permissionDecision"], "deny")

    def test_edit_hook_allows(self):
        with tempfile.TemporaryDirectory() as root:
            res = self.run_hook("guard_edit.py", {
                "cwd": root,
                "tool_input": {"file_path": os.path.join(root, "internal/a/a.go"), "content": "package a\n"},
            })
        self.assertIsNone(res)

    def test_bash_hook_denies(self):
        res = self.run_hook("guard_bash.py", {"cwd": "/", "tool_input": {"command": 'git commit -m "wip"'}})
        self.assertEqual(res["hookSpecificOutput"]["permissionDecision"], "deny")

    def test_malformed_payload_is_ignored(self):
        out = subprocess.run([sys.executable, os.path.join(HOOKS, "guard_bash.py")], input="not json",
                             capture_output=True, text=True, check=True)
        self.assertEqual(out.stdout.strip(), "")


if __name__ == "__main__":
    unittest.main()
