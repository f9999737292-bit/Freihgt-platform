"""Durable planning queue for autopilot phase 0.3A.

This store is not the GitHub Issue execution queue. It does not create Cloud
Agents, merge pull requests, or deploy. A missing budget is null, never zero.
"""

from __future__ import annotations

import json
import os
import time
from contextlib import contextmanager
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Iterator

AGENTS = frozenset("ABCDEFGHI")
CREATED = "CREATED"
READY = "READY"
CLAIMED = "CLAIMED"
RUNNING = "RUNNING"
REVIEW_REQUIRED = "REVIEW_REQUIRED"
COMPLETED = "COMPLETED"
BLOCKED = "BLOCKED"
FAILED = "FAILED"
CANCELLED = "CANCELLED"

STATUSES = (
    CREATED,
    READY,
    CLAIMED,
    RUNNING,
    REVIEW_REQUIRED,
    COMPLETED,
    BLOCKED,
    FAILED,
    CANCELLED,
)

ACTIVE = {CLAIMED, RUNNING}

TRANSITIONS = {
    CREATED: {READY, BLOCKED, CANCELLED},
    READY: {CLAIMED, BLOCKED, CANCELLED},
    CLAIMED: {RUNNING, READY, BLOCKED, FAILED, CANCELLED},
    RUNNING: {REVIEW_REQUIRED, READY, BLOCKED, FAILED, CANCELLED},
    REVIEW_REQUIRED: {COMPLETED, BLOCKED, FAILED, CANCELLED},
    COMPLETED: set(),
    BLOCKED: {READY, CANCELLED},
    FAILED: {READY, CANCELLED},
    CANCELLED: set(),
}

DENIED_ACCESS = (
    "merge",
    "deploy",
    "staging",
    "production",
    "production_secrets",
    "paid_automation",
)


class QueueError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


def can_transition(current: str, target: str) -> bool:
    if current == target:
        return True
    return target in TRANSITIONS.get(current, set())


def _iso(moment: datetime) -> str:
    return moment.astimezone(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def _parse(value: str) -> datetime:
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


@dataclass
class Task:
    id: str
    agent: str
    priority: int
    project: str
    task_type: str
    allowed_paths: tuple[str, ...]
    base_sha: str
    acceptance: tuple[str, ...]
    requested_actions: tuple[str, ...]
    depends_on: tuple[str, ...]
    max_budget_cents: float | None
    requires_cloud_agent: bool
    status: str = CREATED
    lease_owner: str = ""
    lease_expires_at: str = ""
    block_reason: str = ""
    ci_verdict: str = ""
    created_at: str = ""
    history: list[dict] = field(default_factory=list)

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "agent": self.agent,
            "priority": self.priority,
            "project": self.project,
            "task_type": self.task_type,
            "allowed_paths": list(self.allowed_paths),
            "base_sha": self.base_sha,
            "acceptance": list(self.acceptance),
            "access": {name: False for name in DENIED_ACCESS},
            "requested_actions": list(self.requested_actions),
            "depends_on": list(self.depends_on),
            "max_budget_cents": self.max_budget_cents,
            "requires_cloud_agent": self.requires_cloud_agent,
            "status": self.status,
            "lease_owner": self.lease_owner,
            "lease_expires_at": self.lease_expires_at,
            "block_reason": self.block_reason,
            "ci_verdict": self.ci_verdict,
            "created_at": self.created_at,
            "history": list(self.history),
        }

    @classmethod
    def from_dict(cls, raw: dict) -> Task:
        if "max_budget_cents" not in raw:
            budget = None
        else:
            budget = raw["max_budget_cents"]
            if budget is not None:
                budget = float(budget)
        return cls(
            id=str(raw["id"]),
            agent=str(raw["agent"]),
            priority=int(raw["priority"]),
            project=str(raw["project"]),
            task_type=str(raw["task_type"]),
            allowed_paths=tuple(raw.get("allowed_paths") or ()),
            base_sha=str(raw.get("base_sha") or ""),
            acceptance=tuple(raw.get("acceptance") or ()),
            requested_actions=tuple(raw.get("requested_actions") or ()),
            depends_on=tuple(raw.get("depends_on") or ()),
            max_budget_cents=budget,
            requires_cloud_agent=bool(raw.get("requires_cloud_agent", False)),
            status=str(raw.get("status") or CREATED),
            lease_owner=str(raw.get("lease_owner") or ""),
            lease_expires_at=str(raw.get("lease_expires_at") or ""),
            block_reason=str(raw.get("block_reason") or ""),
            ci_verdict=str(raw.get("ci_verdict") or ""),
            created_at=str(raw.get("created_at") or ""),
            history=list(raw.get("history") or []),
        )


