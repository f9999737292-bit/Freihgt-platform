"""Deterministic dispatcher. GitHub Issues are the queue; claim refs prevent double launch."""

from __future__ import annotations

import uuid
from dataclasses import dataclass, field
from datetime import datetime, timezone

from bintrans_autopilot.approval import trusted_approval
from bintrans_autopilot.budget import assess_budget, budget_windows
from bintrans_autopilot.ci_gate import evaluate_ci, required_checks
from bintrans_autopilot.config import AutopilotConfig, runtime_enabled
from bintrans_autopilot.contract import parse_contract, read_state, write_state
from bintrans_autopilot.cursor_api import ACTIVE_RUN_STATUSES, FAILED_RUN_STATUSES, CursorApiError
from bintrans_autopilot.github_api import GithubError
from bintrans_autopilot.policy import (
    assert_feature_branch,
    base_is_current,
    critical_domains,
    dependencies_closed,
    feature_branch,
    shared_path_allowed,
    validate_scope,
)
from bintrans_autopilot.pr_identity import parse_pull_url, pr_identity_error, repo_full_name
from bintrans_autopilot.prompts import fix_prompt, implementation_body, qa_body
from bintrans_autopilot.qa_gate import format_review_diff, merge_allowed, review_diff
from bintrans_autopilot.verdict import parse_verdict
from bintrans_autopilot.states import (
    ACTIVE,
    BLOCKED,
    BUDGET_HOLD,
    CI_CHECK,
    CLAIMED,
    IMPLEMENTING,
    INDEPENDENT_QA,
    NEEDS_CONTROLLER,
    PR_OPEN,
    READY,
    READY_FOR_MERGE,
    can_transition,
)


@dataclass
class DispatchResult:
    enabled: bool
    reason: str
    launched: list[str] = field(default_factory=list)
    skipped: list[str] = field(default_factory=list)
    preserved: list[str] = field(default_factory=list)
    fail_closed: str = ""


def _new_agent_id() -> str:
    return f"bc-{uuid.uuid4()}"


def _status_label(labels: list[str]) -> str:
    for label in labels:
        if label.startswith("autopilot:"):
            return label.split(":", 1)[1]
    return ""


def _with_status(labels: list[str], status: str) -> list[str]:
    kept = [label for label in labels if not label.startswith("autopilot:")]
    if "autopilot" not in kept:
        kept.append("autopilot")
    kept.append(f"autopilot:{status}")
    return kept


def _day(now_iso: str) -> str:
    return now_iso[:10]


