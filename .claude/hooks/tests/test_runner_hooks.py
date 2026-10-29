import os
import sys
import unittest

HOOKS = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, HOOKS)

from dhow_hooks import edit_rules

RUNNER = "apps/runner/internal/"
ROOT = os.path.dirname(os.path.dirname(HOOKS))


def edit(rel, added, old=""):
    return edit_rules.evaluate(rel=rel, root="/nonexistent", segments=[(added, old)], exists=False, final_content=added)


def imp(path):
    return 'package x\n\nimport "github.com/Khaymah/dhow/apps/runner/internal/' + path + '"\n'


class RunnerLayoutRules(unittest.TestCase):
    def test_allowed_locations(self):
        for rel in ("App/App.go", "App/App.Run.go", "Installer/Installer.go", "Installer/Messages.Ar.go", "Config/Config.go", "Identity/Identity.go", "Client/Client.Auth.go",
                    "Executor/Executor.go", "Journal/Journal.go", "Masking/Masker.go", "Logs/Logs.Spill.go",
                    "Sandbox/Sandbox.go", "Sandbox/Fake/Fake.go", "Sandbox/Podman/Podman.Inspect.go", "Sandbox/Wire/Wire.go",
                    "Agent/Agent.go", "Executor/executor_test.go"):
            self.assertIsNone(edit(RUNNER + rel, "package x\n"), rel)

    def test_unknown_layer_blocked(self):
        for rel in ("Utils/Util.go", "agent/Agent.go", "Main.go", "Podman.go"):
            self.assertIn("lives in one of", edit(RUNNER + rel, "package x\n"), rel)

    def test_folder_names_are_pascal_case(self):
        self.assertIn("PascalCase", edit(RUNNER + "Sandbox/fake/Fake.go", "package x\n"))
        self.assertIn("PascalCase", edit(RUNNER + "Logs/spill_files/Spill.go", "package x\n"))

    def test_file_names_are_pascal_case(self):
        self.assertIn("PascalCase", edit(RUNNER + "Executor/executor.go", "package x\n"))
        self.assertIn("PascalCase", edit(RUNNER + "Logs/spill_file.go", "package x\n"))

    def test_test_files_keep_go_naming(self):
        self.assertIsNone(edit(RUNNER + "Masking/masker_fuzz_test.go", "package x\n"))

    def test_command_and_other_trees_untouched(self):
        self.assertIsNone(edit("apps/runner/cmd/dhow-runner/main.go", "package main\n"))
        self.assertIsNone(edit("internal/jobspec/x.go", 'package x\n\nimport "github.com/Khaymah/dhow/internal/runnersig"\n'))


class SharedInternalRules(unittest.TestCase):
    def test_shared_packages_never_import_apps(self):
        for rel in ("internal/jobspec/x.go", "internal/installer/steps.go", "internal/installer/fakesys/fakesys.go"):
            for target in ("apps/runner/internal/Executor", "apps/server/internal/Config"):
                reason = edit(rel, 'package x\n\nimport "github.com/Khaymah/dhow/' + target + '"\n')
                self.assertIn("never import apps/", reason or "", rel)

    def test_shared_tests_and_shared_imports_pass(self):
        self.assertIsNone(edit("internal/installer/steps.go", 'package x\n\nimport "github.com/Khaymah/dhow/internal/buildinfo"\n'))

    def test_the_real_shared_tree_passes(self):
        for folder, _, names in os.walk(os.path.join(ROOT, "internal")):
            for name in names:
                if not name.endswith(".go"):
                    continue
                path = os.path.join(folder, name)
                rel = os.path.relpath(path, ROOT).replace(os.sep, "/")
                with open(path, encoding="utf-8") as fh:
                    self.assertIsNone(edit(rel, fh.read()), rel)


