"""Finite supervision for a dry-run worker.

This module does not spawn a process, open a network client, or read a job payload.
A restart decision is a signal to the caller. It does not complete a job and
it does not clear a PostgreSQL lease. The restart count is stored in a
hash-chained file keyed by the supervisor tenant, not by the caller's worker id.
"""

from __future__ import annotations

import hashlib
import json
import os
import threading
import time
from dataclasses import dataclass
from pathlib import Path

from bintrans_autopilot.config import RESTART_LEDGER_PATH, emergency_stop_active
from bintrans_autopilot.file_lock import exclusive_file_lock

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
_GENESIS = "0" * 64
_FUTURE_SKEW_SECONDS = 1.0


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


def _digest(prev: str, body: dict) -> str:
    encoded = json.dumps(body, sort_keys=True, separators=(",", ":"), ensure_ascii=True)
    return hashlib.sha256(f"{prev}\n{encoded}".encode("ascii")).hexdigest()


class RestartLedger:
    """Append-only restart budget. Update and delete are refused.

    A missing file is not created here. Creating a fresh file would reset the
    restart count. A torn line stays in place and fails verification.
    A shorter prefix that still hashes is not detected. That residual is not
    closed by this file.
    """

    def __init__(self, path: Path):
        self.path = Path(path)
        self._lock_path = self.path.with_name(self.path.name + ".lock")
        self._thread = threading.Lock()
        self.missing = not self.path.is_file()
        self.tampered = False
        if not self.missing:
            try:
                self.verify()
            except SupervisorError:
                self.tampered = True

    @classmethod
    def create(cls, path: Path) -> RestartLedger:
        path = Path(path)
        path.parent.mkdir(parents=True, exist_ok=True)
        lock_path = path.with_name(path.name + ".lock")
        with exclusive_file_lock(lock_path):
            if path.exists():
                raise SupervisorError("state_exists")
            path.write_text("", encoding="utf-8")
        return cls(path)

    def _read(self) -> str:
        try:
            return self.path.read_text(encoding="utf-8")
        except FileNotFoundError:
            raise SupervisorError("state_missing") from None
        except OSError:
            raise SupervisorError("state_tampered") from None

    def _verify_unlocked(self) -> tuple[dict, ...]:
        text = self._read()
        if text == "":
            return ()
        if not text.endswith("\n") or "\r" in text:
            raise SupervisorError("state_tampered")
        records: list[dict] = []
        prev = _GENESIS
        for line in text.split("\n"):
            if line == "":
                continue
            try:
                record = json.loads(line)
                body = {
                    "count": record["count"],
                    "event": record["event"],
                    "reason": record["reason"],
                    "tenant_id": record["tenant_id"],
                }
            except (KeyError, TypeError, json.JSONDecodeError):
                raise SupervisorError("state_tampered") from None
            digest = _digest(prev, body)
            if record.get("prev") != prev or record.get("hash") != digest:
                raise SupervisorError("state_tampered")
            prev = digest
            records.append(record)
        return tuple(records)

    def verify(self) -> tuple[dict, ...]:
        with self._thread:
            with exclusive_file_lock(self._lock_path):
                return self._verify_unlocked()

    def count(self, tenant_id: str) -> int:
        total = 0
        for record in self.verify():
            if record["tenant_id"] == tenant_id and record["event"] == "restart":
                total = int(record["count"])
        return total

    def hold_reason(self, tenant_id: str) -> str:
        reason = ""
        for record in self.verify():
            if record["tenant_id"] == tenant_id and record["event"] == "hold":
                reason = str(record["reason"])
        return reason

    def append(self, tenant_id: str, event: str, reason: str, count: int) -> None:
        if event not in {"restart", "hold"}:
            raise SupervisorError("invalid_input")
        with self._thread:
            with exclusive_file_lock(self._lock_path):
                if self.missing or not self.path.is_file():
                    raise SupervisorError("state_missing")
                if self.tampered:
                    raise SupervisorError("state_tampered")
                records = self._verify_unlocked()
                prev = records[-1]["hash"] if records else _GENESIS
                body = {"count": count, "event": event, "reason": reason, "tenant_id": tenant_id}
                digest = _digest(prev, body)
                line = json.dumps(
                    {"count": count, "event": event, "hash": digest, "prev": prev, "reason": reason, "tenant_id": tenant_id},
                    sort_keys=True,
                    separators=(",", ":"),
                    ensure_ascii=True,
                )
                with self.path.open("a", encoding="utf-8", newline="\n") as handle:
                    handle.write(line + "\n")
                    handle.flush()
                    os.fsync(handle.fileno())

    def clear(self) -> None:
        raise SupervisorError("state_immutable")


