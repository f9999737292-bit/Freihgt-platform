"""R2 security regression tests for budget controller (requires BINTRANS_AUTOPILOT_QUEUE_DSN)."""

from __future__ import annotations

import os
import threading
import unittest
from dataclasses import replace
from datetime import datetime, timedelta, timezone
from urllib.parse import urlsplit

from bintrans_autopilot.budget_controller import (
    BLOCKED,
    OVERRUN,
    RESERVED,
    BudgetController,
    BudgetControllerError,
)
from bintrans_autopilot.config import AutopilotConfig, load_config
from bintrans_autopilot.postgres_queue import MigrationRunner
from bintrans_autopilot.usage_reconciliation import reconcile_launch_usage
from bintrans_autopilot.tests.test_budget_controller_03c2 import (
    _AGENT,
    _RUN,
    _commit_verified,
    _ensure_reconciler_role,
    _ensure_tenant_roles,
    _reseed_tenant_ledgers,
    _tenant_controller,
)

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


def _enabled_config(**kwargs) -> AutopilotConfig:
    base = load_config()
    return replace(
        base,
        autopilot_enabled=True,
        budget_max_charged_cents=300,
        budget_max_charged_cents_daily=500,
        budget_max_charged_cents_monthly=5000,
        test_budget_harness=True,
        **kwargs,
    )


def _controller(dsn: str, *, now=None) -> BudgetController:
    return _tenant_controller(dsn, "tenant-a", config=_enabled_config(), now=now)


