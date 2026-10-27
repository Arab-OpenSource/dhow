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


GOOD_TABLE = (
    'CREATE TABLE "Projects" (\n'
    '    "Id" uuid PRIMARY KEY DEFAULT uuidv7(),\n'
    '    "CreatedAt" timestamptz NOT NULL DEFAULT now(),\n'
    '    "UpdatedAt" timestamptz NOT NULL DEFAULT now(),\n'
    '    "IsDeleted" boolean NOT NULL DEFAULT false,\n'
    '    "Name" text NOT NULL\n'
    ');\n'
)
SERVER = "apps/server/internal/"


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
        self.assertIsNotNone(edit("apps/web/Src/Core/Ui/A.ts", "export const IX = 1;\n// hi\n"))

    def test_ts_directive_allowed(self):
        self.assertIsNone(edit("apps/web/Src/Core/Ui/A.ts", "// eslint-disable-next-line no-alert\nexport const A = 1;\n"))

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
        self.assertIn("config", edit("apps/server/internal/Core/Util/Util.go", 'package util\nvar v = os.Getenv("A")\n'))
        self.assertIsNone(edit("apps/server/internal/Config/Env.go", 'package config\nvar v = os.Getenv("A")\n'))

    def test_fmt_print_blocked(self):
        self.assertIn("slog", edit("apps/server/internal/Core/Util/Util.go", 'package util\nfunc f(){ fmt.Println("x") }\n'))


class TsRules(unittest.TestCase):
    def test_any_blocked(self):
        self.assertIn("any", edit("apps/web/Src/Core/Ui/A.ts", "export const X = (v: any) => v;\n"))

    def test_naming(self):
        self.assertIn("`I`", edit("apps/web/Src/Core/Ui/A.ts", "export interface Props { A: string }\n"))
        self.assertIn("`T`", edit("apps/web/Src/Core/Ui/A.ts", "export type Role = 'a' | 'b';\n"))
        self.assertIn("`E`", edit("apps/web/Src/Core/Ui/A.ts", "export enum Status { Live = 'live' }\n"))
        self.assertIsNone(edit("apps/web/Src/Core/Ui/A.ts", "export interface IProps { A: string }\nexport type TRole = 'a';\nexport enum EStatus { Live = 'live' }\n"))

    def test_console_blocked(self):
        self.assertIsNotNone(edit("apps/web/Src/Core/Ui/A.ts", "console.log(1);\n"))
        self.assertIsNone(edit("apps/web/Src/Core/Ui/A.test.ts", "console.log(1);\n"))

    def test_inner_html_and_eval_blocked(self):
        self.assertIsNotNone(edit("apps/web/Src/Core/Ui/A.tsx", "export const A = () => <div dangerouslySetInnerHTML={x} />;\n"))
        self.assertIsNotNone(edit("apps/web/Src/Core/Ui/A.ts", "const f = new Function('x');\n"))

    def test_function_component_blocked(self):
        self.assertIn("arrow", edit("apps/web/Src/Core/Ui/A.tsx", "export function Card() { return null; }\n"))

    def test_physical_direction_blocked(self):
        self.assertIn("logical", edit("apps/web/Src/Core/Ui/A.tsx", 'export const A = () => <div className="ml-2 text-left" />;\n'))
        self.assertIn("logical", edit("apps/web/Src/Styles/A.css", ".x { margin-left: 4px; }\n"))
        self.assertIsNone(edit("apps/web/Src/Core/Ui/A.tsx", 'export const A = () => <div className="ms-2 text-start" />;\n'))
        self.assertIsNone(edit("apps/web/Src/Styles/A.css", ".x { margin-inline-start: 4px; inset-inline-end: 0; }\n"))


class FileRules(unittest.TestCase):
    def test_generated_blocked(self):
        self.assertIn("generated", edit("apps/server/internal/Modules/System/Gen/System.gen.go", "x", exists=True))
        self.assertIn("generated", edit("apps/server/internal/Database/Sqlc/Instance.sql.go", "x", exists=True))
        self.assertIn("generated", edit("apps/server/internal/legacy/gen/api.gen.go", "x", exists=True))
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
        self.assertIn("table names", edit("apps/server/internal/Database/Migrations/00002_x.sql", "CREATE TABLE projects (\n"))
        self.assertIn("column names", edit("apps/server/internal/Database/Migrations/00002_x.sql", 'CREATE TABLE "Projects" (\n  created_at timestamptz\n'))
        self.assertIsNone(edit("apps/server/internal/Database/Migrations/00002_x.sql", GOOD_TABLE))


