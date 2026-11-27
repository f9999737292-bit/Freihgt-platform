"""LEDGER-001..007, STOP-011..013, and ADAPTER-001..016. Offline transport only."""

from __future__ import annotations

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
    AdapterResult,
    BudgetDeny,
    BudgetGate,
    DispatchRequest,
    FakeDispatchAdapter,
    backup_ledger,
    guarded_ledger_path,
    initialize_campaign,
    open_expected,
    restore_ledger,
)
from bintrans_monitor.dispatch_adapter import (  # noqa: E402
    CredentialProvider,
    GitHubDispatchAdapter,
    GitHubReadAdapter,
    OfflineTransport,
    RealGitHubTransport,
    TransportResponse,
    TrustedRefBinding,
    correlate_run,
)

SHA = "ab" * 20
OTHER_SHA = "cd" * 20
REPO = "example/budget-lab"
WORKFLOW = "monitor-budget"
REF = "refs/heads/main"
TOKEN = "ghp_" + "ExampleToken123"
REAL_GITHUB_API_CALLS = 0


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


def _job(minutes: int = 10) -> list[dict[str, object]]:
    return [{"job_id": "job", "minutes": minutes, "conclusion": "success"}]


class LedgerAndAdapterTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.root = Path(self._tmp.name) / "ledger-root"
        self.root.mkdir()
        self.path = self.root / "ledger.sqlite"
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
        try:
            self.gate.close()
        except sqlite3.ProgrammingError:
            pass
        self._tmp.cleanup()

    def _reconcile(self, dispatch_id: str, run_id: str) -> None:
        request = _request(dispatch_id)
        self.gate.reserve(request)
        self.gate.dispatch(request, FakeDispatchAdapter(AdapterResult("accepted", run_id, 1, SHA)))
        self.gate.reconcile(dispatch_id, _job(), run_id=run_id, run_attempt=1)

    def test_ledger_001_backup_keeps_wal_rows_naive_copy_does_not(self):
        """J007: copying only the database file while the WAL is dirty rewinds spend."""
        self.gate._conn.execute("PRAGMA wal_autocheckpoint=0")
        self._reconcile("d1", "run-1")
        wal = Path(str(self.path) + "-wal")
        self.assertTrue(wal.exists())
        self.assertGreater(wal.stat().st_size, 0)
        naive = self.root / "naive.sqlite"
        naive.write_bytes(self.path.read_bytes())
        second = BudgetGate.open(self.path)
        self.assertEqual(second.recovery_state(), "ACTIVE")
        second.close()
        backup = self.root / "safe.sqlite"
        backup_ledger(self.path, backup, self.root)
        self.assertTrue(backup.is_file())
        self.assertFalse(Path(str(backup) + "-wal").exists())
        safe = sqlite3.connect(backup)
        safe_ids = {row[0] for row in safe.execute("SELECT dispatch_id FROM dispatches")}
        safe_check = safe.execute("PRAGMA integrity_check").fetchone()[0]
        safe.close()
        try:
            copied = sqlite3.connect(naive)
            copied_ids = {row[0] for row in copied.execute("SELECT dispatch_id FROM dispatches")}
            copied.close()
        except sqlite3.DatabaseError:
            copied_ids = set()
        self.assertEqual(safe_check, "ok")
        self.assertIn("d1", safe_ids)
        self.assertNotIn("d1", copied_ids)

    def test_ledger_002_old_snapshot_stays_blocked_until_external_reconciliation(self):
        self._reconcile("d1", "run-1")
        after_one = self.gate.available_minutes()
        self.assertLess(after_one, 60)
        backup = self.root / "old.sqlite"
        self.gate.close()
        backup_ledger(self.path, backup, self.root)
        self.gate = BudgetGate.open(self.path)
        self.gate.set_emergency_stop("off")
        self._reconcile("d2", "run-2")
        after_two = self.gate.available_minutes()
        self.assertLess(after_two, after_one)
        self.gate.close()
        restore_ledger(backup, self.path, self.root, expected_campaign_id="camp-1")
        self.gate = BudgetGate.open(self.path)
        self.assertEqual(self.gate.recovery_state(), "RECOVERY_REQUIRED")
        self.assertEqual(self.gate.available_minutes(), after_one)
        self.assertNotEqual(self.gate.available_minutes(), 60)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("d3"))
        self.assertEqual(caught.exception.code, "recovery_required")
        with self.assertRaises(BudgetDeny):
            self.gate.release_recovery(controller_go=True, external_state_known=False)
        self.gate.record_external_consumption(
            dispatch_id="d2",
            idempotency_key="d2-key",
            run_id="run-2",
            run_attempt=1,
            jobs=_job(),
        )
        recorded = self.gate.available_minutes()
        self.assertEqual(recorded, after_two)
        with self.assertRaises(BudgetDeny):
            self.gate.release_recovery(controller_go=True, external_state_known=True)
        self.gate.attest_external_reconciliation()
        self.gate.release_recovery(controller_go=True, external_state_known=True)
        self.assertEqual(self.gate.available_minutes(), recorded)
        self.assertEqual(self.gate.recovery_state(), "ACTIVE")
        self.gate.set_emergency_stop("off")
        self.gate.reserve(_request("d3"))

    def test_ledger_003_restore_without_external_state_denies(self):
        self._reconcile("d1", "run-1")
        known = self.gate.confirmed_minutes("d1")
        backup = self.root / "old.sqlite"
        self.gate.close()
        backup_ledger(self.path, backup, self.root)
        restore_ledger(backup, self.path, self.root, expected_campaign_id="camp-1")
        self.gate = BudgetGate.open(self.path)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.release_recovery(controller_go=True, external_state_known=False)
        self.assertEqual(caught.exception.code, "recovery_required")
        with self.assertRaises(BudgetDeny):
            self.gate.release_recovery(controller_go=False, external_state_known=True)
        self.assertEqual(self.gate.recovery_state(), "RECOVERY_REQUIRED")
        self.assertEqual(self.gate.confirmed_minutes("d1"), known)
        self.assertLess(self.gate.available_minutes(), 60)

    def test_ledger_004_campaign_identity_mismatch_denies(self):
        other = self.root / "other.sqlite"
        initialize_campaign(other, campaign_id="camp-2", repository=REPO, workflow=WORKFLOW, expected_sha=SHA)
        with self.assertRaises(BudgetDeny) as caught:
            open_expected(self.path, self.root, "camp-2")
        self.assertEqual(caught.exception.code, "campaign_identity_mismatch")
        backup = self.root / "other-backup.sqlite"
        backup_ledger(other, backup, self.root)
        before = self.gate.available_minutes()
        self.gate.close()
        with self.assertRaises(BudgetDeny) as caught:
            restore_ledger(backup, self.path, self.root, expected_campaign_id="camp-1")
        self.assertEqual(caught.exception.code, "campaign_identity_mismatch")
        self.gate = BudgetGate.open(self.path)
        self.assertEqual(self.gate._campaign_read()["campaign_id"], "camp-1")
        self.assertEqual(self.gate.available_minutes(), before)

    def test_ledger_005_corrupt_backup_does_not_replace_the_live_ledger(self):
        self._reconcile("d1", "run-1")
        before = self.gate.available_minutes()
        corrupt = self.root / "broken.sqlite"
        corrupt.write_bytes(b"not a database")
        self.gate.close()
        with self.assertRaises(BudgetDeny) as caught:
            restore_ledger(corrupt, self.path, self.root, expected_campaign_id="camp-1")
        self.assertEqual(caught.exception.code, "corrupted_ledger")
        self.gate = BudgetGate.open(self.path)
        self.assertEqual(self.gate.available_minutes(), before)
        self.assertEqual(self.gate.confirmed_minutes("d1"), 10)

    def test_ledger_006_unsafe_path_is_rejected(self):
        not_dir = self.root / "not-a-directory"
        not_dir.write_text("x", encoding="utf-8")
        with self.assertRaises(BudgetDeny) as caught:
            guarded_ledger_path(not_dir / "ledger.sqlite", not_dir)
        self.assertEqual(caught.exception.code, "unsafe_ledger_path")
        with self.assertRaises(BudgetDeny):
            guarded_ledger_path(self.root.parent / "outside.sqlite", self.root)
        link = self.root / "linked.sqlite"
        try:
            link.symlink_to(self.path)
        except OSError:
            return
        with self.assertRaises(BudgetDeny) as caught:
            guarded_ledger_path(link, self.root)
        self.assertEqual(caught.exception.code, "unsafe_ledger_path")

    def test_ledger_007_concurrent_backup_stays_consistent(self):
        self._reconcile("d1", "run-1")
        self.gate.close()
        errors: list[BaseException] = []

        def write_for_a_moment() -> None:
            conn = sqlite3.connect(self.path, timeout=5, isolation_level=None)
            try:
                conn.execute("BEGIN IMMEDIATE")
                conn.execute("UPDATE campaign SET block_reason = ''")
                time.sleep(0.3)
                conn.execute("COMMIT")
            except BaseException as exc:  # pragma: no cover - reported below
                errors.append(exc)
            finally:
                conn.close()

        destinations = [self.root / "one.sqlite", self.root / "two.sqlite"]

        def copy_to(destination: Path) -> None:
            try:
                backup_ledger(self.path, destination, self.root)
            except BaseException as exc:  # pragma: no cover - reported below
                errors.append(exc)

        threads = [threading.Thread(target=write_for_a_moment)]
        threads.extend(threading.Thread(target=copy_to, args=(destination,)) for destination in destinations)
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(timeout=10)
        self.assertEqual(errors, [])
        for destination in destinations:
            conn = sqlite3.connect(destination)
            self.assertEqual(conn.execute("PRAGMA integrity_check").fetchone()[0], "ok")
            self.assertEqual(conn.execute("SELECT dispatch_id FROM dispatches").fetchone()[0], "d1")
            conn.close()
        self.gate = BudgetGate.open(self.path)

    def test_stop_011_stop_after_intent_cancels_before_the_adapter(self):
        request = _request("stop-11")
        self.gate.reserve(request)
        adapter = FakeDispatchAdapter(AdapterResult("accepted", "run-11", 1, SHA))

        def arm() -> None:
            self.gate.set_emergency_stop("on")

        with self.assertRaises(BudgetDeny) as caught:
            self.gate.dispatch(request, adapter, before_adapter=arm)
        self.assertEqual(caught.exception.code, "emergency_stop")
        self.assertEqual(adapter.calls, 0)
        self.assertEqual(self.gate.snapshot("stop-11").status, "CANCELLED_BEFORE_DISPATCH")
        self.assertEqual(self.gate.available_minutes(), 60)

    def test_stop_012_unknown_stop_denies_and_survives_reopen(self):
        request = _request("stop-12")
        self.gate.reserve(request)
        adapter = FakeDispatchAdapter(AdapterResult("accepted", "run-12", 1, SHA))

        def arm() -> None:
            self.gate.set_emergency_stop("unknown")

        with self.assertRaises(BudgetDeny) as caught:
            self.gate.dispatch(request, adapter, before_adapter=arm)
        self.assertEqual(caught.exception.code, "stop_unknown")
        self.assertEqual(adapter.calls, 0)
        self.gate.close()
        self.gate = BudgetGate.open(self.path)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("stop-12b"))
        self.assertEqual(caught.exception.code, "stop_unknown")

    def test_stop_013_stop_after_a_possible_send_keeps_the_reserve(self):
        request = _request("stop-13")
        self.gate.reserve(request)

        class StopDuringSend(FakeDispatchAdapter):
            def dispatch(self, request: DispatchRequest, binding: object | None = None) -> AdapterResult:
                self.gate.set_emergency_stop("on")
                return super().dispatch(request, binding)

        adapter = StopDuringSend(AdapterResult("accepted", "run-13", 1, SHA))
        adapter.gate = self.gate
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.dispatch(request, adapter)
        self.assertEqual(caught.exception.code, "dispatch_unknown")
        self.assertEqual(adapter.calls, 1)
        self.assertEqual(self.gate.snapshot("stop-13").status, "DISPATCH_UNKNOWN")
        self.assertLess(self.gate.available_minutes(), 60)
        self.assertTrue(self.gate.blocked())
        self.gate.set_emergency_stop("off")
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("stop-13b"))
        self.assertEqual(caught.exception.code, "campaign_blocked")

    def _adapter(self, handler) -> tuple[GitHubDispatchAdapter, OfflineTransport]:
        transport = OfflineTransport(handler)
        adapter = GitHubDispatchAdapter(
            transport,
            repository=REPO,
            workflow=WORKFLOW,
            allowed_ref=REF,
            expected_sha=SHA,
        )
        return adapter, transport

    def _binding(self, **overrides: object) -> TrustedRefBinding:
        values = {"ref": REF, "object_sha": SHA, "repository": REPO, "workflow": WORKFLOW}
        values.update(overrides)
        return TrustedRefBinding(**values)  # type: ignore[arg-type]

    def test_adapter_001_allowlisted_repository_posts_the_branch_and_the_sha_input(self):
        seen: dict[str, object] = {}

        def handler(method: str, path: str, body):
            if method == "GET":
                return TransportResponse(200, {"sha": SHA})
            seen["body"] = body
            seen["path"] = path
            return TransportResponse(204, None)

        adapter, transport = self._adapter(handler)
        result = adapter.dispatch(_request("a1"), self._binding())
        self.assertEqual(result.kind, "unconfirmed")
        self.assertEqual(seen["path"], f"/repos/{REPO}/actions/workflows/{WORKFLOW}/dispatches")
        self.assertEqual(seen["body"]["ref"], "main")
        self.assertEqual(
            seen["body"]["inputs"],
            {"budget_dispatch_id": "a1", "campaign_id": "camp-1", "expected_head_sha": SHA},
        )
        self.assertEqual(sum(1 for method, _path in transport.calls if method == "POST"), 1)

    def test_adapter_002_forbidden_repository_is_not_sent(self):
        adapter, transport = self._adapter(lambda method, path, body: TransportResponse(204, None))
        with self.assertRaises(BudgetDeny) as caught:
            adapter.dispatch(_request("a2", repository="other/repo"), self._binding())
        self.assertEqual(caught.exception.code, "repository_rejected")
        self.assertEqual(transport.calls, [])

    def test_adapter_003_wrong_workflow_is_not_sent(self):
        adapter, transport = self._adapter(lambda method, path, body: TransportResponse(204, None))
        with self.assertRaises(BudgetDeny) as caught:
            adapter.dispatch(_request("a3", workflow="other-workflow"), self._binding())
        self.assertEqual(caught.exception.code, "workflow_rejected")
        self.assertEqual(transport.calls, [])

    def test_adapter_004_unconfirmed_ref_or_sha_is_not_posted(self):
        adapter, transport = self._adapter(lambda method, path, body: TransportResponse(200, {"sha": OTHER_SHA}))
        with self.assertRaises(BudgetDeny) as caught:
            adapter.dispatch(_request("a4"), self._binding(ref="refs/heads/topic"))
        self.assertEqual(caught.exception.code, "sha_unbound")
        self.assertEqual(transport.calls, [])
        with self.assertRaises(BudgetDeny):
            adapter.dispatch(_request("a4b"), self._binding())
        self.assertEqual([method for method, _path in transport.calls], ["GET"])

    def test_adapter_005_empty_success_does_not_confirm_a_run(self):
        def identified(method: str, path: str, body):
            if method == "GET":
                return TransportResponse(200, {"sha": SHA})
            return TransportResponse(200, {"workflow_run_id": 99})

        identified_adapter, _identified_transport = self._adapter(identified)
        identified_result = identified_adapter.dispatch(_request("a5-id-only"), self._binding())
        self.assertEqual(identified_result.kind, "unconfirmed")
        self.assertIsNone(identified_result.run_id)

        def handler(method: str, path: str, body):
            if method == "GET":
                return TransportResponse(200, {"sha": SHA})
            return TransportResponse(204, None)

        adapter, _transport = self._adapter(handler)
        request = _request("a5")
        self.gate.reserve(request)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.dispatch(request, adapter, binding=self._binding())
        self.assertEqual(caught.exception.code, "dispatch_unknown")
        self.assertEqual(self.gate.snapshot("a5").status, "DISPATCH_UNKNOWN")
        self.assertIsNone(self.gate.snapshot("a5").run_id)
        self.assertLess(self.gate.available_minutes(), 60)

    def test_adapter_006_timeout_after_a_possible_post_stays_unknown(self):
        def handler(method: str, path: str, body):
            if method == "GET":
                return TransportResponse(200, {"sha": SHA})
            raise TimeoutError("dispatch timed out")

        adapter, transport = self._adapter(handler)
        request = _request("a6")
        self.gate.reserve(request)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.dispatch(request, adapter, binding=self._binding())
        self.assertNotIn(TOKEN, str(caught.exception))
        self.assertEqual(self.gate.snapshot("a6").status, "DISPATCH_UNKNOWN")
        self.assertEqual(sum(1 for method, _path in transport.calls if method == "POST"), 1)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reserve(_request("a6b"))
        self.assertEqual(caught.exception.code, "unknown_usage")

    def test_adapter_007_ambiguous_runs_are_not_chosen_by_time_or_position(self):
        runs = [
            {"id": "newest", "created_at": "2026-11-27T00:03:00Z", "head_sha": SHA, "workflow": WORKFLOW},
            {"id": "older", "created_at": "2026-11-27T00:01:00Z", "head_sha": SHA, "workflow": WORKFLOW},
        ]
        with self.assertRaises(BudgetDeny) as caught:
            correlate_run(runs, dispatch_id="a7", expected_sha=SHA, workflow=WORKFLOW)
        self.assertEqual(caught.exception.code, "dispatch_unknown")
        runs.append({"id": "bound", "created_at": "2026-11-27T00:00:00Z", "head_sha": SHA, "workflow": WORKFLOW, "budget_dispatch_id": "a7"})
        self.assertEqual(correlate_run(runs, dispatch_id="a7", expected_sha=SHA, workflow=WORKFLOW), "bound")
        runs.append({"id": "second", "created_at": "2026-11-27T00:04:00Z", "head_sha": SHA, "workflow": WORKFLOW, "budget_dispatch_id": "a7"})
        with self.assertRaises(BudgetDeny):
            correlate_run(runs, dispatch_id="a7", expected_sha=SHA, workflow=WORKFLOW)

    def test_adapter_008_dispatch_post_is_not_retried(self):
        def handler(method: str, path: str, body):
            if method == "GET":
                return TransportResponse(200, {"sha": SHA})
            return TransportResponse(204, None)

        adapter, transport = self._adapter(handler)
        adapter.dispatch(_request("a8"), self._binding())
        with self.assertRaises(BudgetDeny) as caught:
            adapter.dispatch(_request("a8"), self._binding())
        self.assertEqual(caught.exception.code, "automatic_retry_denied")
        self.assertEqual(sum(1 for method, _path in transport.calls if method == "POST"), 1)

    def test_adapter_009_wrong_run_attempt_is_charged_and_blocks(self):
        request = _request("a9")
        self.gate.reserve(request)
        self.gate.dispatch(request, FakeDispatchAdapter(AdapterResult("accepted", "run-9", 1, SHA)))
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.reconcile("a9", _job(4), run_id="run-9", run_attempt=2)
        self.assertEqual(caught.exception.code, "rerun_without_reservation")
        self.assertEqual(self.gate.confirmed_minutes("a9"), 4)
        self.assertTrue(self.gate.blocked())
        with self.assertRaises(BudgetDeny):
            self.gate.reserve(_request("a9b"))

    def test_adapter_010_incomplete_job_pages_are_not_complete(self):
        def handler(method: str, path: str, body):
            del method, body
            if path.endswith("page=1"):
                jobs = [{"id": index} for index in range(100)]
                return TransportResponse(200, {"jobs": jobs, "total_count": 150})
            return TransportResponse(200, {"jobs": [], "total_count": 150})

        transport = OfflineTransport(handler)
        reader = GitHubReadAdapter(transport, repository=REPO, workflow=WORKFLOW, expected_sha=SHA, allowed_ref=REF)
        with self.assertRaises(BudgetDeny) as caught:
            reader.collect_jobs("run-10")
        self.assertEqual(caught.exception.code, "usage_incomplete")
        self.assertEqual(len(transport.calls), 2)

    def test_adapter_011_forbidden_endpoint_is_not_called(self):
        adapter, transport = self._adapter(lambda method, path, body: TransportResponse(204, None))
        with self.assertRaises(BudgetDeny) as caught:
            adapter.perform("POST", "/repos/other/repo/actions/workflows/x/dispatches")
        self.assertEqual(caught.exception.code, "endpoint_rejected")
        with self.assertRaises(BudgetDeny):
            adapter.perform("GET", "https://evil.example/repos/example/budget-lab/actions/runs/1")
        self.assertEqual(transport.calls, [])

    def test_adapter_012_redirect_is_not_followed(self):
        def handler(method: str, path: str, body):
            if method == "GET" and "/git/commits/" in path:
                return TransportResponse(200, {"sha": SHA})
            return TransportResponse(302, None, redirected=True, location="https://evil.example/collect")

        adapter, transport = self._adapter(handler)
        request = _request("a12")
        self.gate.reserve(request)
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.dispatch(request, adapter, binding=self._binding())
        self.assertEqual(caught.exception.code, "unsafe_redirect")
        self.assertEqual(self.gate.snapshot("a12").status, "DISPATCH_UNKNOWN")
        self.assertFalse(any("evil.example" in path for _method, path in transport.calls))
        self.assertEqual(sum(1 for method, _path in transport.calls if method == "POST"), 1)

    def test_adapter_013_transport_errors_do_not_leak_credentials(self):
        def handler(method: str, path: str, body):
            if method == "GET":
                return TransportResponse(200, {"sha": SHA})
            raise RuntimeError("authorization failed " + TOKEN)

        adapter, _transport = self._adapter(handler)
        with self.assertRaises(BudgetDeny) as caught:
            adapter.dispatch(_request("a13"), self._binding())
        self.assertEqual(caught.exception.code, "transport_error")
        self.assertNotIn(TOKEN, str(caught.exception))
        self.assertNotIn("ghp_", repr(caught.exception))
        self.assertIsNone(caught.exception.__cause__)
        provider = CredentialProvider()
        self.assertEqual(provider.monitor_token(), "READ_ONLY")
        self.assertEqual(provider.dispatch_principal(), "SEPARATE")
        self.assertEqual(provider.cancel_principal(), "SEPARATE_FROM_MONITOR")
        with self.assertRaises(BudgetDeny) as caught:
            provider.live_secret()
        self.assertEqual(caught.exception.code, "live_credentials_absent")

    def test_adapter_014_untrusted_or_estimated_usage_does_not_release_the_reserve(self):
        request = _request("a14")
        self.gate.reserve(request)
        self.gate.dispatch(request, FakeDispatchAdapter(AdapterResult("accepted", "run-14", 1, SHA)))
        held = self.gate.available_minutes()
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.apply_trusted_usage(
                "a14",
                {"source": "user", "minutes": 0, "jobs": _job(1), "run_id": "run-14", "run_attempt": 1},
            )
        self.assertEqual(caught.exception.code, "usage_untrusted")
        self.assertIsNone(self.gate.confirmed_minutes("a14"))
        self.assertEqual(self.gate.available_minutes(), held)
        with self.assertRaises(BudgetDeny):
            self.gate.apply_trusted_usage(
                "a14",
                {
                    "source": "actions_billing",
                    "billing_authoritative": True,
                    "minutes": 1,
                    "terminal": True,
                    "jobs_complete": True,
                    "pages_complete": True,
                    "estimated_only": False,
                    "repository": REPO,
                    "workflow": WORKFLOW,
                    "expected_sha": SHA,
                    "run_id": "run-14",
                    "run_attempt": 1,
                    "jobs": _job(1),
                },
            )
        self.assertIsNone(self.gate.confirmed_minutes("a14"))

    def test_adapter_015_external_run_without_a_reservation_blocks(self):
        with self.assertRaises(BudgetDeny) as caught:
            self.gate.observe_external_run(
                run_id="run-outside",
                run_attempt=1,
                repository=REPO,
                workflow=WORKFLOW,
                expected_sha=SHA,
            )
        self.assertEqual(caught.exception.code, "untracked_run")
        self.assertTrue(self.gate.blocked())
        with self.assertRaises(BudgetDeny):
            self.gate.reserve(_request("a15"))

    def test_adapter_016_real_transport_is_disabled_and_makes_no_connection(self):
        global REAL_GITHUB_API_CALLS
        calls: list[object] = []
        original = socket.create_connection

        def guarded(*args, **kwargs):
            calls.append(args)
            REAL_GITHUB_API_CALLS += 1
            raise AssertionError("network")

        socket.create_connection = guarded
        try:
            transport = RealGitHubTransport()
            with self.assertRaises(BudgetDeny) as caught:
                transport.exchange("POST", "/repos/example/budget-lab/actions/workflows/monitor-budget/dispatches", {})
            self.assertEqual(caught.exception.code, "real_transport_disabled")
            adapter = GitHubDispatchAdapter(
                transport,
                repository=REPO,
                workflow=WORKFLOW,
                allowed_ref=REF,
                expected_sha=SHA,
            )
            request = _request("a16")
            self.gate.reserve(request)
            with self.assertRaises(BudgetDeny) as caught:
                self.gate.dispatch(request, adapter, binding=self._binding())
            self.assertEqual(caught.exception.code, "real_transport_disabled")
            self.assertEqual(self.gate.snapshot("a16").status, "CANCELLED_BEFORE_DISPATCH")
        finally:
            socket.create_connection = original
        self.assertEqual(calls, [])
        self.assertEqual(REAL_GITHUB_API_CALLS, 0)
        blob = "\n".join(path.read_text(encoding="utf-8") for path in (ROOT / "src" / "bintrans_monitor").glob("*.py"))
        self.assertNotIn("urlopen", blob)
        self.assertNotIn("requests.", blob)
        self.assertNotIn("import socket", blob)
        self.assertNotIn("import subprocess", blob)
        self.assertNotIn("--method POST", blob)


if __name__ == "__main__":
    unittest.main()
