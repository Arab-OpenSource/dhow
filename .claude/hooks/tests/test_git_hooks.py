import os
import sys
import unittest

HOOKS = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, HOOKS)

from dhow_hooks import bash_rules, common

PREFIXES = (
    'git -c user.name="Sabry Dawood" -c user.email=a@b.c',
    "git -c commit.gpgsign=false -C /tmp",
    "git -C '/tmp/a dir' --no-pager",
    "GIT_EDITOR=true git --no-pager -c core.quotepath=off",
)


class GlobalOptionsBeforeCommit(unittest.TestCase):
    def test_a_bad_subject_is_refused(self):
        for prefix in PREFIXES:
            reason = bash_rules.evaluate(prefix + ' commit -s -m "update stuff"', "/") or ""
            self.assertIn("Conventional", reason, prefix)

    def test_an_arabic_message_is_refused(self):
        for prefix in PREFIXES:
            reason = bash_rules.evaluate(prefix + ' commit -m "fix: تصليح"', "/") or ""
            self.assertIn("English", reason, prefix)

    def test_ai_attribution_is_refused(self):
        trailer = "Co-Authored-By: " + "Claude <noreply@" + "anthropic.com>"
        for prefix in PREFIXES:
            reason = bash_rules.evaluate(prefix + ' commit -m "feat: x\n\n' + trailer + '"', "/") or ""
            self.assertIn("AI attribution", reason, prefix)

    def test_skipping_hooks_is_refused(self):
        for prefix in PREFIXES:
            self.assertIsNotNone(bash_rules.evaluate(prefix + ' commit --no-verify -m "fix: a"', "/"), prefix)
            self.assertIsNotNone(bash_rules.evaluate(prefix + ' commit -snm "fix: a"', "/"), prefix)

    def test_a_good_message_is_allowed(self):
        for prefix in PREFIXES:
            cmd = prefix + ' commit -s -m "fix(hooks): match global options" -m "Body with -n inside."'
            self.assertIsNone(bash_rules.evaluate(cmd, "/"), prefix)

    def test_dash_c_directory_after_config_flags_sets_the_workdir(self):
        cmd = 'git -c user.name="A B" -C "/srv/a repo" commit -m "fix: x"'
        self.assertEqual(bash_rules.workdir(cmd, "/"), "/srv/a repo")
        self.assertEqual(bash_rules.workdir("git -C rel commit -m x", "/base"), "/base/rel")
        self.assertEqual(bash_rules.workdir('git -c a="-C /x" commit -m x', "/base"), "/base")


class GlobalOptionsBeforePush(unittest.TestCase):
    def test_force_push_to_main_is_refused(self):
        self.assertIsNotNone(bash_rules.evaluate("git -c http.version=HTTP/1.1 push --force origin main", "/"))

    def test_no_verify_is_refused(self):
        self.assertIsNotNone(bash_rules.evaluate("git -C /tmp push --no-verify origin m1/web", "/"))

    def test_a_branch_push_is_allowed(self):
        self.assertIsNone(bash_rules.evaluate("git -C /tmp -c a=b push -u origin fix/followups-2", "/"))


class MentionedCommits(unittest.TestCase):
    def test_a_commit_inside_a_string_is_not_checked(self):
        for cmd in ('echo "git -c user.name=x commit -m wip"',
                    "echo 'run git -C /tmp commit -m update stuff; done'",
                    'printf "%s" "a; git -c a=b commit -m \\"wip\\""',
                    'grep -n "git commit -m wip" notes.txt',
                    "echo git commit -m wip"):
            self.assertIsNone(bash_rules.evaluate(cmd, "/"), cmd)

    def test_the_shared_matcher(self):
        commit = common.git_subcommand("commit")
        self.assertIsNotNone(common.find_git(commit, 'git -c a="b c" -C d --no-pager commit -m x'))
        self.assertIsNotNone(common.find_git(commit, 'cd a && git -c a=b commit'))
        self.assertIsNone(common.find_git(commit, 'git -c a=b log --grep commit'))
        self.assertIsNone(common.find_git(commit, 'echo "x; git commit"'))


if __name__ == "__main__":
    unittest.main()