class FileLock:
    """Exclusive lock file. A stale lock older than the ttl can be replaced."""

    def __init__(self, path: Path, ttl_seconds: float = 30.0):
        self.path = path
        self.ttl_seconds = ttl_seconds
        self._fd: int | None = None

    def acquire(self, now: datetime, timeout: float = 5.0) -> None:
        deadline = time.monotonic() + timeout
        self.path.parent.mkdir(parents=True, exist_ok=True)
        while True:
            try:
                self._fd = os.open(self.path, os.O_CREAT | os.O_EXCL | os.O_RDWR)
                expiry = _iso(now + timedelta(seconds=self.ttl_seconds))
                os.write(self._fd, f"{os.getpid()} {expiry}\n".encode("ascii"))
                return
            except FileExistsError:
                if self._stale(now):
                    try:
                        self.path.unlink()
                    except FileNotFoundError:
                        pass
                    continue
                if time.monotonic() >= deadline:
                    raise QueueError("lock_timeout")
                time.sleep(0.01)

    def _stale(self, now: datetime) -> bool:
        try:
            text = self.path.read_text(encoding="ascii")
        except OSError:
            return False
        parts = text.split()
        if len(parts) < 2:
            return True
        try:
            return _parse(parts[1]) <= now
        except ValueError:
            return True

    def release(self) -> None:
        fd = self._fd
        self._fd = None
        if fd is None:
            return
        os.close(fd)
        try:
            self.path.unlink()
        except FileNotFoundError:
            pass


class TaskStore:
    def __init__(self, path: Path):
        self.path = path
        self.lock = FileLock(path.with_suffix(".lock"))

    @contextmanager
    def exclusive(self, now: datetime) -> Iterator[dict]:
        self.lock.acquire(now)
        try:
            data = self._read()
            yield data
            self._write(data)
        finally:
            self.lock.release()

    def update(self, now: datetime, mutate):
        with self.exclusive(now) as data:
            return mutate(data)

    def _read(self) -> dict:
        if not self.path.exists():
            return {"version": 1, "tasks": {}, "journal": []}
        raw = json.loads(self.path.read_text(encoding="utf-8"))
        raw.setdefault("tasks", {})
        raw.setdefault("journal", [])
        return raw

    def _write(self, data: dict) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        temporary = self.path.with_suffix(".tmp")
        temporary.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        os.replace(temporary, self.path)

    def enqueue(self, task: Task, now: datetime, actor: str) -> Task:
        if not task.created_at:
            task.created_at = _iso(now)
        task.status = CREATED
        task.history = [
            {
                "at": task.created_at,
                "actor": actor,
                "from": "",
                "to": CREATED,
                "reason": "created",
            }
        ]
        with self.exclusive(now) as data:
            if task.id in data["tasks"]:
                raise QueueError("duplicate_task")
            data["tasks"][task.id] = task.to_dict()
            data["journal"].append(
                {"at": task.created_at, "actor": actor, "task_id": task.id, "action": "enqueued"}
            )
        return task

    def get(self, task_id: str, now: datetime) -> Task | None:
        with self.exclusive(now) as data:
            raw = data["tasks"].get(task_id)
            return Task.from_dict(raw) if raw else None

    def list_tasks(self, now: datetime) -> list[Task]:
        with self.exclusive(now) as data:
            return [Task.from_dict(item) for item in data["tasks"].values()]

    def journal(self, now: datetime) -> list[dict]:
        with self.exclusive(now) as data:
            return list(data["journal"])
