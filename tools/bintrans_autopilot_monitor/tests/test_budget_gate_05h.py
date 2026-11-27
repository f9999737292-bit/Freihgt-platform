"""WF-STATIC, correlation, and full-reservation tests. Offline only."""

from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "src"))

from bintrans_monitor.budget_gate import (  # noqa: E402
    AdapterResult,
    BudgetDeny,
    BudgetGate,
    DispatchRequest,
    FakeDispatchAdapter,
    initialize_campaign,
)
from bintrans_monitor.dispatch_adapter import (  # noqa: E402
    GitHubDispatchAdapter,
    OfflineTransport,
    TransportResponse,
    TrustedDispatchPrincipal,
    TrustedRefBinding,
    correlate_run,
)

SHA = "ab" * 20
OTHER_SHA = "cd" * 20
REPO = "example/budget-lab"
TRUSTED = TrustedDispatchPrincipal(actor_id=4242, repository_id=1001, workflow_id=77, head_branch="main")
WORKFLOW = "monitor-budget"
REF = "refs/heads/main"
DISPATCH_WORKFLOW = ROOT / "candidate" / ".github" / "workflows" / "bintrans-autopilot-budget-dispatch.yml"
CLASSIFIER = ROOT / "candidate" / ".github" / "workflows" / "bintrans-autopilot-monitor.yml"
ACTIVE = ROOT.parents[1] / ".github" / "workflows" / "bintrans-autopilot-budget-dispatch.yml"
LAB_REPOSITORY = "f9999737292-bit/bintrans-autopilot-security-lab"
PRODUCT_REPOSITORY = "f9999737292-bit/Freihgt-platform"
LAB_DISPATCH_FIXTURE = ROOT / "tests" / "fixtures" / "lab-budget-dispatch.yml"
LAB_MONITOR_FIXTURE = ROOT / "tests" / "fixtures" / "lab-monitor.yml"


def assert_isolated_repository_binding(test: unittest.TestCase, text: str, allowed: str) -> None:
    test.assertEqual(allowed, LAB_REPOSITORY)
    test.assertNotEqual(allowed, PRODUCT_REPOSITORY)
    test.assertNotIn(PRODUCT_REPOSITORY, text)
    test.assertNotIn("inputs.repository", text)
    test.assertNotIn("repository_dispatch:", text)
    repository_lines = [line.strip() for line in text.splitlines() if line.strip().startswith("repository:")]
    test.assertTrue(repository_lines)
    test.assertTrue(all(line == "repository: " + allowed for line in repository_lines), repository_lines)
    test.assertIn("github.repository == '" + allowed + "'", text)
    test.assertIn('if [ "$GITHUB_REPOSITORY" != "' + allowed + '" ]; then', text)
    test.assertIn("exit 3", text)


def _request(dispatch_id: str, **overrides: object) -> DispatchRequest:
    values = {
        "dispatch_id": dispatch_id,
        "idempotency_key": dispatch_id + "-key",
        "repository": REPO,
        "workflow": WORKFLOW,
        "expected_sha": SHA,
        "job_count": 1,
        "per_job_minutes": 10,
        "paid_ai_calls": 0,
        "run_attempt": 1,
        "campaign_id": "camp-1",
    }
    values.update(overrides)
    return DispatchRequest(**values)  # type: ignore[arg-type]


def _named(run_id: str, dispatch_id: str, **overrides: object) -> dict[str, object]:
    values = {
        "id": run_id,
        "name": f"bintrans-budget camp-1 {dispatch_id} {SHA}",
        "display_title": f"bintrans-budget camp-1 {dispatch_id} {SHA}",
        "head_sha": SHA,
        "head_branch": "main",
        "event": "workflow_dispatch",
        "workflow": WORKFLOW,
        "workflow_id": 77,
        "repository": REPO,
        "repository_id": 1001,
        "actor": {"id": 4242, "login": "dispatch-bot"},
        "triggering_actor": {"id": 4242, "login": "dispatch-bot"},
        "run_attempt": 1,
        "created_at": "2026-11-27T00:01:00Z",
    }
    values.update(overrides)
    return values


class HostedContractTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.path = Path(self._tmp.name) / "ledger.sqlite"
        initialize_campaign(self.path, campaign_id="camp-1", repository=REPO, workflow=WORKFLOW, expected_sha=SHA)
        self.gate = BudgetGate.open(self.path)
        self.gate.set_emergency_stop("off")

    def tearDown(self):
        self.gate.close()
        self._tmp.cleanup()

    def test_wf_static_001_permissions_triggers_and_timeouts(self):
        text = DISPATCH_WORKFLOW.read_text(encoding="utf-8")
        self.assertIn("workflow_dispatch:", text)
        self.assertNotIn("schedule:", text)
        self.assertNotIn("pull_request:", text)
        self.assertNotIn("pull_request_target:", text)
        self.assertNotIn("contents: write", text)
        self.assertNotIn("actions: write", text)
        self.assertIn("actions: read", text)
        self.assertIn("contents: read", text)
        self.assertNotIn("api.cursor.com", text)
        self.assertNotIn("openai", text.lower())
        self.assertNotIn("anthropic", text.lower())
        self.assertEqual(text.count("timeout-minutes:"), 2)
        self.assertIn("timeout-minutes: 2", text)
        self.assertIn("timeout-minutes: 10", text)
        self.assertIn("persist-credentials: false", text)
        self.assertIn("actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683", text)
        self.assertFalse(ACTIVE.exists())
        self.assertTrue(CLASSIFIER.exists())

    def test_wf_static_002_required_inputs_and_no_shell_interpolation(self):
        text = DISPATCH_WORKFLOW.read_text(encoding="utf-8")
        for name in ("budget_dispatch_id:", "campaign_id:", "expected_head_sha:"):
            self.assertIn(name, text)
        self.assertEqual(text.count("required: true"), 3)
        self.assertNotIn("inputs.run_id", text)
        allowed = ("run-name:", "EXPECTED_HEAD_SHA:", "BUDGET_DISPATCH_ID:", "CAMPAIGN_ID:", "ref:")
        for line in text.splitlines():
            if "${{ inputs." in line:
                self.assertTrue(line.strip().startswith(allowed), line)
        self.assertIn('printf \'%s\' "$EXPECTED_HEAD_SHA"', text)
        self.assertIn('printf \'%s\' "$BUDGET_DISPATCH_ID"', text)
        self.assertIn('printf \'%s\' "$CAMPAIGN_ID"', text)

    def test_wf_static_003_sha_guard_precedes_checkout(self):
        text = DISPATCH_WORKFLOW.read_text(encoding="utf-8")
        guard = text.find("SHA_GUARD=FAIL")
        checkout = text.find("actions/checkout@")
        self.assertGreater(guard, 0)
        self.assertLess(guard, checkout)
        self.assertLess(text.find("needs: sha-guard"), checkout)
        self.assertIn("ref: ${{ inputs.expected_head_sha }}", text)
        self.assertNotIn("gh api", text)
        self.assertIn("CHECKOUT_EXECUTED=NO", text)
        self.assertIn("MONITOR_EXECUTED=NO", text)
        self.assertIn("MONITOR_API_CALLS=0", text)
        self.assertIn("RESULT=REJECTED", text)

    def test_wf_static_004_isolated_repository_binding(self):
        dispatch = LAB_DISPATCH_FIXTURE.read_text(encoding="utf-8")
        monitor = LAB_MONITOR_FIXTURE.read_text(encoding="utf-8")
        for text in (dispatch, monitor):
            assert_isolated_repository_binding(self, text, LAB_REPOSITORY)
        self.assertEqual(dispatch.count("github.repository == '" + LAB_REPOSITORY + "'"), 2)
        self.assertIn("ref: ${{ inputs.expected_head_sha }}", dispatch)
        self.assertLess(dispatch.find("SHA_GUARD=FAIL"), dispatch.find("actions/checkout@"))
        self.assertFalse(ACTIVE.exists())
        mutated = dispatch.replace(LAB_REPOSITORY, PRODUCT_REPOSITORY)
        with self.assertRaises(AssertionError):
            assert_isolated_repository_binding(self, mutated, LAB_REPOSITORY)
        foreign = [_named("foreign", "corr-4", repository=PRODUCT_REPOSITORY)]
        with self.assertRaises(BudgetDeny) as caught:
            correlate_run(
                foreign,
                dispatch_id="corr-4",
                expected_sha=SHA,
                workflow=WORKFLOW,
                campaign_id="camp-1",
                repository=LAB_REPOSITORY,
                trusted=TRUSTED,
            )
        self.assertEqual(caught.exception.code, "repository_rejected")

    def test_corr_001_one_visible_run_name_binds_that_run_id(self):
        runs = [
            _named("newest", "other-dispatch", created_at="2026-11-27T00:09:00Z"),
            _named("bound-run", "corr-1", created_at="2026-11-27T00:01:00Z"),
        ]
        self.assertEqual(
            correlate_run(runs, dispatch_id="corr-1", expected_sha=SHA, workflow=WORKFLOW, campaign_id="camp-1", trusted=TRUSTED, repository=REPO),
            "bound-run",
        )

    def test_corr_002_missing_or_ambiguous_correlation_keeps_the_reserve(self):
        with self.assertRaises(BudgetDeny) as missing:
            correlate_run([], dispatch_id="corr-2", expected_sha=SHA, workflow=WORKFLOW, campaign_id="camp-1", trusted=TRUSTED)
        self.assertEqual(missing.exception.code, "dispatch_unknown")
        duplicated = [_named("one", "corr-2"), _named("two", "corr-2")]
        with self.assertRaises(BudgetDeny) as ambiguous:
            correlate_run(duplicated, dispatch_id="corr-2", expected_sha=SHA, workflow=WORKFLOW, campaign_id="camp-1", trusted=TRUSTED)
        self.assertEqual(ambiguous.exception.code, "dispatch_unknown")

        def handler(method: str, path: str, body):
            if method == "GET":
                return TransportResponse(200, {"sha": SHA})
            return TransportResponse(204, None)

        adapter = GitHubDispatchAdapter(
            OfflineTransport(handler), repository=REPO, workflow=WORKFLOW, allowed_ref=REF, expected_sha=SHA
        )
        request = _request("corr-2")
        self.gate.reserve(request)
        with self.assertRaises(BudgetDeny):
            self.gate.dispatch(
                request,
                adapter,
                binding=TrustedRefBinding(REF, SHA, REPO, WORKFLOW),
            )
        self.assertEqual(self.gate.snapshot("corr-2").status, "DISPATCH_UNKNOWN")
        self.assertIsNone(self.gate.confirmed_minutes("corr-2"))
        self.assertLess(self.gate.available_minutes(), 60)
        with self.assertRaises(BudgetDeny) as blocked:
            self.gate.reserve(_request("corr-2b"))
        self.assertEqual(blocked.exception.code, "unknown_usage")

    def test_corr_003_wrong_identity_blocks_reconciliation(self):
        wrong_sha = [_named("run-sha", "corr-3", head_sha=OTHER_SHA, name=f"bintrans-budget camp-1 corr-3 {OTHER_SHA}", display_title=f"bintrans-budget camp-1 corr-3 {OTHER_SHA}")]
        with self.assertRaises(BudgetDeny) as sha:
            correlate_run(wrong_sha, dispatch_id="corr-3", expected_sha=SHA, workflow=WORKFLOW, campaign_id="camp-1", trusted=TRUSTED, repository=REPO)
        self.assertEqual(sha.exception.code, "sha_mismatch")
        wrong_workflow = [_named("run-wf", "corr-3", workflow="other-workflow")]
        with self.assertRaises(BudgetDeny) as workflow:
            correlate_run(wrong_workflow, dispatch_id="corr-3", expected_sha=SHA, workflow=WORKFLOW, campaign_id="camp-1", trusted=TRUSTED)
        self.assertEqual(workflow.exception.code, "workflow_rejected")
        wrong_attempt = [_named("run-try", "corr-3", run_attempt=2)]
        with self.assertRaises(BudgetDeny) as attempt:
            correlate_run(wrong_attempt, dispatch_id="corr-3", expected_sha=SHA, workflow=WORKFLOW, campaign_id="camp-1", trusted=TRUSTED)
        self.assertEqual(attempt.exception.code, "rerun_without_reservation")
        request = _request("corr-3")
        self.gate.reserve(request)
        with self.assertRaises(BudgetDeny):
            self.gate.dispatch(request, FakeDispatchAdapter(AdapterResult("unconfirmed")))
        with self.assertRaises(BudgetDeny) as blocked:
            self.gate.bind_observed_run(
                "corr-3", run_id="run-sha", run_attempt=1, expected_sha=OTHER_SHA, repository=REPO, workflow=WORKFLOW
            )
        self.assertEqual(blocked.exception.code, "sha_mismatch")
        self.assertTrue(self.gate.blocked())
        self.assertIsNone(self.gate.confirmed_minutes("corr-3"))

    def test_bill_reg_001_d1_through_d6_hold_58_minutes(self):
        plans = (("d1", 10), ("d2", 10), ("d3", 10), ("d4", 2), ("d5", 10), ("d6", 10))
        for dispatch_id, per_job in plans:
            request = _request(dispatch_id, per_job_minutes=per_job)
            self.gate.reserve(request)
            self.gate.dispatch(request, FakeDispatchAdapter(AdapterResult("accepted", "run-" + dispatch_id, 1, SHA)))
            self.gate.retain_full_reservation(
                dispatch_id, run_id="run-" + dispatch_id, run_attempt=1, repository=REPO, workflow=WORKFLOW, expected_sha=SHA
            )
            self.assertIsNone(self.gate.confirmed_minutes(dispatch_id))
            self.assertEqual(self.gate.snapshot(dispatch_id).status, "RESERVATION_HELD")
        self.assertEqual(self.gate.available_minutes(), 2)
        before = self.gate.available_minutes()
        self.gate.retain_full_reservation(
            "d6", run_id="run-d6", run_attempt=1, repository=REPO, workflow=WORKFLOW, expected_sha=SHA
        )
        self.assertEqual(self.gate.available_minutes(), before)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d7"))
        self.assertEqual(caught.exception.code, "insufficient_minutes")

    def test_bill_reg_002_unknown_or_short_budget_blocks_the_next_dispatch(self):
        request = _request("unknown-run")
        self.gate.reserve(request)
        with self.assertRaises(BudgetDeny):
            self.gate.dispatch(request, FakeDispatchAdapter(AdapterResult("unconfirmed")))
        with self.assertRaises(BudgetDeny) as unknown:
            self.gate.reserve(_request("next-after-unknown"))
        self.assertEqual(unknown.exception.code, "unknown_usage")

    def test_external_run_and_ref_race_do_not_release_a_reserve(self):
        decoy = [_named("moved", "race-1", head_sha=OTHER_SHA, name=f"bintrans-budget camp-1 race-1 {OTHER_SHA}", display_title=f"bintrans-budget camp-1 race-1 {OTHER_SHA}", created_at="2026-11-27T00:09:00Z")]
        with self.assertRaises(BudgetDeny) as raced:
            correlate_run(decoy, dispatch_id="race-1", expected_sha=SHA, workflow=WORKFLOW, campaign_id="camp-1", trusted=TRUSTED, repository=REPO)
        self.assertEqual(raced.exception.code, "sha_mismatch")
        seen: dict[str, object] = {}

        def handler(method: str, path: str, body):
            if method == "GET":
                return TransportResponse(200, {"sha": SHA})
            seen["body"] = body
            return TransportResponse(204, None)

        adapter = GitHubDispatchAdapter(
            OfflineTransport(handler), repository=REPO, workflow=WORKFLOW, allowed_ref=REF, expected_sha=SHA
        )
        request = _request("race-1")
        self.gate.reserve(request)
        with self.assertRaises(BudgetDeny):
            self.gate.dispatch(request, adapter, binding=TrustedRefBinding(REF, SHA, REPO, WORKFLOW))
        self.assertEqual(seen["body"]["ref"], "main")
        self.assertNotEqual(seen["body"]["ref"], SHA)
        self.assertEqual(self.gate.snapshot("race-1").status, "DISPATCH_UNKNOWN")
        with self.assertRaises(BudgetDeny) as external:
            self.gate.observe_external_run(run_id="outside", run_attempt=1, repository=REPO, workflow=WORKFLOW, expected_sha=SHA)
        self.assertEqual(external.exception.code, "untracked_run")
        with self.assertRaises(BudgetDeny):
            self.gate.reserve(_request("race-2"))


if __name__ == "__main__":
    unittest.main()