class RunnerImportRules(unittest.TestCase):
    def test_app_imports_every_layer(self):
        for layer in ("Config", "Identity", "Client", "Executor", "Journal", "Masking", "Logs", "Sandbox", "Sandbox/Fake"):
            self.assertIsNone(edit(RUNNER + "App/App.go", imp(layer)), layer)

    def test_executor_imports_the_layers_it_drives(self):
        for layer in ("Client", "Identity", "Journal", "Logs", "Masking", "Sandbox", "Config"):
            self.assertIsNone(edit(RUNNER + "Executor/Executor.go", imp(layer)), layer)

    def test_nothing_imports_app(self):
        for src in ("Executor/Executor.go", "Client/Client.go", "Config/Config.go", "Sandbox/Fake/Fake.go"):
            self.assertIn("composition root", edit(RUNNER + src, imp("App")), src)

    def test_only_app_imports_executor(self):
        for src in ("Client/Client.go", "Logs/Logs.go", "Sandbox/Sandbox.go"):
            self.assertIn("forbidden import", edit(RUNNER + src, imp("Executor")), src)

    def test_leaf_layers_stay_leaves(self):
        cases = (
            ("Masking/Masker.go", "Config"), ("Config/Config.go", "Identity"), ("Journal/Journal.go", "Client"),
            ("Sandbox/Sandbox.go", "Logs"), ("Logs/Logs.go", "Client"), ("Client/Client.go", "Journal"),
            ("Identity/Identity.go", "Client"), ("Sandbox/Fake/Fake.go", "Masking"),
        )
        for src, dst in cases:
            self.assertIn("forbidden import", edit(RUNNER + src, imp(dst)), src + " -> " + dst)

    def test_allowed_lower_layers(self):
        cases = (("Logs/Logs.go", "Masking"), ("Client/Client.go", "Identity"), ("Identity/Identity.go", "Config"),
                 ("Sandbox/Fake/Fake.go", "Sandbox"), ("Sandbox/Fake/Fake.go", "Config"), ("Journal/Journal.go", "Config"))
        for src, dst in cases:
            self.assertIsNone(edit(RUNNER + src, imp(dst)), src + " -> " + dst)

    def test_the_installer_sees_config_and_identity_only(self):
        self.assertIsNone(edit(RUNNER + "App/App.go", imp("Installer")))
        for dst in ("Config", "Identity"):
            self.assertIsNone(edit(RUNNER + "Installer/Installer.go", imp(dst)), dst)
        for dst in ("Client", "Executor", "Journal", "Sandbox", "App"):
            self.assertIsNotNone(edit(RUNNER + "Installer/Installer.go", imp(dst)), dst)
        for src in ("Executor/Executor.go", "Client/Client.go", "Identity/Identity.go"):
            self.assertIn("forbidden import", edit(RUNNER + src, imp("Installer")), src)

    def test_the_agent_imports_only_the_sandbox_wire(self):
        self.assertIsNone(edit(RUNNER + "Agent/Agent.go", imp("Sandbox/Wire")))
        self.assertIsNone(edit(RUNNER + "App/App.go", imp("Agent")))
        for dst in ("Config", "Executor", "Client", "Identity", "Journal", "Logs", "Masking"):
            self.assertIn("forbidden import", edit(RUNNER + "Agent/Agent.go", imp(dst)), dst)

    def test_nothing_but_app_imports_the_agent(self):
        for src in ("Executor/Executor.go", "Sandbox/Podman/Podman.go", "Sandbox/Sandbox.go", "Config/Config.go"):
            self.assertIn("forbidden import", edit(RUNNER + src, imp("Agent")), src)

    def test_the_podman_driver_stays_a_sandbox_leaf(self):
        for dst in ("Sandbox", "Sandbox/Wire", "Config"):
            self.assertIsNone(edit(RUNNER + "Sandbox/Podman/Podman.go", imp(dst)), dst)
        for dst in ("Masking", "Logs", "Executor"):
            self.assertIn("forbidden import", edit(RUNNER + "Sandbox/Podman/Podman.go", imp(dst)), dst)

    def test_the_egress_proxy_is_a_leaf_the_executor_drives(self):
        self.assertIsNone(edit(RUNNER + "Egress/Egress.go", "package egress\n"))
        self.assertIsNone(edit(RUNNER + "Executor/Executor.go", imp("Egress")))
        for dst in ("Sandbox", "Logs", "Config", "Executor"):
            self.assertIn("forbidden import", edit(RUNNER + "Egress/Egress.go", imp(dst)), dst)
        for src in ("Sandbox/Podman/Podman.go", "Agent/Agent.go", "Logs/Logs.go"):
            self.assertIn("forbidden import", edit(RUNNER + src, imp("Egress")), src)

    def test_unknown_runner_package_blocked(self):
        self.assertIn("not a runner layer", edit(RUNNER + "App/App.go", imp("Utils")))

    def test_server_code_is_never_imported(self):
        src = 'package x\n\nimport "github.com/Khaymah/dhow/apps/server/internal/Config"\n'
        self.assertIn("never imports server code", edit(RUNNER + "Client/Client.go", src))

    def test_grouped_and_aliased_imports_are_scanned(self):
        src = ('package x\n\nimport (\n\t"fmt"\n\texec "github.com/Khaymah/dhow/apps/runner/internal/Executor"\n)\n')
        self.assertIn("forbidden import", edit(RUNNER + "Logs/Logs.go", src))

    def test_existing_imports_are_not_rescanned(self):
        line = '\t"github.com/Khaymah/dhow/apps/runner/internal/Executor"\n'
        self.assertIsNone(edit(RUNNER + "Logs/Logs.go", "x := 1\n" + line, old=line))

    def test_tests_are_exempt(self):
        self.assertIsNone(edit(RUNNER + "Logs/logs_test.go", imp("Executor")))

    def test_the_real_runner_tree_passes(self):
        base = os.path.join(ROOT, RUNNER)
        if not os.path.isdir(base):
            self.skipTest("apps/runner/internal is not in this checkout")
        checked = 0
        for folder, _, files in os.walk(base):
            for name in files:
                if not name.endswith(".go"):
                    continue
                path = os.path.join(folder, name)
                rel = os.path.relpath(path, ROOT).replace(os.sep, "/")
                with open(path, encoding="utf-8") as fh:
                    content = fh.read()
                self.assertIsNone(edit_rules.evaluate(rel=rel, root=ROOT, segments=[(content, "")], exists=True,
                                                      final_content=content), rel)
                checked += 1
        self.assertGreater(checked, 0)


if __name__ == "__main__":
    unittest.main()
