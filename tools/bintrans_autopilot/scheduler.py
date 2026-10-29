"""Dry-run scheduler for the 0.3A planning queue.

Mode is fixed to DRY_RUN. This module does not import the Cursor client and
does not call merge or deployment commands. Unknown spend stays unknown.

Path checks and the file lock are local to one host. They do not authorize
NFS, multi-host leasing, or a 24/7 distributed worker.
"""

from __future__ import annotations

import os
import re
from collections.abc import Mapping
from dataclasses import dataclass
from datetime import datetime, timedelta
from pathlib import Path

from bintrans_autopilot.config import load_config
from bintrans_autopilot.task_queue import (
    ACTIVE,
    BLOCKED,
    CI_CONFIRMED,
    CI_LOST,
    CLAIMED,
    COMPLETED,
    CREATED,
    READY,
    REVIEW_REQUIRED,
    RUNNING,
    TERMINAL,
    QueueError,
    Task,
    TaskStore,
    _iso,
    _parse,
    can_transition,
)

_SHA = re.compile(r"^[0-9a-f]{40}$")
_DRIVE = re.compile(r"^[A-Za-z]:")
# Dry-run records no executable action. An action is forbidden until it is
# added here. This set is not an executor.
_ALLOWED_REQUESTED_ACTIONS = frozenset()
_ALLOWED_JOURNAL_ACTIONS = frozenset(
    {
        "idle",
        "waiting",
        "dry_run_planned",
        "ci_recorded",
        "ci_confirmation_lost",
        "transition",
        "refused",
    }
)


class SchedulerModeError(QueueError):
    pass


def task_path_components(path: str) -> tuple[str, ...] | None:
    """Return relative components, or None when the path is rejected.

    ``.`` and empty components are rejected rather than collapsed. ``..`` is
    rejected as its own component. Absolute paths, drive paths, and UNC paths
    are rejected. Character stripping is not used as normalization.
    """
    if not isinstance(path, str) or path == "" or "\x00" in path:
        return None
    if path != path.strip():
        return None
    normalized = path.replace("\\", "/")
    if normalized.startswith("/") or _DRIVE.match(normalized):
        return None
    parts = tuple(normalized.split("/"))
    if any(part in {"", ".", ".."} for part in parts):
        return None
    return parts


def prefix_components(prefix: str) -> tuple[str, ...] | None:
    """Parse a catalog prefix. One trailing slash marks a directory prefix."""
    if not isinstance(prefix, str) or prefix == "" or "\x00" in prefix:
        return None
    normalized = prefix.replace("\\", "/")
    if normalized.startswith("/") or _DRIVE.match(normalized):
        return None
    if normalized.endswith("/"):
        normalized = normalized[:-1]
    if normalized == "":
        return None
    parts = tuple(normalized.split("/"))
    if any(part in {"", ".", ".."} for part in parts):
        return None
    return parts


def components_under(prefix: tuple[str, ...], path: tuple[str, ...]) -> bool:
    return len(path) >= len(prefix) and path[: len(prefix)] == prefix


def resolve_task_path(root: Path, relative: str) -> Path | None:
    """Resolve a task path against the canonical workspace root.

    Textual checks happen first. Each component is then resolved so a symlink
    or junction in any parent is rejected when it leaves the root. A relative
    root is not joined to the current directory.
    """
    parts = task_path_components(relative)
    if parts is None or not root.is_absolute():
        return None
    try:
        root_real = root.resolve()
        if not root_real.is_dir():
            return None
        current = root_real
        for part in parts:
            current = (current / part).resolve()
            current.relative_to(root_real)
    except (OSError, ValueError):
        return None
    return current


def trusted_workspace_root(env: Mapping[str, str] | None = None) -> Path:
    """Return the checkout that contains this package.

    Local runs use that checkout only. They do not use the process current
    directory. In GitHub Actions, ``GITHUB_WORKSPACE`` is accepted only when
    it is the same canonical directory. Any other value is refused.
    """
    source = os.environ if env is None else env
    checkout = Path(__file__).resolve().parent.parent.parent
    if not _is_package_checkout(checkout):
        raise SchedulerModeError("workspace_root_unavailable")
    canonical = checkout.resolve()
    if (source.get("GITHUB_ACTIONS") or "").strip().lower() == "true":
        raw = (source.get("GITHUB_WORKSPACE") or "").strip()
        if not raw:
            raise SchedulerModeError("workspace_root_unavailable")
        try:
            actions_root = Path(raw).resolve()
        except OSError as exc:
            raise SchedulerModeError("workspace_root_unavailable") from exc
        if actions_root != canonical:
            raise SchedulerModeError("workspace_root_unavailable")
    return canonical


