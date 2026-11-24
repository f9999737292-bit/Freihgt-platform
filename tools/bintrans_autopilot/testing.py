"""Isolated bindings for unit tests.

The production constructors do not accept an env map, a config override, or a
replacement path. Those bindings exist only behind the test marker.
"""

from __future__ import annotations

from pathlib import Path

from bintrans_autopilot.audit_gateway import AuditGateway
from bintrans_autopilot.audit_journal import AuditJournal
from bintrans_autopilot.supervisor import RestartLedger, WorkerSupervisor
from bintrans_autopilot.test_bind import BIND


def open_supervisor(
    *,
    tenant_id: str,
    state_path: Path,
    env: dict[str, str] | None = None,
    stop_file: Path | None = None,
    config_stop: bool | None = False,
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
    supervisor = WorkerSupervisor.__new__(WorkerSupervisor)
    supervisor._configure(
        mode="dry_run",
        tenant_id=tenant_id,
        max_restarts=max_restarts,
        heartbeat_timeout_seconds=heartbeat_timeout_seconds,
        base_backoff_seconds=base_backoff_seconds,
        max_backoff_seconds=max_backoff_seconds,
        max_dead_jobs=max_dead_jobs,
        clock=clock,
        bind=BIND,
        env={} if env is None else env,
        stop_file=stop_file,
        config_stop=config_stop,
        state_path=path,
    )
    return supervisor


def open_gateway(
    *,
    journal_path: Path,
    env: dict[str, str] | None = None,
    observed_head: str = "",
) -> AuditGateway:
    path = Path(journal_path)
    if not path.exists():
        AuditJournal.create(path)
    gateway = AuditGateway.__new__(AuditGateway)
    gateway._configure(
        mode="dry_run",
        observed_head=observed_head,
        bind=BIND,
        env={} if env is None else env,
        journal_path=path,
    )
    return gateway


def append_packet_batch(path: str, label: str, count: int) -> None:
    journal = AuditJournal(Path(path))
    for index in range(count):
        journal.append("packet", {"label": label, "n": index})