class ServerLayoutRules(unittest.TestCase):
    def test_allowed_locations(self):
        for rel in ("App/App.go", "Config/Env.go", "Core/Errors/AppError.go", "Database/Migrations/Migrator.go",
                    "Modules/System/System.Service.go", "Modules/System/Models/Instance.Model.go",
                    "Modules/System/Test/Unit/Service_test.go", "Core/I18n/I18n.Ar.go"):
            self.assertIsNone(edit(SERVER + rel, "package x\n"), rel)

    def test_unknown_top_level_folder_blocked(self):
        for rel in ("handlers/health.go", "services/Health.go", "Utils/Util.go", "Main.go"):
            self.assertIn("Modules/<Module>", edit(SERVER + rel, "package x\n"), rel)

    def test_files_directly_in_nested_layers_blocked(self):
        for rel in ("Core/Util.go", "Database/Util.go", "Modules/Util.go"):
            self.assertIn("named subfolder", edit(SERVER + rel, "package x\n"), rel)

    def test_folder_names_are_pascal_case(self):
        self.assertIn("PascalCase", edit(SERVER + "Core/eventbus/EventBus.go", "package x\n"))
        self.assertIn("PascalCase", edit(SERVER + "Modules/System/test/Unit/A_test.go", "package x\n"))
        self.assertIn("PascalCase", edit(SERVER + "Modules/system/System.Service.go", "package x\n"))

    def test_file_names_are_pascal_case(self):
        self.assertIn("PascalCase", edit(SERVER + "Modules/System/system.service.go", "package x\n"))
        self.assertIn("PascalCase", edit(SERVER + "Core/Errors/app_error.go", "package x\n"))
        self.assertIn("PascalCase", edit(SERVER + "App/server.go", "package x\n"))

    def test_test_and_generated_files_exempt_from_file_naming(self):
        self.assertIsNone(edit(SERVER + "Core/Errors/app_error_test.go", "package x\n"))
        self.assertIsNone(edit(SERVER + "Core/Errors/AppError_test.go", "package x\n"))

    def test_files_outside_server_internal_untouched(self):
        self.assertIsNone(edit("apps/server/cmd/dhow-server/main.go", "package main\n"))
        self.assertIsNone(edit("internal/logging/logging.go", "package logging\n"))


def imp(path):
    return 'package x\n\nimport "github.com/Arab-OpenSource/dhow/apps/server/internal/' + path + '"\n'


class ServerImportRules(unittest.TestCase):
    def test_module_cannot_import_another_module(self):
        r = edit(SERVER + "Modules/Billing/Billing.Service.go", imp("Modules/System/Models"))
        self.assertIn("EventBus", r)
        self.assertIn("Modules/Billing must not import Modules/System", r)

    def test_module_may_import_itself_and_core_and_database(self):
        for path in ("Modules/Billing/Models", "Core/Errors", "Database/Sqlc", "Config"):
            self.assertIsNone(edit(SERVER + "Modules/Billing/Billing.Service.go", imp(path)), path)

    def test_core_cannot_import_modules_or_app(self):
        self.assertIn("Only App imports modules", edit(SERVER + "Core/Response/Response.Helper.go", imp("Modules/System")))
        self.assertIn("composition root", edit(SERVER + "Core/Response/Response.Helper.go", imp("App")))

    def test_core_may_import_core(self):
        self.assertIsNone(edit(SERVER + "Core/Response/Response.Helper.go", imp("Core/Errors")))

    def test_config_cannot_import_core_modules_or_app(self):
        for path in ("Core/Errors", "Modules/System", "App"):
            self.assertIsNotNone(edit(SERVER + "Config/Env.go", imp(path)), path)

    def test_database_cannot_import_modules_core_or_app(self):
        for path in ("Modules/System", "Core/Errors", "App"):
            self.assertIsNotNone(edit(SERVER + "Database/Migrations/Migrator.go", imp(path)), path)

    def test_only_app_imports_modules(self):
        self.assertIsNone(edit(SERVER + "App/App.go", imp("Modules/System")))
        self.assertIsNone(edit(SERVER + "App/App.go", imp("Core/Middleware")))
        self.assertIsNotNone(edit(SERVER + "Config/Logger.go", imp("Modules/System")))
        self.assertIsNotNone(edit(SERVER + "Database/Migrations/Migrator.go", imp("Modules/System")))

    def test_modules_cannot_import_app(self):
        self.assertIn("composition root", edit(SERVER + "Modules/System/System.Routes.go", imp("App")))

    def test_aliased_and_grouped_imports_are_scanned(self):
        src = ('package x\n\nimport (\n\t"fmt"\n\tapperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System"\n)\n')
        self.assertIsNotNone(edit(SERVER + "Core/Errors/AppError.go", src))

    def test_existing_imports_are_not_rescanned(self):
        line = '\t"github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System"\n'
        self.assertIsNone(edit(SERVER + "Core/Errors/AppError.go", "x := 1\n" + line, old=line))

    def test_test_files_and_test_folders_are_exempt(self):
        self.assertIsNone(edit(SERVER + "Core/Errors/AppError_test.go", imp("Modules/System")))
        self.assertIsNone(edit(SERVER + "Modules/System/Test/Support/Fakes.go", imp("App")))

    def test_other_module_imports_are_not_flagged_outside_the_server(self):
        self.assertIsNone(edit("internal/buildinfo/x.go", imp("Modules/System")))


