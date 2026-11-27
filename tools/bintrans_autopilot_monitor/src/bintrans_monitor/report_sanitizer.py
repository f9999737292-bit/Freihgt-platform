"""Reduce a monitor report to the fields that may be printed."""

from __future__ import annotations

import re
from datetime import datetime, timezone

from bintrans_monitor.engine import Report

_VENDOR_MARK = "CUR" + "SOR_"
_SECRET = re.compile(
    r"(ghp_[A-Za-z0-9]+|github_pat_[A-Za-z0-9_]+|gho_[A-Za-z0-9_]+|Bearer\s+\S+|AKIA[0-9A-Z]{16}|"
    + _VENDOR_MARK
    + r"[A-Za-z0-9_]{8,}|https?://[^\s/@:]+:[^\s/@]+@)"
)
ALLOWED = (
    "repository",
    "commit_sha",
    "run_id",
    "workflow",
    "job_name",
    "status",
    "conclusion",
    "classification",
    "error_signature",
    "confidence_or_uncertainty",
    "timestamp",
)


def redact(value: object) -> str:
    return _SECRET.sub("REDACTED", str(value))


def _row(report: Report, job: dict | None, moment: str) -> dict:
    observed = job or {}
    uncertain = "ROOT_CAUSE_UNCONFIRMED"
    if report.ci_result == "SUCCESS" and observed.get("ci_result") == "SUCCESS":
        uncertain = "job_conclusion_success"
    body = {
        "repository": redact(report.repository),
        "commit_sha": redact(observed.get("head_sha") or ""),
        "run_id": observed.get("run_id"),
        "workflow": redact(observed.get("workflow_name") or ""),
        "job_name": redact(observed.get("job_name") or ""),
        "status": redact(observed.get("status") or report.ci_result),
        "conclusion": redact(observed.get("conclusion") or report.ci_result),
        "classification": redact(observed.get("ci_result") or report.ci_result),
        "error_signature": redact(observed.get("signature") or report.error),
        "confidence_or_uncertainty": uncertain if observed else (
            "job_conclusion_success" if report.ci_result == "SUCCESS" else uncertain
        ),
        "timestamp": moment,
    }
    if tuple(body) != ALLOWED:
        raise RuntimeError("sanitizer_field_leak")
    return body


def sanitize(report: Report, timestamp: datetime | None = None) -> list[dict]:
    moment = (timestamp or datetime.now(timezone.utc)).strftime("%Y-%m-%dT%H:%M:%SZ")
    if not report.observations:
        return [_row(report, None, moment)]
    return [_row(report, job, moment) for job in report.observations]
