"""Reject dispatcher runs that are not the trusted workflow on main.

This module must not read Cursor or GitHub credentials.
"""

from __future__ import annotations

TRUSTED_REPOSITORY = "f9999737292-bit/Freihgt-platform"
TRUSTED_REF = "refs/heads/main"
TRUSTED_WORKFLOW = "bintrans-autopilot-dispatcher.yml@refs/heads/main"
TRUSTED_EVENTS = {"schedule", "workflow_dispatch", "issues"}
UNTRUSTED_EVENTS = {
    "pull_request",
    "pull_request_target",
    "pull_request_review",
    "pull_request_review_comment",
}


def context_from_environ(env: dict[str, str]) -> dict[str, str]:
    """Read only the workflow identity. Credential variables are not consulted."""
    return {
        "event_name": env.get("GITHUB_EVENT_NAME", ""),
        "ref": env.get("GITHUB_REF", ""),
        "repository": env.get("GITHUB_REPOSITORY", ""),
        "workflow_ref": env.get("GITHUB_WORKFLOW_REF", ""),
        "base_ref": env.get("GITHUB_BASE_REF", ""),
        "head_ref": env.get("GITHUB_HEAD_REF", ""),
    }


def evaluate_workflow_context(context: dict[str, str]) -> tuple[bool, str]:
    event = (context.get("event_name") or "").strip()
    if event in UNTRUSTED_EVENTS:
        return False, "untrusted_event"
    if event not in TRUSTED_EVENTS:
        return False, "untrusted_event"
    if (context.get("ref") or "").strip() != TRUSTED_REF:
        return False, "untrusted_ref"
    if (context.get("repository") or "").strip() != TRUSTED_REPOSITORY:
        return False, "untrusted_repository"
    workflow_ref = (context.get("workflow_ref") or "").strip()
    if TRUSTED_WORKFLOW not in workflow_ref or not workflow_ref.endswith("@refs/heads/main"):
        return False, "untrusted_workflow_file"
    if (context.get("base_ref") or "").strip() or (context.get("head_ref") or "").strip():
        return False, "pull_request_context"
    return True, "trusted_main"
