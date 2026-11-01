"""R3 authority regression tests (requires BINTRANS_AUTOPILOT_QUEUE_DSN)."""

from __future__ import annotations

import os
import unittest
from dataclasses import replace
from bintrans_autopilot.budget_controller import (
    BLOCKED,
    COMMITTED,
    BudgetController,
    BudgetControllerError,
)
from bintrans_autopilot.postgres_queue import MigrationRunner
from bintrans_autopilot.tests.test_budget_controller_03c2 import (
    _AGENT,
    _RUN,
    _commit_verified,
    _enabled_config,
    _ensure_reconciler_role,
    _ensure_tenant_roles,
    _owner_connect,
    _reseed_tenant_ledgers,
    _tenant_connect,
    _tenant_controller,
)

_AGENT2 = "bc-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
_RUN2 = "run-bbbbbbbb-cccc-dddd-eeee-ffffffffffff"


def _dsn() -> str:
    return os.environ.get("BINTRANS_AUTOPILOT_QUEUE_DSN", "").strip()


class BudgetControllerR3Tests(unittest.TestCase):
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
        self.controller = _tenant_controller(self.dsn, "tenant-a")

    def test_runtime_cannot_pass_fake_reserve_window(self) -> None:
        connection = _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test")
        try:
            import psycopg

            with self.assertRaises(psycopg.errors.UndefinedFunction):
                connection.execute(
                    """
                    SELECT autopilot_queue.budget_protocol_reserve(
                        'launch-fake', 10, 1000000, '2099-01-01'::date, '2099-01-01'::date
                    )
                    """
                )
        finally:
            connection.close()

    def test_runtime_cannot_commit_with_forged_cents(self) -> None:
        self.controller.request_reserve("launch-forge", "tenant-a", 20)
        connection = _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test")
        try:
            import psycopg

            with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                connection.execute(
                    """
                    SELECT autopilot_queue.budget_reconciler_apply_usage(
                        'launch-forge', %s, %s, '0', '150', true,
                        'fake-verification-id', repeat('a', 64), 'cursor_usage_api'
                    )
                    """,
                    (_AGENT, _RUN),
                )
        finally:
            connection.close()
        row = self.controller.get_reservation("launch-forge")
        self.assertEqual(row.state, "RESERVED")

    def test_window_usage_rls_hides_other_tenant(self) -> None:
        self.controller.request_reserve("launch-a", "tenant-a", 30)
        other = BudgetController(
            lambda: _tenant_connect(self.dsn, "tenant_b_runtime", "tenant_b_test"),
            admin_connect=lambda: _owner_connect(self.dsn),
            config=_enabled_config(),
        )
        other.request_reserve("launch-b", "tenant-b", 30)
        connection = _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test")
        try:
            rows = connection.execute(
                """
                SELECT tenant_id FROM autopilot_queue.budget_window_usage
                WHERE scope = 'tenant'
                """
            ).fetchall()
            tenants = {str(row[0]) for row in rows}
            self.assertIn("tenant-a", tenants)
            self.assertNotIn("tenant-b", tenants)
        finally:
            connection.close()

    def test_conflicting_recommit_raises(self) -> None:
        self.controller.request_reserve("launch-idem", "tenant-a", 100)
        _commit_verified(self.controller, "launch-idem", raw_cost_text="40", charged_text="40")
        with self.assertRaises(BudgetControllerError) as raised:
            _commit_verified(
                self.controller,
                "launch-idem",
                raw_cost_text="99",
                charged_text="99",
                agent_id=_AGENT2,
                run_id=_RUN2,
            )
        self.assertEqual(raised.exception.code, "idempotency_conflict")
        row = self.controller.get_reservation("launch-idem")
        self.assertEqual(row.state, COMMITTED)
        self.assertEqual(row.charged_text, "40")

    def test_harness_does_not_bypass_autopilot_disabled(self) -> None:
        os.environ.pop("AUTOPILOT_ENABLED", None)
        controller = BudgetController(
            lambda: _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test"),
            admin_connect=lambda: _owner_connect(self.dsn),
            config=replace(_enabled_config(), autopilot_enabled=False),
        )
        row = controller.request_reserve("launch-off", "tenant-a", 10)
        self.assertEqual(row.state, BLOCKED)
        self.assertEqual(row.block_reason, "config_disabled")
        os.environ["AUTOPILOT_ENABLED"] = "true"

    def test_overrun_blocks_subsequent_reserve_under_global_gate(self) -> None:
        self.controller.request_reserve("launch-gate", "tenant-a", 50)
        _commit_verified(self.controller, "launch-gate", raw_cost_text="60", charged_text="60")
        late = self.controller.request_reserve("launch-late", "tenant-a", 10)
        self.assertEqual(late.state, BLOCKED)
        self.assertEqual(late.block_reason, "overrun_open")

    def test_runtime_cannot_call_global_lock_helper(self) -> None:
        connection = _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test")
        try:
            import psycopg

            with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                connection.execute("SELECT autopilot_queue.budget_global_lock_row()")
        finally:
            connection.close()


if __name__ == "__main__":
    unittest.main()
