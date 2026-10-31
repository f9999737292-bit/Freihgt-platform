"""Budget controller tests. Database cases require BINTRANS_AUTOPILOT_QUEUE_DSN."""

from __future__ import annotations

import os
import threading
import unittest
from dataclasses import replace
from decimal import Decimal
from urllib.parse import urlsplit

from bintrans_autopilot.budget_controller import (
    BLOCKED,
    COMMITTED,
    OVERRUN,
    RESERVED,
    BudgetController,
    BudgetControllerError,
    RELEASED,
    limits_from_config,
)
from bintrans_autopilot.config import AutopilotConfig, load_config
from bintrans_autopilot.postgres_queue import MigrationRunner, QueueError
from bintrans_autopilot.usage_reconciliation import parse_usage_payload, reconcile_launch_usage

_AGENT = "bc-33a0edbc-8fed-47fa-85b7-cc139e900375"
_RUN = "run-a51658e1-665d-4f7c-9135-c0d720b2210b"

_TENANTS = {
    "tenant-a": ("tenant_a_runtime", "tenant_a_test"),
    "tenant-b": ("tenant_b_runtime", "tenant_b_test"),
}


def _dsn() -> str:
    return os.environ.get("BINTRANS_AUTOPILOT_QUEUE_DSN", "").strip()


def _owner_connect(dsn: str):
    import psycopg

    return psycopg.connect(dsn, connect_timeout=3)


def _tenant_connect(dsn: str, user: str, password: str):
    import psycopg

    parts = urlsplit(dsn)
    return psycopg.connect(
        host=parts.hostname or "localhost",
        port=parts.port or 5432,
        dbname=parts.path.lstrip("/"),
        user=user,
        password=password,
        connect_timeout=3,
    )


def _enabled_config() -> AutopilotConfig:
    base = load_config()
    return replace(
        base,
        budget_max_charged_cents=300,
        budget_max_charged_cents_daily=500,
        budget_max_charged_cents_monthly=5000,
        test_budget_harness=True,
    )


def _tenant_controller(dsn: str, tenant_key: str, *, config: AutopilotConfig | None = None) -> BudgetController:
    role, password = _TENANTS[tenant_key]
    return BudgetController(
        lambda: _tenant_connect(dsn, role, password),
        admin_connect=lambda: _owner_connect(dsn),
        config=config or _enabled_config(),
    )


def _ensure_tenant_roles(dsn: str, runner: MigrationRunner) -> None:
    from psycopg import sql

    connection = _owner_connect(dsn)
    try:
        for _tenant, (role, password) in _TENANTS.items():
            found = connection.execute("SELECT 1 FROM pg_roles WHERE rolname = %s", (role,)).fetchone()
            statement = "ALTER ROLE {} PASSWORD {}" if found else (
                "CREATE ROLE {} LOGIN PASSWORD {} NOSUPERUSER NOBYPASSRLS "
                "NOCREATEROLE NOCREATEDB NOREPLICATION INHERIT"
            )
            connection.execute(sql.SQL(statement).format(sql.Identifier(role), sql.Literal(password)))
        connection.commit()
    finally:
        connection.close()
    for tenant, (role, _password) in _TENANTS.items():
        runner.register_principal(tenant, role)


class StaticBudgetControllerTests(unittest.TestCase):
    def test_limits_from_config_use_integer_cents(self) -> None:
        config = _enabled_config()
        limits = limits_from_config(config)
        self.assertEqual(limits.per_launch_cap_cents, 300)
        self.assertEqual(limits.daily_limit_cents, 500)

    def test_parse_usage_distinguishes_unknown_and_zero(self) -> None:
        unknown = parse_usage_payload({"rawCostCents": 12.5})
        self.assertFalse(unknown.charged_known)
        zero = parse_usage_payload({"rawCostCents": 1, "chargedCents": 0})
        self.assertTrue(zero.charged_known)
        self.assertEqual(zero.charged_text, "0")

    def test_controller_refuses_live_mode(self) -> None:
        with self.assertRaises(BudgetControllerError) as raised:
            BudgetController(lambda: None, mode="live")
        self.assertEqual(raised.exception.code, "controller_disabled")

    def test_reserve_cents_rejects_bool(self) -> None:
        controller = BudgetController(lambda: None, config=_enabled_config())
        with self.assertRaises(BudgetControllerError) as raised:
            controller.request_reserve("launch-x", "tenant-a", True)  # type: ignore[arg-type]
        self.assertEqual(raised.exception.code, "invalid_amount")

    def test_source_has_no_cloud_agent_create(self) -> None:
        from pathlib import Path

        source = (Path(__file__).resolve().parents[1] / "budget_controller.py").read_text(encoding="utf-8")
        self.assertNotIn("create_agent", source)
        reconciliation = (Path(__file__).resolve().parents[1] / "usage_reconciliation.py").read_text(encoding="utf-8")
        self.assertNotIn("create_agent", reconciliation)


