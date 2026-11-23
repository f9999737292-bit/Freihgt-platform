"""One-pass dry-run readiness report.

The check command prints the closed switches and returns. It does not claim a
job, start a worker, call Cursor, or merge. A cycle evaluates observations
already collected by the caller.
"""

from __future__ import annotations

import os
from dataclasses import dataclass

from bintrans_autopilot.audit_gateway import AuditGateway, AuditPacket
from bintrans_autopilot.config import EMERGENCY_STOP_FILE
from bintrans_autopilot.supervisor import SupervisorDecision, WorkerObservation, WorkerSupervisor

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


@dataclass(frozen=True)
class CycleReport:
    decisions: tuple[SupervisorDecision, ...]
    audit_reason: str
    merge_authorized: bool = False
    production_enabled: bool = False
    paid_automation_enabled: bool = False
    cloud_agents_started: int = 0


def _locked(env: dict[str, str]) -> str:
    for name in _LOCKED_TRUE:
        if str(env.get(name, "")).strip().lower() == "true":
            return name
    return ""


def readiness_lines(env: dict[str, str] | None = None) -> tuple[int, tuple[str, ...]]:
    source = dict(os.environ if env is None else env)
    locked = _locked(source)
    stopped = source.get("AUTOPILOT_EMERGENCY_STOP", "").strip().lower() == "true" or EMERGENCY_STOP_FILE.exists()
    if locked:
        lines = (
            "READINESS_CHECK=REFUSED",
            f"REASON={locked}",
            "AUTOPILOT_ENABLED=NO",
            "PAID_AUTOMATION_ENABLED=NO",
            "EXTERNAL_TOOLS_ENABLED=NO",
            "AUTO_MERGE=NO",
            "AUTO_DEPLOY=NO",
            "WORKER_ENABLED=NO",
            "PRODUCTION_24X7_ENABLED=NO",
            "CLOUD_AGENTS_STARTED=0",
        )
        return 2, lines
    lines = (
        "READINESS_CHECK=DRY_RUN",
        "AUTOPILOT_ENABLED=NO",
        "PAID_AUTOMATION_ENABLED=NO",
        "EXTERNAL_TOOLS_ENABLED=NO",
        "AUTO_MERGE=NO",
        "AUTO_DEPLOY=NO",
        "WORKER_ENABLED=NO",
        f"EMERGENCY_STOP={'YES' if stopped else 'NO'}",
        "PRODUCTION_24X7_ENABLED=NO",
        "CLOUD_AGENTS_STARTED=0",
    )
    return 0, lines


def readiness_check(env: dict[str, str] | None = None) -> int:
    code, lines = readiness_lines(env)
    for line in lines:
        print(line)
    return code


class ReadinessCycle:
    """Apply supervisor and audit decisions once. There is no resident loop."""

    def __init__(self, supervisor: WorkerSupervisor, audit: AuditGateway):
        self.supervisor = supervisor
        self.audit = audit
        self.cloud_agents_started = 0
        self.merge_authorized = False

    def run(
        self,
        observations: tuple[WorkerObservation, ...] | list[WorkerObservation],
        *,
        emergency_stop: bool = False,
        packet: AuditPacket | None = None,
    ) -> CycleReport:
        decisions = tuple(self.supervisor.observe(item, emergency_stop=emergency_stop) for item in observations)
        if packet is None:
            reason = "audit_not_submitted"
        else:
            reason = self.audit.submit(packet).reason
        return CycleReport(decisions, reason)
