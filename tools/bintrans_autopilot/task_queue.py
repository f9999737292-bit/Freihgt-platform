"""Durable planning queue for autopilot phase 0.3A.

This store is not the GitHub Issue execution queue. It does not create Cloud
Agents, merge pull requests, or deploy. A missing budget is null, never zero.

The file lock is a single-host OS lock. It is not an NFS lock and it is not a
multi-host lease. Lock expiry is not used to take another process's lock.
"""

from __future__ import annotations

import errno
import json
import os
import threading
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
CI_CONFIRMED = "CONFIRMED"
CI_LOST = "LOST"

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
TERMINAL = frozenset({COMPLETED, CANCELLED})

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
    if current in TERMINAL:
        return False
    if current == target:
        return True
    return target in TRANSITIONS.get(current, set())


def _iso(moment: datetime) -> str:
    return moment.astimezone(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def _parse(value: str) -> datetime:
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


_BUSY_ERRNOS = {13, 36, errno.EACCES, errno.EAGAIN, getattr(errno, "EWOULDBLOCK", errno.EAGAIN)}


def _lock_is_busy(exc: OSError) -> bool:
    if isinstance(exc, (BlockingIOError, PermissionError)):
        return True
    if getattr(exc, "winerror", None) in {33, 36}:
        return True
    return exc.errno in _BUSY_ERRNOS


def _lock_fd(fd: int) -> None:
    os.lseek(fd, 0, os.SEEK_SET)
    if os.name == "nt":
        _windows_file_lock(fd, unlock=False)
        return
    import fcntl

    fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)


def _unlock_fd(fd: int) -> None:
    os.lseek(fd, 0, os.SEEK_SET)
    if os.name == "nt":
        _windows_file_lock(fd, unlock=True)
        return
    import fcntl

    fcntl.flock(fd, fcntl.LOCK_UN)


def _windows_file_lock(fd: int, *, unlock: bool) -> None:
    """Non-blocking exclusive byte lock. FAIL_IMMEDIATELY never waits."""
    import ctypes
    from ctypes import wintypes
    import msvcrt

    class _Overlapped(ctypes.Structure):
        _fields_ = [
            ("Internal", ctypes.c_void_p),
            ("InternalHigh", ctypes.c_void_p),
            ("Offset", wintypes.DWORD),
            ("OffsetHigh", wintypes.DWORD),
            ("hEvent", ctypes.c_void_p),
        ]

    kernel = ctypes.windll.kernel32
    handle = wintypes.HANDLE(msvcrt.get_osfhandle(fd))
    overlapped = _Overlapped()
    if unlock:
        kernel.UnlockFileEx.argtypes = [
            wintypes.HANDLE,
            wintypes.DWORD,
            wintypes.DWORD,
            wintypes.DWORD,
            ctypes.POINTER(_Overlapped),
        ]
        kernel.UnlockFileEx.restype = wintypes.BOOL
        ok = kernel.UnlockFileEx(handle, 0, 1, 0, ctypes.byref(overlapped))
    else:
        kernel.LockFileEx.argtypes = [
            wintypes.HANDLE,
            wintypes.DWORD,
            wintypes.DWORD,
            wintypes.DWORD,
            wintypes.DWORD,
            ctypes.POINTER(_Overlapped),
        ]
        kernel.LockFileEx.restype = wintypes.BOOL
        # LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY
        ok = kernel.LockFileEx(handle, 0x00000003, 0, 1, 0, ctypes.byref(overlapped))
    if ok:
        return
    error = ctypes.WinError()
    if not unlock and getattr(error, "winerror", None) in {32, 33}:
        raise PermissionError(error.errno, error.strerror, error.filename)
    raise error


def _fsync_directory(directory: Path) -> None:
    """Sync the directory entry where the OS supports it.

    Windows has no directory fsync. Same-directory replace plus file fsync is
    the local durability guarantee there. This does not extend to NFS.
    """
    flags = getattr(os, "O_DIRECTORY", None)
    if flags is None:
        return
    try:
        fd = os.open(directory, os.O_RDONLY | flags)
    except OSError:
        return
    try:
        os.fsync(fd)
    except OSError:
        return
    finally:
        os.close(fd)


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
    lease_generation: int = 0
    block_reason: str = ""
    ci_verdict: str = ""
    ci_confirmation: str = ""
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
            "lease_generation": self.lease_generation,
            "block_reason": self.block_reason,
            "ci_verdict": self.ci_verdict,
            "ci_confirmation": self.ci_confirmation,
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
            lease_generation=int(raw.get("lease_generation") or 0),
            block_reason=str(raw.get("block_reason") or ""),
            ci_verdict=str(raw.get("ci_verdict") or ""),
            ci_confirmation=str(raw.get("ci_confirmation") or ""),
            created_at=str(raw.get("created_at") or ""),
            history=list(raw.get("history") or []),
        )


