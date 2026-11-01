"""R5 release safety and identity binding tests."""

from __future__ import annotations

import os
import threading
import unittest

from bintrans_autopilot.budget_controller import BLOCKED, BudgetController, BudgetControllerError
from bintrans_autopilot.postgres_queue import MigrationRunner
from bintrans_autopilot.tests.test_budget_controller_03c2 import (
    _AGENT,
    _RUN,
    _commit_verified,
    _ensure_reconciler_role,
    _ensure_tenant_roles,
    _mark_unknown_verified,
    _owner_connect,
    _reseed_tenant_ledgers,
    _tenant_controller,
)
from bintrans_autopilot.usage_reconciliation import reconcile_launch_usage

_AGENT2 = "bc-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
_RUN2 = "run-bbbbbbbb-cccc-dddd-eeee-ffffffffffff"


def _dsn() -> str:
    return os.environ.get("BINTRANS_AUTOPILOT_QUEUE_DSN", "").strip()


class BudgetControllerR5Tests(unittest.TestCase):
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
            reg = connection.execute(
                "SELECT to_regclass('autopilot_queue.budget_verified_usage') AS name"
            ).fetchone()[0]
            if reg:
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

    def test_runtime_release_blocked_after_unknown_usage(self) -> None:
        self.controller.request_reserve(
            "launch-unk-rel", "tenant-a", 30, agent_id=_AGENT, run_id=_RUN
        )
        _mark_unknown_verified(self.controller, "launch-unk-rel", raw_cost_text="1")
        with self.assertRaises(BudgetControllerError) as raised:
            self.controller.release("launch-unk-rel")
        self.assertEqual(raised.exception.code, "unknown_usage_open")
        gate = self.controller.evaluate_kill_switch()
        self.assertFalse(gate.allowed)

    def test_runtime_release_blocked_after_identity_bound(self) -> None:
        self.controller.request_reserve(
            "launch-bound", "tenant-a", 25, agent_id=_AGENT, run_id=_RUN
        )
        with self.assertRaises(BudgetControllerError) as raised:
            self.controller.release("launch-bound")
        self.assertEqual(raised.exception.code, "launch_identity_bound")

    def test_unbound_reserve_release_still_allowed(self) -> None:
        self.controller.request_reserve("launch-cancel", "tenant-a", 20)
        released = self.controller.release("launch-cancel")
        self.assertEqual(released.state, "RELEASED")

    def test_duplicate_run_id_on_reserve_rejected(self) -> None:
        self.controller.request_reserve(
            "launch-a", "tenant-a", 20, agent_id=_AGENT, run_id=_RUN
        )
        with self.assertRaises(BudgetControllerError) as raised:
            self.controller.request_reserve(
                "launch-b", "tenant-a", 20, agent_id=_AGENT2, run_id=_RUN
            )
        self.assertEqual(raised.exception.code, "run_id_taken")

    def test_reconcile_binds_unbound_historical_reserve(self) -> None:
        self.controller.request_reserve("launch-hist", "tenant-a", 20)
        result = reconcile_launch_usage(
            self.controller,
            launch_id="launch-hist",
            agent_id=_AGENT,
            run_id=_RUN,
            usage_fetcher=lambda _a: {"chargedCents": 0},
        )
        self.assertEqual(result.status, "committed")
        with self.assertRaises(BudgetControllerError) as raised:
            self.controller.release("launch-hist")
        self.assertEqual(raised.exception.code, "invalid_state")

    def test_verified_zero_after_bound_launch(self) -> None:
        self.controller.request_reserve(
            "launch-zero", "tenant-a", 40, agent_id=_AGENT, run_id=_RUN
        )
        result = reconcile_launch_usage(
            self.controller,
            launch_id="launch-zero",
            agent_id=_AGENT,
            run_id=_RUN,
            usage_fetcher=lambda _a: {"chargedCents": 0},
        )
        self.assertEqual(result.status, "committed")
        row = self.controller.get_reservation("launch-zero")
        self.assertEqual(row.charged_text, "0")

    def test_identity_incomplete_on_reserve_rejected(self) -> None:
        with self.assertRaises(BudgetControllerError) as raised:
            self.controller.request_reserve(
                "launch-bad", "tenant-a", 10, agent_id=_AGENT, run_id=None
            )
        self.assertEqual(raised.exception.code, "identity_incomplete")

    def test_concurrent_release_and_reconcile(self) -> None:
        self.controller.request_reserve("launch-race", "tenant-a", 30)
        outcomes: list[str] = []
        barrier = threading.Barrier(2)

        def releaser() -> None:
            barrier.wait(timeout=5)
            try:
                self.controller.release("launch-race")
                outcomes.append("released")
            except BudgetControllerError as exc:
                outcomes.append(exc.code)

        def reconciler() -> None:
            barrier.wait(timeout=5)
            result = reconcile_launch_usage(
                self.controller,
                launch_id="launch-race",
                agent_id=_AGENT,
                run_id=_RUN,
                usage_fetcher=lambda _a: {"chargedCents": 5},
            )
            outcomes.append(result.status)

        t1 = threading.Thread(target=releaser)
        t2 = threading.Thread(target=reconciler)
        t1.start()
        t2.start()
        t1.join(10)
        t2.join(10)
        row = self.controller.get_reservation("launch-race")
        if row.state == "COMMITTED":
            self.assertIn("committed", outcomes)
            self.assertNotIn("released", outcomes)
        else:
            self.assertEqual(row.state, "RELEASED")
            self.assertIn("released", outcomes)

    def test_overrun_global_gate_preserved(self) -> None:
        self.controller.request_reserve(
            "launch-over", "tenant-a", 10, agent_id=_AGENT, run_id=_RUN
        )
        _commit_verified(self.controller, "launch-over", raw_cost_text="20", charged_text="20")
        late = self.controller.request_reserve("launch-late", "tenant-a", 5)
        self.assertEqual(late.state, BLOCKED)


if __name__ == "__main__":
    unittest.main()