class Dispatcher:
    def __init__(self, config: AutopilotConfig, github, cursor, env: dict[str, str], now_iso: str, window: tuple[int, int]):
        self.config = config
        self.github = github
        self.cursor = cursor
        self.env = env
        self.now_iso = now_iso
        self.window = window

    def run(self) -> DispatchResult:
        enabled, reason = runtime_enabled(self.config, self.env)
        if not enabled:
            return DispatchResult(enabled=False, reason=reason)
        if not self.env.get("CURSOR_API_KEY", "").strip():
            return DispatchResult(enabled=True, reason="missing_api_key", fail_closed="missing_api_key")
        now = datetime.fromisoformat(self.now_iso.replace("Z", "+00:00"))
        if now.tzinfo is None:
            now = now.replace(tzinfo=timezone.utc)
        # Spend windows come from now, including the UTC month. The constructor
        # window is not a substitute: a same-day window would hide earlier charges.
        budget = assess_budget(self.config, self.cursor, budget_windows(now), now, self._active_count())
        issues = self.github.list_issues()
        if getattr(self.github, "truncated", False):
            return DispatchResult(
                enabled=True,
                reason="queue_truncated",
                preserved=[str(issue.number) for issue in issues],
            )
        if budget.preserve_task_state:
            return DispatchResult(
                enabled=True,
                reason=budget.reason,
                preserved=[str(issue.number) for issue in issues],
            )
        if not budget.allowed:
            self._hold_ready(issues, budget.reason)
            return DispatchResult(enabled=True, reason=budget.reason)
        if not issues:
            return DispatchResult(enabled=True, reason="empty_queue")
        result = DispatchResult(enabled=True, reason="ok")
        status_by_task = self._status_index(issues)
        launches_today = [self._launches_today(issues)]
        for issue in issues:
            self._handle_issue(issue, status_by_task, launches_today, result)
        return result

    def _hold_ready(self, issues, reason: str) -> None:
        for issue in issues:
            contract, error = parse_contract(issue.body)
            state = read_state(issue.body)
            current = (state or {}).get("status") or _status_label(issue.labels) or (contract.status if contract else "")
            if current == READY and not error:
                self._save(issue, self._state_from(contract, BUDGET_HOLD, reason=reason), BUDGET_HOLD)

    def _handle_issue(self, issue, status_by_task, launches_today: list[int], result: DispatchResult) -> None:
        state = read_state(issue.body)
        claim_ref = ""
        contract, error = parse_contract(issue.body)
        if contract:
            claim_ref = f"refs/autopilot/claims/{contract.task_id}"
        if state and state.get("status") in ACTIVE:
            self._reconcile(issue, state, result)
            return
        if error or contract is None:
            result.skipped.append(f"invalid_contract:{getattr(issue, 'number', '?')}:{error}")
            return
        current = (state or {}).get("status") or _status_label(issue.labels) or contract.status
        if _status_label(issue.labels) and _status_label(issue.labels) != contract.status and not state:
            result.skipped.append(f"status_mismatch:{contract.task_id}")
            return
        if current != READY:
            result.skipped.append(f"not_ready:{contract.task_id}:{current}")
            return
        try:
            events = self.github.issue_events(issue.number)
            comments = self.github.issue_comments(issue.number)
        except GithubError:
            result.skipped.append(f"approval_lookup_failed:{contract.task_id}")
            return
        approval_error = trusted_approval(
            contract.digest,
            issue.labels,
            events,
            comments,
            self.config.contract_approvers,
        )
        if approval_error:
            result.skipped.append(f"{approval_error}:{contract.task_id}")
            return
        scope_error = validate_scope(contract, self.config)
        if scope_error:
            self._save(issue, self._state_from(contract, NEEDS_CONTROLLER, reason=scope_error), NEEDS_CONTROLLER)
            result.skipped.append(f"{scope_error}:{contract.task_id}")
            return
        shared_error = shared_path_allowed(contract, self.config, issue.labels)
        if shared_error:
            result.skipped.append(f"{shared_error}:{contract.task_id}")
            return
        dependency_error = dependencies_closed(contract, status_by_task)
        if dependency_error:
            result.skipped.append(f"{dependency_error}:{contract.task_id}")
            return
        stale = base_is_current(contract, self.github.origin_main_sha())
        if stale:
            self._save(issue, self._state_from(contract, NEEDS_CONTROLLER, reason=stale), NEEDS_CONTROLLER)
            result.skipped.append(f"{stale}:{contract.task_id}")
            return
        if launches_today[0] >= self.config.max_launches_per_day:
            result.skipped.append(f"daily_launch_cap:{contract.task_id}")
            return
        active = self._active_count()
        if active >= self.config.max_parallel_agents:
            result.skipped.append(f"concurrency_cap:{contract.task_id}")
            return
        domains = critical_domains(contract, self.config)
        if self._domain_blocked(domains):
            result.skipped.append(f"domain_cap:{contract.task_id}")
            return
        if self.github.ref_exists(claim_ref):
            if state and state.get("agent_id"):
                self._reconcile(issue, state, result)
            else:
                result.skipped.append(f"claim_held:{contract.task_id}")
            return
        if self.github.create_ref(claim_ref, contract.base_sha) != "created":
            result.skipped.append(f"claim_held:{contract.task_id}")
            return
        branch = feature_branch(self.config, contract.task_id)
        assert_feature_branch(self.config, branch)
        branch_ref = self.github.create_ref(f"refs/heads/{branch}", contract.base_sha)
        if branch_ref == "exists" and self.github.get_ref_sha(f"refs/heads/{branch}") != contract.base_sha:
            self._save(issue, self._state_from(contract, NEEDS_CONTROLLER, reason="feature_ref_rejected"), NEEDS_CONTROLLER)
            result.skipped.append(f"feature_ref_rejected:{contract.task_id}")
            return
        agent_id = _new_agent_id()
        frozen = self._state_from(contract, CLAIMED, agent_id=agent_id, branch=branch, reason="")
        frozen["launch_dates"] = [self.now_iso[:10]]
        self._save(issue, frozen, CLAIMED)
        try:
            created = self.cursor.create_agent(implementation_body(self.config, contract, branch, agent_id))
        except CursorApiError as exc:
            if exc.status == 409 and "agent_id_conflict" in (exc.code or "agent_id_conflict"):
                created = self.cursor.get_agent(agent_id)
            else:
                frozen["last_error"] = f"cursor_http_{exc.status}"
                self._save(issue, frozen, CLAIMED)
                result.preserved.append(contract.task_id)
                return
        run = created.get("run") or {}
        frozen["run_id"] = run.get("id") or created.get("latestRunId") or ""
        frozen["status"] = IMPLEMENTING
        self._save(issue, frozen, IMPLEMENTING)
        result.launched.append(contract.task_id)
        launches_today[0] += 1

    def _reconcile(self, issue, state: dict, result: DispatchResult) -> None:
        agent_id = state.get("agent_id") or ""
        run_id = state.get("run_id") or ""
        task_id = state.get("task_id") or "?"
        if not agent_id:
            result.skipped.append(f"claim_held:{task_id}")
            return
        try:
            if not run_id:
                agent = self.cursor.get_agent(agent_id)
                run_id = agent.get("latestRunId") or ""
                state["run_id"] = run_id
            if not run_id:
                result.preserved.append(task_id)
                return
            run = self.cursor.get_run(agent_id, run_id)
        except CursorApiError as exc:
            if exc.status == 404 and state.get("status") == CLAIMED:
                self._retry_same_agent(issue, state, result)
                return
            state["last_error"] = f"cursor_http_{exc.status}"
            self._save(issue, state, state.get("status") or CLAIMED)
            result.preserved.append(task_id)
            return
        status = run.get("status") or ""
        if status in ACTIVE_RUN_STATUSES:
            result.skipped.append(f"agent_still_active:{task_id}")
            return
        if status == "FINISHED":
            run_id = run.get("id") or state.get("run_id") or ""
            task_status = state.get("status") or ""
            already = task_status in {PR_OPEN, CI_CHECK, INDEPENDENT_QA} or (
                bool(run_id) and state.get("handled_run_id") == run_id
            )
            if already:
                state["handled_run_id"] = run_id
                self._continue_after_finish(issue, state, result)
                return
            self._consume_finished_run(issue, state, run, result)
            return
        if status in FAILED_RUN_STATUSES:
            self._on_failed_run(issue, state, run, result)
            return
        if status == "CANCELLED":
            state["last_error"] = "cancelled"
            self._save(issue, state, NEEDS_CONTROLLER)
            result.skipped.append(f"cancelled:{task_id}")
            return
        state["last_error"] = f"unknown_run_status:{status}"
        self._save(issue, state, state.get("status") or CLAIMED)
        result.preserved.append(task_id)

    def _consume_finished_run(self, issue, state: dict, run: dict, result: DispatchResult) -> None:
        task_id = state["task_id"]
        run_id = run.get("id") or state.get("run_id") or ""
        git = run.get("git") or {}
        branches = git.get("branches") or []
        pr_url = ""
        branch_name = state.get("branch") or ""
        for item in branches:
            if item.get("branch"):
                branch_name = item["branch"]
            if item.get("prUrl"):
                pr_url = item["prUrl"]
        state["handled_run_id"] = run_id
        if branch_name in self.config.protected_branches:
            state["last_error"] = "protected_branch_push"
            self._save(issue, state, BLOCKED)
            result.skipped.append(f"protected_branch_push:{task_id}")
            return
        repo = repo_full_name(self.config.repo_url)
        parsed = parse_pull_url(pr_url)
        if parsed is None or parsed[0] != repo:
            state["pr_url"] = pr_url
            if not pr_url:
                state["last_error"] = "missing_pr"
            elif parsed is None:
                state["last_error"] = "pr_url_invalid"
            else:
                state["last_error"] = "pr_repo_mismatch"
            self._save(issue, state, NEEDS_CONTROLLER)
            result.skipped.append(f"{state['last_error']}:{task_id}")
            return
        state["pr_url"] = pr_url
        state["pr_number"] = parsed[1]
        state["branch"] = branch_name or state.get("branch") or ""
        try:
            self.github.ensure_draft(parsed[1])
        except GithubError:
            state["handled_run_id"] = ""
            state["last_error"] = "github_read_failed"
            self._save(issue, state, state.get("status") or IMPLEMENTING)
            result.preserved.append(task_id)
            return
        self._continue_after_finish(issue, state, result)

    def _continue_after_finish(self, issue, state: dict, result: DispatchResult) -> None:
        status = state.get("status") or ""
        if status in {CLAIMED, IMPLEMENTING}:
            if not state.get("pr_number"):
                state["last_error"] = "missing_pr"
                self._save(issue, state, NEEDS_CONTROLLER)
                return
            self._save(issue, state, PR_OPEN)
            if state.get("status") != PR_OPEN:
                return
            self._save(issue, state, CI_CHECK)
            if state.get("status") != CI_CHECK:
                return
        elif status == PR_OPEN:
            self._save(issue, state, CI_CHECK)
            if state.get("status") != CI_CHECK:
                return
        status = state.get("status") or ""
        if status == CI_CHECK:
            self._gate_ci_and_maybe_launch_qa(issue, state, result)
            return
        if status == INDEPENDENT_QA:
            self._poll_qa_agent(issue, state, result)
            return
        result.skipped.append(f"finish_already_handled:{state.get('task_id')}")

    def _gate_ci_and_maybe_launch_qa(self, issue, state: dict, result: DispatchResult) -> None:
        task_id = state["task_id"]
        pr_number = int(state.get("pr_number") or 0)
        try:
            pr = self.github.pull(pr_number)
            files = self.github.pull_files(pr_number)
            head_sha = str(pr.get("head_sha") or "")
            checks = self.github.check_runs(head_sha) if len(head_sha) == 40 else []
        except GithubError:
            state["last_error"] = "github_read_failed"
            self._save(issue, state, CI_CHECK)
            result.preserved.append(task_id)
            return
        identity = pr_identity_error(
            pr,
            repo=repo_full_name(self.config.repo_url),
            branch=state.get("branch") or "",
            claimed_url=state.get("pr_url") or "",
        )
        if identity:
            state["last_error"] = identity
            self._save(issue, state, NEEDS_CONTROLLER)
            result.skipped.append(f"{identity}:{task_id}")
            return
        allowed = merge_allowed(
            pr.get("base_sha", ""),
            self.github.origin_main_sha(),
            pr.get("head_branch", ""),
            self.config.protected_branches,
        )
        state["merge_allowed"] = allowed
        state["head_sha"] = str(pr.get("head_sha") or "")
        contract = _contract_from_state(issue.body, state)
        qa = review_diff(files, contract, self.config, issue.labels)
        if not qa.ok:
            self._request_fix_or_block(issue, state, qa.failures, result)
            return
        if not allowed:
            state["last_error"] = "stale_pr"
            self._save(issue, state, NEEDS_CONTROLLER)
            result.skipped.append(f"stale_pr:{task_id}")
            return
        head_sha = state["head_sha"]
        ok, reason = evaluate_ci(checks, required_checks([item.path for item in files]), head_sha)
        if not ok:
            state["last_error"] = reason
            self._save(issue, state, CI_CHECK)
            result.skipped.append(f"ci_not_ready:{task_id}:{reason}")
            return
        if state.get("qa_agent_id") and state.get("qa_head_sha") == head_sha:
            self._save(issue, state, INDEPENDENT_QA)
            self._poll_qa_agent(issue, state, result)
            return
        if state.get("qa_agent_id"):
            self._clear_qa(state)
        diff_text, diff_error = format_review_diff(files)
        if diff_error:
            state["last_error"] = diff_error
            self._save(issue, state, NEEDS_CONTROLLER)
            result.skipped.append(f"{diff_error}:{task_id}")
            return
        self._launch_qa(issue, state, contract, diff_text, head_sha, result)

    def _ready_merge_gate(self, state: dict) -> tuple[bool, str]:
        pr_number = int(state.get("pr_number") or 0)
        try:
            pr = self.github.pull(pr_number)
            files = self.github.pull_files(pr_number)
            head_sha = str(pr.get("head_sha") or "")
            checks = self.github.check_runs(head_sha) if len(head_sha) == 40 else []
        except GithubError:
            return False, "github_read_failed"
        identity = pr_identity_error(
            pr,
            repo=repo_full_name(self.config.repo_url),
            branch=state.get("branch") or "",
            claimed_url=state.get("pr_url") or "",
        )
        if identity:
            return False, identity
        if head_sha != (state.get("qa_head_sha") or ""):
            return False, "head_sha_changed"
        allowed = merge_allowed(
            pr.get("base_sha", ""),
            self.github.origin_main_sha(),
            pr.get("head_branch", ""),
            self.config.protected_branches,
        )
        state["merge_allowed"] = allowed
        if not allowed:
            return False, "stale_pr"
        return evaluate_ci(checks, required_checks([item.path for item in files]), head_sha)

    def _request_fix_or_block(self, issue, state: dict, failures: list[str], result: DispatchResult) -> None:
        task_id = state["task_id"]
        cycles = int(state.get("qa_cycles") or 0)
        findings = "\n".join(failures)
        self.github.comment(issue.number, "QA findings for the implementation agent:\n" + findings)
        if cycles >= self.config.max_qa_fix_cycles:
            state["last_error"] = "qa_failed"
            self._save(issue, state, BLOCKED)
            result.skipped.append(f"qa_blocked:{task_id}")
            return
        try:
            follow = self.cursor.follow_up(state["agent_id"], fix_prompt(findings))
        except CursorApiError as exc:
            state["last_error"] = f"cursor_http_{exc.status}"
            self._save(issue, state, state.get("status") or CI_CHECK)
            result.preserved.append(task_id)
            return
        state["qa_cycles"] = cycles + 1
        new_run = (follow.get("run") or {}).get("id") or state.get("run_id")
        if new_run != state.get("handled_run_id"):
            state["handled_run_id"] = ""
        state["run_id"] = new_run
        self._clear_qa(state)
        state["status"] = IMPLEMENTING
        self._save(issue, state, IMPLEMENTING)
        result.skipped.append(f"qa_fix_cycle:{task_id}")

    def _clear_qa(self, state: dict) -> None:
        state["qa_agent_id"] = ""
        state["qa_run_id"] = ""
        state["qa_head_sha"] = ""
        state["qa_verdict"] = ""
        state["qa_reported"] = False

    def _launch_qa(self, issue, state: dict, contract, diff_text: str, head_sha: str, result: DispatchResult) -> None:
        qa_agent_id = _new_agent_id()
        try:
            created = self.cursor.create_agent(qa_body(contract, qa_agent_id, diff_text))
        except CursorApiError as exc:
            state["last_error"] = f"cursor_http_{exc.status}"
            self._save(issue, state, CI_CHECK)
            result.preserved.append(state["task_id"])
            return
        state["qa_agent_id"] = created.get("agent", {}).get("id") or qa_agent_id
        state["qa_run_id"] = (created.get("run") or {}).get("id") or ""
        state["qa_head_sha"] = head_sha
        state["qa_verdict"] = ""
        state["qa_reported"] = False
        state["status"] = INDEPENDENT_QA
        self._save(issue, state, INDEPENDENT_QA)
        result.launched.append(f"qa:{state['task_id']}")

    def _poll_qa_agent(self, issue, state: dict, result: DispatchResult) -> None:
        task_id = state["task_id"]
        try:
            run = self.cursor.get_run(state["qa_agent_id"], state.get("qa_run_id") or "")
        except CursorApiError as exc:
            state["last_error"] = f"cursor_http_{exc.status}"
            self._save(issue, state, INDEPENDENT_QA)
            result.preserved.append(task_id)
            return
        status = run.get("status") or ""
        if status in ACTIVE_RUN_STATUSES:
            result.skipped.append(f"qa_still_active:{task_id}")
            return
        if status != "FINISHED":
            state["last_error"] = f"qa_run_{status}"
            self._save(issue, state, NEEDS_CONTROLLER)
            return
        findings = run.get("result") or ""
        verdict = parse_verdict(findings)
        state["qa_verdict"] = verdict
        if verdict == "CHANGES_REQUIRED":
            self._request_fix_or_block(issue, state, [findings], result)
            return
        if verdict not in {"PASS", "PASS_WITH_NITS"}:
            state["last_error"] = "qa_verdict_rejected" if verdict == "BLOCKED" else "qa_verdict_missing"
            self._save(issue, state, NEEDS_CONTROLLER)
            result.skipped.append(f"{state['last_error']}:{task_id}")
            return
        ready, reason = self._ready_merge_gate(state)
        if not ready:
            state["last_error"] = reason
            self._save(issue, state, INDEPENDENT_QA)
            result.skipped.append(f"ci_blocked_ready:{task_id}:{reason}")
            return
        if not state.get("qa_reported"):
            self.github.comment(issue.number, "Agent I findings (read-only, not applied by Agent I):\n" + findings)
            state["qa_reported"] = True
        self._save(issue, state, READY_FOR_MERGE)
        result.skipped.append(f"qa_complete:{task_id}")

    def _on_failed_run(self, issue, state: dict, run: dict, result: DispatchResult) -> None:
        task_id = state["task_id"]
        run_id = run.get("id") or state.get("run_id")
        counted = list(state.get("counted_failure_run_ids") or [])
        if run_id in counted:
            result.skipped.append(f"failure_already_counted:{task_id}")
            return
        counted.append(run_id)
        state["counted_failure_run_ids"] = counted
        limit = min(int(state.get("max_retries") or 0), self.config.max_retries_cap)
        if len(counted) > limit:
            state["last_error"] = "retries_exhausted"
            self._save(issue, state, BLOCKED)
            result.skipped.append(f"blocked:{task_id}")
            return
        try:
            follow = self.cursor.follow_up(
                state["agent_id"],
                "The previous cloud agent run failed. Retry the same Task Contract. Do not widen allowed paths.",
            )
        except CursorApiError as exc:
            state["last_error"] = f"cursor_http_{exc.status}"
            self._save(issue, state, state.get("status") or IMPLEMENTING)
            result.preserved.append(task_id)
            return
        new_run = (follow.get("run") or {}).get("id") or state.get("run_id")
        if new_run != state.get("handled_run_id"):
            state["handled_run_id"] = ""
        state["run_id"] = new_run
        self._clear_qa(state)
        state["status"] = IMPLEMENTING
        self._save(issue, state, IMPLEMENTING)
        result.skipped.append(f"retry:{task_id}")

    def _retry_same_agent(self, issue, state: dict, result: DispatchResult) -> None:
        """A create that never produced an agent is retried with the same agent id."""
        task_id = state.get("task_id") or "?"
        branch = state.get("branch") or ""
        try:
            contract = _contract_from_state(issue.body, state)
            assert_feature_branch(self.config, branch)
            created = self.cursor.create_agent(implementation_body(self.config, contract, branch, state["agent_id"]))
        except (CursorApiError, ValueError) as exc:
            state["last_error"] = f"cursor_retry_held:{getattr(exc, 'status', 'invalid')}"
            self._save(issue, state, CLAIMED)
            result.preserved.append(task_id)
            return
        state["run_id"] = (created.get("run") or {}).get("id") or created.get("latestRunId") or ""
        self._save(issue, state, IMPLEMENTING)
        result.launched.append(task_id)

    def _save(self, issue, state: dict, status: str) -> None:
        previous = (read_state(issue.body) or {}).get("status") or _status_label(issue.labels) or status
        if not can_transition(previous, status):
            attempted = status
            status = NEEDS_CONTROLLER
            state["last_error"] = f"illegal_transition:{previous}->{attempted}"
        state["status"] = status
        state["updated_at"] = self.now_iso
        issue.body = write_state(issue.body, state)
        issue.labels = _with_status(issue.labels, status)
        self.github.update_issue(issue)

    def _state_from(self, contract, status: str, agent_id: str = "", branch: str = "", reason: str = "") -> dict:
        return {
            "agent_id": agent_id,
            "allowed_paths": list(contract.allowed_paths),
            "base_sha": contract.base_sha,
            "branch": branch,
            "contract_hash": contract.digest,
            "counted_failure_run_ids": [],
            "forbidden_paths": list(contract.forbidden_paths),
            "last_error": reason,
            "launch_dates": [],
            "max_retries": contract.max_retries,
            "merge_allowed": False,
            "pr_number": 0,
            "pr_url": "",
            "qa_cycles": 0,
            "required_tests": list(contract.required_tests),
            "risk_level": contract.risk_level,
            "run_id": "",
            "status": status,
            "task_id": contract.task_id,
        }

    def _status_index(self, issues) -> dict[str, str]:
        index = {}
        for issue in issues:
            state = read_state(issue.body)
            contract, error = parse_contract(issue.body)
            if error or contract is None:
                continue
            index[contract.task_id] = (state or {}).get("status") or contract.status
        return index

    def _launches_today(self, issues) -> int:
        day = _day(self.now_iso)
        count = 0
        for issue in issues:
            state = read_state(issue.body) or {}
            count += sum(1 for item in state.get("launch_dates") or [] if item == day)
        return count

    def _active_count(self) -> int:
        count = 0
        for issue in self.github.list_issues():
            state = read_state(issue.body) or {}
            if state.get("status") in ACTIVE:
                count += 1
        return count

    def _domain_blocked(self, domains: set[str]) -> bool:
        if not domains:
            return False
        counts = {domain: 0 for domain in domains}
        for issue in self.github.list_issues():
            state = read_state(issue.body) or {}
            if state.get("status") not in ACTIVE:
                continue
            seen = set()
            for path in state.get("allowed_paths") or []:
                name = self.config.domain_for_path(path)
                if name in counts:
                    seen.add(name)
            for name in seen:
                counts[name] += 1
        for domain in domains:
            limit = next(item.max_parallel for item in self.config.critical_domains if item.name == domain)
            if counts[domain] >= limit:
                return True
        return False


def _contract_from_state(body: str, state: dict):
    parsed, _error = parse_contract(body)
    if parsed is None:
        raise ValueError("contract missing during reconcile")
    # Frozen allow-list from claim time wins over a later issue edit.
    return parsed.__class__(
        task_id=parsed.task_id,
        module=parsed.module,
        owner_agent=parsed.owner_agent,
        priority=parsed.priority,
        risk_level=state.get("risk_level") or parsed.risk_level,
        base_sha=state.get("base_sha") or parsed.base_sha,
        allowed_paths=tuple(state.get("allowed_paths") or parsed.allowed_paths),
        forbidden_paths=tuple(state.get("forbidden_paths") or parsed.forbidden_paths),
        dependencies=parsed.dependencies,
        acceptance_criteria=parsed.acceptance_criteria,
        required_tests=tuple(state.get("required_tests") or parsed.required_tests),
        max_retries=int(state.get("max_retries") if state.get("max_retries") is not None else parsed.max_retries),
        status=state.get("status") or parsed.status,
        objective=parsed.objective,
    )
