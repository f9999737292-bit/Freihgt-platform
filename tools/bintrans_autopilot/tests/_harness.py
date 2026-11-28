"""Fixtures for autopilot unit tests.

This module is imported only by tests. It is not a production API.
Assigning a module global from a test does not make the Python process an
untrusted subject: the same process can still read and write its own files.
"""

from __future__ import annotations

from pathlib import Path

import bintrans_autopilot.audit_gateway as audit_gateway_module
import bintrans_autopilot.supervisor as supervisor_module
from bintrans_autopilot.audit_gateway import AuditGateway
from bintrans_autopilot.audit_journal import AuditJournal
from bintrans_autopilot.supervisor import RestartLedger, WorkerSupervisor


def open_supervisor(
    *,
    tenant_id: str,
    state_path: Path,
    clock=None,
    max_restarts: int = 3,
    heartbeat_timeout_seconds: float = 30,
    base_backoff_seconds: float = 1,
    max_backoff_seconds: float = 60,
    max_dead_jobs: int = 100,
) -> WorkerSupervisor:
    path = Path(state_path)
    if not path.exists():
        RestartLedger.create(path)
    saved = supervisor_module.RESTART_LEDGER_PATH
    supervisor_module.RESTART_LEDGER_PATH = path
    try:
        return WorkerSupervisor(
            tenant_id=tenant_id,
            clock=clock,
            max_restarts=max_restarts,
            heartbeat_timeout_seconds=heartbeat_timeout_seconds,
            base_backoff_seconds=base_backoff_seconds,
            max_backoff_seconds=max_backoff_seconds,
            max_dead_jobs=max_dead_jobs,
        )
    finally:
        supervisor_module.RESTART_LEDGER_PATH = saved


def open_gateway(*, journal_path: Path, observed_head: str = "") -> AuditGateway:
    path = Path(journal_path)
    if not path.exists():
        AuditJournal.create(path)
    saved = audit_gateway_module.AUDIT_JOURNAL_PATH
    audit_gateway_module.AUDIT_JOURNAL_PATH = path
    try:
        return AuditGateway(observed_head=observed_head)
    finally:
        audit_gateway_module.AUDIT_JOURNAL_PATH = saved


def append_packet_batch(path: str, label: str, count: int) -> None:
    journal = AuditJournal(Path(path))
    for index in range(count):
        journal.append("packet", {"label": label, "n": index})
