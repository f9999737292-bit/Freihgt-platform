"""Dry-run worker for the autopilot queue.

This is not a resident process. It does not start on import. A caller may
invoke run_once or a finite run_pending. Delivery stays at-least-once: a
completed queue row is not proof that an external effect ran once.

The production switches stay off. mode must be the string dry_run. Environment
variables cannot turn those switches on, and a job payload is never executed.
"""

from __future__ import annotations

import logging
import os
import threading
import time
from dataclasses import dataclass
from typing import Callable

from bintrans_autopilot.postgres_queue import (
    PENDING,
    PostgresQueue,
    QueueError,
    _assert_runtime_binding,
    _cursor_for,
    _one,
)

logger = logging.getLogger("bintrans_autopilot.worker")

_ALLOWLIST = ("docs", "ping")
_LOCKED_OFF = (
    "AUTOPILOT_ENABLED",
    "PAID_AUTOMATION_ENABLED",
    "EXTERNAL_TOOLS_ENABLED",
    "AUTO_MERGE_ENABLED",
    "AUTO_DEPLOY_ENABLED",
    "WORKER_ENABLED",
)
_LOST_LEASE = {"lease_expired", "stale_lease", "not_owner"}


class WorkerError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


@dataclass
class WorkerMetrics:
    queue_depth: int = 0
    oldest_pending_age: int = 0
    running_jobs: int = 0
    failed_jobs: int = 0
    dead_jobs: int = 0
    lease_expirations: int = 0
    worker_heartbeat: float = 0.0
    jobs_completed_total: int = 0


@dataclass(frozen=True)
class JobContext:
    job_id: str
    tenant_id: str
    job_type: str
    attempt_count: int
    renew: Callable[[], None]


def _dry_run_handler(_context: JobContext) -> None:
    return None


