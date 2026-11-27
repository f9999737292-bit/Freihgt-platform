"""Trusted actor and event correlation. Offline only."""

from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "src"))

from bintrans_monitor.budget_gate import (  # noqa: E402
    BudgetDeny,
    BudgetGate,
    DispatchRequest,
    initialize_campaign,
)
from bintrans_monitor.dispatch_adapter import (  # noqa: E402
    GitHubDispatchAdapter,
    GitHubReadAdapter,
    OfflineTransport,
    TransportResponse,
    TrustedDispatchPrincipal,
    TrustedRefBinding,
    correlate_run,
    inspect_run_inventory,
)

SHA = "ab" * 20
OTHER_SHA = "cd" * 20
REPO = "example/budget-lab"
WORKFLOW = "monitor-budget"
REF = "refs/heads/main"
TRUSTED = TrustedDispatchPrincipal(actor_id=4242, repository_id=1001, workflow_id=77, head_branch="main")


def _user(actor_id: int = 4242, login: str = "dispatch-bot") -> dict[str, object]:
    return {"id": actor_id, "login": login}


def _run(run_id: str, dispatch_id: str = "d1", **overrides: object) -> dict[str, object]:
    values: dict[str, object] = {
        "id": run_id,
        "name": f"bintrans-budget camp-1 {dispatch_id} {SHA}",
        "display_title": f"bintrans-budget camp-1 {dispatch_id} {SHA}",
        "head_sha": SHA,
        "head_branch": "main",
        "event": "workflow_dispatch",
        "workflow": WORKFLOW,
        "workflow_id": 77,
        "repository": {"id": 1001, "full_name": REPO},
        "actor": _user(),
        "triggering_actor": _user(),
        "run_attempt": 1,
        "campaign_id": "camp-1",
        "budget_dispatch_id": dispatch_id,
    }
    values.update(overrides)
    return values


def _request(dispatch_id: str) -> DispatchRequest:
    return DispatchRequest(
        dispatch_id=dispatch_id,
        idempotency_key=dispatch_id + "-key",
        repository=REPO,
        workflow=WORKFLOW,
        expected_sha=SHA,
        job_count=1,
        per_job_minutes=10,
        paid_ai_calls=0,
        run_attempt=1,
        campaign_id="camp-1",
    )


class ActorEventTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.path = Path(self._tmp.name) / "ledger.sqlite"
        initialize_campaign(self.path, campaign_id="camp-1", repository=REPO, workflow=WORKFLOW, expected_sha=SHA)
        self.gate = BudgetGate.open(self.path)
        self.gate.set_emergency_stop("off")

    def tearDown(self):
        self.gate.close()
        self._tmp.cleanup()

    def _match(self, runs: list[dict[str, object]], dispatch_id: str = "d1") -> str:
        return correlate_run(
            runs,
            dispatch_id=dispatch_id,
            expected_sha=SHA,
            workflow=WORKFLOW,
            campaign_id="camp-1",
            repository=REPO,
            trusted=TRUSTED,
        )

    def _deny(self, runs: list[dict[str, object]], dispatch_id: str = "d1") -> str:
        with self.assertRaises(BudgetDeny) as caught:
            self._match(runs, dispatch_id)
        return caught.exception.code

    def _incident(self, reason: str, run_id: str) -> None:
        before = self.gate.available_minutes()
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.record_security_incident(reason, run_id)
        self.assertEqual(caught.exception.code, reason)
        self.assertTrue(self.gate.blocked())
        self.assertEqual(self.gate.available_minutes(), before)
        self.assertIsNone(self.gate.confirmed_minutes("held"))
        with self.assertRaises(BudgetDeny) as nxt:
            self.gate.reserve(_request("next-dispatch"))
        self.assertEqual(nxt.exception.code, "campaign_blocked")

    def test_actor_001_trusted_actor_does_not_release_the_reserve(self):
        self.gate.reserve(_request("held"))
        self.assertEqual(self._match([_run("run-1", "held")], "held"), "run-1")
        self.assertIsNone(self.gate.confirmed_minutes("held"))
        self.assertEqual(self.gate.available_minutes(), 49)

    def test_actor_002_unknown_actor_is_denied(self):
        self.assertEqual(self._deny([_run("run-2", actor=_user(99, "other"))]), "actor_rejected")

    def test_actor_003_same_login_and_different_actor_id_is_denied(self):
        self.assertEqual(self._deny([_run("run-3", actor=_user(99, "dispatch-bot"))]), "actor_rejected")

    def test_actor_004_missing_actor_is_denied(self):
        run = _run("run-4")
        del run["actor"]
        self.assertEqual(self._deny([run]), "missing_identity")

    def test_actor_005_unexpected_triggering_actor_is_denied(self):
        run = _run("run-5", triggering_actor=_user(99, "rerunner"))
        self.assertEqual(self._deny([run]), "triggering_actor_rejected")

    def test_event_001_workflow_dispatch_can_match(self):
        self.assertEqual(self._match([_run("event-1")]), "event-1")

    def test_event_002_push_is_denied(self):
        self.assertEqual(self._deny([_run("event-2", event="push")]), "event_rejected")

    def test_event_003_pull_request_is_denied(self):
        self.assertEqual(self._deny([_run("event-3", event="pull_request")]), "event_rejected")

    def test_event_004_missing_event_is_denied(self):
        run = _run("event-4")
        del run["event"]
        self.assertEqual(self._deny([run]), "missing_identity")

    def test_event_005_disallowed_run_attempt_is_denied(self):
        self.assertEqual(self._deny([_run("event-5", run_attempt=2)]), "rerun_without_reservation")

    def test_corr_004_external_run_with_the_same_identifier_is_denied(self):
        external = _run("external", actor=_user(99, "stranger"), triggering_actor=_user(99, "stranger"))
        self.assertEqual(self._deny([external]), "actor_rejected")

    def test_corr_005_duplicate_identifiers_are_unknown(self):
        runs = [_run("one"), _run("two")]
        decision, inspections = inspect_run_inventory(
            runs,
            dispatch_id="d1",
            expected_sha=SHA,
            workflow=WORKFLOW,
            campaign_id="camp-1",
            repository=REPO,
            trusted=TRUSTED,
        )
        self.assertEqual(decision, "dispatch_unknown")
        self.assertEqual([item.run_id for item in inspections], ["one", "two"])
        self.gate.reserve(_request("held"))
        self._incident("dispatch_unknown", "one")

    def test_corr_006_external_run_is_not_hidden_by_filtering(self):
        runs = [
            _run("external", "other-dispatch", actor=_user(99, "stranger"), triggering_actor=_user(99, "stranger")),
            _run("trusted", "d1"),
        ]
        decision, inspections = inspect_run_inventory(
            runs,
            dispatch_id="d1",
            expected_sha=SHA,
            workflow=WORKFLOW,
            campaign_id="camp-1",
            repository=REPO,
            trusted=TRUSTED,
        )
        self.assertEqual(decision, "external_dispatch")
        self.assertEqual([item.run_id for item in inspections], ["external", "trusted"])
        self.assertTrue(any(item.incident and item.run_id == "external" for item in inspections))
        reader = GitHubReadAdapter(
            OfflineTransport(lambda method, path, body: TransportResponse(200, {"workflow_runs": runs})),
            repository=REPO,
            workflow=WORKFLOW,
            expected_sha=SHA,
            allowed_ref=REF,
        )
        path = reader.list_runs_path()
        self.assertNotIn("actor=", path)
        self.assertNotIn("event=", path)
        self.assertNotIn("head_sha=", path)
        checker = GitHubDispatchAdapter(
            OfflineTransport(lambda method, request_path, body: TransportResponse(204, None)),
            repository=REPO,
            workflow=WORKFLOW,
            allowed_ref=REF,
            expected_sha=SHA,
        )
        with self.assertRaises(BudgetDeny) as filtered:
            checker.perform("GET", path + "&actor=dispatch-bot")
        self.assertEqual(filtered.exception.code, "inventory_filter_rejected")

    def test_corr_007_unknown_dispatch_keeps_the_full_reserve(self):
        self.gate.reserve(_request("held"))
        self.assertEqual(self._deny([]), "dispatch_unknown")
        self.assertEqual(self.gate.snapshot("held").status, "RESERVED")
        self.assertIsNone(self.gate.confirmed_minutes("held"))
        self.assertEqual(self.gate.available_minutes(), 49)

    def test_ref_001_ref_change_before_dispatch_is_not_sent(self):
        calls: list[str] = []

        def handler(method: str, path: str, body):
            calls.append(method)
            if method == "GET":
                return TransportResponse(200, {"sha": OTHER_SHA})
            return TransportResponse(204, None)

        adapter = GitHubDispatchAdapter(
            OfflineTransport(handler), repository=REPO, workflow=WORKFLOW, allowed_ref=REF, expected_sha=SHA
        )
        request = _request("held")
        self.gate.reserve(request)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.dispatch(request, adapter, binding=TrustedRefBinding(REF, SHA, REPO, WORKFLOW))
        self.assertEqual(caught.exception.code, "ref_changed")
        self.assertEqual(calls, ["GET"])
        self.assertEqual(self.gate.snapshot("held").status, "DISPATCH_UNKNOWN")
        self.assertEqual(self.gate.available_minutes(), 49)
        with self.assertRaises(BudgetDeny) as nxt:
            self.gate.reserve(_request("after-ref"))
        self.assertEqual(nxt.exception.code, "unknown_usage")

    def test_ref_002_sha_mismatch_after_dispatch_keeps_the_reserve(self):
        self.gate.reserve(_request("held"))
        mismatched = _run(
            "moved",
            "held",
            head_sha=OTHER_SHA,
            name=f"bintrans-budget camp-1 held {OTHER_SHA}",
            display_title=f"bintrans-budget camp-1 held {OTHER_SHA}",
        )
        self.assertEqual(self._deny([mismatched], "held"), "sha_mismatch")
        self._incident("sha_mismatch", "moved")
