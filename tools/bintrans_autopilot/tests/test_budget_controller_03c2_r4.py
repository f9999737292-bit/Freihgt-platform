"""R4 verified usage authority tests (requires BINTRANS_AUTOPILOT_QUEUE_DSN)."""

from __future__ import annotations

import os
import unittest

from bintrans_autopilot.budget_controller import BLOCKED, COMMITTED, BudgetController, BudgetControllerError
from bintrans_autopilot.cursor_api import CursorApiError
from bintrans_autopilot.postgres_queue import MigrationRunner
from bintrans_autopilot.tests.test_budget_controller_03c2 import (
    _AGENT,
    _RUN,
    _commit_verified,
    _ensure_reconciler_role,
    _ensure_tenant_roles,
    _owner_connect,
    _reseed_tenant_ledgers,
    _tenant_connect,
    _tenant_controller,
    _usage_refs,
)
from bintrans_autopilot.usage_reconciliation import reconcile_launch_usage

_AGENT2 = "bc-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
_RUN2 = "run-bbbbbbbb-cccc-dddd-eeee-ffffffffffff"


def _dsn() -> str:
    return os.environ.get("BINTRANS_AUTOPILOT_QUEUE_DSN", "").strip()


class BudgetControllerR4Tests(unittest.TestCase):
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
            _reseed_tenant_ledgers(cls.dsn, cls.runner)

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
        _reseed_tenant_ledgers(self.dsn, self.runner)
        self.controller = _tenant_controller(self.dsn, "tenant-a")

    def test_runtime_commit_without_verification_rejected(self) -> None:
        self.controller.request_reserve("launch-no-verify", "tenant-a", 20)
        with self.assertRaises(BudgetControllerError) as raised:
            self.controller.commit_usage(
                "launch-no-verify",
                raw_cost_text="1",
                charged_text="1",
                charged_known=True,
                agent_id=_AGENT,
                run_id=_RUN,
            )
        self.assertEqual(raised.exception.code, "verified_usage_required")

    def test_runtime_commit_without_reconciler_role_rejected(self) -> None:
        runtime_only = BudgetController(
            lambda: _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test"),
            admin_connect=lambda: _owner_connect(self.dsn),
        )
        runtime_only.sync_tenant_limits("tenant-a")
        runtime_only.request_reserve("launch-no-reconciler", "tenant-a", 20)
        payload = {"chargedCents": 1}
        verification_id, payload_digest = _usage_refs(payload)
        with self.assertRaises(BudgetControllerError) as raised:
            runtime_only.commit_usage(
                "launch-no-reconciler",
                raw_cost_text="1",
                charged_text="1",
                charged_known=True,
                agent_id=_AGENT,
                run_id=_RUN,
                verification_id=verification_id,
                payload_digest=payload_digest,
            )
        self.assertEqual(raised.exception.code, "reconciler_required")

    def test_null_and_empty_charge_paths(self) -> None:
        self.controller.request_reserve("launch-null", "tenant-a", 20)
        payload = {"rawCostCents": 5}
        verification_id, payload_digest = _usage_refs(payload)
        with self.assertRaises(BudgetControllerError) as raised:
            self.controller.commit_usage(
                "launch-null",
                raw_cost_text="5",
                charged_text=None,
                charged_known=True,
                agent_id=_AGENT,
                run_id=_RUN,
                verification_id=verification_id,
                payload_digest=payload_digest,
            )
        self.assertEqual(raised.exception.code, "unknown_usage")

        result = reconcile_launch_usage(
            self.controller,
            launch_id="launch-null",
            agent_id=_AGENT,
            run_id=_RUN,
            usage_fetcher=lambda _aid: {"rawCostCents": 5},
        )
        self.assertEqual(result.status, "pending")
        row = self.controller.get_reservation("launch-null")
        self.assertTrue(row.usage_unknown)

    def test_verified_zero_charge_commits(self) -> None:
        self.controller.request_reserve("launch-zero", "tenant-a", 20)
        result = reconcile_launch_usage(
            self.controller,
            launch_id="launch-zero",
            agent_id=_AGENT,
            run_id=_RUN,
            usage_fetcher=lambda _aid: {"rawCostCents": 0, "chargedCents": 0},
        )
        self.assertEqual(result.status, "committed")
        row = self.controller.get_reservation("launch-zero")
        self.assertEqual(row.state, COMMITTED)
        self.assertEqual(row.charged_text, "0")

    def test_identity_mismatch_on_reconcile(self) -> None:
        self.controller.request_reserve(
            "launch-id", "tenant-a", 20, agent_id=_AGENT, run_id=_RUN
        )
        result = reconcile_launch_usage(
            self.controller,
            launch_id="launch-id",
            agent_id=_AGENT2,
            run_id=_RUN,
            usage_fetcher=lambda _aid: {"chargedCents": 1},
        )
        self.assertEqual(result.status, "failed")
        self.assertEqual(result.code, "identity_mismatch")

    def test_reconciliation_idempotent(self) -> None:
        self.controller.request_reserve("launch-idem", "tenant-a", 30)
        fetch = lambda _aid: {"rawCostCents": 2, "chargedCents": 2}
        first = reconcile_launch_usage(
            self.controller,
            launch_id="launch-idem",
            agent_id=_AGENT,
            run_id=_RUN,
            usage_fetcher=fetch,
        )
        second = reconcile_launch_usage(
            self.controller,
            launch_id="launch-idem",
            agent_id=_AGENT,
            run_id=_RUN,
            usage_fetcher=fetch,
        )
        self.assertEqual(first.status, "committed")
        self.assertEqual(second.status, "committed")

    def test_api_error_classes(self) -> None:
        for status, expected in (
            (401, "blocked"),
            (403, "blocked"),
            (404, "blocked"),
            (429, "retry"),
            (503, "retry"),
            (0, "retry"),
        ):
            launch = f"launch-api-{status}"
            self.controller.request_reserve(launch, "tenant-a", 10)

            def fetch(_aid: str, code=status) -> dict:
                raise CursorApiError(code, "err", "err")

            result = reconcile_launch_usage(
                self.controller,
                launch_id=launch,
                agent_id=_AGENT,
                run_id=_RUN,
                usage_fetcher=fetch,
            )
            self.assertEqual(result.status, expected, msg=f"status={status}")

    def test_overrun_still_recorded_via_verified_path(self) -> None:
        self.controller.request_reserve("launch-over", "tenant-a", 10)
        _commit_verified(self.controller, "launch-over", raw_cost_text="20", charged_text="20")
        row = self.controller.get_reservation("launch-over")
        self.assertEqual(row.state, "OVERRUN")
        late = self.controller.request_reserve("launch-after-over", "tenant-a", 5)
        self.assertEqual(late.state, BLOCKED)

    def test_sql_forgery_blocked_for_runtime(self) -> None:
        self.controller.request_reserve("launch-sql", "tenant-a", 10)
        connection = _tenant_connect(self.dsn, "tenant_a_runtime", "tenant_a_test")
        try:
            import psycopg

            with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                connection.execute(
                    """
                    SELECT autopilot_queue.budget_reconciler_apply_usage(
                        'tenant-a', 'launch-sql', %s, %s, NULL, '0', true,
                        'forged-id-12345678', repeat('b', 64), 'cursor_usage_api'
                    )
                    """,
                    (_AGENT, _RUN),
                )
        finally:
            connection.close()


if __name__ == "__main__":
    unittest.main()
