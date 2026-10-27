import re

DIRECTIVE = re.compile(
    r"^//(go:|line |export |nolint:|lint:ignore|\s*Code generated .* DO NOT EDIT\.)")


def find_comments(src):
    out = []
    i, n = 0, len(src)
    while i < n:
        c = src[i]
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
            continue
        if c == '"':
            i += 1
            while i < n and src[i] != '"' and src[i] != "\n":
                i += 2 if src[i] == "\\" else 1
            i += 1
            continue
        if c == "'":
            i += 1
            while i < n and src[i] != "'" and src[i] != "\n":
                i += 2 if src[i] == "\\" else 1
            i += 1
            continue
        if c == "`":
            j = src.find("`", i + 1)
            i = (j + 1) if j != -1 else n
            continue
        i += 1
    return out


def is_allowed(kind, text):
    if kind == "line" and DIRECTIVE.match(text):
        return True
    body = text[2:] if kind == "line" else text[2:-2]
    return body.strip().lower().startswith("important:")