def _is_package_checkout(root: Path) -> bool:
    config = root / "tools" / "bintrans_autopilot" / "autopilot.config.json"
    git_entry = root / ".git"
    try:
        if not config.is_file() or not (git_entry.is_dir() or git_entry.is_file()):
            return False
        config.resolve().relative_to(root.resolve())
    except (OSError, ValueError):
        return False
    return True


@dataclass(frozen=True)
class Policy:
    mode: str = "DRY_RUN"
    autopilot_enabled: bool = False
    paid_automation_enabled: bool = False
    launch_budget_cents: float | None = 0
    observed_charged_cents: float | None = None
    approved_prefixes: tuple[str, ...] = ()
    forbidden_prefixes: tuple[str, ...] = ()
    workspace_root: str = ""

    def __post_init__(self) -> None:
        if self.mode != "DRY_RUN":
            raise SchedulerModeError("live_mode_refused")


@dataclass(frozen=True)
class TickResult:
    ok: bool
    reason: str
    task_id: str = ""
    status: str = ""
    cloud_agents_started: int = 0
    merge_attempted: bool = False
    deploy_attempted: bool = False


class Scheduler:
    def __init__(
        self,
        store: TaskStore,
        scheduler_id: str,
        policy: Policy,
        now,
        lease_seconds: int = 900,
    ):
        if policy.mode != "DRY_RUN":
            raise SchedulerModeError("live_mode_refused")
        self.store = store
        self.scheduler_id = scheduler_id
        self.policy = policy
        self.now = now
        self.lease_seconds = lease_seconds
        self._fences: dict[str, int] = {}

    def tick(self, stop_after: str = "review") -> TickResult:
        moment = self.now()

        def apply(data: dict) -> TickResult:
            self._recover(data, moment)
            blocked = self._prepare(data, moment)
            chosen = self._next_ready(data)
            if chosen is None:
                if blocked:
                    return blocked[0]
                self._log(data, moment, "", "idle", "no_ready_task")
                return TickResult(True, "no_ready_task")
            reason = self._execution_block(chosen, data)
            if reason:
                self._move(data, chosen, BLOCKED, moment, reason)
                return TickResult(False, reason, chosen.id, BLOCKED)
            self._lease(data, chosen, moment)
            if stop_after == "claimed":
                return TickResult(True, "claimed", chosen.id, CLAIMED)
            self._move(data, chosen, RUNNING, moment, "dry_run_started")
            self._log(data, moment, chosen.id, "dry_run_planned", "no_side_effects")
            self._move(data, chosen, REVIEW_REQUIRED, moment, "dry_run_no_side_effects")
            return TickResult(True, "review_required", chosen.id, REVIEW_REQUIRED)

        return self.store.update(moment, apply)

    def finish(self, task_id: str) -> TickResult:
        """Continue a claimed task only for the fence this scheduler observed."""
        moment = self.now()

        def apply(data: dict) -> TickResult:
            task = self._task(data, task_id)
            if task is None:
                return TickResult(False, "missing_task", task_id)
            if task.status in TERMINAL:
                return TickResult(False, "terminal_immutable", task_id, task.status)
            if task.status not in {CLAIMED, RUNNING}:
                return TickResult(False, "not_leased", task_id, task.status)
            if self._fences.get(task.id) != task.lease_generation or task.lease_owner != self.scheduler_id:
                return TickResult(False, "lease_lost", task_id, task.status)
            if self._expired(task, moment):
                self._move(data, task, READY, moment, "lease_expired")
                return TickResult(False, "lease_lost", task_id, READY)
            if task.status == CLAIMED:
                self._move(data, task, RUNNING, moment, "dry_run_started")
            self._log(data, moment, task.id, "dry_run_planned", "no_side_effects")
            self._move(data, task, REVIEW_REQUIRED, moment, "dry_run_no_side_effects")
            return TickResult(True, "review_required", task.id, REVIEW_REQUIRED)

        return self.store.update(moment, apply)

    def record_ci(self, task_id: str, result: str) -> TickResult:
        moment = self.now()
        verdict = {"success": "SUCCESS", "failure": "FAIL", "pending": "PENDING"}.get(result, "")

        def apply(data: dict) -> TickResult:
            task = self._task(data, task_id)
            if task is None:
                return TickResult(False, "missing_task", task_id)
            if task.status in TERMINAL:
                return self._record_terminal_ci(data, task, moment, verdict)
            if not verdict:
                self._move(data, task, BLOCKED, moment, "ci_unknown")
                return TickResult(False, "ci_unknown", task_id, BLOCKED)
            task.ci_verdict = verdict
            data["tasks"][task.id] = task.to_dict()
            self._log(data, moment, task_id, "ci_recorded", verdict)
            return TickResult(verdict == "SUCCESS", verdict, task_id, task.status)

        return self.store.update(moment, apply)

    def _record_terminal_ci(self, data: dict, task: Task, moment: datetime, verdict: str) -> TickResult:
        """Terminal status stays put. A late failure withdraws CI confirmation."""
        self._log(data, moment, task.id, "ci_recorded", verdict or "UNKNOWN")
        if task.status != COMPLETED or verdict not in {"FAIL", ""}:
            return TickResult(False, "terminal_immutable", task.id, task.status)
        if task.ci_confirmation != CI_LOST or (verdict == "FAIL" and task.ci_verdict != "FAIL"):
            if verdict == "FAIL":
                task.ci_verdict = "FAIL"
            task.ci_confirmation = CI_LOST
            task.history.append(
                {
                    "at": _iso(moment),
                    "actor": self.scheduler_id,
                    "from": task.status,
                    "to": task.status,
                    "reason": "ci_confirmation_lost" if verdict == "FAIL" else "ci_unknown",
                }
            )
            data["tasks"][task.id] = task.to_dict()
        reason = "ci_confirmation_lost" if verdict == "FAIL" else "ci_unknown"
        self._log(data, moment, task.id, "ci_confirmation_lost", "status_unchanged")
        return TickResult(False, reason, task.id, task.status)

    def accept_review(self, task_id: str) -> TickResult:
        moment = self.now()

        def apply(data: dict) -> TickResult:
            task = self._task(data, task_id)
            if task is None:
                return TickResult(False, "missing_task", task_id)
            if task.status in TERMINAL:
                return TickResult(False, "terminal_immutable", task_id, task.status)
            if task.status != REVIEW_REQUIRED:
                return TickResult(False, "not_in_review", task_id, task.status)
            if task.ci_confirmation == CI_LOST:
                return TickResult(False, "ci_confirmation_lost", task_id, task.status)
            if task.ci_verdict != "SUCCESS":
                reason = "ci_unknown" if not task.ci_verdict else "ci_not_pass"
                self._move(data, task, BLOCKED, moment, reason)
                return TickResult(False, reason, task_id, BLOCKED)
            task.ci_confirmation = CI_CONFIRMED
            self._move(data, task, COMPLETED, moment, "review_accepted")
            return TickResult(True, "completed", task_id, COMPLETED)

        return self.store.update(moment, apply)

    def _prepare(self, data: dict, moment: datetime) -> list[TickResult]:
        blocked: list[TickResult] = []
        for raw in list(data["tasks"].values()):
            task = Task.from_dict(raw)
            if task.status != CREATED:
                continue
            reason = self._ready_block(task, data)
            if reason == "dependency_pending":
                self._log(data, moment, task.id, "waiting", reason)
                continue
            if reason:
                self._move(data, task, BLOCKED, moment, reason)
                blocked.append(TickResult(False, reason, task.id, BLOCKED))
                continue
            self._move(data, task, READY, moment, "gates_passed")
        return blocked

    def _recover(self, data: dict, moment: datetime) -> None:
        for raw in list(data["tasks"].values()):
            task = Task.from_dict(raw)
            if task.status not in ACTIVE:
                continue
            if not task.lease_expires_at or not task.lease_owner:
                self._move(data, task, BLOCKED, moment, "missing_lease")
                continue
            if self._expired(task, moment):
                task.lease_owner = ""
                task.lease_expires_at = ""
                self._move(data, task, READY, moment, "lease_expired")

    def _next_ready(self, data: dict) -> Task | None:
        if any(Task.from_dict(raw).status in ACTIVE for raw in data["tasks"].values()):
            return None
        ready = [Task.from_dict(raw) for raw in data["tasks"].values() if Task.from_dict(raw).status == READY]
        ready.sort(key=lambda item: (item.priority, item.created_at, item.id))
        return ready[0] if ready else None

    def _ready_block(self, task: Task, data: dict) -> str:
        identity = self._identity_block(task)
        if identity:
            return identity
        if task.ci_confirmation == CI_LOST:
            return "ci_confirmation_lost"
        if any(action not in _ALLOWED_REQUESTED_ACTIONS for action in task.requested_actions):
            return "forbidden_action"
        for dep in task.depends_on:
            other = data["tasks"].get(dep)
            if other is None:
                return "missing_dependency"
            if other.get("status") != COMPLETED:
                return "dependency_pending"
        return ""

    def _execution_block(self, task: Task, data: dict) -> str:
        identity = self._identity_block(task)
        if identity:
            return identity
        if task.ci_confirmation == CI_LOST:
            return "ci_confirmation_lost"
        if any(action not in _ALLOWED_REQUESTED_ACTIONS for action in task.requested_actions):
            return "forbidden_action"
        paid = self._paid_block(task)
        if paid:
            return paid
        for dep in task.depends_on:
            other = data["tasks"].get(dep)
            if other is None:
                return "missing_dependency"
            if other.get("status") != COMPLETED:
                return "dependency_pending"
        return ""

    def _identity_block(self, task: Task) -> str:
        if task.agent not in set("ABCDEFGHI"):
            return "invalid_agent"
        if not task.project or not task.task_type:
            return "missing_scope"
        if not task.acceptance:
            return "missing_acceptance"
        if not _SHA.fullmatch(task.base_sha):
            return "invalid_base_sha"
        if not task.allowed_paths:
            return "missing_paths"
        if self._workspace_root_block():
            return "workspace_root_unavailable"
        for path in task.allowed_paths:
            parts = task_path_components(path)
            if parts is None:
                return "invalid_path"
            if self._prefix_hit(self.policy.forbidden_prefixes, parts, invalid_matches=True):
                return "forbidden_path"
            if self.policy.approved_prefixes and not self._prefix_hit(
                self.policy.approved_prefixes, parts, invalid_matches=False
            ):
                return "unapproved_path"
            if resolve_task_path(Path(self.policy.workspace_root), path) is None:
                return "path_escape"
        if task.max_budget_cents is None:
            return "unknown_budget"
        if task.max_budget_cents < 0:
            return "invalid_budget"
        return ""

    def _prefix_hit(self, prefixes: tuple[str, ...], parts: tuple[str, ...], *, invalid_matches: bool) -> bool:
        for prefix in prefixes:
            parsed = prefix_components(prefix)
            if parsed is None:
                if invalid_matches:
                    return True
                continue
            if components_under(parsed, parts):
                return True
        return False

    def _workspace_root_block(self) -> str:
        raw = self.policy.workspace_root.strip()
        if not raw:
            return "workspace_root_unavailable"
        root = Path(raw)
        if not root.is_absolute():
            return "workspace_root_unavailable"
        try:
            if not root.resolve().is_dir():
                return "workspace_root_unavailable"
        except OSError:
            return "workspace_root_unavailable"
        return ""

    def _paid_block(self, task: Task) -> str:
        if not task.requires_cloud_agent:
            return ""
        if self.policy.observed_charged_cents is None and self.policy.launch_budget_cents not in (0, 0.0):
            return "unknown_spend"
        if self.policy.launch_budget_cents is None:
            return "unknown_budget"
        if not self.policy.paid_automation_enabled:
            return "paid_automation_disabled"
        if self.policy.launch_budget_cents <= 0 or task.max_budget_cents == 0:
            return "paid_budget_zero"
        return "dry_run_refuses_paid_launch"

    def _lease(self, data: dict, task: Task, moment: datetime) -> None:
        task.lease_generation += 1
        task.lease_owner = self.scheduler_id
        task.lease_expires_at = _iso(moment + timedelta(seconds=self.lease_seconds))
        self._fences[task.id] = task.lease_generation
        self._move(data, task, CLAIMED, moment, "claimed")

    def _expired(self, task: Task, moment: datetime) -> bool:
        if not task.lease_expires_at:
            return True
        return _parse(task.lease_expires_at) <= moment

    def _task(self, data: dict, task_id: str) -> Task | None:
        raw = data["tasks"].get(task_id)
        return Task.from_dict(raw) if raw else None

    def _move(self, data: dict, task: Task, target: str, moment: datetime, reason: str) -> None:
        if task.status in TERMINAL:
            return
        if not can_transition(task.status, target):
            target_reason = f"illegal_transition:{task.status}->{target}"
            if task.status != BLOCKED and can_transition(task.status, BLOCKED):
                previous = task.status
                task.status = BLOCKED
                task.block_reason = target_reason
                task.history.append(
                    {"at": _iso(moment), "actor": self.scheduler_id, "from": previous, "to": BLOCKED, "reason": target_reason}
                )
                data["tasks"][task.id] = task.to_dict()
            return
        previous = task.status
        task.status = target
        if target == BLOCKED:
            task.block_reason = reason
        if target not in ACTIVE:
            task.lease_owner = ""
            task.lease_expires_at = ""
        task.history.append(
            {"at": _iso(moment), "actor": self.scheduler_id, "from": previous, "to": target, "reason": reason}
        )
        data["tasks"][task.id] = task.to_dict()
        self._log(data, moment, task.id, "transition", f"{previous}->{target}:{reason}")

    def _log(self, data: dict, moment: datetime, task_id: str, action: str, detail: str) -> None:
        if action not in _ALLOWED_JOURNAL_ACTIONS:
            action = "refused"
            detail = "side_effect_refused"
        data["journal"].append(
            {
                "at": _iso(moment),
                "actor": self.scheduler_id,
                "task_id": task_id,
                "action": action,
                "detail": detail,
            }
        )


