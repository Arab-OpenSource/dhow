import os
import subprocess
import sys
import tempfile
import unittest

HOOKS = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, HOOKS)

from dhow_hooks import arabic_rules, bash_rules, docs_rules, edit_rules

ROOT = os.path.dirname(os.path.dirname(HOOKS))
COMMIT = 'git commit -s -m "docs: update the guide"'


def git(repo, *args):
    subprocess.run(["git", "-C", repo, *args], capture_output=True, check=True)


def write(repo, rel, text="# x\n"):
    path = os.path.join(repo, rel)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)


class Repo:
    def __enter__(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.path = self.tmp.name
        git(self.path, "init", "-q")
        git(self.path, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init")
        return self

    def __exit__(self, *exc):
        self.tmp.cleanup()

    def add(self, *rels, text="# x\n"):
        for rel in rels:
            write(self.path, rel, text)
        git(self.path, "add", "--", *rels)

    def commit(self):
        git(self.path, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "c")


class DocsPairCommitRule(unittest.TestCase):
    def test_counterparts(self):
        self.assertEqual(docs_rules.counterpart("docs/install/compose.md"), "docs/en/install/compose.md")
        self.assertEqual(docs_rules.counterpart("docs/en/install/compose.md"), "docs/install/compose.md")
        self.assertEqual(docs_rules.counterpart("README.md"), "README.en.md")
        self.assertEqual(docs_rules.counterpart("SECURITY.en.md"), "SECURITY.md")
        for rel in ("docs/logo.png", "Plan/Status.md", "CLAUDE.md", "apps/web/README.md", "LICENSE.md"):
            self.assertIsNone(docs_rules.counterpart(rel), rel)

    def test_an_arabic_page_alone_is_refused_and_names_the_english_one(self):
        with Repo() as repo:
            repo.add("docs/guides/jobs.md")
            reason = bash_rules.evaluate(COMMIT, repo.path)
        self.assertIn("docs/en/guides/jobs.md", reason)
        self.assertIn("ADR/0064", reason)

    def test_an_english_page_alone_is_refused_and_names_the_arabic_one(self):
        with Repo() as repo:
            repo.add("docs/en/guides/jobs.md")
            self.assertIn("without docs/guides/jobs.md", bash_rules.evaluate(COMMIT, repo.path))

    def test_both_versions_together_pass(self):
        with Repo() as repo:
            repo.add("docs/guides/jobs.md", "docs/en/guides/jobs.md", "apps/x.go")
            self.assertIsNone(bash_rules.evaluate(COMMIT, repo.path))

    def test_root_files_pair_with_their_en_md(self):
        with Repo() as repo:
            repo.add("CONTRIBUTING.md")
            self.assertIn("CONTRIBUTING.en.md", bash_rules.evaluate(COMMIT, repo.path))
            repo.add("CONTRIBUTING.en.md")
            self.assertIsNone(bash_rules.evaluate(COMMIT, repo.path))

    def test_a_deleted_page_needs_its_counterpart_deleted(self):
        with Repo() as repo:
            repo.add("docs/old.md", "docs/en/old.md")
            repo.commit()
            git(repo.path, "rm", "-q", "docs/old.md")
            self.assertIn("docs/en/old.md", bash_rules.evaluate(COMMIT, repo.path))
            git(repo.path, "rm", "-q", "docs/en/old.md")
            self.assertIsNone(bash_rules.evaluate(COMMIT, repo.path))

    def test_a_pure_move_into_docs_en_counts_as_paired(self):
        with Repo() as repo:
            repo.add("docs/install/compose.md", "README.md", text="# Install\n\nSome English text.\n" * 5)
            repo.commit()
            os.makedirs(os.path.join(repo.path, "docs/en/install"))
            git(repo.path, "mv", "docs/install/compose.md", "docs/en/install/compose.md")
            git(repo.path, "mv", "README.md", "README.en.md")
            self.assertIsNone(bash_rules.evaluate(COMMIT, repo.path))

    def test_a_rename_inside_one_version_is_not_paired(self):
        with Repo() as repo:
            repo.add("docs/a.md", "docs/en/a.md", text="# A\n\nSome text that git can follow.\n" * 5)
            repo.commit()
            git(repo.path, "mv", "docs/a.md", "docs/b.md")
            reason = bash_rules.evaluate(COMMIT, repo.path)
        self.assertIn("docs/a.md without docs/en/a.md", reason)
        self.assertIn("docs/b.md without docs/en/b.md", reason)

    def test_other_files_are_not_checked(self):
        with Repo() as repo:
            repo.add("Plan/Status.md", "docs/diagram.png", "apps/web/Src/Main.tsx", "CLAUDE.md")
            self.assertIsNone(bash_rules.evaluate(COMMIT, repo.path))

    def test_commit_all_sees_unstaged_changes(self):
        with Repo() as repo:
            repo.add("docs/a.md", "docs/en/a.md")
            repo.commit()
            write(repo.path, "docs/a.md", "# changed\n")
            self.assertIsNone(bash_rules.evaluate(COMMIT, repo.path))
            self.assertIn("docs/en/a.md", bash_rules.evaluate('git commit -a -m "docs: x"', repo.path))
            self.assertIn("docs/en/a.md", bash_rules.evaluate('git commit -sam "docs: x"', repo.path))
            self.assertIsNone(bash_rules.evaluate('git commit -s -m "docs: a message with -a inside"', repo.path))

    def test_config_flags_cd_and_dash_c_directory_forms(self):
        with Repo() as repo:
            repo.add("docs/a.md")
            for cmd in ('git -c user.name="A B" -c user.email=a@b.c commit -s -m "docs: x"',
                        'cd ' + repo.path + ' && git commit -m "docs: x"',
                        'git -C ' + repo.path + ' commit -m "docs: x"'):
                cwd = "/" if repo.path in cmd else repo.path
                self.assertIn("docs/en/a.md", bash_rules.evaluate(cmd, cwd) or "", cmd)

    def test_outside_a_repository_and_other_commands_pass(self):
        with tempfile.TemporaryDirectory() as plain:
            self.assertIsNone(bash_rules.evaluate(COMMIT, plain))
        with Repo() as repo:
            repo.add("docs/a.md")
            self.assertIsNone(bash_rules.evaluate("git status", repo.path))
            self.assertIsNone(bash_rules.evaluate('echo "git commit"', repo.path))


def edit(rel, added, old="", final=None):
    return edit_rules.evaluate(rel=rel, root="/nonexistent", segments=[(added, old)], exists=False,
                               final_content=final if final is not None else added)


class ArabicDocsRegister(unittest.TestCase):
    def test_colloquial_arabic_in_docs_is_refused(self):
        for rel in ("docs/install/compose.md", "docs/guides/jobs.md", "README.md", "CONTRIBUTING.md", "SECURITY.md"):
            reason = edit(rel, "# العنوان\n\nالسيرفر مش بيشتغل دلوقتي.\n")
            self.assertIn("ADR/0062", reason or "", rel)

    def test_code_blocks_and_inline_code_are_skipped(self):
        page = "# العنوان\n\nاكتب `مش` كما هي.\n\n```text\nمش دلوقتي\n```\n"
        self.assertIsNone(edit("docs/install/compose.md", page))
        self.assertIsNone(edit("docs/install/compose.md", "مش دلوقتي\n", final=page))

    def test_english_mirror_plan_and_claude_md_are_not_checked(self):
        for rel in ("docs/en/install/compose.md", "README.en.md", "CONTRIBUTING.en.md", "CLAUDE.md",
                    "Plan/Status.md", "Plan/ADR/0064-arabic-first-documentation.md", "docs/logo.svg"):
            self.assertNotIn("ADR/0062", edit(rel, "الحالة دلوقتي مش تمام.\n") or "", rel)

    def test_simple_arabic_docs_pass(self):
        self.assertIsNone(edit("docs/install/compose.md", "# البدء السريع\n\nلم نتمكن من الاتصال. حاول مرة أخرى.\n"))

    def test_every_arabic_doc_in_the_repository_is_simple_arabic(self):
        found = [rel for rel in ("README.md", "CONTRIBUTING.md", "SECURITY.md") if os.path.exists(os.path.join(ROOT, rel))]
        for folder, _, names in os.walk(os.path.join(ROOT, "docs")):
            for name in names:
                rel = os.path.relpath(os.path.join(folder, name), ROOT).replace(os.sep, "/")
                if arabic_rules.is_arabic_doc(rel):
                    found.append(rel)
        self.assertIn("docs/install/configuration.md", found)
        for rel in found:
            with open(os.path.join(ROOT, rel), encoding="utf-8") as fh:
                text = fh.read()
            self.assertIsNone(arabic_rules.check_arabic_register(rel=rel, added=text, old="", final_content=text), rel)


if __name__ == "__main__":
    unittest.main()
