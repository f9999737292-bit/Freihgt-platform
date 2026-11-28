"""Record an execution claim. Do not treat the claim as Agent J.

The reviewer name, test result, and verdict in a packet are untrusted text.
This process cannot prove who wrote them: that proof has to come from a
GitHub review and a CI check on the exact SHA, read by the trusted workflow.
Those reads are not available here, so the finding gate stays blocked.
"""

from __future__ import annotations

import os
import re
from dataclasses import dataclass

from bintrans_autopilot.audit_journal import AuditJournal, AuditJournalError
from bintrans_autopilot.config import AUDIT_JOURNAL_PATH

_LOCKED_TRUE = (
    "AUTOPILOT_ENABLED",
    "PAID_AUTOMATION_ENABLED",
    "EXTERNAL_TOOLS_ENABLED",
    "AUTO_MERGE",
    "AUTO_MERGE_ENABLED",
    "AUTO_DEPLOY",
    "AUTO_DEPLOY_ENABLED",
    "WORKER_ENABLED",
    "PRODUCTION_24X7_ENABLED",
)
_VERDICTS = {"PASS", "PASS_WITH_NITS", "CHANGES_REQUIRED", "BLOCKED"}
_RESULTS = {"PASS", "FAIL", "NOT_RUN"}
_SEVERITIES = {"LOW", "MEDIUM", "HIGH", "CRITICAL"}
_BLOCKING_SEVERITIES = {"HIGH", "CRITICAL"}
_BLOCKING_VERDICTS = {"CHANGES_REQUIRED", "BLOCKED"}
_SHA = re.compile(r"^[0-9a-f]{40}$")


class AuditError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


@dataclass(frozen=True)
class AuditFinding:
    finding_id: str
    severity: str
    blocks_merge: bool


@dataclass(frozen=True)
class AuditPacket:
    commit_sha: str
    test_command: str
    test_result: str
    reviewer: str
    verdict: str
    findings: tuple[AuditFinding, ...] = ()


@dataclass(frozen=True)
class AuditDecision:
    reason: str
    audit_recorded: bool
    finding_gate: str
    merge_authorized: bool = False
    production_enabled: bool = False
    paid_automation_enabled: bool = False
    reviewer_authenticated: bool = False
    ci_verified: bool = False


class AuditGateway:
    def __init__(self, *, mode: str = "dry_run", observed_head: str = ""):
        if mode != "dry_run":
            raise AuditError("audit_disabled")
        for name in _LOCKED_TRUE:
            if str(os.environ.get(name, "")).strip().lower() == "true":
                raise AuditError("audit_disabled")
        if observed_head and not _SHA.fullmatch(observed_head):
            raise AuditError("invalid_commit")
        try:
            journal = AuditJournal(AUDIT_JOURNAL_PATH)
        except AuditJournalError as exc:
            raise AuditError(exc.code) from None
        self.mode = "dry_run"
        self.merge_authorized = False
        self.production_enabled = False
        self.reviewer_authenticated = False
        self.ci_verified = False
        self.observed_head = observed_head
        self.journal = journal

    def submit(self, packet: AuditPacket) -> AuditDecision:
        if not _SHA.fullmatch(packet.commit_sha or ""):
            raise AuditError("invalid_commit")
        if packet.test_result not in _RESULTS or packet.verdict not in _VERDICTS:
            raise AuditError("invalid_input")
        if not packet.test_command.strip():
            raise AuditError("invalid_input")
        for finding in packet.findings:
            if finding.severity not in _SEVERITIES or not finding.finding_id.strip():
                raise AuditError("invalid_input")
        try:
            records = self.journal.verify()
        except AuditJournalError as exc:
            raise AuditError(exc.code) from None
        claimed = {
            "claimed_result": packet.test_result,
            "claimed_reviewer": packet.reviewer,
            "claimed_verdict": packet.verdict,
            "commit_sha": packet.commit_sha,
            "severities": [item.severity for item in packet.findings],
        }
        try:
            self.journal.append("packet", claimed)
        except AuditJournalError as exc:
            raise AuditError(exc.code) from None
        if _already_blocked(records, packet.commit_sha):
            return self._blocked("prior_block_stands")
        if _must_block(packet):
            reason = "blocking_severity" if _high_severity(packet) else "blocking_finding"
            try:
                self.journal.append("block", {"commit_sha": packet.commit_sha, "reason": reason})
            except AuditJournalError as exc:
                raise AuditError(exc.code) from None
            return self._blocked(reason)
        if self.observed_head and packet.commit_sha != self.observed_head:
            return self._blocked("sha_mismatch")
        return self._blocked("trust_boundary_absent")

    def packets(self) -> tuple[dict, ...]:
        try:
            records = self.journal.verify()
        except AuditJournalError as exc:
            raise AuditError(exc.code) from None
        return tuple(record["body"] for record in records if record["kind"] == "packet")

    def _blocked(self, reason: str) -> AuditDecision:
        return AuditDecision(reason, True, "blocked")


def _already_blocked(records: tuple[dict, ...], commit_sha: str) -> bool:
    return any(record["kind"] == "block" and record["body"].get("commit_sha") == commit_sha for record in records)


def _high_severity(packet: AuditPacket) -> bool:
    return any(item.severity in _BLOCKING_SEVERITIES for item in packet.findings)


def _must_block(packet: AuditPacket) -> bool:
    if packet.verdict in _BLOCKING_VERDICTS or _high_severity(packet):
        return True
    return any(item.blocks_merge for item in packet.findings)