class BudgetControllerR2Tests(unittest.TestCase):
    dsn = ""

    @classmethod
    def setUpClass(cls) -> None:
        cls.dsn = _dsn()
        if not cls.dsn:
            return
        cls._prior_autopilot_env = os.environ.get("AUTOPILOT_ENABLED")
        os.environ["AUTOPILOT_ENABLED"] = "true"
        cls.runner = MigrationRunner(lambda: _owner_connect(cls.dsn))
        cls.runner.up()
        if cls.runner.version() >= 2:
            cls.runner.down_budget_migration(allow_destructive_test=True)
            cls.runner.up()
        _ensure_tenant_roles(cls.dsn, cls.runner)
        _ensure_reconciler_role(cls.dsn)
        if cls.dsn and cls.runner.version() >= 2:
            _reseed_tenant_ledgers(cls.dsn)

    @classmethod
    def tearDownClass(cls) -> None:
        prior = getattr(cls, "_prior_autopilot_env", None)
        if prior is None:
            os.environ.pop("AUTOPILOT_ENABLED", None)
        else:
            os.environ["AUTOPILOT_ENABLED"] = prior

    def setUp(self) -> None:
        if not self.dsn:
            self.skipTest("postgres_unavailable")
        connection = _owner_connect(self.dsn)
        try:
            if connection.execute(
                "SELECT to_regclass('autopilot_queue.budget_verified_usage') AS name"
            ).fetchone()[0]:
                connection.execute("DELETE FROM autopilot_queue.budget_verified_usage")
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
        _reseed_tenant_ledgers(self.dsn)
        self.controller = _controller(self.dsn)

    def test_runtime_cannot_directly_update_reservations(self) -> None:
        self.controller.request_reserve("launch-dml", "tenant-a", 20)
        connection = _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test")
        try:
            import psycopg

            with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                connection.execute(
                    "UPDATE autopilot_queue.budget_reservations SET state = 'RELEASED' WHERE launch_id = 'launch-dml'"
                )
                connection.commit()
        finally:
            connection.close()
        row = self.controller.get_reservation("launch-dml")
        self.assertEqual(row.state, RESERVED)

    def test_runtime_cannot_zero_global_counters_via_write_function(self) -> None:
        self.controller.request_reserve("launch-sd", "tenant-a", 40)
        connection = _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test")
        try:
            import psycopg

            with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                connection.execute(
                    """
                    SELECT autopilot_queue.budget_global_write_counters(
                        CURRENT_DATE, date_trunc('month', CURRENT_DATE)::date,
                        0, 0, 0, 0
                    )
                    """
                )
        finally:
            connection.close()

    def test_global_overrun_beyond_limit_is_recorded(self) -> None:
        admin = BudgetController(
            lambda: _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test"),
            admin_connect=lambda: _owner_connect(self.dsn),
            config=replace(_enabled_config(), budget_max_charged_cents_daily=500),
        )
        admin.sync_tenant_limits("tenant-a")
        self.controller.request_reserve("launch-big", "tenant-a", 300)
        row = _commit_verified(
            self.controller,
            "launch-big",
            raw_cost_text="600",
            charged_text="600",
        )
        self.assertEqual(row.state, OVERRUN)
        self.assertEqual(row.charged_text, "600")
        gate = self.controller.evaluate_kill_switch()
        self.assertFalse(gate.allowed)
        with self.assertRaises(BudgetControllerError) as raised:
            self.controller.release("launch-big")
        self.assertEqual(raised.exception.code, "overrun_open")

    def test_daily_rollover_allows_fresh_day_reserve(self) -> None:
        day1 = datetime(2026, 10, 10, 12, tzinfo=timezone.utc)
        day2 = day1 + timedelta(days=1)
        c1 = _controller(self.dsn, now=lambda: day1)
        c2 = _controller(self.dsn, now=lambda: day2)
        for tenant in ("tenant-a",):
            admin = BudgetController(
                lambda: _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test"),
                admin_connect=lambda: _owner_connect(self.dsn),
                config=_enabled_config(),
                now=lambda: day1,
            )
            admin.sync_tenant_limits(tenant)
        first = c1.request_reserve("launch-day1", "tenant-a", 100)
        self.assertEqual(first.state, RESERVED)
        _commit_verified(c1, "launch-day1", raw_cost_text="100", charged_text="100")
        second = c2.request_reserve("launch-day2", "tenant-a", 100)
        self.assertEqual(second.state, RESERVED)

    def test_emergency_stop_not_bypassed_by_test_harness(self) -> None:
        try:
            os.environ["AUTOPILOT_EMERGENCY_STOP"] = "true"
            controller = _controller(self.dsn)
            gate = controller.evaluate_kill_switch()
            self.assertFalse(gate.allowed)
            self.assertEqual(gate.reason, "emergency_stop_env")
            row = controller.request_reserve("launch-estop", "tenant-a", 10)
            self.assertEqual(row.state, BLOCKED)
        finally:
            os.environ.pop("AUTOPILOT_EMERGENCY_STOP", None)

    def test_fractional_usage_rounds_up_for_ledger(self) -> None:
        self.controller.request_reserve("launch-ceil", "tenant-a", 50)
        _commit_verified(
            self.controller,
            "launch-ceil",
            raw_cost_text="3.49",
            charged_text="3.49",
        )
        connection = _owner_connect(self.dsn)
        try:
            row = connection.execute(
                """
                SELECT committed_cents
                FROM autopilot_queue.budget_window_usage
                WHERE scope = 'tenant'
                  AND tenant_id = 'tenant-a'
                  AND bucket = 'day'
                ORDER BY bucket_start DESC
                LIMIT 1
                """
            ).fetchone()
            self.assertEqual(int(row[0]), 4)
        finally:
            connection.close()

    def test_reconcile_nan_usage_fails_closed(self) -> None:
        self.controller.request_reserve("launch-nan", "tenant-a", 10)

        def fetch(_agent_id: str) -> dict:
            return {"rawCostCents": "NaN", "chargedCents": "NaN"}

        result = reconcile_launch_usage(
            self.controller,
            launch_id="launch-nan",
            agent_id=_AGENT,
            run_id=_RUN,
            usage_fetcher=fetch,
        )
        self.assertEqual(result.status, "failed")
        self.assertEqual(result.code, "invalid_amount")

    def test_kill_switch_race_after_overrun_commit(self) -> None:
        self.controller.request_reserve("launch-race", "tenant-a", 50)
        _commit_verified(self.controller, "launch-race", raw_cost_text="60", charged_text="60")
        outcomes: list[str] = []
        barrier = threading.Barrier(2)

        def waiter() -> None:
            barrier.wait(timeout=5)
            row = _controller(self.dsn).request_reserve("launch-race-wait", "tenant-a", 10)
            outcomes.append(row.state)

        thread = threading.Thread(target=waiter)
        thread.start()
        barrier.wait(timeout=5)
        thread.join(5)
        self.assertEqual(outcomes, [BLOCKED])


if __name__ == "__main__":
    unittest.main()
