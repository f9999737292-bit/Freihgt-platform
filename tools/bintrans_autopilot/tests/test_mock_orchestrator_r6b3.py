"""Isolated mock-only orchestration. No Cloud Agent and no shared-environment migration."""

from __future__ import annotations

import os
import threading
import unittest
from pathlib import Path

from bintrans_autopilot.budget_controller import (
    COMMITTED,
    OVERRUN,
    RESERVED,
    BudgetController,
    BudgetControllerError,
)
from bintrans_autopilot.mock_orchestrator import AuditJournal, MockOrchestrator, MockTask, OrchestratorError
from bintrans_autopilot.postgres_queue import MigrationRunner, PostgresQueue, QueueError
from bintrans_autopilot.tests.test_budget_controller_03c2 import (
    _AGENT,
    _RUN,
    _TENANTS,
    _clear_budget_fixtures,
    _enabled_config,
    _ensure_reconciler_role,
    _ensure_tenant_roles,
    _owner_connect,
    _reconciler_connect,
    _reseed_tenant_ledgers,
    _tenant_connect,
)
from bintrans_autopilot.tests.test_launch_lifecycle_r6b import _ADAPTER_LOGIN, _RECOVERY_LOGIN, _connect, _ensure_launch_roles


def _dsn() -> str:
    return os.environ.get("BINTRANS_AUTOPILOT_QUEUE_DSN", "").strip()


