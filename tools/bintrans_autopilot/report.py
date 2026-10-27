"""Read-only daily report. It does not recompute an enterprise completion percent."""

from __future__ import annotations

from bintrans_autopilot.contract import read_state
from bintrans_autopilot.states import (
    BLOCKED,
    BUDGET_HOLD,
    CI_CHECK,
    DONE,
    INDEPENDENT_QA,
    MERGED,
    NEEDS_CONTROLLER,
    PR_OPEN,
    READY_FOR_MERGE,
)


def render_report(issues, now_iso: str, usage_note: str) -> str:
    rows = []
    for issue in issues:
        state = read_state(issue.body) or {}
        rows.append(
            {
                "number": issue.number,
                "task_id": state.get("task_id") or "",
                "status": state.get("status") or "",
                "updated_at": state.get("updated_at") or "",
                "pr_url": state.get("pr_url") or "",
                "last_error": state.get("last_error") or "",
                "launch_dates": state.get("launch_dates") or [],
            }
        )
    day = now_iso[:10]

    def recent(row) -> bool:
        return str(row["updated_at"]).startswith(day)

    completed = [row for row in rows if row["status"] in {DONE, MERGED} and recent(row)]
    open_prs = [row for row in rows if row["status"] in {PR_OPEN, CI_CHECK, INDEPENDENT_QA, READY_FOR_MERGE}]
    qa_passed = [row for row in rows if row["status"] == READY_FOR_MERGE]
    needs_decision = [row for row in rows if row["status"] in {NEEDS_CONTROLLER, BUDGET_HOLD}]
    blocked = [row for row in rows if row["status"] == BLOCKED]
    failing_tests = [
        row
        for row in rows
        if row["last_error"] in {"qa_failed", "retries_exhausted"} or str(row["last_error"]).startswith("qa_")
    ]
    launches = sum(1 for row in rows for item in row["launch_dates"] if item == day)
    lines = [
        "# BINTRANS autopilot daily report",
        "",
        f"Generated: {now_iso}",
        "",
        "## Completed in the last 24 hours",
        _bullet(completed),
        "",
        "## Open pull requests",
        _bullet(open_prs),
        "",
        "## Passed independent QA",
        _bullet(qa_passed),
        "",
        "## Needs a controller decision",
        _bullet(needs_decision),
        "",
        "## Failing checks",
        _bullet(failing_tests),
        "",
        "## Blocked",
        _bullet(blocked),
        "",
        "## Resource use",
        f"- Launches recorded today: {launches}",
        f"- Cursor usage: {usage_note}",
        "- This report does not call the Cloud Agents create endpoint.",
        "",
        "## Enterprise completion",
        "ENTERPRISE_COMPLETION_PERCENT=UNCHANGED",
        "METHODOLOGY=NOT_APPROVED_FOR_RECOMPUTATION",
        "Queue counts are not a readiness percentage.",
        "Do not invent or adjust a completion percent from merged pull requests.",
        "",
    ]
    return "\n".join(lines)


def _bullet(rows: list[dict]) -> str:
    if not rows:
        return "- none"
    return "\n".join(
        f"- {row['task_id'] or ('issue ' + str(row['number']))}: {row['status']} {row['pr_url']}".rstrip()
        for row in rows
    )
