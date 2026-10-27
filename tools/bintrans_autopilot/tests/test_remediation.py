"""Regression tests for controller findings AUTO-311-001 through AUTO-311-006.

Cursor calls stay on the fake client. No paid launch and no network.
"""

from __future__ import annotations

import io
import json
import os
import re
import unittest
from contextlib import redirect_stdout
from datetime import datetime, timezone

from bintrans_autopilot.budget import BudgetWindows, assess_budget, budget_windows
from bintrans_autopilot.ci_gate import evaluate_ci, required_checks
from bintrans_autopilot.config import load_config
from bintrans_autopilot.pr_identity import pr_identity_error
from bintrans_autopilot.qa_gate import DiffFile, format_review_diff
from bintrans_autopilot.verdict import parse_verdict
from bintrans_autopilot.workflow_guard import TRUSTED_REPOSITORY, evaluate_workflow_context
from test_dispatcher import (
    MAIN,
    NOW,
    ROOT,
    FakeCursor,
    FakeGithub,
    enabled_config,
    ready_issue,
    run_dispatch,
    state_of,
)

HEAD = "b" * 40
OTHER = "c" * 40
REPO = "f9999737292-bit/Freihgt-platform"
PR_URL = f"https://github.com/{REPO}/pull/311"
AGENT = "bc-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
PATCH = "@@ -1 +1,2 @@\n+func Foo() {}\n"
PATH = "services/shipment-service/internal/foo.go"


def green(sha: str) -> list[dict]:
    return [
        {
            "name": name,
            "head_sha": sha,
            "status": "completed",
            "conclusion": "success",
            "started_at": "2026-10-27T00:00:00Z",
        }
        for name in required_checks([PATH])
    ]


def finished_issue(cursor, github, *, patch=PATCH, checks=None, pr_url=PR_URL, head_repo=REPO, number=311, head_sha=HEAD):
    state = {
        "agent_id": AGENT,
        "allowed_paths": [PATH],
        "base_sha": MAIN,
        "branch": "autopilot/bt-100",
        "counted_failure_run_ids": [],
        "forbidden_paths": ["services/identity-service/**"],
        "last_error": "",
        "launch_dates": [],
        "max_retries": 1,
        "required_tests": ["go test ./..."],
        "risk_level": "LOW",
        "run_id": "run-1",
        "status": "IMPLEMENTING",
        "task_id": "BT-100",
    }
    issue = ready_issue()
    issue.body += "\n<!-- bintrans-autopilot-state\n" + json.dumps(state) + "\n-->\n"
    issue.labels = ["autopilot", "autopilot-contract-approved", "autopilot:IMPLEMENTING"]
    cursor.runs[(AGENT, "run-1")] = {
        "id": "run-1",
        "status": "FINISHED",
        "git": {"branches": [{"branch": "autopilot/bt-100", "prUrl": pr_url}]},
    }
    github.files[number] = [DiffFile(PATH, patch)]
    github.pulls[number] = {
        "number": number,
        "repo": REPO,
        "head_repo": head_repo,
        "head_branch": "autopilot/bt-100",
        "head_sha": head_sha,
        "base_sha": MAIN,
        "draft": True,
        "node_id": "PR_NODE",
        "html_url": pr_url,
    }
    github.checks[head_sha] = list(checks if checks is not None else [])
    github.issues = [issue]
    return issue


def trusted_context(**overrides) -> dict[str, str]:
    context = {
        "event_name": "schedule",
        "ref": "refs/heads/main",
        "repository": TRUSTED_REPOSITORY,
        "workflow_ref": f"{TRUSTED_REPOSITORY}/.github/workflows/bintrans-autopilot-dispatcher.yml@refs/heads/main",
        "base_ref": "",
        "head_ref": "",
    }
    context.update(overrides)
    return context