class DatabaseRules(unittest.TestCase):
    MIGRATION = "apps/server/internal/Database/Migrations/00002_projects.sql"

    def test_table_with_base_columns_allowed(self):
        self.assertIsNone(edit(self.MIGRATION, GOOD_TABLE))

    def test_instance_singleton_is_the_only_exception(self):
        singleton = 'CREATE TABLE "Instance" (\n    "Id" smallint PRIMARY KEY DEFAULT 1,\n    "CreatedAt" timestamptz NOT NULL DEFAULT now()\n);\n'
        self.assertIsNone(edit(self.MIGRATION, singleton))
        self.assertIn("base column", edit(self.MIGRATION, singleton.replace("Instance", "Settings")))

    def test_each_base_column_is_required(self):
        for column in ('"Id" uuid PRIMARY KEY DEFAULT uuidv7()', '"CreatedAt" timestamptz NOT NULL DEFAULT now()',
                       '"UpdatedAt" timestamptz NOT NULL DEFAULT now()', '"IsDeleted" boolean NOT NULL DEFAULT false'):
            broken = GOOD_TABLE.replace("    " + column + ",\n", "")
            self.assertIn(column, edit(self.MIGRATION, broken), column)

    def test_base_column_types_are_checked(self):
        self.assertIn("uuidv7", edit(self.MIGRATION, GOOD_TABLE.replace("uuid PRIMARY KEY DEFAULT uuidv7()", "bigserial PRIMARY KEY")))

    def test_translation_tables_follow_the_same_rule(self):
        translations = GOOD_TABLE.replace("Projects", "ProjectTranslations").replace('"Name" text NOT NULL', '"Locale" text NOT NULL')
        self.assertIsNone(edit(self.MIGRATION, translations))
        self.assertIsNotNone(edit(self.MIGRATION, 'CREATE TABLE "ProjectTranslations" (\n  "Locale" text\n);\n'))

    def test_existing_table_not_rechecked_on_edit(self):
        old = 'CREATE TABLE "Legacy" (\n  "Id" int\n);\n'
        self.assertIsNone(edit(self.MIGRATION, old + "\n", old=old))

    def test_timestamp_without_time_zone_blocked(self):
        self.assertIn("timestamptz", edit(self.MIGRATION, GOOD_TABLE.replace('"Name" text NOT NULL', '"Seen" timestamp NOT NULL')))
        self.assertIn("timestamptz", edit(self.MIGRATION, GOOD_TABLE.replace('"Name" text NOT NULL', '"Seen" TIMESTAMP(3)')))
        self.assertIsNone(edit(self.MIGRATION, GOOD_TABLE.replace('"Name" text NOT NULL', '"Seen" timestamp with time zone')))

    def test_varchar_blocked(self):
        self.assertIn("TEXT", edit(self.MIGRATION, GOOD_TABLE.replace('"Name" text NOT NULL', '"Name" VARCHAR(50) NOT NULL')))
        self.assertIn("TEXT", edit(self.MIGRATION, GOOD_TABLE.replace('"Name" text NOT NULL', '"Name" character varying(50)')))

    def test_ar_en_columns_blocked(self):
        for column in ("NameAr", "NameEn", "DescriptionAr"):
            r = edit(self.MIGRATION, GOOD_TABLE.replace('"Name" text NOT NULL', '"%s" text NOT NULL' % column))
            self.assertIn("Translations", r, column)
        self.assertIsNone(edit(self.MIGRATION, GOOD_TABLE.replace('"Name" text NOT NULL', '"Token" text NOT NULL')))

    def test_delete_from_in_module_queries_blocked(self):
        path = "apps/server/internal/Modules/System/Queries/Project.sql"
        self.assertIn("soft", edit(path, '-- name: DeleteProject :exec\nDELETE FROM "Projects" WHERE "Id" = $1;\n'))
        self.assertIsNone(edit(path, '-- name: DeleteProject :exec\nUPDATE "Projects" SET "IsDeleted" = true WHERE "Id" = $1;\n'))
        self.assertIsNone(edit("apps/server/internal/Database/Migrations/00003_x.sql", 'DELETE FROM "Instance" WHERE "Id" = 2;\n'))


class OpenapiRules(unittest.TestCase):
    PATH = "packages/openapi/openapi.yaml"

    def test_kebab_case_paths_allowed(self):
        self.assertIsNone(edit(self.PATH, "paths:\n  /health/live:\n    get: {}\n  /projects/{ProjectId}/deploy-keys:\n    get: {}\n"))

    def test_non_kebab_segments_blocked(self):
        for path in ("/healthLive", "/deploy_keys", "/Projects", "/projects/deployKeys"):
            r = edit(self.PATH, "paths:\n  %s:\n    get: {}\n" % path)
            self.assertIn("kebab-case", r, path)

    def test_other_keys_are_not_treated_as_paths(self):
        self.assertIsNone(edit(self.PATH, "servers:\n  - url: /api/v1\ncomponents:\n  schemas:\n    PageMeta: {}\n"))


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
