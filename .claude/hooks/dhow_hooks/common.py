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
