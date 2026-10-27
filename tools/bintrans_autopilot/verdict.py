"""Agent I must return exactly one structured verdict."""

from __future__ import annotations

import re

VERDICTS = ("PASS", "PASS_WITH_NITS", "CHANGES_REQUIRED", "BLOCKED")
VERDICT_LINE = re.compile(r"^BINTRANS_QA_VERDICT:\s*(PASS|PASS_WITH_NITS|CHANGES_REQUIRED|BLOCKED)\s*$", re.MULTILINE)


def parse_verdict(text: str) -> str:
    """Return the single verdict line, or empty when the report is not verifiable."""
    found = VERDICT_LINE.findall(text or "")
    if len(found) != 1:
        return ""
    verdict = found[0]
    if verdict not in VERDICTS:
        return ""
    return verdict