class MockOrchestratorR6B3Tests(unittest.TestCase):
    dsn = ""

    @classmethod
    def setUpClass(cls) -> None:
        cls.dsn = _dsn()
        if not cls.dsn:
            raise unittest.SkipTest("BINTRANS_AUTOPILOT_QUEUE_DSN is required")
        cls._prior_autopilot_env = os.environ.get("AUTOPILOT_ENABLED")
        os.environ["AUTOPILOT_ENABLED"] = "true"
        cls.runner = MigrationRunner(lambda: _owner_connect(cls.dsn))
        if cls.runner.up() < 15 and cls.runner.version() < 15:
            raise unittest.SkipTest("autopilot queue migration 0015 is required")
        _clear_budget_fixtures(cls.dsn)
        _ensure_tenant_roles(cls.dsn, cls.runner)
        _ensure_reconciler_role(cls.dsn)
        _ensure_launch_roles(cls.dsn)
        _reseed_tenant_ledgers(cls.dsn, cls.runner, config=_enabled_config())

    @classmethod
    def tearDownClass(cls) -> None:
        if not hasattr(cls, "_prior_autopilot_env"):
            return
        if cls._prior_autopilot_env is None:
            os.environ.pop("AUTOPILOT_ENABLED", None)
        else:
            os.environ["AUTOPILOT_ENABLED"] = cls._prior_autopilot_env

    def setUp(self) -> None:
        _clear_budget_fixtures(self.dsn)
        connection = _owner_connect(self.dsn)
        try:
            connection.execute("DELETE FROM autopilot_queue.jobs")
            connection.commit()
        finally:
            connection.close()
        _reseed_tenant_ledgers(self.dsn, config=_enabled_config())
        self._set_control(mode="MOCK_ONLY", estop=False)
        self.audit_path = Path(os.environ.get("TEMP", ".")) / "r6b3-audit.jsonl"
        if self.audit_path.exists():
            self.audit_path.unlink()

    def test_normal_mock_execution_settles_once(self) -> None:
        runner = self._orchestrator()
        submitted = runner.submit(self._task("launch-normal", "idem-normal"))
        self.assertTrue(submitted.created)
        self.assertEqual(runner.run_once(), "committed")
        self.assertEqual(runner.run_once(), "idle")
        row = runner.budget.get_reservation("launch-normal")
        self.assertEqual(row.state, COMMITTED)
        self.assertEqual(row.charged_text, "4")
        self.assertEqual(self._window_committed(), 4)
        self.assertEqual(runner.cloud_agents_started, 0)
        self.assertEqual(runner.external_calls, 0)
        text = self.audit_path.read_text(encoding="utf-8")
        self.assertIn("budget_settled", text)
        self.assertNotIn("lease_token", text)

    def test_duplicate_submit_and_duplicate_settlement(self) -> None:
        runner = self._orchestrator()
        task = self._task("launch-dup", "idem-dup")
        first = runner.submit(task)
        second = runner.submit(task)
        self.assertEqual(first.job_id, second.job_id)
        self.assertFalse(second.created)
        self.assertEqual(runner.run_once(), "committed")
        again = runner.budget.commit_usage(
            "launch-dup",
            raw_cost_text="4",
            charged_text="4",
            charged_known=True,
            agent_id=_AGENT,
            run_id=_RUN,
            verification_id=self._verification("4"),
            payload_digest=self._digest("4"),
            source="mock_orchestrator",
        )
        self.assertEqual(again.state, COMMITTED)
        self.assertEqual(self._window_committed(), 4)

    def test_concurrent_orchestrators_charge_once(self) -> None:
        runner = self._orchestrator()
        other = self._orchestrator(owner="worker-b")
        runner.submit(self._task("launch-race", "idem-race"))
        outcomes: list[str] = []
        barrier = threading.Barrier(2)

        def run(active: MockOrchestrator) -> None:
            barrier.wait(timeout=5)
            outcomes.append(active.run_once())

        threads = [threading.Thread(target=run, args=(item,)) for item in (runner, other)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(10)
        self.assertCountEqual(outcomes, ["committed", "idle"])
        self.assertEqual(self._window_committed(), 4)

    def test_timeout_retries_without_settlement(self) -> None:
        runner = self._orchestrator()
        runner.submit(self._task("launch-timeout", "idem-timeout", scenario="timeout"))
        self.assertEqual(runner.run_once(), "retry")
        self.assertEqual(self._launch_state("launch-timeout"), "SEND_STARTED")
        self.assertEqual(runner.budget.get_reservation("launch-timeout").state, RESERVED)
        self.assertEqual(self._window_committed(), 0)
        self.assertEqual(self._verified_rows(), 0)

    def test_lease_expiry_and_stale_owner(self) -> None:
        runner = self._orchestrator()
        submitted = runner.submit(self._task("launch-lease", "idem-lease"))
        queue = runner.queue
        lease = queue.claim("tenant-a", "worker-a", 30)
        self.assertEqual(lease.job_id, submitted.job_id)
        self._expire(submitted.job_id)
        with self.assertRaises(QueueError) as raised:
            queue.complete("tenant-a", lease.job_id, lease.lease_owner, lease.lease_token)
        self.assertIn(raised.exception.code, {"lease_expired", "stale_lease"})
        self.assertEqual(runner.run_once(), "committed")
        self.assertEqual(self._window_committed(), 4)

    def test_crash_before_settlement_retries_once(self) -> None:
        calls = {"n": 0}
        real = lambda: _reconciler_connect(self.dsn)

        def flaky():
            calls["n"] += 1
            if calls["n"] == 1:
                raise QueueError("store_unavailable")
            return real()

        runner = self._orchestrator(reconcile_connect=flaky)
        runner.submit(self._task("launch-crash", "idem-crash"))
        self.assertEqual(runner.run_once(), "retry")
        self.assertEqual(self._launch_state("launch-crash"), "LAUNCH_CONFIRMED")
        self.assertEqual(runner.budget.get_reservation("launch-crash").state, RESERVED)
        self.assertEqual(runner.run_once(), "committed")
        self.assertEqual(self._window_committed(), 4)

    def test_estop_and_disabled_do_not_settle(self) -> None:
        runner = self._orchestrator()
        runner.submit(self._task("launch-stop", "idem-stop"))
        self._set_control(mode="MOCK_ONLY", estop=True)
        self.assertEqual(runner.run_once(), "stopped")
        self.assertIsNone(runner.budget.get_reservation("launch-stop"))
        self._set_control(mode="DISABLED", estop=False)
        fresh = self._orchestrator()
        with self.assertRaises(OrchestratorError) as raised:
            fresh.submit(self._task("launch-off", "idem-off"))
        self.assertEqual(raised.exception.code, "launch_mode_blocked")
        self.assertEqual(self._window_committed(), 0)

    def test_missing_intent_and_invalid_proof_are_denied(self) -> None:
        runner = self._orchestrator()
        runner.budget.request_reserve("launch-gap", "tenant-a", 4)
        with self.assertRaises(BudgetControllerError) as raised:
            runner.budget.commit_usage(
                "launch-gap",
                raw_cost_text="4",
                charged_text="4",
                charged_known=True,
                agent_id=_AGENT,
                run_id=_RUN,
                verification_id=self._verification("4"),
                payload_digest=self._digest("4"),
            )
        self.assertEqual(raised.exception.code, "not_found")
        self.assertEqual(runner.budget.get_reservation("launch-gap").state, RESERVED)
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute("SELECT autopilot_queue.launch_protocol_record_intent(%s, %s, %s, 1, %s)", ("tenant-a", "launch-gap", _AGENT, "a" * 64))
            adapter.execute("SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)", ("tenant-a", "launch-gap"))
            with self.assertRaises(Exception) as proof:
                adapter.execute(
                    "SELECT autopilot_queue.launch_protocol_report_outbound_result(%s, %s, %s, %s)",
                    ("tenant-a", "launch-gap", "HTTP_409", "b" * 64),
                )
            self.assertIn("invalid_caller_or_state", str(proof.exception).lower())
            adapter.rollback()
        finally:
            adapter.close()
        self.assertEqual(runner.budget.get_reservation("launch-gap").state, RESERVED)
        self.assertEqual(self._window_committed(), 0)

    def test_cross_tenant_and_unauthorized_execute(self) -> None:
        runner = self._orchestrator()
        runner.submit(self._task("launch-tenant", "idem-tenant"))
        foreign = self._orchestrator(tenant_id="tenant-b", owner="worker-b")
        self.assertEqual(foreign.run_once(), "idle")
        runtime = _tenant_connect(self.dsn, _TENANTS["tenant-a"][0], _TENANTS["tenant-a"][1])
        try:
            with self.assertRaises(Exception):
                runtime.execute(
                    "SELECT autopilot_queue.launch_protocol_record_intent(%s, %s, %s, 1, %s)",
                    ("tenant-a", "launch-denied", _AGENT, "a" * 64),
                )
                runtime.commit()
            runtime.rollback()
        finally:
            runtime.close()
        self.assertIsNone(self._launch_state("launch-denied"))

    def test_null_recovery_token_rejected(self) -> None:
        runner = self._orchestrator()
        runner.submit(self._task("launch-null", "idem-null", scenario="timeout"))
        self.assertEqual(runner.run_once(), "retry")
        recovery = _connect(self.dsn, _RECOVERY_LOGIN, _RECOVERY_LOGIN)
        try:
            with self.assertRaises(Exception) as raised:
                recovery.execute(
                    "SELECT autopilot_queue.launch_protocol_mark_unknown(%s, %s, NULL)",
                    ("tenant-a", "launch-null"),
                )
                recovery.commit()
            recovery.rollback()
        finally:
            recovery.close()
        self.assertIn("invalid_lease", str(raised.exception).lower())
        self.assertEqual(self._launch_state("launch-null"), "SEND_STARTED")

    def test_overrun_stays_visible_and_blocks_next_reserve(self) -> None:
        runner = self._orchestrator()
        runner.submit(self._task("launch-over", "idem-over", reserve_cents=5, charged_cents=400))
        self.assertEqual(runner.run_once(), "overrun")
        row = runner.budget.get_reservation("launch-over")
        self.assertEqual(row.state, OVERRUN)
        self.assertEqual(row.charged_text, "400")
        self.assertEqual(self._window_committed(), 400)
        with self.assertRaises(OrchestratorError) as raised:
            runner.submit(self._task("launch-next", "idem-next"))
        self.assertEqual(raised.exception.code, "overrun_open")

    def test_payload_cannot_select_a_callable(self) -> None:
        runner = self._orchestrator()
        with self.assertRaises(QueueError) as raised:
            runner.queue.enqueue(
                tenant_id="tenant-a",
                job_type="mock_launch",
                payload={
                    "launch_id": "launch-bad",
                    "agent_id": _AGENT,
                    "run_id": _RUN,
                    "scenario": "create_success",
                    "reserve_cents": 4,
                    "charged_cents": 4,
                    "callable": "os.system",
                },
                idempotency_key="idem-bad",
            )
        self.assertEqual(raised.exception.code, "invalid_input")

    def test_source_has_no_network_or_migration_down(self) -> None:
        source = Path(__file__).resolve().parents[1].joinpath("mock_orchestrator.py").read_text(encoding="utf-8")
        self.assertNotIn("subprocess", source)
        self.assertNotIn("cursor_api", source)
        self.assertNotIn("github_api", source)
        self.assertNotIn("down_budget_migration", source)
        self.assertNotIn("http://", source)
        self.assertNotIn("https://", source)

    def _orchestrator(self, *, tenant_id: str = "tenant-a", owner: str = "worker-a", reconcile_connect=None) -> MockOrchestrator:
        role, password = _TENANTS[tenant_id]
        runtime = lambda: _tenant_connect(self.dsn, role, password)
        budget = BudgetController(
            runtime,
            reconcile_connect=reconcile_connect or (lambda: _reconciler_connect(self.dsn)),
            config=_enabled_config(),
        )
        return MockOrchestrator(
            PostgresQueue(runtime),
            runtime,
            lambda: _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN),
            budget,
            tenant_id=tenant_id,
            owner=owner,
            config=_enabled_config(),
            audit=AuditJournal(self.audit_path),
        )

    def _task(self, launch_id: str, key: str, **overrides) -> MockTask:
        values = {
            "tenant_id": "tenant-a",
            "idempotency_key": key,
            "launch_id": launch_id,
            "agent_id": _AGENT,
            "run_id": _RUN,
            "reserve_cents": 4,
            "charged_cents": 4,
            "scenario": "create_success",
        }
        values.update(overrides)
        return MockTask(**values)

    def _set_control(self, *, mode: str, estop: bool) -> None:
        connection = _owner_connect(self.dsn)
        try:
            connection.execute(
                """
                UPDATE autopilot_queue.autopilot_launch_control
                SET autopilot_launch_mode = %s, autopilot_estop = %s
                WHERE id = 1
                """,
                (mode, estop),
            )
            connection.commit()
        finally:
            connection.close()

    def _expire(self, job_id: str) -> None:
        connection = _owner_connect(self.dsn)
        try:
            connection.execute(
                """
                UPDATE autopilot_queue.jobs
                SET lease_expires_at = clock_timestamp() - interval '1 second'
                WHERE job_id = %s
                """,
                (job_id,),
            )
            connection.commit()
        finally:
            connection.close()

    def _launch_state(self, launch_id: str) -> str | None:
        connection = _owner_connect(self.dsn)
        try:
            row = connection.execute(
                "SELECT lifecycle_state FROM autopilot_queue.launch_intents WHERE launch_id = %s",
                (launch_id,),
            ).fetchone()
            connection.rollback()
        finally:
            connection.close()
        return None if row is None else row[0]

    def _window_committed(self) -> int:
        connection = _owner_connect(self.dsn)
        try:
            row = connection.execute(
                """
                SELECT COALESCE(sum(committed_cents), 0)
                FROM autopilot_queue.budget_window_usage
                WHERE tenant_id = 'tenant-a' AND bucket = 'day'
                """
            ).fetchone()
            connection.rollback()
        finally:
            connection.close()
        return int(row[0])

    def _verified_rows(self) -> int:
        connection = _owner_connect(self.dsn)
        try:
            row = connection.execute("SELECT count(*) FROM autopilot_queue.budget_verified_usage").fetchone()
            connection.rollback()
        finally:
            connection.close()
        return int(row[0])

    def _verification(self, charged: str) -> str:
        verification, _digest = self._refs(charged)
        return verification

    def _digest(self, charged: str) -> str:
        _verification, digest = self._refs(charged)
        return digest

    def _refs(self, charged: str) -> tuple[str, str]:
        from bintrans_autopilot.usage_verification import usage_verification_refs

        return usage_verification_refs(_AGENT, _RUN, {"rawCostCents": charged, "chargedCents": charged})
