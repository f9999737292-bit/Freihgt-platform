"""Record an execution result for Agent J.

A packet stored here is evidence. It is not an independent review, and a
PASS written by the implementation agent does not authorize merge, deploy, or
a paid launch. Promotion stays closed in this stage.
"""

from __future__ import annotations

import os
import re
from dataclasses import dataclass

_LOCKED_TRUE = (
    "AUTOPILOT_ENABLED",
    "PAID_AUTOMATION_ENABLED",
    "EXTERNAL_TOOLS_ENABLED",
    "AUTO_MERGE",
    "AUTO_MERGE_ENABLED",
    "AUTO_DEPLOY",
    "AUTO_DEPLOY_ENABLED",
    "WORKER_ENABLED",
)
_INDEPENDENT_REVIEWER = "agent-j"
_VERDICTS = {"PASS", "PASS_WITH_NITS", "CHANGES_REQUIRED", "BLOCKED"}
_RESULTS = {"PASS", "FAIL", "NOT_RUN"}
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


class AuditGateway:
    def __init__(self, *, mode: str = "dry_run", env: dict[str, str] | None = None):
        if mode != "dry_run":
            raise AuditError("audit_disabled")
        source = os.environ if env is None else env
        for name in _LOCKED_TRUE:
            if str(source.get(name, "")).strip().lower() == "true":
                raise AuditError("audit_disabled")
        self.mode = "dry_run"
        self.merge_authorized = False
        self.production_enabled = False
        self._packets: list[AuditPacket] = []

    def submit(self, packet: AuditPacket) -> AuditDecision:
        if not _SHA.fullmatch(packet.commit_sha or ""):
            raise AuditError("invalid_commit")
        if packet.test_result not in _RESULTS or packet.verdict not in _VERDICTS:
            raise AuditError("invalid_input")
        if not packet.test_command.strip():
            raise AuditError("invalid_input")
        self._packets.append(packet)
        reviewer = packet.reviewer.strip().lower()
        if reviewer != _INDEPENDENT_REVIEWER:
            return AuditDecision("self_approval_rejected", True, "blocked")
        if packet.test_result != "PASS":
            return AuditDecision("tests_not_passed", True, "blocked")
        if packet.verdict in _BLOCKING_VERDICTS or any(item.blocks_merge for item in packet.findings):
            return AuditDecision("blocking_finding", True, "blocked")
        return AuditDecision("recorded_for_controller", True, "clear")

    def packets(self) -> tuple[AuditPacket, ...]:
        return tuple(self._packets)
