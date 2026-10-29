"""Dry-run scheduler for the 0.3A planning queue.

Mode is fixed to DRY_RUN. This module does not import the Cursor client and
does not call merge or deployment commands. Unknown spend stays unknown.
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from datetime import datetime, timedelta

from bintrans_autopilot.config import load_config
from bintrans_autopilot.task_queue import (
    ACTIVE,
    BLOCKED,
    CANCELLED,
    CLAIMED,
    COMPLETED,
    CREATED,
    FAILED,
    READY,
    REVIEW_REQUIRED,
    RUNNING,
    QueueError,
    Task,
    TaskStore,
    _iso,
    _parse,
    can_transition,
)

_SHA = re.compile(r"^[0-9a-f]{40}$")
_FORBIDDEN_ACTIONS = frozenset({"merge", "deploy", "staging", "production", "production_secrets"})
_SIDE_EFFECTS = frozenset({"merge", "deploy", "staging_change", "production_change", "cloud_agent_create"})


class SchedulerModeError(QueueError):
    pass


@dataclass(frozen=True)
class Policy:
    mode: str = "DRY_RUN"
    autopilot_enabled: bool = False
    paid_automation_enabled: bool = False
    launch_budget_cents: float | None = 0
    observed_charged_cents: float | None = None
    approved_prefixes: tuple[str, ...] = ()
    forbidden_prefixes: tuple[str, ...] = ()

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
        """Continue a claimed task only while its lease still belongs to this scheduler."""
        moment = self.now()

        def apply(data: dict) -> TickResult:
            task = self._task(data, task_id)
            if task is None:
                return TickResult(False, "missing_task", task_id)
            if task.status not in {CLAIMED, RUNNING}:
                return TickResult(False, "not_leased", task_id, task.status)
            if task.lease_owner != self.scheduler_id or self._expired(task, moment):
                if task.status in ACTIVE and self._expired(task, moment):
                    self._move(data, task, READY, moment, "lease_expired")
                    task.lease_owner = ""
                    task.lease_expires_at = ""
                    data["tasks"][task.id] = task.to_dict()
                return TickResult(False, "lease_lost", task_id, data["tasks"][task.id]["status"])
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
            if not verdict:
                self._move(data, task, BLOCKED, moment, "ci_unknown")
                return TickResult(False, "ci_unknown", task_id, BLOCKED)
            task.ci_verdict = verdict
            data["tasks"][task.id] = task.to_dict()
            self._log(data, moment, task_id, "ci_recorded", verdict)
            if verdict != "SUCCESS" and task.status == COMPLETED:
                self._move(data, task, BLOCKED, moment, "ci_not_pass")
            return TickResult(verdict == "SUCCESS", verdict, task_id, task.status)

        return self.store.update(moment, apply)

    def accept_review(self, task_id: str) -> TickResult:
        moment = self.now()

        def apply(data: dict) -> TickResult:
            task = self._task(data, task_id)
            if task is None:
                return TickResult(False, "missing_task", task_id)
            if task.status != REVIEW_REQUIRED:
                return TickResult(False, "not_in_review", task_id, task.status)
            if task.ci_verdict != "SUCCESS":
                reason = "ci_unknown" if not task.ci_verdict else "ci_not_pass"
                self._move(data, task, BLOCKED, moment, reason)
                return TickResult(False, reason, task_id, BLOCKED)
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
        if _FORBIDDEN_ACTIONS.intersection(task.requested_actions):
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
        for path in task.allowed_paths:
            normalized = path.replace("\\", "/").lstrip("./")
            if not normalized or normalized.startswith("/") or ".." in normalized.split("/"):
                return "invalid_path"
            if any(normalized == prefix.rstrip("/") or normalized.startswith(prefix) for prefix in self.policy.forbidden_prefixes):
                return "forbidden_path"
            if self.policy.approved_prefixes and not any(
                normalized == prefix.rstrip("/") or normalized.startswith(prefix) for prefix in self.policy.approved_prefixes
            ):
                return "unapproved_path"
        if task.max_budget_cents is None:
            return "unknown_budget"
        if task.max_budget_cents < 0:
            return "invalid_budget"
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
        task.lease_owner = self.scheduler_id
        task.lease_expires_at = _iso(moment + timedelta(seconds=self.lease_seconds))
        self._move(data, task, CLAIMED, moment, "claimed")

    def _expired(self, task: Task, moment: datetime) -> bool:
        if not task.lease_expires_at:
            return True
        return _parse(task.lease_expires_at) <= moment

    def _task(self, data: dict, task_id: str) -> Task | None:
        raw = data["tasks"].get(task_id)
        return Task.from_dict(raw) if raw else None

    def _move(self, data: dict, task: Task, target: str, moment: datetime, reason: str) -> None:
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
        if action in _SIDE_EFFECTS:
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
    del env
    config = load_config()
    if config.autopilot_enabled:
        raise SchedulerModeError("config_autopilot_enabled")
    return Policy(
        mode="DRY_RUN",
        autopilot_enabled=False,
        paid_automation_enabled=False,
        launch_budget_cents=float(config.budget_max_charged_cents),
        observed_charged_cents=None,
        approved_prefixes=config.approved_path_prefixes,
        forbidden_prefixes=config.hard_forbidden_prefixes,
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
    return 0
