"""Static pull request checks. Agent I does not run inside this module."""

from __future__ import annotations

import re
from dataclasses import dataclass, field

from bintrans_autopilot.paths import any_match
from bintrans_autopilot.policy import critical_domains

SECRET_PATTERNS = (
    re.compile(r"-----BEGIN (?:RSA |OPENSSH |EC |DSA |PRIVATE )?PRIVATE KEY-----"),
    re.compile(r"\bAKIA[0-9A-Z]{16}\b"),
    re.compile(r"\bgithub_pat_[A-Za-z0-9_]{20,}\b"),
    re.compile(r"\bcursor_[A-Za-z0-9]{20,}\b"),
    re.compile(r"(?i)\b(?:JWT_SECRET|POSTGRES_PASSWORD|CONTROL_TOWER_DATABASE_URL)\s*=\s*\S+"),
)

WEAK_TEST_PATTERNS = (
    re.compile(r"\bt\.Skip\s*\("),
    re.compile(r"\bpytest\.skip\s*\("),
    re.compile(r"\bit\.skip\s*\("),
    re.compile(r"\btest\.skip\s*\("),
    re.compile(r"\bxit\s*\("),
    re.compile(r"\bxdescribe\s*\("),
    re.compile(r"\bdescribe\.skip\s*\("),
    re.compile(r"\bfdescribe\s*\("),
    re.compile(r"\bfit\s*\("),
)


@dataclass(frozen=True)
class DiffFile:
    path: str
    patch: str


@dataclass
class QaResult:
    ok: bool
    failures: list[str] = field(default_factory=list)


MAX_REVIEW_DIFF_CHARS = 120_000


def format_review_diff(files: list[DiffFile]) -> tuple[str, str]:
    """Full patch text for Agent I. A missing hunk is not replaced by the file name."""
    if not files:
        return "", "empty_diff"
    parts: list[str] = []
    for item in files:
        patch = item.patch or ""
        if not patch.strip():
            return "", f"diff_patch_missing:{item.path}"
        parts.append(f"--- {item.path}\n{patch}")
    text = "\n".join(parts)
    if len(text) > MAX_REVIEW_DIFF_CHARS:
        return "", "diff_too_large"
    return text, ""


def added_lines(patch: str) -> list[str]:
    lines = []
    for line in patch.splitlines():
        if line.startswith("+++") or line.startswith("---"):
            continue
        if line.startswith("+"):
            lines.append(line[1:])
    return lines


def review_diff(files: list[DiffFile], contract, config, labels: list[str]) -> QaResult:
    failures: list[str] = []
    if not files:
        failures.append("empty_diff")
    for item in files:
        allowed = any_match(contract.allowed_paths, item.path)
        forbidden = any_match(contract.forbidden_paths, item.path) or any_match(config.hard_forbidden_prefixes, item.path)
        if not allowed or forbidden:
            failures.append(f"unauthorized_path:{item.path}")
        added = "\n".join(added_lines(item.patch))
        for pattern in SECRET_PATTERNS:
            if pattern.search(added):
                failures.append(f"secret:{item.path}")
                break
        if _looks_like_test(item.path):
            for pattern in WEAK_TEST_PATTERNS:
                if pattern.search(added):
                    failures.append(f"weakened_test:{item.path}")
                    break
    domains = critical_domains(contract, config)
    if domains and ("autopilot-shared-path-approved" not in labels or contract.risk_level not in {"HIGH", "CRITICAL"}):
        failures.append("critical_domain_without_approval:" + ",".join(sorted(domains)))
    return QaResult(ok=not failures, failures=failures)


def _looks_like_test(path: str) -> bool:
    name = path.lower()
    return (
        name.endswith("_test.go")
        or name.endswith(".test.ts")
        or name.endswith(".test.js")
        or name.endswith(".spec.ts")
        or "/test_" in name
        or name.endswith("_test.py")
        or "/tests/" in name
    )


def merge_allowed(pr_base_sha: str, origin_main_sha: str, head_branch: str, protected: tuple[str, ...]) -> bool:
    if head_branch in protected:
        return False
    if pr_base_sha != origin_main_sha:
        return False
    return True
