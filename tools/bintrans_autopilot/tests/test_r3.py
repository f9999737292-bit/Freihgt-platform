"""Regression tests for Agent I findings H1-H3 and M1-M4. Mock Cursor only."""

from __future__ import annotations

import json
import unittest

from bintrans_autopilot.ci_gate import evaluate_ci, merge_checks, required_checks
from bintrans_autopilot.contract import parse_contract
from bintrans_autopilot.cursor_api import CursorApiError
from bintrans_autopilot.github_api import GithubError, GithubRest
from bintrans_autopilot.qa_gate import DiffFile
from bintrans_autopilot.states import can_transition
from test_dispatcher import MAIN, body, enabled_config, ready_issue, run_dispatch, state_of
from test_remediation import (
    AGENT,
    HEAD,
    PATH,
    REPO,
    FakeCursor,
    FakeGithub,
    finished_issue,
    green,
)

PATCH = "@@ -1 +1,2 @@\n+func Foo() {}\n"


class PagedGithub(GithubRest):
    def __init__(self, pages, error: GithubError | None = None):
        super().__init__("token", "octo/example")
        self.pages = pages
        self.error = error
        self.urls: list[str] = []

    def _request(self, method, url, payload=None):
        self.urls.append(url)
        if self.error:
            raise self.error
        page = int(url.rsplit("page=", 1)[-1])
        if page > len(self.pages):
            return 200, []
        return 200, self.pages[page - 1]


def file_page(start: int, count: int, *, patch: str | None = "@@\n+kept\n") -> list[dict]:
    rows = []
    for offset in range(count):
        item = {"filename": f"services/shipment-service/file-{start + offset}.go"}
        if patch is not None:
            item["patch"] = patch
        rows.append(item)
    return rows


def failed_run_issue():
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
        "run_id": "run-1",
        "status": "IMPLEMENTING",
        "task_id": "BT-100",
    }
    issue = ready_issue()
    issue.body += "\n<!-- bintrans-autopilot-state\n" + json.dumps(state) + "\n-->\n"
    issue.labels = ["autopilot", "autopilot-contract-approved", "autopilot:IMPLEMENTING"]
    return issue


def claimed_without_run(number: int = 1, task_id: str = "BT-100", agent: str = AGENT) -> object:
    issue = ready_issue(number=number, TASK_ID=task_id)
    issue.body += "\n<!-- bintrans-autopilot-state\n" + json.dumps({
        "agent_id": agent,
        "allowed_paths": [PATH],
        "base_sha": MAIN,
        "branch": f"autopilot/{task_id.lower()}",
        "counted_failure_run_ids": [],
        "last_error": "",
        "launch_dates": ["2026-10-27"],
        "max_retries": 1,
        "run_id": "",
        "status": "CLAIMED",
        "task_id": task_id,
    }) + "\n-->\n"
    issue.labels = ["autopilot", "autopilot-contract-approved", "autopilot:CLAIMED"]
    return issue