def policy_from_config(env: dict[str, str] | None = None) -> Policy:
    """Read committed limits. Paid automation stays off in this phase."""
    config = load_config()
    if config.autopilot_enabled:
        raise SchedulerModeError("config_autopilot_enabled")
    root = trusted_workspace_root(env)
    return Policy(
        mode="DRY_RUN",
        autopilot_enabled=False,
        paid_automation_enabled=False,
        launch_budget_cents=float(config.budget_max_charged_cents),
        observed_charged_cents=None,
        approved_prefixes=config.approved_path_prefixes,
        forbidden_prefixes=config.hard_forbidden_prefixes,
        workspace_root=str(root),
    )


def dry_run_main(argv: list[str] | None = None) -> int:
    del argv
    try:
        policy = policy_from_config()
    except SchedulerModeError as exc:
        print("DRY_RUN=YES")
        print("AUTOPILOT_ENABLED=NO")
        print("PAID_AUTOMATION_ENABLED=NO")
        print("CLOUD_AGENTS_STARTED=0")
        print("OBSERVED_CHARGED_CENTS=UNKNOWN")
        print(f"REASON={exc.code}")
        return 2
    print("DRY_RUN=YES")
    print("AUTOPILOT_ENABLED=NO")
    print("PAID_AUTOMATION_ENABLED=NO")
    print("CLOUD_AGENTS_STARTED=0")
    print("OBSERVED_CHARGED_CENTS=UNKNOWN")
    print(f"LAUNCH_BUDGET_CENTS={policy.launch_budget_cents}")
    print("PAID_LAUNCH=BLOCKED")
    print("MERGE=BLOCKED")
    print("DEPLOY=BLOCKED")
    print("HOST_LOCK=LOCAL_ONLY")
    print("NFS_LOCK=NO")
    print("WORKSPACE_ROOT=TRUSTED")
    return 0
