"""Path matching for Task Contract allow and deny lists."""

from __future__ import annotations

import fnmatch


def normalize(path: str) -> str:
    return path.replace("\\", "/").lstrip("./")


def matches(pattern: str, path: str) -> bool:
    pattern = normalize(pattern)
    path = normalize(path)
    if pattern.endswith("/**"):
        prefix = pattern[:-3].rstrip("/")
        return path == prefix or path.startswith(prefix + "/")
    if pattern.endswith("/*"):
        prefix = pattern[:-2].rstrip("/")
        if not path.startswith(prefix + "/"):
            return False
        return "/" not in path[len(prefix) + 1 :]
    return fnmatch.fnmatch(path, pattern)


def any_match(patterns: tuple[str, ...] | list[str], path: str) -> bool:
    return any(matches(pattern, path) for pattern in patterns)


def under_prefix(prefixes: tuple[str, ...], path: str) -> bool:
    normalized = normalize(path)
    for prefix in prefixes:
        root = normalize(prefix).rstrip("/")
        if normalized == root or normalized.startswith(root + "/"):
            return True
    return False


def domains_for(paths: tuple[str, ...] | list[str], config) -> set[str]:
    found: set[str] = set()
    for path in paths:
        domain = config.domain_for_path(path)
        if domain:
            found.add(domain)
    return found
