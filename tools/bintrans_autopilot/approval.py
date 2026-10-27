"""Controller approval is an allowlisted GitHub actor plus a contract digest.

A label or a STATUS heading written by the implementing agent is not approval.
"""

from __future__ import annotations

import re

DIGEST_LINE = re.compile(r"^AUTOPILOT_CONTRACT_SHA256=([0-9a-f]{64})$", re.MULTILINE)
APPROVAL_LABEL = "autopilot-contract-approved"


def trusted_approval(contract_digest: str, labels: list[str], events: list[dict], comments: list[dict], controllers: tuple[str, ...]) -> str:
    if APPROVAL_LABEL not in labels:
        return "contract_not_approved"
    if not controllers:
        return "controller_allowlist_empty"
    actors = [
        str(event.get("actor") or "")
        for event in events
        if event.get("event") == "labeled" and event.get("label") == APPROVAL_LABEL
    ]
    if not actors:
        return "approval_actor_missing"
    approver = actors[-1]
    if approver not in controllers:
        return "approval_actor_unauthorized"
    digests = []
    for comment in comments:
        author = str(comment.get("user") or "")
        if author != approver:
            continue
        digests.extend(DIGEST_LINE.findall(str(comment.get("body") or "")))
    if contract_digest not in digests:
        return "approval_digest_mismatch"
    return ""
