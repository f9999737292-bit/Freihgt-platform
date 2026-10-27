"""Pull request identity. A swapped number, fork, or branch cannot pass QA."""

from __future__ import annotations

import re

PULL_URL = re.compile(r"^https://github\.com/([^/\s]+/[^/\s]+)/pull/(\d+)$")


def parse_pull_url(url: str) -> tuple[str, int] | None:
    match = PULL_URL.match((url or "").strip())
    if not match:
        return None
    return match.group(1), int(match.group(2))


def repo_full_name(repo_url: str) -> str:
    trimmed = repo_url.rstrip("/")
    if trimmed.startswith("https://github.com/"):
        return trimmed.removeprefix("https://github.com/")
    return trimmed


def pr_identity_error(pr: dict, *, repo: str, branch: str, claimed_url: str) -> str:
    if pr.get("repo") != repo or pr.get("head_repo") != repo:
        return "pr_repo_mismatch"
    if pr.get("head_branch") != branch:
        return "pr_branch_mismatch"
    head_sha = str(pr.get("head_sha") or "")
    if len(head_sha) != 40 or any(char not in "0123456789abcdef" for char in head_sha):
        return "pr_sha_missing"
    parsed = parse_pull_url(claimed_url)
    if parsed is None:
        return "pr_url_invalid"
    url_repo, url_number = parsed
    if url_repo != repo or url_number != pr.get("number"):
        return "pr_url_mismatch"
    return ""