class WorkerSupervisor:
    """Counts restarts for one tenant and holds that tenant when the budget is spent."""

    def __init__(
        self,
        *,
        mode: str = "dry_run",
        tenant_id: str = "",
        max_restarts: int = 3,
        heartbeat_timeout_seconds: float = 30,
        base_backoff_seconds: float = 1,
        max_backoff_seconds: float = 60,
        max_dead_jobs: int = 100,
        clock=None,
    ):
        if mode != "dry_run":
            raise SupervisorError("supervisor_disabled")
        if _locked(None):
            raise SupervisorError("supervisor_disabled")
        if isinstance(max_restarts, bool) or not isinstance(max_restarts, int) or not 1 <= max_restarts <= 8:
            raise SupervisorError("invalid_input")
        if heartbeat_timeout_seconds <= 0 or base_backoff_seconds <= 0 or max_backoff_seconds < base_backoff_seconds:
            raise SupervisorError("invalid_input")
        if isinstance(max_dead_jobs, bool) or not isinstance(max_dead_jobs, int) or max_dead_jobs < 1:
            raise SupervisorError("invalid_input")
        if not tenant_id:
            raise SupervisorError("invalid_input")
        self.mode = "dry_run"
        self.tenant_id = tenant_id
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
        self._not_before: dict[str, float] = {}
        self._ledger = RestartLedger(RESTART_LEDGER_PATH)

    def observe(self, observation: WorkerObservation, *, emergency_stop: bool = False) -> SupervisorDecision:
        if not observation.worker_id or not observation.tenant_id:
            raise SupervisorError("invalid_input")
        if self._ledger.missing:
            return SupervisorDecision("hold", "state_missing", 0, 0.0)
        if self._ledger.tampered:
            return SupervisorDecision("hold", "state_tampered", 0, 0.0)
        if observation.tenant_id != self.tenant_id:
            return SupervisorDecision("hold", "tenant_mismatch", 0, 0.0)
        count = self._ledger.count(self.tenant_id)
        if emergency_stop or self._emergency_stop():
            return SupervisorDecision("stop", "emergency_stop", count, 0.0)
        held = self._ledger.hold_reason(self.tenant_id)
        if held:
            return SupervisorDecision("hold", held, count, 0.0)
        if observation.dead_jobs >= self.max_dead_jobs:
            self._ledger.append(self.tenant_id, "hold", "resource_limit", count)
            return SupervisorDecision("hold", "resource_limit", count, 0.0)
        now = float(self._clock())
        if float(observation.heartbeat_at) > now + _FUTURE_SKEW_SECONDS:
            return SupervisorDecision("hold", "heartbeat_in_future", count, 0.0)
        stale = (now - float(observation.heartbeat_at)) > self.heartbeat_timeout_seconds
        if not stale:
            return SupervisorDecision("continue", "healthy", count, 0.0)
        waiting = self._not_before.get(self.tenant_id, 0.0)
        if now < waiting:
            return SupervisorDecision("wait", "backoff", count, waiting - now)
        if count >= self.max_restarts:
            self._ledger.append(self.tenant_id, "hold", "restart_budget_exhausted", count)
            return SupervisorDecision("hold", "restart_budget_exhausted", count, 0.0)
        count += 1
        self._ledger.append(self.tenant_id, "restart", "heartbeat_lost", count)
        backoff = min(self.max_backoff_seconds, self.base_backoff_seconds * (2 ** (count - 1)))
        self._not_before[self.tenant_id] = now + backoff
        reason = "stuck_job" if observation.running_jobs > 0 else "heartbeat_lost"
        return SupervisorDecision("restart", reason, count, backoff)

    def release_hold(self, _worker_id: str) -> None:
        """A caller cannot clear a persisted hold or restart count."""
        raise SupervisorError("hold_requires_controller")

    def _emergency_stop(self) -> bool:
        return emergency_stop_active()