class R3Tests(unittest.TestCase):
    def test_pull_files_reads_past_the_first_page_and_fails_closed(self):
        pages = [file_page(0, 100), file_page(100, 1)]
        client = PagedGithub(pages)
        files = client.pull_files(311)
        self.assertEqual(len(files), 101)
        self.assertEqual(files[-1].path, "services/shipment-service/file-100.go")
        self.assertEqual(len(client.urls), 2)

        truncated = PagedGithub([file_page(index * 100, 100) for index in range(5)])
        with self.assertRaises(GithubError) as raised:
            truncated.pull_files(311)
        self.assertIn("pull files truncated", str(raised.exception))

        broken = PagedGithub([], error=GithubError(503, "unavailable"))
        with self.assertRaises(GithubError) as raised:
            broken.pull_files(311)
        self.assertIn("pull files api error", str(raised.exception))

        duplicate = PagedGithub([file_page(0, 100), file_page(0, 1)])
        with self.assertRaises(GithubError) as raised:
            duplicate.pull_files(311)
        self.assertIn("pull files incomplete", str(raised.exception))

        omitted = PagedGithub([[{"filename": PATH}]])
        listed = omitted.pull_files(7)
        self.assertEqual(listed[0].patch, "")

    def test_dispatcher_reviews_files_beyond_the_first_hundred(self):
        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD))
        files = [DiffFile(PATH, PATCH) for _ in range(100)]
        files.append(DiffFile("services/identity-service/hidden.go", "@@\n+hidden\n"))
        github.files[311] = files
        result = run_dispatch(github, cursor, enabled_config(max_qa_fix_cycles=0))
        self.assertEqual(cursor.creates, [])
        self.assertEqual(state_of(github.issues[0])["status"], "BLOCKED")
        self.assertIn("unauthorized_path:services/identity-service/hidden.go", github.comments[-1][1])
        self.assertIn("qa_blocked:BT-100", result.skipped)

        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD))
        files = [DiffFile(PATH, PATCH) for _ in range(100)]
        files.append(DiffFile(PATH, ""))
        github.files[311] = files
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")
        self.assertTrue(any(item.startswith(f"diff_patch_missing:{PATH}") for item in result.skipped))

        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD))
        github.pull_files_error = GithubError(503, "pull files api error")
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertEqual(state_of(github.issues[0])["status"], "BLOCKED")
        self.assertIn("pull_files_blocked:BT-100", result.skipped)

    def test_parallel_cap_covers_qa_agents_and_in_pass_budget(self):
        cursor = FakeCursor()
        github = FakeGithub([])
        first = finished_issue(cursor, github, checks=green(HEAD), install=False)
        second = finished_issue(
            cursor,
            github,
            checks=green(HEAD),
            task_id="BT-200",
            agent="bc-bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
            number=312,
            branch="autopilot/bt-200",
            run_id="run-b",
            install=False,
        )
        github.issues = [first, second]
        result = run_dispatch(github, cursor, enabled_config(max_parallel_agents=1))
        self.assertEqual(len(cursor.creates), 1)
        self.assertTrue(any(item.startswith("concurrency_cap:") for item in result.skipped))
        restarted = run_dispatch(github, cursor, enabled_config(max_parallel_agents=1))
        self.assertEqual(len(cursor.creates), 1)
        self.assertTrue(any(item.startswith("concurrency_cap:") for item in restarted.skipped))

        qa_state = state_of(github.issues[0])
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["status"] = "FINISHED"
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["result"] = "BINTRANS_QA_VERDICT: PASS\n"
        released = run_dispatch(github, cursor, enabled_config(max_parallel_agents=1))
        self.assertEqual(len(cursor.creates), 2)
        self.assertIn("qa:BT-200", released.launched)

        cursor = FakeCursor()
        github = FakeGithub([])
        github.issues = [
            finished_issue(cursor, github, checks=green(HEAD), install=False),
            finished_issue(
                cursor,
                github,
                checks=green(HEAD),
                task_id="BT-200",
                agent="bc-bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
                number=312,
                branch="autopilot/bt-200",
                run_id="run-b",
                install=False,
            ),
        ]
        result = run_dispatch(
            github,
            cursor,
            enabled_config(
                max_parallel_agents=3,
                budget_max_charged_cents=0,
                budget_max_charged_cents_daily=25,
                budget_max_charged_cents_monthly=10_000_000,
                budget_launch_reserve_cents=20,
                budget_reserve_per_active_agent_cents=20,
            ),
        )
        self.assertEqual(len(cursor.creates), 1)
        self.assertTrue(any("daily_budget_exceeded" in item for item in result.skipped))

        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD), launch_dates=["2026-10-27"])
        result = run_dispatch(github, cursor, enabled_config(max_launches_per_day=1))
        self.assertEqual(cursor.creates, [])
        self.assertTrue(any(item.startswith("daily_launch_cap:") for item in result.skipped))

    def test_agent_branch_cannot_replace_the_claim(self):
        for reported in ("autopilot/other-task", "feat/not-the-claim-branch"):
            cursor = FakeCursor()
            github = FakeGithub([])
            finished_issue(cursor, github, checks=green(HEAD), git_branch=reported)
            result = run_dispatch(github, cursor)
            saved = state_of(github.issues[0])
            self.assertEqual(cursor.creates, [])
            self.assertEqual(saved["status"], "NEEDS_CONTROLLER")
            self.assertEqual(saved["branch"], "autopilot/bt-100")
            self.assertEqual(saved["claimed_branch"], "autopilot/bt-100")
            self.assertIn("claimed_branch_mismatch:BT-100", result.skipped)

        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD), pr_head="feat/not-the-claim-branch")
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertIn("pr_branch_mismatch:BT-100", result.skipped)
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")

        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD), head_repo="evil/fork")
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertIn("pr_repo_mismatch:BT-100", result.skipped)

        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD), pr_url="https://github.com/evil/other/pull/311")
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertIn("pr_repo_mismatch:BT-100", result.skipped)

    def test_release_gate_failure_blocks_ready_even_when_task_ci_passed(self):
        checks = green(HEAD)
        for item in checks:
            if item["name"] == "scripts-check":
                item["conclusion"] = "failure"
        self.assertNotIn("scripts-check", required_checks([PATH]))
        self.assertIn("scripts-check", merge_checks([PATH]))
        ok, reason = evaluate_ci(checks, merge_checks([PATH]), HEAD)
        self.assertFalse(ok)
        self.assertEqual(reason, "check_not_success:scripts-check")
        task_ok, task_reason = evaluate_ci(checks, required_checks([PATH]), HEAD)
        self.assertTrue(task_ok)
        self.assertEqual(task_reason, "ci_pass")

        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=checks)
        run_dispatch(github, cursor)
        self.assertEqual(len(cursor.creates), 1)
        qa_state = state_of(github.issues[0])
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["status"] = "FINISHED"
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["result"] = "BINTRANS_QA_VERDICT: PASS\n"
        result = run_dispatch(github, cursor)
        self.assertNotEqual(state_of(github.issues[0])["status"], "READY_FOR_MERGE")
        self.assertTrue(any("scripts-check" in item for item in result.skipped))

        for item in checks:
            if item["name"] == "scripts-check":
                item["conclusion"] = "skipped"
        ok, reason = evaluate_ci(checks, merge_checks([PATH]), HEAD)
        self.assertFalse(ok)
        self.assertEqual(reason, "check_not_success:scripts-check")

    def test_objective_change_invalidates_approval(self):
        original, error = parse_contract(body())
        changed, changed_error = parse_contract(body(OBJECTIVE="Ship a different behavior."))
        self.assertEqual(error, "")
        self.assertEqual(changed_error, "")
        self.assertNotEqual(original.digest, changed.digest)
        self.assertEqual(original.canonical()["objective"], "Add the foo helper.")
        cursor = FakeCursor()
        github = FakeGithub([ready_issue(OBJECTIVE="Ship a different behavior.")])
        github.approval_digest = original.digest
        result = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertIn("approval_digest_mismatch:BT-100", result.skipped)

    def test_stale_qa_head_is_not_reused_after_restart(self):
        cursor = FakeCursor()
        github = FakeGithub([])
        finished_issue(cursor, github, checks=green(HEAD))
        run_dispatch(github, cursor)
        self.assertEqual(len(cursor.creates), 1)
        moved = "d" * 40
        github.pulls[311]["head_sha"] = moved
        github.checks[moved] = green(moved)
        while_running = run_dispatch(github, cursor)
        self.assertEqual(len(cursor.creates), 1)
        self.assertIn("qa_still_active:BT-100", while_running.skipped)
        self.assertEqual(state_of(github.issues[0])["status"], "INDEPENDENT_QA")
        qa_state = state_of(github.issues[0])
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["status"] = "FINISHED"
        cursor.runs[(qa_state["qa_agent_id"], qa_state["qa_run_id"])]["result"] = "BINTRANS_QA_VERDICT: PASS\n"
        after = run_dispatch(github, cursor)
        saved = state_of(github.issues[0])
        self.assertEqual(len(cursor.creates), 1)
        self.assertEqual(saved["status"], "NEEDS_CONTROLLER")
        self.assertEqual(saved.get("qa_verdict") or "", "")
        self.assertIn(f"head_sha_changed:{HEAD}:{moved}", saved["last_error"])
        self.assertTrue(any("head_sha_changed" in item for item in after.skipped))
        restarted = run_dispatch(github, cursor)
        self.assertEqual(len(cursor.creates), 1)
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")
        self.assertIn("not_ready:BT-100:NEEDS_CONTROLLER", restarted.skipped)

    def test_uncertain_follow_up_does_not_burn_or_repeat_the_retry(self):
        cursor = FakeCursor()
        issue = failed_run_issue()
        cursor.runs[(AGENT, "run-1")] = {"id": "run-1", "status": "ERROR"}
        github = FakeGithub([issue])
        cursor.fail_follow = CursorApiError(400, "rejected", "no run")
        first = run_dispatch(github, cursor)
        self.assertEqual(cursor.follow_attempts, 1)
        self.assertEqual(cursor.follows, [])
        self.assertEqual(state_of(github.issues[0])["status"], "IMPLEMENTING")
        self.assertEqual(state_of(github.issues[0])["counted_failure_run_ids"], [])
        self.assertIn("BT-100", first.preserved)
        cursor.fail_follow = None
        second = run_dispatch(github, cursor)
        self.assertEqual(len(cursor.follows), 1)
        self.assertIn("retry:BT-100", second.skipped)
        self.assertEqual(state_of(github.issues[0])["counted_failure_run_ids"], ["run-1"])

        cursor = FakeCursor()
        issue = failed_run_issue()
        cursor.runs[(AGENT, "run-1")] = {"id": "run-1", "status": "ERROR"}
        github = FakeGithub([issue])
        cursor.fail_follow = CursorApiError(503, "unavailable", "maybe sent")
        run_dispatch(github, cursor)
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")
        self.assertEqual(cursor.follow_attempts, 1)
        cursor.fail_follow = None
        run_dispatch(github, cursor)
        self.assertEqual(cursor.follow_attempts, 1)
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")

        cursor = FakeCursor()
        issue = failed_run_issue()
        cursor.runs[(AGENT, "run-1")] = {"id": "run-1", "status": "ERROR"}
        github = FakeGithub([issue])
        cursor.follow_body = {"run": {}}
        run_dispatch(github, cursor)
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")
        attempts = cursor.follow_attempts
        run_dispatch(github, cursor)
        self.assertEqual(cursor.follow_attempts, attempts)
        self.assertEqual(state_of(github.issues[0])["counted_failure_run_ids"], [])

    def test_state_machine_rejects_backward_qa_transitions(self):
        self.assertFalse(can_transition("CI_CHECK", "PR_OPEN"))
        self.assertTrue(can_transition("CI_CHECK", "INDEPENDENT_QA"))
        self.assertTrue(can_transition("CI_CHECK", "BLOCKED"))
        self.assertTrue(can_transition("INDEPENDENT_QA", "READY_FOR_MERGE"))
        self.assertTrue(can_transition("INDEPENDENT_QA", "NEEDS_CONTROLLER"))
        self.assertFalse(can_transition("INDEPENDENT_QA", "CI_CHECK"))
        self.assertFalse(can_transition("READY_FOR_MERGE", "MERGED"))

    def test_workflow_still_rejects_pull_request_execution(self):
        from pathlib import Path

        workflow = Path(__file__).resolve().parents[3].joinpath(".github/workflows/bintrans-autopilot-dispatcher.yml").read_text(encoding="utf-8")
        self.assertNotIn("\n  pull_request:\n", "\n" + workflow)
        self.assertIn("workflow_guard_main", workflow)
        self.assertLess(workflow.find("workflow_guard_main"), workflow.find("secrets.CURSOR_API_KEY"))

    def test_finished_stale_qa_releases_the_openapi_slot(self):
        qa_agent = "bc-dddddddd-dddd-dddd-dddd-dddddddddddd"
        moved = "d" * 40
        labels = ["autopilot", "autopilot-contract-approved", "autopilot-shared-path-approved"]
        holder = ready_issue(
            number=311,
            TASK_ID="BT-100",
            RISK_LEVEL="HIGH",
            ALLOWED_PATHS="- packages/openapi/openapi.yaml",
            labels=labels + ["autopilot:INDEPENDENT_QA"],
        )
        holder.body += "\n<!-- bintrans-autopilot-state\n" + json.dumps({
            "agent_id": AGENT,
            "allowed_paths": ["packages/openapi/openapi.yaml"],
            "base_sha": MAIN,
            "branch": "autopilot/bt-100",
            "claimed_base_sha": MAIN,
            "claimed_branch": "autopilot/bt-100",
            "expected_pr_head": "autopilot/bt-100",
            "expected_repository": REPO,
            "handled_run_id": "run-1",
            "launch_dates": [],
            "max_retries": 1,
            "pr_number": 311,
            "pr_url": f"https://github.com/{REPO}/pull/311",
            "qa_agent_id": qa_agent,
            "qa_head_sha": HEAD,
            "qa_run_id": "qa-run",
            "qa_verdict": "",
            "risk_level": "HIGH",
            "run_id": "run-1",
            "status": "INDEPENDENT_QA",
            "task_id": "BT-100",
        }) + "\n-->\n"
        waiting = ready_issue(
            number=2,
            TASK_ID="BT-200",
            RISK_LEVEL="HIGH",
            ALLOWED_PATHS="- packages/openapi/schemas/foo.yaml",
            labels=labels + ["autopilot:READY"],
        )
        cursor = FakeCursor()
        cursor.runs[(AGENT, "run-1")] = {"id": "run-1", "status": "FINISHED"}
        cursor.runs[(qa_agent, "qa-run")] = {"id": "qa-run", "status": "RUNNING", "result": "BINTRANS_QA_VERDICT: PASS\n"}
        github = FakeGithub([holder, waiting])
        github.pulls[311] = {
            "number": 311,
            "repo": REPO,
            "head_repo": REPO,
            "head_branch": "autopilot/bt-100",
            "head_sha": moved,
            "base_sha": MAIN,
            "draft": True,
            "html_url": f"https://github.com/{REPO}/pull/311",
        }
        held = run_dispatch(github, cursor)
        self.assertEqual(cursor.creates, [])
        self.assertEqual(state_of(github.issues[0])["status"], "INDEPENDENT_QA")
        self.assertIn("qa_still_active:BT-100", held.skipped)
        self.assertIn("domain_cap:BT-200", held.skipped)

        cursor.runs[(qa_agent, "qa-run")]["status"] = "FINISHED"
        released = run_dispatch(github, cursor)
        saved = state_of(github.issues[0])
        self.assertEqual(saved["status"], "NEEDS_CONTROLLER")
        self.assertEqual(saved.get("qa_verdict") or "", "")
        self.assertIn(f"head_sha_changed:{HEAD}:{moved}", saved["last_error"])
        self.assertNotEqual(saved["status"], "READY_FOR_MERGE")
        self.assertEqual(len(cursor.creates), 1)
        self.assertIn("BT-200", released.launched)
        self.assertNotIn("qa:BT-100", released.launched)
        self.assertNotIn("qa:BT-200", released.launched)
        self.assertEqual(cursor.creates[0]["mode"], "agent")

    def test_claim_retry_checks_budget_without_counting_its_own_slot(self):
        cursor = FakeCursor()
        github = FakeGithub([claimed_without_run()])
        result = run_dispatch(github, cursor, enabled_config(max_parallel_agents=1, max_launches_per_day=1))
        self.assertEqual(len(cursor.creates), 1)
        self.assertEqual(cursor.create_attempts, 1)
        self.assertEqual(state_of(github.issues[0])["status"], "IMPLEMENTING")
        self.assertIn("BT-100", result.launched)

        other = ready_issue(number=2, TASK_ID="BT-200")
        other.body += "\n<!-- bintrans-autopilot-state\n" + json.dumps({
            "agent_id": "bc-eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
            "launch_dates": ["2026-10-27"],
            "run_id": "run-other",
            "status": "IMPLEMENTING",
            "task_id": "BT-200",
        }) + "\n-->\n"
        other.labels = ["autopilot", "autopilot-contract-approved", "autopilot:IMPLEMENTING"]
        cursor = FakeCursor()
        cursor.runs[("bc-eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", "run-other")] = {"id": "run-other", "status": "RUNNING"}
        github = FakeGithub([claimed_without_run(), other])
        blocked = run_dispatch(github, cursor, enabled_config(max_parallel_agents=1, max_launches_per_day=2))
        self.assertEqual(cursor.creates, [])
        self.assertIn("concurrency_cap:BT-100", blocked.skipped)
        self.assertEqual(state_of(github.issues[0])["status"], "CLAIMED")

        cursor = FakeCursor()

        def fail_later(start, end, original=cursor.cloud_charged_cents):
            if len(cursor.spend_calls) >= 2:
                raise CursorApiError(503, "spend", "unavailable")
            return original(start, end)

        cursor.cloud_charged_cents = fail_later
        github = FakeGithub([claimed_without_run()])
        denied = run_dispatch(github, cursor)
        self.assertEqual(cursor.create_attempts, 0)
        self.assertIn("spend_api_error:BT-100", denied.skipped)
        self.assertEqual(state_of(github.issues[0])["status"], "CLAIMED")

        cursor = FakeCursor()
        cursor.fail_create = CursorApiError(503, "unavailable", "maybe sent")
        github = FakeGithub([claimed_without_run()])
        uncertain = run_dispatch(github, cursor)
        self.assertEqual(cursor.create_attempts, 1)
        self.assertEqual(cursor.creates, [])
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")
        self.assertIn("claim_retry_uncertain:BT-100", uncertain.skipped)
        cursor.fail_create = None
        run_dispatch(github, cursor)
        self.assertEqual(cursor.create_attempts, 1)

        cursor = FakeCursor()
        cursor.create_body = {"agent": {"id": AGENT}, "run": {}}
        github = FakeGithub([claimed_without_run()])
        empty = run_dispatch(github, cursor)
        self.assertEqual(cursor.create_attempts, 1)
        self.assertEqual(state_of(github.issues[0])["status"], "NEEDS_CONTROLLER")
        self.assertIn("claim_retry_uncertain:BT-100", empty.skipped)
        cursor.create_body = None
        run_dispatch(github, cursor)
        self.assertEqual(cursor.create_attempts, 1)


if __name__ == "__main__":
    unittest.main()