class PostgresBudgetControllerTests(unittest.TestCase):
    dsn = ""

    @classmethod
    def setUpClass(cls) -> None:
        cls.dsn = _dsn()
        if not cls.dsn:
            return
        cls.runner = MigrationRunner(lambda: _owner_connect(cls.dsn))
        cls.runner.up()
        if cls.runner.version() >= 2:
            cls.runner.down_budget_migration(allow_destructive_test=True)
            cls.runner.up()
        _ensure_tenant_roles(cls.dsn, cls.runner)

    def setUp(self) -> None:
        if not self.dsn:
            self.skipTest("postgres_unavailable")
        connection = _owner_connect(self.dsn)
        try:
            connection.execute("DELETE FROM autopilot_queue.budget_reservations")
            connection.execute("DELETE FROM autopilot_queue.budget_window_usage")
            connection.execute("DELETE FROM autopilot_queue.budget_tenant_ledger")
            connection.execute(
                """
                UPDATE autopilot_queue.budget_global_ledger
                SET daily_reserved_cents = 0,
                    monthly_reserved_cents = 0,
                    daily_committed_cents = 0,
                    monthly_committed_cents = 0
                """
            )
            connection.commit()
        finally:
            connection.close()
        self.limits_admin = BudgetController(
            lambda: _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test"),
            admin_connect=lambda: _owner_connect(self.dsn),
            config=_enabled_config(),
        )
        for tenant in _TENANTS:
            self.limits_admin.sync_tenant_limits(tenant)
        self.controller = _tenant_controller(self.dsn, "tenant-a")

    def test_zero_budget_gate_blocks_reserve(self) -> None:
        controller = _tenant_controller(self.dsn, "tenant-a", config=load_config())
        gate = controller.evaluate_kill_switch()
        self.assertFalse(gate.allowed)
        row = controller.request_reserve("launch-zero", "tenant-a", 10)
        self.assertEqual(row.state, BLOCKED)
        self.assertEqual(row.block_reason, "zero_budget")

    def test_fresh_reserved_does_not_block_kill_switch(self) -> None:
        row = self.controller.request_reserve("launch-pending", "tenant-a", 50)
        self.assertEqual(row.state, RESERVED)
        gate = self.controller.evaluate_kill_switch()
        self.assertTrue(gate.allowed)

    def test_concurrent_reservations_respect_shared_limit(self) -> None:
        outcomes: list[str] = []
        barrier = threading.Barrier(2)

        def worker(launch_id: str) -> None:
            barrier.wait(timeout=5)
            ctrl = _tenant_controller(self.dsn, "tenant-a")
            row = ctrl.request_reserve(launch_id, "tenant-a", 260)
            outcomes.append(row.state)

        threads = [
            threading.Thread(target=worker, args=("launch-a",)),
            threading.Thread(target=worker, args=("launch-b",)),
        ]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(5)
            self.assertFalse(thread.is_alive())
        self.assertEqual(sorted(outcomes), sorted([RESERVED, BLOCKED]))

    def test_global_budget_blocks_second_tenant(self) -> None:
        first = _tenant_controller(self.dsn, "tenant-a").request_reserve("launch-ga", "tenant-a", 250)
        self.assertEqual(first.state, RESERVED)
        second = _tenant_controller(self.dsn, "tenant-b").request_reserve("launch-gb", "tenant-b", 260)
        self.assertEqual(second.state, BLOCKED)
        self.assertEqual(second.block_reason, "global_budget_exceeded")

    def test_repeat_launch_id_is_idempotent(self) -> None:
        first = self.controller.request_reserve("launch-repeat", "tenant-a", 40)
        second = self.controller.request_reserve("launch-repeat", "tenant-a", 40)
        self.assertEqual(first.state, RESERVED)
        self.assertEqual(second.state, RESERVED)

    def test_unknown_usage_blocks_new_reserve(self) -> None:
        reserved = self.controller.request_reserve("launch-unknown", "tenant-a", 50)
        self.assertEqual(reserved.state, RESERVED)
        self.controller.mark_usage_unknown(
            "launch-unknown",
            agent_id=_AGENT,
            run_id=_RUN,
            raw_cost_text="10",
        )
        gate = self.controller.evaluate_kill_switch()
        self.assertFalse(gate.allowed)
        self.assertEqual(gate.reason, "unknown_usage_open")

    def test_partial_commit_and_release(self) -> None:
        reserved = self.controller.request_reserve("launch-commit", "tenant-a", 80)
        committed = self.controller.commit_usage(
            "launch-commit",
            raw_cost_text="12.5",
            charged_text="12",
            charged_known=True,
            agent_id=_AGENT,
            run_id=_RUN,
        )
        self.assertEqual(committed.state, COMMITTED)
        self.assertEqual(committed.charged_text, "12")
        reserved2 = self.controller.request_reserve("launch-release", "tenant-a", 30)
        released = self.controller.release("launch-release")
        self.assertEqual(released.state, RELEASED)

    def test_charge_above_reserve_records_overrun(self) -> None:
        self.controller.request_reserve("launch-over", "tenant-a", 10)
        row = self.controller.commit_usage(
            "launch-over",
            raw_cost_text="20",
            charged_text="20",
            charged_known=True,
            agent_id=_AGENT,
            run_id=_RUN,
        )
        self.assertEqual(row.state, OVERRUN)
        self.assertEqual(row.charged_text, "20")
        gate = self.controller.evaluate_kill_switch()
        self.assertFalse(gate.allowed)
        self.assertEqual(gate.reason, "overrun_open")
        with self.assertRaises(BudgetControllerError) as raised:
            self.controller.release("launch-over")
        self.assertEqual(raised.exception.code, "overrun_open")

    def test_reconciliation_deduplicates_and_handles_timeout(self) -> None:
        self.controller.request_reserve("launch-reconcile", "tenant-a", 60)

        def fetch(_agent_id: str) -> dict:
            return {"rawCostCents": 10.5, "chargedCents": 10}

        first = reconcile_launch_usage(
            self.controller,
            launch_id="launch-reconcile",
            agent_id=_AGENT,
            run_id=_RUN,
            usage_fetcher=fetch,
        )
        self.assertEqual(first.status, "committed")
        second = reconcile_launch_usage(
            self.controller,
            launch_id="launch-reconcile",
            agent_id=_AGENT,
            run_id=_RUN,
            usage_fetcher=fetch,
        )
        self.assertEqual(second.status, "committed")

        def retry(_agent_id: str) -> dict:
            from bintrans_autopilot.cursor_api import CursorApiError

            raise CursorApiError(503, "temporary", "temporary")

        pending = self.controller.request_reserve("launch-retry", "tenant-a", 25)
        self.assertEqual(pending.state, RESERVED)
        result = reconcile_launch_usage(
            self.controller,
            launch_id="launch-retry",
            agent_id=_AGENT,
            run_id=_RUN,
            usage_fetcher=retry,
        )
        self.assertEqual(result.status, "retry")

    def test_tenant_isolation_for_reservations(self) -> None:
        self.controller.request_reserve("launch-a", "tenant-a", 20)
        _tenant_controller(self.dsn, "tenant-b").request_reserve("launch-b", "tenant-b", 20)
        tenant_a = _tenant_controller(self.dsn, "tenant-a")
        row = tenant_a.get_reservation("launch-a")
        self.assertIsNotNone(row)
        self.assertEqual(row.tenant_id, "tenant-a")
        self.assertIsNone(tenant_a.get_reservation("launch-b"))

    def test_owner_connection_cannot_mutate_budget(self) -> None:
        owner = BudgetController(
            lambda: _owner_connect(self.dsn),
            admin_connect=lambda: _owner_connect(self.dsn),
            config=_enabled_config(),
        )
        with self.assertRaises(BudgetControllerError) as raised:
            owner.request_reserve("launch-owner", "tenant-a", 10)
        self.assertEqual(raised.exception.code, "runtime_privileged")

    def test_transaction_rollback_does_not_persist_reserve(self) -> None:
        connection = _owner_connect(self.dsn)
        try:
            connection.execute("BEGIN")
            connection.execute(
                """
                INSERT INTO autopilot_queue.budget_tenant_ledger (
                    tenant_id, utc_day, utc_month, daily_limit_cents, monthly_limit_cents
                ) VALUES ('tenant-a', CURRENT_DATE, date_trunc('month', CURRENT_DATE)::date, 500, 5000)
                ON CONFLICT (tenant_id) DO NOTHING
                """
            )
            connection.execute(
                """
                INSERT INTO autopilot_queue.budget_reservations (
                    launch_id, tenant_id, state, reserve_cents,
                    ledger_utc_day, ledger_utc_month, usage_unknown, charged_known
                ) VALUES (
                    'launch-rollback', 'tenant-a', 'RESERVED', 15,
                    CURRENT_DATE, date_trunc('month', CURRENT_DATE)::date, false, false
                )
                """
            )
            connection.execute("ROLLBACK")
        finally:
            connection.close()
        self.assertIsNone(self.controller.get_reservation("launch-rollback"))

    def test_kill_switch_env_blocks_even_with_test_config(self) -> None:
        try:
            os.environ["PAID_AUTOMATION_ENABLED"] = "true"
            with self.assertRaises(BudgetControllerError):
                BudgetController(
                    lambda: _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test"),
                    config=_enabled_config(),
                )
        finally:
            os.environ.pop("PAID_AUTOMATION_ENABLED", None)

    def test_budget_migration_down_requires_empty_open_reservations(self) -> None:
        self.controller.request_reserve("launch-open", "tenant-a", 10)
        with self.assertRaises(QueueError) as raised:
            self.runner.down()
        self.assertEqual(raised.exception.code, "migration_blocked")
        self.controller.release("launch-open")
        self.assertEqual(self.runner.down_budget_migration(allow_destructive_test=True), 1)
        self.runner.up()
        _ensure_tenant_roles(self.dsn, self.runner)


if __name__ == "__main__":
    unittest.main()
