import re

TS_EXT = (".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs")

GENERATED_PATH = re.compile(
    r"(^|/)(gen|sqlc)/|\.gen\.go$|_gen\.go$|"
    r"(^|/)apps/web/Src/Core/Api/Schema\.d\.ts$|(^|/)apps/web/Src/Styles/Tokens\.css$|"
    r"(^|/)(go\.sum|pnpm-lock\.yaml)$", re.IGNORECASE)


def is_ts(path):
    return path.lower().endswith(TS_EXT) and not path.endswith(".d.ts")


def is_test(path):
    return path.endswith("_test.go") or re.search(r"\.(test|spec)\.[cm]?[jt]sx?$", path) is not None


def new_lines(added, old):
    old_set = {ln.strip() for ln in old.splitlines()}
    return [ln for ln in added.splitlines() if ln.strip() and ln.strip() not in old_set]


CMD_START = r"(?:^|[\n;&|(]|&&|\|\||\bthen\b|\bdo\b|\belse\b)\s*"
SHELL_VALUE = r"(?:[^\s\"';&|]|\"[^\"]*\"|'[^']*')+"
GIT_GLOBAL_OPTION = (r"(?:-c\s+" + SHELL_VALUE + r"|-C\s+" + SHELL_VALUE + r"|--git-dir[= ]" + SHELL_VALUE +
                     r"|--work-tree[= ]" + SHELL_VALUE + r"|--no-pager|-P|--no-replace-objects|--literal-pathspecs)")
GIT_PREFIX = CMD_START + r"(?:\w+=\S+\s+)*git((?:\s+" + GIT_GLOBAL_OPTION + r")*)\s+"
QUOTED_STRING = re.compile(r"\"(?:[^\"\\]|\\.)*\"|'[^']*'")


def mask_strings(command):
    return QUOTED_STRING.sub(lambda m: m.group(0)[0] + "x" * (len(m.group(0)) - 2) + m.group(0)[-1], command)


def git_subcommand(subcommand):
    return re.compile(GIT_PREFIX + r"(?:" + subcommand + r")\b")


def find_git(pattern, command):
    return pattern.search(mask_strings(command))


def git_global_options(command, match):
    return command[match.start(1):match.end(1)]