class RemediationTests(unittest.TestCase):
    def test_pull_request_workflow_cannot_reach_cursor_secrets(self):
        workflow = (ROOT / ".github/workflows/bintrans-autopilot-dispatcher.yml").read_text(encoding="utf-8")
        self.assertIsNone(re.search(r"(?m)^  pull_request:", workflow))
        self.assertIsNone(re.search(r"(?m)^  pull_request_target:", workflow))
        self.assertEqual(workflow.count("ref: main"), 2)
        self.assertGreaterEqual(workflow.count("workflow_guard_main"), 2)
        first_guard = workflow.find("workflow_guard_main")
        first_secret = workflow.find("secrets.CURSOR_API_KEY")
        second_guard = workflow.find("workflow_guard_main", first_guard + 1)
        second_secret = workflow.find("secrets.CURSOR_API_KEY", first_secret + 1)
        self.assertLess(first_guard, first_secret)
        self.assertLess(second_guard, second_secret)
        guard_source = (ROOT / "tools/bintrans_autopilot/workflow_guard.py").read_text(encoding="utf-8")
        entry_source = (ROOT / "tools/bintrans_autopilot/workflow_guard_main.py").read_text(encoding="utf-8")
        for source in (guard_source, entry_source):
            self.assertNotIn("CURSOR_API_KEY", source)
            self.assertNotIn("GITHUB_TOKEN", source)
        ok, reason = evaluate_workflow_context(trusted_context(event_name="pull_request_target", base_ref="main", head_ref="feat/x"))
        self.assertFalse(ok)
        self.assertEqual(reason, "untrusted_event")
        ok, reason = evaluate_workflow_context(
            trusted_context(workflow_ref=f"{TRUSTED_REPOSITORY}/.github/workflows/bintrans-autopilot-dispatcher.yml@refs/heads/feat/evil")
        )
        self.assertFalse(ok)
        self.assertEqual(reason, "untrusted_workflow_file")
        ok, reason = evaluate_workflow_context(trusted_context())
        self.assertTrue(ok)
        self.assertEqual(reason, "trusted_main")

    def test_actions_pull_request_returns_before_autopilot_startup(self):
        from bintrans_autopilot.__main__ import main

        keys = [
            "GITHUB_ACTIONS",
            "GITHUB_EVENT_NAME",
            "GITHUB_REF",
            "GITHUB_REPOSITORY",
            "GITHUB_WORKFLOW_REF",
            "GITHUB_BASE_REF",
            "GITHUB_HEAD_REF",
            "AUTOPILOT_ENABLED",
        ]
        saved = {key: os.environ.get(key) for key in keys}
        try:
            os.environ["GITHUB_ACTIONS"] = "true"
            os.environ["GITHUB_EVENT_NAME"] = "pull_request"
            os.environ["GITHUB_REF"] = "refs/pull/311/merge"
            os.environ["GITHUB_REPOSITORY"] = REPO
            os.environ["GITHUB_WORKFLOW_REF"] = f"{REPO}/.github/workflows/bintrans-autopilot-dispatcher.yml@refs/pull/311/merge"
            os.environ["GITHUB_BASE_REF"] = "main"
            os.environ["GITHUB_HEAD_REF"] = "feat/bintrans-autopilot-24x7-0.1a"
            os.environ["AUTOPILOT_ENABLED"] = ""
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertEqual(main(["dispatch"]), 2)
            self.assertIn("UNTRUSTED_WORKFLOW=untrusted_event", output.getvalue())
            self.assertIn("PAID_AGENTS_LAUNCHED=NO", output.getvalue())
        finally:
            for key, value in saved.items():
                if value is None:
                    os.environ.pop(key, None)
                else:
                    os.environ[key] = value

    def test_finished_run_is_not_reprocessed_and_qa_waits_for_exact_head_ci(self):
        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=[])
        first = run_dispatch(github, cursor)
        self.assertIn("ci_not_ready:BT-100", " ".join(first.skipped))
        self.assertEqual(state_of(github.issues[0])["status"], "CI_CHECK")
        self.assertNotIn("illegal_transition", state_of(github.issues[0])["last_error"])
        self.assertEqual(cursor.creates, [])
        second = run_dispatch(github, cursor)
        saved = state_of(github.issues[0])
        self.assertEqual(saved["status"], "CI_CHECK")
        self.assertNotIn("illegal_transition", saved["last_error"])
        self.assertEqual(cursor.creates, [])
        self.assertEqual(github.drafts, [311])
        github.checks[HEAD] = green(HEAD)
        third = run_dispatch(github, cursor)
        self.assertEqual(len(cursor.creates), 1)
        prompt = cursor.creates[0]["prompt"]["text"]
        self.assertIn("+func Foo() {}", prompt)
        self.assertIn("BINTRANS_QA_VERDICT:", prompt)
        self.assertNotIn("repos", cursor.creates[0])
        self.assertEqual(state_of(github.issues[0])["status"], "INDEPENDENT_QA")
        self.assertEqual(third.launched, ["qa:BT-100"])
        fourth = run_dispatch(github, cursor)
        self.assertEqual(len(cursor.creates), 1)
        self.assertIn("qa_still_active:BT-100", fourth.skipped)
        self.assertEqual(state_of(github.issues[0])["status"], "INDEPENDENT_QA")
        self.assertNotIn("illegal_transition", state_of(github.issues[0]).get("last_error") or "")
        qa_state = state_of(github.issues[0])
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["status"] = "FINISHED"
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["result"] = (
            "The word BLOCKING appears in a quoted line.\nBINTRANS_QA_VERDICT: PASS\n"
        )
        github.checks[HEAD][0]["conclusion"] = "skipped"
        fifth = run_dispatch(github, cursor)
        self.assertEqual(state_of(github.issues[0])["status"], "INDEPENDENT_QA")
        self.assertTrue(any(item.startswith("ci_blocked_ready:BT-100") for item in fifth.skipped))
        self.assertNotEqual(state_of(github.issues[0])["status"], "READY_FOR_MERGE")
        github.checks[HEAD][0]["conclusion"] = "success"
        sixth = run_dispatch(github, cursor)
        self.assertEqual(state_of(github.issues[0])["status"], "READY_FOR_MERGE")
        self.assertEqual(len(cursor.creates), 1)
        self.assertIn("qa_complete:BT-100", sixth.skipped)

    def test_newer_failed_check_blocks_an_older_success(self):
        checks = [
            {"name": "repository-safety", "head_sha": HEAD, "status": "completed", "conclusion": "success", "started_at": "2026-10-27T00:00:00Z"},
            {"name": "repository-safety", "head_sha": HEAD, "status": "completed", "conclusion": "failure", "started_at": "2026-10-27T01:00:00Z"},
            {"name": "bintrans-autopilot-unit", "head_sha": HEAD, "status": "completed", "conclusion": "success", "started_at": "2026-10-27T01:00:00Z"},
        ]
        ok, reason = evaluate_ci(checks, ("repository-safety", "bintrans-autopilot-unit"), HEAD)
        self.assertFalse(ok)
        self.assertEqual(reason, "check_not_success:repository-safety")
        mismatch = [dict(item, head_sha=OTHER) for item in green(HEAD)]
        ok, reason = evaluate_ci(mismatch, required_checks([PATH]), HEAD)
        self.assertFalse(ok)
        self.assertEqual(reason, "check_sha_mismatch")

    def test_agent_i_does_not_launch_on_a_filename_only_diff(self):
        text, error = format_review_diff([DiffFile(PATH, "")])
        self.assertEqual(text, "")
        self.assertEqual(error, f"diff_patch_missing:{PATH}")
        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, patch="", checks=green(HEAD))
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")
        self.assertIn("diff_patch_missing", result.skipped[0])

    def test_unstructured_and_blocking_verdicts_do_not_become_ready(self):
        self.assertEqual(parse_verdict("BINTRANS_QA_VERDICT: PASS\nBINTRANS_QA_VERDICT: BLOCKED\n"), "")
        self.assertEqual(parse_verdict("BINTRANS_QA_VERDICT: PASS_WITH_NITS\n"), "PASS_WITH_NITS")
        self.assertEqual(parse_verdict("BINTRANS_QA_VERDICT: BLOCKED\n"), "BLOCKED")
        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD))
        run_dispatch(github, cursor)
        qa_state = state_of(github.issues[0])
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["status"] = "FINISHED"
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["result"] = "No structured line.\n"
        result = run_dispatch(github, cursor)
        self.assertIn("qa_verdict_missing:BT-100", result.skipped)
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")
        self.assertNotEqual(state_of(github.issues[0])["status"], "READY_FOR_MERGE")

    def test_head_change_after_qa_blocks_ready_for_merge(self):
        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD))
        run_dispatch(github, cursor)
        qa_state = state_of(github.issues[0])
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["status"] = "FINISHED"
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["result"] = "BINTRANS_QA_VERDICT: PASS_WITH_NITS\n"
        moved = "d" * 40
        github.pulls[311]["head_sha"] = moved
        github.checks[moved] = green(moved)
        result = run_dispatch(github, cursor)
        self.assertEqual(state_of(github.issues[0])["status"], "INDEPENDENT_QA")
        self.assertTrue(any("head_sha_changed" in item for item in result.skipped))

    def test_changes_required_does_not_mark_ready(self):
        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD))
        run_dispatch(github, cursor)
        qa_state = state_of(github.issues[0])
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["status"] = "FINISHED"
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["result"] = "BINTRANS_QA_VERDICT: CHANGES_REQUIRED\n"
        result = run_dispatch(github, cursor)
        self.assertIn("qa_fix_cycle:BT-100", result.skipped)
        self.assertEqual(state_of(github.issues[0])["status"], "IMPLEMENTING")
        self.assertNotEqual(state_of(github.issues[0])["status"], "READY_FOR_MERGE")

    def test_daily_reserve_and_monthly_window_fail_closed(self):
        now = datetime.fromisoformat(NOW.replace("Z", "+00:00"))
        windows = budget_windows(now)
        month_start = datetime(2026, 10, 1, tzinfo=timezone.utc)
        self.assertEqual(windows.monthly_start_ms, int(month_start.timestamp() * 1000))
        self.assertLess(windows.monthly_start_ms, windows.daily_start_ms)
        cursor = FakeCursor()
        decision = assess_budget(load_config(), cursor, windows, now, 0)
        self.assertEqual(decision.reason, "budget_not_authorized")
        self.assertEqual(cursor.spend_calls, [])
        bad = BudgetWindows(1, 2, 3, 4)
        self.assertEqual(assess_budget(enabled_config(), FakeCursor(), bad, now, 0).reason, "budget_window_mismatch")
        held = FakeCursor()
        result = run_dispatch(
            FakeGithub([ready_issue()]),
            held,
            enabled_config(
                budget_max_charged_cents=0,
                budget_max_charged_cents_daily=30,
                budget_max_charged_cents_monthly=10_000_000,
                budget_launch_reserve_cents=40,
                budget_reserve_per_active_agent_cents=1,
            ),
        )
        self.assertEqual(result.reason, "daily_budget_exceeded")
        self.assertEqual(held.creates, [])
        self.assertGreaterEqual(len(held.spend_calls), 2)
        monthly = FakeCursor()
        monthly.charged_by_window = {
            (windows.daily_start_ms, windows.daily_end_ms): 0,
            (windows.monthly_start_ms, windows.monthly_end_ms): 80,
        }
        result = run_dispatch(
            FakeGithub([ready_issue()]),
            monthly,
            enabled_config(
                budget_max_charged_cents=0,
                budget_max_charged_cents_daily=10_000,
                budget_max_charged_cents_monthly=50,
                budget_launch_reserve_cents=10,
                budget_reserve_per_active_agent_cents=1,
            ),
        )
        self.assertEqual(result.reason, "monthly_budget_exceeded")
        self.assertEqual(monthly.creates, [])
        inconsistent = FakeCursor()
        inconsistent.charged_by_window = {
            (windows.daily_start_ms, windows.daily_end_ms): 100,
            (windows.monthly_start_ms, windows.monthly_end_ms): 10,
        }
        issue = ready_issue()
        original = issue.body
        result = run_dispatch(FakeGithub([issue]), inconsistent)
        self.assertEqual(result.reason, "spend_inconsistent")
        self.assertEqual(issue.body, original)
        ultra = FakeCursor()
        ultra.has_admin_key = False
        issue = ready_issue()
        original = issue.body
        result = run_dispatch(FakeGithub([issue]), ultra, enabled_config(cursor_plan="ultra"))
        self.assertEqual(result.reason, "personal_ultra_no_team_admin_api")
        self.assertEqual(issue.body, original)
        self.assertEqual(ultra.creates, [])
        self.assertEqual(ultra.spend_calls, [])
        bare = FakeCursor()
        issue = ready_issue()
        original = issue.body
        result = run_dispatch(FakeGithub([issue]), bare, enabled_config(budget_launch_reserve_cents=0))
        self.assertEqual(result.reason, "budget_reserve_not_configured")
        self.assertEqual(issue.body, original)
        active = FakeCursor()
        decision = assess_budget(
            enabled_config(
                budget_max_charged_cents=0,
                budget_max_charged_cents_daily=40,
                budget_max_charged_cents_monthly=10_000_000,
                budget_launch_reserve_cents=10,
                budget_reserve_per_active_agent_cents=20,
            ),
            active,
            windows,
            now,
            2,
        )
        self.assertEqual(decision.reason, "daily_budget_exceeded")
        self.assertEqual(decision.charged_cents, 50)

    def test_contract_approval_requires_allowlisted_actor_and_digest(self):
        cursor = FakeCursor()
        github = FakeGithub([ready_issue()])
        github.approval_actor = "intruder"
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertIn("approval_actor_unauthorized:BT-100", result.skipped)
        cursor = FakeCursor()
        github = FakeGithub([ready_issue()])
        github.approval_digest = "ab" * 32
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertIn("approval_digest_mismatch:BT-100", result.skipped)
        cursor = FakeCursor()
        github = FakeGithub([ready_issue()])
        github.comment_user = "someone-else"
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertIn("approval_digest_mismatch:BT-100", result.skipped)
        cursor = FakeCursor()
        issue = ready_issue(labels=["autopilot", "autopilot:READY"])
        result = run_dispatch(FakeGithub([issue]), cursor)
        self.assertEqual(cursor.creates, [])
        self.assertIn("contract_not_approved:BT-100", result.skipped)

    def test_pr_identity_rejects_fork_and_swapped_url(self):
        self.assertEqual(
            pr_identity_error(
                {"repo": REPO, "head_repo": "evil/fork", "head_branch": "autopilot/bt-100", "head_sha": HEAD, "number": 311},
                repo=REPO,
                branch="autopilot/bt-100",
                claimed_url=PR_URL,
            ),
            "pr_repo_mismatch",
        )
        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD), head_repo="evil/fork")
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")
        self.assertIn("pr_repo_mismatch:BT-100", result.skipped)
        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD), pr_url="https://github.com/evil/other/pull/311")
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertIn("pr_repo_mismatch:BT-100", result.skipped)


if __name__ == "__main__":
    unittest.main()