class FileLock:
    """Single-host OS file lock.

    Ownership is the operating-system lock, not deletion of the lock file.
    A timestamp in the file never transfers ownership. A dead holder's lock
    is released by the OS when its process exits. NFS and multi-host locking
    are outside this phase.
    """

    def __init__(self, path: Path):
        self.path = path
        self._fd: int | None = None
        self._guard = threading.Lock()

    def acquire(self, timeout: float = 5.0) -> None:
        deadline = time.monotonic() + timeout
        remaining = deadline - time.monotonic()
        if remaining <= 0 or not self._guard.acquire(timeout=remaining):
            raise QueueError("lock_timeout")
        try:
            self._acquire_os(deadline)
        except BaseException:
            self._guard.release()
            raise

    def _acquire_os(self, deadline: float) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        try:
            fd = os.open(self.path, os.O_CREAT | os.O_RDWR, 0o600)
        except PermissionError as exc:
            raise QueueError("lock_unavailable") from exc
        except OSError as exc:
            raise QueueError("lock_unavailable") from exc
        try:
            while True:
                try:
                    _lock_fd(fd)
                    self._fd = fd
                    return
                except OSError as exc:
                    if not _lock_is_busy(exc):
                        raise QueueError("lock_unavailable") from exc
                    if time.monotonic() >= deadline:
                        raise QueueError("lock_timeout") from exc
                time.sleep(0.01)
        except BaseException:
            os.close(fd)
            self._fd = None
            raise

    def release(self) -> None:
        fd = self._fd
        self._fd = None
        try:
            if fd is None:
                return
            try:
                _unlock_fd(fd)
            finally:
                os.close(fd)
        finally:
            self._guard.release()


class TaskStore:
    def __init__(self, path: Path):
        self.path = path
        self.lock = FileLock(path.with_suffix(".lock"))

    @contextmanager
    def exclusive(self, now: datetime) -> Iterator[dict]:
        del now
        self.lock.acquire()
        try:
            data = self._load()
            yield data
            self._write(data)
        finally:
            self.lock.release()

    def update(self, now: datetime, mutate):
        with self.exclusive(now) as data:
            return mutate(data)

    def _read(self) -> dict:
        self.lock.acquire()
        try:
            return self._load()
        finally:
            self.lock.release()

    def _load(self) -> dict:
        primary_exists = self.path.exists()
        if primary_exists:
            primary = self._valid_document(self.path)
            if primary is not None:
                return primary
        backup_path = self._backup_path()
        backup_exists = backup_path.exists()
        if backup_exists:
            backup = self._valid_document(backup_path)
            if backup is not None:
                return backup
        if primary_exists or backup_exists:
            raise QueueError("store_corrupt")
        return {"version": 1, "tasks": {}, "journal": []}

    def _valid_document(self, path: Path) -> dict | None:
        if not path.exists():
            return None
        try:
            raw = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, UnicodeError, json.JSONDecodeError):
            return None
        if not isinstance(raw, dict):
            return None
        if not isinstance(raw.get("tasks"), dict) or not isinstance(raw.get("journal"), list):
            return None
        return raw

    def _backup_path(self) -> Path:
        return self.path.with_name(self.path.name + ".bak")

    def _temporary_path(self) -> Path:
        return self.path.with_name(self.path.name + ".tmp")

    def _corrupt_path(self) -> Path:
        return self.path.with_name(self.path.name + ".corrupt")

    def _write(self, data: dict) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        payload = (json.dumps(data, indent=2, sort_keys=True) + "\n").encode("utf-8")
        temporary = self._temporary_path()
        self._write_durable(temporary, payload)
        if self.path.exists() and self._valid_document(self.path) is None:
            quarantine = self._corrupt_path()
            if quarantine.exists():
                temporary.unlink(missing_ok=True)
                raise QueueError("store_corrupt")
            os.replace(self.path, quarantine)
            _fsync_directory(self.path.parent)
        os.replace(temporary, self.path)
        _fsync_directory(self.path.parent)
        self._write_durable(self._backup_path().with_name(self.path.name + ".bak.tmp"), self.path.read_bytes())
        os.replace(self._backup_path().with_name(self.path.name + ".bak.tmp"), self._backup_path())
        _fsync_directory(self.path.parent)

    def _write_durable(self, path: Path, payload: bytes) -> None:
        fd = os.open(path, os.O_CREAT | os.O_TRUNC | os.O_RDWR, 0o600)
        try:
            os.write(fd, payload)
            os.fsync(fd)
        finally:
            os.close(fd)

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
        del now
        raw = self._read().get("tasks", {}).get(task_id)
        return Task.from_dict(raw) if raw else None

    def list_tasks(self, now: datetime) -> list[Task]:
        del now
        return [Task.from_dict(item) for item in self._read().get("tasks", {}).values()]

    def journal(self, now: datetime) -> list[dict]:
        del now
        return list(self._read().get("journal", []))
