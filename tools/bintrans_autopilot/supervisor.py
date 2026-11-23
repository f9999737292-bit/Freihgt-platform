"""Finite supervision for a dry-run worker.

This module does not spawn a process, open a network client, or read a job payload.
A restart decision is a signal to the caller. It does not complete a job and
it does not clear a PostgreSQL lease. Lost work stays on the queue until the
existing lease expiry path requeues it.
"""

from __future__ import annotations

import os
import time
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


class SupervisorError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


@dataclass(frozen=True)
class WorkerObservation:
    worker_id: str
    tenant_id: str
    heartbeat_at: float
    running_jobs: int = 0
    queue_depth: int = 0
    dead_jobs: int = 0
    failed_jobs: int = 0


@dataclass(frozen=True)
class SupervisorDecision:
    action: str
    reason: str
    restart_count: int
    backoff_seconds: float
    completes_job: bool = False


def _locked(env: dict[str, str] | None = None) -> str:
    source = os.environ if env is None else env
    for name in _LOCKED_TRUE:
        if str(source.get(name, "")).strip().lower() == "true":
            return name
    return ""


class WorkerSupervisor:
    """Counts restarts per worker and holds the worker when the budget is spent."""

    def __init__(
        self,
        *,
        mode: str = "dry_run",
        max_restarts: int = 3,
        heartbeat_timeout_seconds: float = 30,
        base_backoff_seconds: float = 1,
        max_backoff_seconds: float = 60,
        max_dead_jobs: int = 100,
        clock=None,
        env: dict[str, str] | None = None,
    ):
        if mode != "dry_run":
            raise SupervisorError("supervisor_disabled")
        locked = _locked(env)
        if locked:
            raise SupervisorError("supervisor_disabled")
        if isinstance(max_restarts, bool) or not isinstance(max_restarts, int) or not 1 <= max_restarts <= 8:
            raise SupervisorError("invalid_input")
        if heartbeat_timeout_seconds <= 0 or base_backoff_seconds <= 0 or max_backoff_seconds < base_backoff_seconds:
            raise SupervisorError("invalid_input")
        if isinstance(max_dead_jobs, bool) or not isinstance(max_dead_jobs, int) or max_dead_jobs < 1:
            raise SupervisorError("invalid_input")
        self.mode = "dry_run"
        self.worker_enabled = False
        self.autopilot_enabled = False
        self.paid_automation_enabled = False
        self.external_tools_enabled = False
        self.auto_merge_enabled = False
        self.auto_deploy_enabled = False
        self.max_restarts = max_restarts
        self.heartbeat_timeout_seconds = float(heartbeat_timeout_seconds)
        self.base_backoff_seconds = float(base_backoff_seconds)
        self.max_backoff_seconds = float(max_backoff_seconds)
        self.max_dead_jobs = max_dead_jobs
        self._clock = time.monotonic if clock is None else clock
        self._restarts: dict[str, int] = {}
        self._held: dict[str, str] = {}
        self._not_before: dict[str, float] = {}

    def observe(self, observation: WorkerObservation, *, emergency_stop: bool = False) -> SupervisorDecision:
        if not observation.worker_id or not observation.tenant_id:
            raise SupervisorError("invalid_input")
        worker_id = observation.worker_id
        count = self._restarts.get(worker_id, 0)
        if emergency_stop:
            return SupervisorDecision("stop", "emergency_stop", count, 0.0)
        if worker_id in self._held:
            return SupervisorDecision("hold", self._held[worker_id], count, 0.0)
        if observation.dead_jobs >= self.max_dead_jobs:
            self._held[worker_id] = "resource_limit"
            return SupervisorDecision("hold", "resource_limit", count, 0.0)
        now = float(self._clock())
        stale = (now - float(observation.heartbeat_at)) > self.heartbeat_timeout_seconds
        if not stale:
            return SupervisorDecision("continue", "healthy", count, 0.0)
        waiting = self._not_before.get(worker_id, 0.0)
        if now < waiting:
            return SupervisorDecision("wait", "backoff", count, waiting - now)
        if count >= self.max_restarts:
            self._held[worker_id] = "restart_budget_exhausted"
            return SupervisorDecision("hold", "restart_budget_exhausted", count, 0.0)
        count += 1
        self._restarts[worker_id] = count
        backoff = min(self.max_backoff_seconds, self.base_backoff_seconds * (2 ** (count - 1)))
        self._not_before[worker_id] = now + backoff
        reason = "stuck_job" if observation.running_jobs > 0 else "heartbeat_lost"
        return SupervisorDecision("restart", reason, count, backoff)

    def release_hold(self, worker_id: str) -> None:
        """Operator reset. This does not set WORKER_ENABLED or start a process."""
        self._held.pop(worker_id, None)
        self._restarts.pop(worker_id, None)
        self._not_before.pop(worker_id, None)
