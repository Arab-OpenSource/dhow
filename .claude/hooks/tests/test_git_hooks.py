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
    "command git",
    "env GIT_EDITOR=true A=1 git",
    "env -i git",
    "time git",
    "sudo git",
    "sudo -u root -E git",
    "\\git",
    "/usr/bin/git",
    "./bin/git",
    "git -p",
    "git --bare",
    "git --namespace=x",
    "git --exec-path=/usr/lib/git-core",
    "git -cfoo=bar",
    "git -c foo=bar -C /tmp -p",
    "git --git-dir /tmp/.git --work-tree /tmp",
)

WRAPPED = (
    "{{ {cmd}; }}",
    "! {cmd}",
    "`{cmd}`",
    'echo "$({cmd})"',
    "true && {cmd}",
    "if true; then {cmd}; fi",
)


class WaysToStartGit(unittest.TestCase):
    def test_every_start_is_checked(self):
        for wrap in WRAPPED:
            for prefix in ("git", "env A=1 git", "/usr/bin/git -p"):
                cmd = wrap.format(cmd=prefix + " commit -m 'update stuff'")
                self.assertIn("Conventional", bash_rules.evaluate(cmd, "/") or "", cmd)

    def test_a_config_key_named_commit_is_not_the_subcommand(self):
        self.assertIsNone(bash_rules.evaluate("git -c commit.gpgsign=false log -m", "/"))
        self.assertIsNone(bash_rules.evaluate("mygit commit -m 'update stuff'", "/"))
        self.assertIsNone(bash_rules.evaluate("gitx commit -m 'update stuff'", "/"))


class HooksPathSkipsHooks(unittest.TestCase):
    def test_hooks_path_is_refused_on_commit_and_push(self):
        for cmd in ('git -c core.hooksPath=/dev/null commit -m "fix: a"',
                    'git -ccore.hookspath=/tmp commit -m "fix: a"',
                    'git -c core.hooksPath="" -C /tmp commit -m "fix: a"',
                    'GIT_CONFIG_PARAMETERS="\'core.hooksPath\'=\'/x\'" git commit -m "fix: a"',
                    'GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/x git commit -m "fix: a"',
                    "git -c core.hooksPath=/dev/null push origin fix/a",
                    "sudo git -c core.HooksPath=/x push -u origin fix/a"):
            self.assertEqual("Never skip git hooks (--no-verify).", bash_rules.evaluate(cmd, "/"), cmd)

    def test_other_config_is_allowed(self):
        self.assertIsNone(bash_rules.evaluate('git -c core.quotepath=off commit -m "fix: a"', "/"))
        self.assertIsNone(bash_rules.evaluate("git -c http.version=HTTP/1.1 push origin fix/a", "/"))


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
