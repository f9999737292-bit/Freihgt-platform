"""BUDGET-001 through BUDGET-022. Temporary SQLite and a fake adapter only."""

from __future__ import annotations

import multiprocessing
import socket
import sqlite3
import sys
import tempfile
import threading
import time
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "src"))

from bintrans_monitor.budget_gate import (  # noqa: E402
    AdapterCrashed,
    AdapterResult,
    BudgetDeny,
    BudgetGate,
    DispatchRequest,
    FakeDispatchAdapter,
    ResponseLost,
    initialize_campaign,
)

SHA = "ab" * 20
REPO = "example/budget-lab"
WORKFLOW = "monitor-budget"


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
    }
    values.update(overrides)
    return DispatchRequest(**values)  # type: ignore[arg-type]


def _cross_process_reserve(path: str, dispatch_id: str, queue) -> None:
    sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))
    from bintrans_monitor.budget_gate import BudgetDeny as Deny
    from bintrans_monitor.budget_gate import BudgetGate as Gate
    from bintrans_monitor.budget_gate import DispatchRequest as Request

    try:
        gate = Gate.open(Path(path))
        gate.reserve(
            Request(
                dispatch_id=dispatch_id,
                idempotency_key=dispatch_id + "-key",
                repository=REPO,
                workflow=WORKFLOW,
                expected_sha=SHA,
                job_count=1,
                per_job_minutes=10,
                paid_ai_calls=0,
                run_attempt=1,
            )
        )
        queue.put("ok")
    except Deny as exc:
        queue.put("deny:" + exc.code)
    except Exception as exc:
        queue.put("err:" + type(exc).__name__ + ":" + str(exc))


class BudgetGateTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.path = Path(self._tmp.name) / "ledger.sqlite"
        initialize_campaign(
            self.path,
            campaign_id="camp-1",
            repository=REPO,
            workflow=WORKFLOW,
            expected_sha=SHA,
        )
        self.gate = BudgetGate.open(self.path)
        self.gate.set_emergency_stop("off")

    def tearDown(self):
        self.gate.close()
        self._tmp.cleanup()

    def _run(self, dispatch_id: str, **overrides: object):
        request = _request(dispatch_id, **overrides)
        outcome = self.gate.reserve(request)
        return request, outcome

    def _accept(self, request: DispatchRequest, run_id: str = "run-1"):
        adapter = FakeDispatchAdapter(AdapterResult("accepted", run_id, 1, SHA))
        outcome = self.gate.dispatch(request, adapter)
        return adapter, outcome

    def test_budget_001_sufficient_balance_reserves(self):
        _, outcome = self._run("d-001")
        self.assertEqual(outcome.status, "RESERVED")
        self.assertEqual(outcome.reserved_minutes, 11)
        self.assertEqual(self.gate.available_minutes(), 49)

    def test_budget_002_insufficient_balance_denies(self):
        for index in range(5):
            request, _ = self._run(f"d-002-{index}")
            self._accept(request, f"run-002-{index}")
            self.gate.reconcile(
                request.dispatch_id,
                [{"job_id": "job", "minutes": 11, "conclusion": "success"}],
                run_id=f"run-002-{index}",
                run_attempt=1,
            )
        self.assertEqual(self.gate.available_minutes(), 5)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d-002-next"))
        self.assertEqual(caught.exception.code, "insufficient_minutes")

    def test_budget_003_unknown_usage_denies_next_reserve(self):
        request, _ = self._run("d-003")
        self._accept(request)
        self.gate.note_usage_delayed("d-003")
        self.assertEqual(self.gate.available_minutes(), 49)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d-003-next"))
        self.assertEqual(caught.exception.code, "unknown_usage")

    def test_budget_004_concurrent_reservation_allows_one(self):
        barrier = threading.Barrier(2)
        results: list[str] = []

        def attempt(dispatch_id: str):
            gate = BudgetGate.open(self.path)
            try:
                barrier.wait(5)
                gate.reserve(_request(dispatch_id))
                results.append("ok")
            except BudgetDeny as exc:
                results.append(exc.code)
            finally:
                gate.close()

        first = threading.Thread(target=attempt, args=("d-004-a",))
        second = threading.Thread(target=attempt, args=("d-004-b",))
        first.start()
        second.start()
        first.join(10)
        second.join(10)
        self.assertEqual(sorted(results), ["concurrent_run", "ok"])

    def test_budget_005_duplicate_dispatch_id_is_denied(self):
        self._run("d-005")
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d-005"))
        self.assertEqual(caught.exception.code, "duplicate_dispatch_id")
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d-005-other", idempotency_key="d-005-key"))
        self.assertEqual(caught.exception.code, "duplicate_idempotency_key")

    def test_budget_006_controller_restart_keeps_reservation(self):
        self._run("d-006")
        self.gate.close()
        restored = BudgetGate.open(self.path)
        self.gate = restored
        self.assertEqual(restored.snapshot("d-006").status, "RESERVED")
        self.assertEqual(restored.available_minutes(), 49)

    def test_budget_007_unfinished_reserve_survives_crash(self):
        self._run("d-007")
        self.gate.close()
        restored = BudgetGate.open(self.path)
        self.gate = restored
        self.assertEqual(restored.snapshot("d-007").status, "RESERVED")
        with self.assertRaises(BudgetDeny) as caught:
            restored.reserve(_request("d-007-next"))
        self.assertEqual(caught.exception.code, "concurrent_run")

    def test_budget_008_cancelled_workflow_still_consumes_minutes(self):
        request, _ = self._run("d-008")
        self._accept(request, "run-008")
        self.gate.reconcile(
            "d-008",
            [{"job_id": "job", "minutes": 4, "conclusion": "cancelled"}],
            run_id="run-008",
            run_attempt=1,
        )
        self.assertEqual(self.gate.confirmed_minutes("d-008"), 4)
        self.assertEqual(self.gate.available_minutes(), 56)
        self.assertFalse(self.gate.blocked())

    def test_budget_009_failed_workflow_still_consumes_minutes(self):
        request, _ = self._run("d-009")
        self._accept(request, "run-009")
        self.gate.reconcile(
            "d-009",
            [{"job_id": "job", "minutes": 6, "conclusion": "failure"}],
            run_id="run-009",
            run_attempt=1,
        )
        self.assertEqual(self.gate.confirmed_minutes("d-009"), 6)
        self.assertEqual(self.gate.snapshot("d-009").status, "RECONCILED")

    def test_budget_010_delayed_usage_keeps_the_full_reserve(self):
        request, _ = self._run("d-010")
        self._accept(request, "run-010")
        before = self.gate.available_minutes()
        self.gate.note_usage_delayed("d-010")
        self.assertEqual(self.gate.snapshot("d-010").status, "TERMINAL_PENDING_RECONCILIATION")
        self.assertIsNone(self.gate.confirmed_minutes("d-010"))
        self.assertEqual(self.gate.available_minutes(), before)

    def test_budget_011_direct_run_without_ledger_blocks_campaign(self):
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.observe_external_run(
                run_id="run-foreign",
                run_attempt=1,
                repository=REPO,
                workflow=WORKFLOW,
                expected_sha=SHA,
            )
        self.assertEqual(caught.exception.code, "untracked_run")
        self.assertTrue(self.gate.blocked())
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d-011"))
        self.assertEqual(caught.exception.code, "campaign_blocked")

    def test_budget_012_rerun_without_new_reservation_blocks_campaign(self):
        request, _ = self._run("d-012")
        self._accept(request, "run-012")
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.observe_external_run(
                run_id="run-012",
                run_attempt=2,
                repository=REPO,
                workflow=WORKFLOW,
                expected_sha=SHA,
            )
        self.assertEqual(caught.exception.code, "rerun_without_reservation")
        self.assertTrue(self.gate.blocked())

    def test_budget_013_multiple_jobs_sum_once(self):
        request, _ = self._run("d-013", job_count=2, per_job_minutes=4)
        self.assertEqual(request.job_count, 2)
        self.assertEqual(self.gate.snapshot("d-013").reserved_minutes, 9)
        self._accept(request, "run-013")
        self.gate.reconcile(
            "d-013",
            [
                {"job_id": "job-a", "minutes": 4, "conclusion": "success"},
                {"job_id": "job-b", "minutes": 3, "conclusion": "timed_out"},
            ],
            run_id="run-013",
            run_attempt=1,
        )
        self.assertEqual(self.gate.confirmed_minutes("d-013"), 7)
        self.assertEqual(self.gate.available_minutes(), 53)

    def test_budget_014_exhaustion_denies_the_next_reserve(self):
        for index in range(5):
            request, _ = self._run(f"d-014-{index}")
            self._accept(request, f"run-014-{index}")
            self.gate.reconcile(
                request.dispatch_id,
                [{"job_id": "job", "minutes": 11, "conclusion": "success"}],
                run_id=f"run-014-{index}",
                run_attempt=1,
            )
        request, outcome = self._run("d-014-tail", per_job_minutes=4)
        self.assertEqual(outcome.reserved_minutes, 5)
        self._accept(request, "run-014-tail")
        self.gate.reconcile(
            "d-014-tail",
            [{"job_id": "job", "minutes": 5, "conclusion": "success"}],
            run_id="run-014-tail",
            run_attempt=1,
        )
        self.assertEqual(self.gate.available_minutes(), 0)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d-014-over"))
        self.assertEqual(caught.exception.code, "insufficient_minutes")

    def test_budget_015_corrupt_or_unreadable_ledger_denies(self):
        broken = Path(self._tmp.name) / "broken.sqlite"
        broken.write_bytes(b"this is not a database")
        with self.assertRaises(BudgetDeny) as caught:
            BudgetGate.open(broken)
        self.assertEqual(caught.exception.code, "corrupted_ledger")
        self.assertTrue(broken.exists())
        with self.assertRaises(BudgetDeny) as caught:
            BudgetGate.open(broken)
        self.assertEqual(caught.exception.code, "corrupted_ledger")

    def test_budget_016_emergency_stop_during_reservation_denies(self):
        self.gate.set_emergency_stop("unknown")
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d-016-unknown"))
        self.assertEqual(caught.exception.code, "stop_unknown")
        self.gate.set_emergency_stop("on")
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d-016-on"))
        self.assertEqual(caught.exception.code, "emergency_stop")
        self.gate.set_emergency_stop("off")
        started = threading.Event()

        def publish_stop():
            holder = sqlite3.connect(self.path, timeout=5.0, isolation_level=None)
            holder.execute("BEGIN IMMEDIATE")
            started.set()
            time.sleep(0.3)
            holder.execute("UPDATE campaign SET stop_state = 'on'")
            holder.execute("COMMIT")
            holder.close()

        updater = threading.Thread(target=publish_stop)
        updater.start()
        self.assertTrue(started.wait(5))
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d-016-race"))
        updater.join(5)
        self.assertEqual(caught.exception.code, "emergency_stop")
        self.gate.set_emergency_stop("off")
        request, _ = self._run("d-016-cancel")
        self.gate.set_emergency_stop("on")
        adapter = FakeDispatchAdapter(AdapterResult("accepted", "run-should-not", 1, SHA))
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.dispatch(request, adapter)
        self.assertEqual(caught.exception.code, "emergency_stop")
        self.assertEqual(adapter.calls, 0)
        self.assertEqual(self.gate.snapshot("d-016-cancel").status, "CANCELLED_BEFORE_DISPATCH")
        self.assertEqual(self.gate.available_minutes(), 60)

    def test_budget_017_lost_dispatch_response_stays_unknown(self):
        request, _ = self._run("d-017")
        adapter = FakeDispatchAdapter(AdapterResult("timeout"))
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.dispatch(request, adapter)
        self.assertEqual(caught.exception.code, "dispatch_unknown")
        self.assertEqual(adapter.calls, 1)
        self.assertEqual(self.gate.snapshot("d-017").status, "DISPATCH_UNKNOWN")
        self.assertIsNone(self.gate.snapshot("d-017").run_id)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d-017-next"))
        self.assertEqual(caught.exception.code, "unknown_usage")
        lost_path = Path(self._tmp.name) / "lost.sqlite"
        initialize_campaign(lost_path, campaign_id="camp-lost", repository=REPO, workflow=WORKFLOW, expected_sha=SHA)
        lost_gate = BudgetGate.open(lost_path)
        try:
            lost_gate.set_emergency_stop("off")
            lost_request = _request("d-017b")
            lost_gate.reserve(lost_request)
            with self.assertRaises(BudgetDeny):
                lost_gate.dispatch(lost_request, FakeDispatchAdapter(ResponseLost()))
            self.assertEqual(lost_gate.snapshot("d-017b").status, "DISPATCH_UNKNOWN")
        finally:
            lost_gate.close()

    def test_budget_018_crash_after_dispatch_keeps_reserve_without_run_id(self):
        request, _ = self._run("d-018")
        adapter = FakeDispatchAdapter(AdapterCrashed())
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.dispatch(request, adapter)
        self.assertEqual(caught.exception.code, "dispatch_unknown")
        self.assertEqual(adapter.calls, 1)
        self.assertIsNone(self.gate.snapshot("d-018").run_id)
        self.assertEqual(self.gate.available_minutes(), 49)
        other = Path(self._tmp.name) / "crash-before-run.sqlite"
        initialize_campaign(other, campaign_id="camp-2", repository=REPO, workflow=WORKFLOW, expected_sha=SHA)
        crashed = BudgetGate.open(other)
        crashed.set_emergency_stop("off")
        pending = _request("d-018-intent")
        crashed.reserve(pending)
        crashed._mark_intent_or_cancel(pending)
        crashed.close()
        restored = BudgetGate.open(other)
        try:
            self.assertEqual(restored.snapshot("d-018-intent").status, "DISPATCH_UNKNOWN")
            with self.assertRaises(BudgetDeny) as caught:
                restored.reserve(_request("d-018-again"))
            self.assertEqual(caught.exception.code, "unknown_usage")
        finally:
            restored.close()

    def test_budget_019_overrun_records_actual_and_blocks(self):
        request, _ = self._run("d-019")
        self._accept(request, "run-019")
        self.gate.reconcile(
            "d-019",
            [{"job_id": "job", "minutes": 15, "conclusion": "success"}],
            run_id="run-019",
            run_attempt=1,
        )
        self.assertEqual(self.gate.confirmed_minutes("d-019"), 15)
        self.assertTrue(self.gate.blocked())
        self.assertEqual(self.gate.available_minutes(), 45)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d-019-next"))
        self.assertEqual(caught.exception.code, "campaign_blocked")

    def test_budget_020_repeat_reconcile_does_not_double_charge(self):
        request, _ = self._run("d-020")
        self._accept(request, "run-020")
        jobs = [{"job_id": "job", "minutes": 8, "conclusion": "failure"}]
        self.gate.reconcile("d-020", jobs, run_id="run-020", run_attempt=1)
        self.gate.reconcile("d-020", jobs, run_id="run-020", run_attempt=1)
        self.assertEqual(self.gate.confirmed_minutes("d-020"), 8)
        self.assertEqual(self.gate.available_minutes(), 52)

    def test_budget_021_missing_ledger_is_not_recreated(self):
        self.gate.close()
        self.path.unlink()
        for suffix in ("-wal", "-shm"):
            sidecar = Path(str(self.path) + suffix)
            if sidecar.exists():
                sidecar.unlink()
        with self.assertRaises(BudgetDeny) as caught:
            BudgetGate.open(self.path)
        self.assertEqual(caught.exception.code, "missing_ledger")
        self.assertFalse(self.path.exists())
        self.gate = BudgetGate.open(self.path) if False else _NullGate()

    def test_budget_022_separate_processes_reserve_once(self):
        context = multiprocessing.get_context("spawn")
        queue = context.Queue()
        first = context.Process(target=_cross_process_reserve, args=(str(self.path), "d-022-a", queue))
        second = context.Process(target=_cross_process_reserve, args=(str(self.path), "d-022-b", queue))
        first.start()
        second.start()
        first.join(30)
        second.join(30)
        self.assertEqual(first.exitcode, 0)
        self.assertEqual(second.exitcode, 0)
        results = sorted([queue.get(timeout=5), queue.get(timeout=5)])
        self.assertEqual(results, ["deny:concurrent_run", "ok"])

    def test_fake_adapter_reject_malformed_duplicate_and_sha(self):
        request, _ = self._run("d-reject")
        rejected = FakeDispatchAdapter(AdapterResult("rejected"))
        outcome = self.gate.dispatch(request, rejected)
        self.assertEqual(outcome.status, "REJECTED")
        self.assertEqual(rejected.calls, 1)
        self.assertEqual(self.gate.available_minutes(), 60)
        request, _ = self._run("d-malformed")
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.dispatch(request, FakeDispatchAdapter(RuntimeError("bad payload")))
        self.assertEqual(caught.exception.code, "dispatch_unknown")
        self.gate.close()
        fresh_path = Path(self._tmp.name) / "replay.sqlite"
        initialize_campaign(fresh_path, campaign_id="camp-replay", repository=REPO, workflow=WORKFLOW, expected_sha=SHA)
        replay_gate = BudgetGate.open(fresh_path)
        self.gate = replay_gate
        replay_gate.set_emergency_stop("off")
        request, _ = self._run("d-replay")
        self._accept(request, "run-replay")
        replay_gate.replay_adapter_result("d-replay", AdapterResult("accepted", "run-replay", 1, SHA))
        self.assertEqual(replay_gate.snapshot("d-replay").status, "RUNNING")
        with self.assertRaises(BudgetDeny) as caught:
            replay_gate.replay_adapter_result("d-replay", AdapterResult("accepted", "run-other", 1, SHA))
        self.assertEqual(caught.exception.code, "unexpected_run")

    def test_network_isolation_and_paid_ai_deny(self):
        source = (ROOT / "src" / "bintrans_monitor" / "budget_gate.py").read_text(encoding="utf-8")
        self.assertNotIn("urlopen", source)
        self.assertNotIn("requests.", source)
        self.assertNotIn("import socket", source)
        calls = {"n": 0}
        original = socket.create_connection

        def guarded(*args, **kwargs):
            calls["n"] += 1
            raise AssertionError("network")

        socket.create_connection = guarded
        try:
            request, _ = self._run("d-net")
            self._accept(request, "run-net")
        finally:
            socket.create_connection = original
        self.assertEqual(calls["n"], 0)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d-ai", paid_ai_calls=1))
        self.assertEqual(caught.exception.code, "paid_ai_denied")


class _NullGate:
    def close(self):
        return None


if __name__ == "__main__":
    unittest.main()