class AutopilotWorker:
    """Claims through PostgresQueue and runs only the built-in dry-run handlers."""

    def __init__(
        self,
        queue: PostgresQueue,
        connect,
        *,
        mode: str,
        tenant_id: str,
        owner: str,
        max_in_flight: int = 1,
        lease_seconds: int = 30,
        handlers: dict[str, Callable[[JobContext], None]] | None = None,
        allow: set[str] | frozenset[str] | None = None,
    ):
        if mode != "dry_run":
            raise WorkerError("worker_disabled")
        if queue.autopilot_enabled or queue.paid_automation_enabled:
            raise WorkerError("automation_disabled")
        for name in _LOCKED_OFF:
            if os.environ.get(name, "").strip().lower() == "true":
                raise WorkerError("worker_disabled")
        if isinstance(max_in_flight, bool) or not isinstance(max_in_flight, int) or not 1 <= max_in_flight <= 4:
            raise WorkerError("invalid_input")
        if not callable(connect):
            raise WorkerError("invalid_input")
        selected = {name: _dry_run_handler for name in _ALLOWLIST}
        if allow is None:
            allow = set(_ALLOWLIST)
        if not set(allow) or not set(allow) <= set(_ALLOWLIST):
            raise WorkerError("handler_rejected")
        if handlers is not None:
            if not set(handlers) <= set(_ALLOWLIST):
                raise WorkerError("handler_rejected")
            for name, handler in handlers.items():
                if not callable(handler):
                    raise WorkerError("handler_rejected")
                selected[name] = handler
        self.queue = queue
        self._connect = connect
        self.mode = "dry_run"
        self.worker_enabled = False
        self.autopilot_enabled = False
        self.paid_automation_enabled = False
        self.external_tools_enabled = False
        self.auto_merge_enabled = False
        self.auto_deploy_enabled = False
        self.external_calls = 0
        self.cloud_agents_started = 0
        self.tenant_id = tenant_id
        self.owner = owner
        self.max_in_flight = max_in_flight
        self.lease_seconds = lease_seconds
        self._allow = set(allow)
        self._handlers = selected
        self.metrics = WorkerMetrics()
        self._stop = False
        self._started = False
        self._stopped_emitted = False
        self._inflight = 0
        self._lock = threading.Lock()

    def stop(self) -> None:
        with self._lock:
            self._stop = True

    def run_pending(self, *, limit: int = 1) -> int:
        if isinstance(limit, bool) or not isinstance(limit, int) or not 1 <= limit <= 100:
            raise WorkerError("invalid_input")
        completed = 0
        for _ in range(limit):
            outcome = self.run_once()
            if outcome == "completed":
                completed += 1
                continue
            if outcome in {"idle", "stopped", "saturated", "unavailable"}:
                break
        return completed

    def run_once(self) -> str:
        self._ensure_started()
        if not self._acquire():
            if self._stop:
                self._emit_stopped()
                return "stopped"
            return "saturated"
        try:
            return self._run_acquired()
        finally:
            self._release()
            self._heartbeat()

    def snapshot(self) -> WorkerMetrics:
        try:
            connection = self._connect()
        except QueueError:
            self._emit("database_unavailable", "", "store_unavailable")
            raise
        except Exception:
            self._emit("database_unavailable", "", "store_unavailable")
            raise WorkerError("store_unavailable") from None
        try:
            with _cursor_for(connection) as cursor:
                _assert_runtime_binding(cursor, self.tenant_id)
                cursor.execute(
                    """
                    SELECT count(*) FILTER (WHERE status = 'PENDING') AS queue_depth,
                           COALESCE(EXTRACT(EPOCH FROM (
                               clock_timestamp() - min(created_at) FILTER (WHERE status = 'PENDING')
                           )), 0) AS oldest_pending_age,
                           count(*) FILTER (WHERE status = 'RUNNING') AS running_jobs,
                           count(*) FILTER (WHERE status = 'FAILED') AS failed_jobs,
                           count(*) FILTER (WHERE status = 'DEAD') AS dead_jobs
                    FROM autopilot_queue.jobs
                    """
                )
                row = _one(cursor)
            if row is None:
                raise WorkerError("store_unavailable")
            connection.commit()
        except QueueError:
            connection.rollback()
            raise
        except Exception:
            connection.rollback()
            self._emit("database_unavailable", "", "store_unavailable")
            raise WorkerError("store_unavailable") from None
        finally:
            connection.close()
        values = row if isinstance(row, dict) else {
            "queue_depth": row[0],
            "oldest_pending_age": row[1],
            "running_jobs": row[2],
            "failed_jobs": row[3],
            "dead_jobs": row[4],
        }
        self.metrics.queue_depth = int(values["queue_depth"])
        self.metrics.oldest_pending_age = int(values["oldest_pending_age"])
        self.metrics.running_jobs = int(values["running_jobs"])
        self.metrics.failed_jobs = int(values["failed_jobs"])
        self.metrics.dead_jobs = int(values["dead_jobs"])
        self._heartbeat()
        return self.metrics

    def _run_acquired(self) -> str:
        try:
            moved = self.queue.requeue_expired(self.tenant_id)
        except QueueError as exc:
            self._emit("database_unavailable" if exc.code == "store_unavailable" else "job_failed", "", exc.code)
            return "unavailable" if exc.code == "store_unavailable" else "failed"
        if moved:
            self.metrics.lease_expirations += moved
            self._emit("recovery_completed", "", str(moved))
        try:
            lease = self.queue.claim(self.tenant_id, self.owner, self.lease_seconds)
        except QueueError as exc:
            self._emit("database_unavailable" if exc.code == "store_unavailable" else "job_failed", "", exc.code)
            return "unavailable" if exc.code == "store_unavailable" else "failed"
        if lease is None:
            return "idle"
        self._emit("job_claimed", lease.job_id, PENDING)
        try:
            status = self.queue.get_status(self.tenant_id, lease.job_id)
        except QueueError as exc:
            self._emit("database_unavailable" if exc.code == "store_unavailable" else "job_failed", lease.job_id, exc.code)
            return "unavailable" if exc.code == "store_unavailable" else "failed"
        if status is None or status.job_type not in self._allow:
            return self._fail(lease, "handler_rejected", retryable=False)
        lease_lost = {"code": ""}

        def renew() -> None:
            try:
                self.queue.renew_lease(
                    lease.tenant_id, lease.job_id, lease.lease_owner, lease.lease_token, self.lease_seconds
                )
            except QueueError as exc:
                lease_lost["code"] = exc.code
                raise
            self._emit("lease_renewed", lease.job_id, "running")

        context = JobContext(lease.job_id, lease.tenant_id, status.job_type, lease.attempt_count, renew)
        try:
            self._handlers[status.job_type](context)
        except QueueError as exc:
            self._emit("job_failed", lease.job_id, exc.code)
            return "failed"
        except Exception:
            return self._fail(lease, "handler_failed", retryable=True)
        if lease_lost["code"]:
            self._emit("job_failed", lease.job_id, lease_lost["code"])
            return "failed"
        try:
            self.queue.complete(lease.tenant_id, lease.job_id, lease.lease_owner, lease.lease_token)
        except QueueError as exc:
            if exc.code == "store_unavailable":
                self._emit("database_unavailable", lease.job_id, exc.code)
                return "unavailable"
            self._emit("job_failed", lease.job_id, exc.code)
            return "failed"
        with self._lock:
            self.metrics.jobs_completed_total += 1
        self._emit("job_completed", lease.job_id, "succeeded")
        return "completed"

    def _fail(self, lease, error_code: str, *, retryable: bool) -> str:
        try:
            saved = self.queue.fail(
                lease.tenant_id,
                lease.job_id,
                lease.lease_owner,
                lease.lease_token,
                error_code,
                retryable=retryable,
            )
        except QueueError as exc:
            if exc.code == "store_unavailable":
                self._emit("database_unavailable", lease.job_id, exc.code)
                return "unavailable"
            self._emit("job_failed", lease.job_id, exc.code)
            return "failed"
        if saved.status == "DEAD":
            self._emit("job_dead", lease.job_id, error_code)
            return "dead"
        if saved.status == "PENDING":
            self._emit("retry_scheduled", lease.job_id, error_code)
            return "retry"
        self._emit("job_failed", lease.job_id, saved.status.lower())
        return "failed"

    def _ensure_started(self) -> None:
        with self._lock:
            if self._started:
                return
            self._started = True
        self._heartbeat()
        self._emit("worker_started", "", "dry_run")

    def _emit_stopped(self) -> None:
        with self._lock:
            if self._stopped_emitted:
                return
            self._stopped_emitted = True
        self._emit("worker_stopped", "", "dry_run")

    def _acquire(self) -> bool:
        with self._lock:
            if self._stop or self._inflight >= self.max_in_flight:
                return False
            self._inflight += 1
            return True

    def _release(self) -> None:
        with self._lock:
            self._inflight -= 1

    def _heartbeat(self) -> None:
        self.metrics.worker_heartbeat = time.time()

    def _emit(self, event: str, job_id: str, result: str) -> None:
        logger.info("event=%s tenant=%s job=%s result=%s", event, self.tenant_id, job_id, result)
