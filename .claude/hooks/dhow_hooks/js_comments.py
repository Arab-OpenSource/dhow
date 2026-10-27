import re

CODE_EXT = (".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs")

DIRECTIVE = re.compile(
    r"^/{2,}\s*(@ts-(expect-error|ignore|nocheck|check)\b|<reference\b|eslint-|"
    r"prettier-ignore\b|biome-ignore\b|c8 ignore\b|v8 ignore\b|istanbul ignore\b|"
    r"@jsxImportSource\b|@vite-ignore\b|webpackChunkName|#?__PURE__|@preserve|@license)")
BLOCK_DIRECTIVE = re.compile(
    r"^/\*\s*(eslint-|@ts-|#?__PURE__|webpackChunkName|@vite-ignore|prettier-ignore|"
    r"biome-ignore|c8 ignore|v8 ignore|istanbul ignore|@jsxImportSource|@preserve|"
    r"@license|global\s)")

_DIV_AFTER = set(
    "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_$)]}`'\"/")


def _regex_allowed(prev):
    return prev == "" or prev not in _DIV_AFTER


def _consume_regex(src, i, n):
    j = i + 1
    in_class = False
    while j < n:
        ch = src[j]
        if ch == "\\":
            j += 2
            continue
        if ch == "\n":
            return None
        if ch == "[":
            in_class = True
        elif ch == "]":
            in_class = False
        elif ch == "/" and not in_class:
            j += 1
            while j < n and src[j].isalpha():
                j += 1
            return j
        j += 1
    return None


def find_comments(src):
    out = []
    i, n = 0, len(src)
    prev = ""
    stack = [{"kind": "code", "interp": False, "depth": 0}]
    while i < n:
        f = stack[-1]
        c = src[i]
        if f["kind"] == "code":
            two = src[i:i + 2]
            if two == "//":
                j = src.find("\n", i)
                if j == -1:
                    j = n
                out.append((i, j, "line"))
                i = j
                continue
            if two == "/*":
                j = src.find("*/", i + 2)
                end = (j + 2) if j != -1 else n
                out.append((i, end, "block"))
                i = end
                prev = "/"
                continue
            if c in "'\"":
                q = c
                i += 1
                while i < n:
                    if src[i] == "\\":
                        i += 2
                        continue
                    if src[i] == q:
                        i += 1
                        break
                    if src[i] == "\n":
                        break
                    i += 1
                prev = q
                continue
            if c == "`":
                stack.append({"kind": "tmpl", "interp": False, "depth": 0})
                i += 1
                prev = "`"
                continue
            if c == "/":
                if _regex_allowed(prev):
                    end = _consume_regex(src, i, n)
                    if end is not None:
                        i = end
                        prev = "/"
                        continue
                prev = "/"
                i += 1
                continue
            if c in " \t\r\n":
                i += 1
                continue
            if c == "{":
                f["depth"] += 1
                prev = c
                i += 1
                continue
            if c == "}":
                if f["interp"] and f["depth"] == 0:
                    stack.pop()
                    i += 1
                    prev = "`"
                    continue
                if f["depth"] > 0:
                    f["depth"] -= 1
                prev = c
                i += 1
                continue
            prev = c
            i += 1
            continue
        else:
            if c == "\\":
                i += 2
                continue
            if c == "`":
                stack.pop()
                i += 1
                prev = "`"
                continue
            if src[i:i + 2] == "${":
                stack.append({"kind": "code", "interp": True, "depth": 0})
                i += 2
                prev = "{"
                continue
            i += 1
            continue
    return out


def _inner(kind, text):
    body = text[2:] if kind == "line" else (text[2:-2] if text.endswith("*/") else text[2:])
    return body.lstrip("*/ \t\r\n")


def is_allowed(kind, text):
    if kind == "line" and DIRECTIVE.match(text):
        return True
    if kind == "block" and BLOCK_DIRECTIVE.match(text):
        return True
    return _inner(kind, text).lower().startswith("important:")
