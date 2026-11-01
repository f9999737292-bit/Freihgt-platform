"""R6B launch lifecycle + mock boundary tests (RT catalog subset)."""

from __future__ import annotations

import importlib
import os
import sys
import threading
import types
import unittest
from urllib.parse import urlsplit

from bintrans_autopilot.budget_controller import BudgetControllerError
from bintrans_autopilot.launch.import_guard import forbidden_import_violations
from bintrans_autopilot.launch.mock_transport import MockCursorTransport
from bintrans_autopilot.postgres_queue import MigrationRunner
from bintrans_autopilot.tests.test_budget_controller_03c2 import (
    _AGENT,
    _RUN,
    _ensure_reconciler_role,
    _ensure_tenant_roles,
    _owner_connect,
    _reconciler_connect,
    _tenant_controller,
)

_ADAPTER_LOGIN = "launch_adapter_test"
_RECOVERY_LOGIN = "launch_recovery_test"
_BODY_HASH = "a" * 64


def _dsn() -> str:
    return os.environ.get("BINTRANS_AUTOPILOT_QUEUE_DSN", "").strip()


def _connect(dsn: str, user: str, password: str):
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


def _ensure_launch_roles(dsn: str) -> None:
    from psycopg import sql

    connection = _owner_connect(dsn)
    try:
        for login, cap in (
            (_ADAPTER_LOGIN, "autopilot_launch_adapter"),
            (_RECOVERY_LOGIN, "autopilot_launch_recovery"),
        ):
            found = connection.execute("SELECT 1 FROM pg_roles WHERE rolname = %s", (login,)).fetchone()
            stmt = (
                "ALTER ROLE {} PASSWORD {}"
                if found
                else "CREATE ROLE {} LOGIN PASSWORD {} NOSUPERUSER NOBYPASSRLS "
                "NOCREATEROLE NOCREATEDB NOREPLICATION INHERIT"
            )
            connection.execute(
                sql.SQL(stmt).format(sql.Identifier(login), sql.Literal(login))
            )
            connection.execute(
                sql.SQL("GRANT {} TO {}").format(sql.Identifier(cap), sql.Identifier(login))
            )
        connection.execute(
            "UPDATE autopilot_queue.autopilot_launch_control SET autopilot_launch_mode = 'MOCK_ONLY' WHERE id = 1"
        )
        connection.commit()
    finally:
        connection.close()


def _record_intent(dsn: str, tenant: str, launch_id: str) -> None:
    conn = _connect(dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
    try:
        conn.execute(
            """
            SELECT autopilot_queue.launch_protocol_record_intent(
                %s, %s, %s, %s, %s
            )
            """,
            (tenant, launch_id, _AGENT, 1, _BODY_HASH),
        )
        conn.commit()
    finally:
        conn.close()


class MockBoundaryTests(unittest.TestCase):
    def test_rt19b_no_forbidden_imports(self) -> None:
        self.assertEqual(forbidden_import_violations(), [])

    def test_rt19_launch_import_does_not_load_cursor_api(self) -> None:
        before = set(sys.modules)
        importlib.import_module("bintrans_autopilot.launch")
        importlib.import_module("bintrans_autopilot.launch.mock_transport")
        after = set(sys.modules)
        loaded = after - before
        self.assertFalse(any(n.startswith("bintrans_autopilot.cursor_api") for n in loaded))

    def test_rt31_subclass_forbidden(self) -> None:
        with self.assertRaises(TypeError):
            type("BadMock", (MockCursorTransport,), {})

    def test_mock_transport_exact_type(self) -> None:
        transport = MockCursorTransport("definite_reject")
        self.assertIs(type(transport), MockCursorTransport)


class LaunchLifecyclePostgresTests(unittest.TestCase):
    dsn = ""

    @classmethod
    def setUpClass(cls) -> None:
        cls.dsn = _dsn()
        if not cls.dsn:
            return
        cls._prior_autopilot_env = os.environ.get("AUTOPILOT_ENABLED")
        os.environ["AUTOPILOT_ENABLED"] = "true"
        try:
            cls.runner = MigrationRunner(lambda: _owner_connect(cls.dsn))
            cls.runner.up()
        except Exception:
            cls.dsn = ""
            return
        if cls.runner.version() >= 2:
            cls.runner.down_budget_migration(allow_destructive_test=True)
            cls.runner.up()
        _ensure_tenant_roles(cls.dsn, cls.runner)
        _ensure_reconciler_role(cls.dsn)
        if cls.runner.version() >= 8:
            _ensure_launch_roles(cls.dsn)

    @classmethod
    def tearDownClass(cls) -> None:
        if cls.dsn:
            owner = _owner_connect(cls.dsn)
            try:
                owner.execute("DELETE FROM autopilot_queue.recovery_poll_audit")
                owner.execute("DELETE FROM autopilot_queue.launch_intents")
                owner.execute("DELETE FROM autopilot_queue.budget_reservations")
                owner.commit()
            finally:
                owner.close()
        prior = getattr(cls, "_prior_autopilot_env", None)
        if prior is None:
            os.environ.pop("AUTOPILOT_ENABLED", None)
        else:
            os.environ["AUTOPILOT_ENABLED"] = prior

    def setUp(self) -> None:
        if not self.dsn or self.runner.version() < 8:
            self.skipTest("launch_migration_unavailable")
        connection = _owner_connect(self.dsn)
        try:
            connection.execute("DELETE FROM autopilot_queue.recovery_poll_audit")
            connection.execute("DELETE FROM autopilot_queue.launch_intents")
            connection.execute("DELETE FROM autopilot_queue.budget_verified_usage")
            connection.execute("DELETE FROM autopilot_queue.budget_reservations")
            connection.execute("DELETE FROM autopilot_queue.budget_window_usage")
            connection.execute("DELETE FROM autopilot_queue.budget_tenant_ledger")
            connection.execute(
                """
                UPDATE autopilot_queue.budget_global_ledger
                SET daily_reserved_cents = 0, monthly_reserved_cents = 0,
                    daily_committed_cents = 0, monthly_committed_cents = 0
                """
            )
            connection.execute(
                """
                UPDATE autopilot_queue.autopilot_launch_control
                SET autopilot_launch_mode = 'MOCK_ONLY',
                    autopilot_estop = false,
                    max_recovery_polls = 24
                WHERE id = 1
                """
            )
            connection.commit()
        finally:
            connection.close()
        self.controller = _tenant_controller(self.dsn, "tenant-a")
        self.controller.sync_tenant_limits("tenant-a")

    def test_rt02_release_blocked_after_intent(self) -> None:
        reserved = self.controller.request_reserve("launch-intent", "tenant-a", 25)
        self.assertEqual(reserved.state, "RESERVED")
        _record_intent(self.dsn, "tenant-a", "launch-intent")
        with self.assertRaises(BudgetControllerError) as raised:
            self.controller.release("launch-intent")
        self.assertEqual(raised.exception.code, "launch_intent_recorded")

    def test_rt07_stale_intent_recovery_cancel(self) -> None:
        self.controller.request_reserve("launch-stale", "tenant-a", 30)
        _record_intent(self.dsn, "tenant-a", "launch-stale")
        owner = _owner_connect(self.dsn)
        try:
            owner.execute(
                """
                UPDATE autopilot_queue.launch_intents
                SET intent_recorded_at = clock_timestamp() - interval '2 days'
                """
            )
            owner.commit()
        finally:
            owner.close()
        recovery = _connect(self.dsn, _RECOVERY_LOGIN, _RECOVERY_LOGIN)
        try:
            token = recovery.execute(
                """
                SELECT autopilot_queue.launch_protocol_recovery_lease_acquire(%s, %s)
                """,
                ("tenant-a", "launch-stale"),
            ).fetchone()[0]
            recovery.execute(
                """
                SELECT autopilot_queue.launch_protocol_cancel_before_send(%s, %s, %s)
                """,
                ("tenant-a", "launch-stale", token),
            )
            recovery.commit()
        finally:
            recovery.close()
        row = self.controller.get_reservation("launch-stale")
        self.assertEqual(row.state, "RELEASED")

    def test_rt10_max_polls_manual(self) -> None:
        self.controller.request_reserve("launch-poll", "tenant-a", 20)
        _record_intent(self.dsn, "tenant-a", "launch-poll")
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", "launch-poll"),
            )
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_unknown(%s, %s)",
                ("tenant-a", "launch-poll"),
            )
            adapter.commit()
        finally:
            adapter.close()
        owner = _owner_connect(self.dsn)
        try:
            owner.execute(
                "UPDATE autopilot_queue.autopilot_launch_control SET max_recovery_polls = 2 WHERE id = 1"
            )
            owner.commit()
        finally:
            owner.close()
        recovery = _connect(self.dsn, _RECOVERY_LOGIN, _RECOVERY_LOGIN)
        try:
            token = recovery.execute(
                "SELECT autopilot_queue.launch_protocol_recovery_lease_acquire(%s, %s)",
                ("tenant-a", "launch-poll"),
            ).fetchone()[0]
            decision = recovery.execute(
                """
                SELECT decision FROM autopilot_queue.launch_protocol_recovery_poll_attempt(%s, %s, %s)
                """,
                ("tenant-a", "launch-poll", token),
            ).fetchone()[0]
            self.assertEqual(decision, "CONTINUE")
            decision = recovery.execute(
                """
                SELECT decision FROM autopilot_queue.launch_protocol_recovery_poll_attempt(%s, %s, %s)
                """,
                ("tenant-a", "launch-poll", token),
            ).fetchone()[0]
            self.assertEqual(decision, "MANUAL")
            recovery.execute(
                "SELECT autopilot_queue.launch_protocol_escalate_manual(%s, %s, %s)",
                ("tenant-a", "launch-poll", token),
            )
            recovery.commit()
        finally:
            recovery.close()
        owner = _owner_connect(self.dsn)
        try:
            state = owner.execute(
                "SELECT lifecycle_state FROM autopilot_queue.launch_intents WHERE launch_id = %s",
                ("launch-poll",),
            ).fetchone()[0]
            self.assertEqual(state, "MANUAL_RECONCILIATION_REQUIRED")
        finally:
            owner.close()

    def test_rt41_confirm_requires_planned_agent(self) -> None:
        self.controller.request_reserve("launch-confirm", "tenant-a", 20)
        _record_intent(self.dsn, "tenant-a", "launch-confirm")
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", "launch-confirm"),
            )
            with self.assertRaises(Exception) as raised:
                adapter.execute(
                    """
                    SELECT autopilot_queue.launch_protocol_confirm(%s, %s, %s, %s)
                    """,
                    (
                        "tenant-a",
                        "launch-confirm",
                        "bc-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
                        _RUN,
                    ),
                )
            self.assertIn("identity_conflict", str(raised.exception).lower())
            adapter.rollback()
        finally:
            adapter.close()

    def test_rt43_global_block_on_unknown(self) -> None:
        self.controller.request_reserve("launch-unk", "tenant-a", 15)
        _record_intent(self.dsn, "tenant-a", "launch-unk")
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", "launch-unk"),
            )
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_unknown(%s, %s)",
                ("tenant-a", "launch-unk"),
            )
            adapter.commit()
        finally:
            adapter.close()
        blocked = self.controller.request_reserve("launch-other", "tenant-a", 10)
        self.assertEqual(blocked.state, "BLOCKED")
        self.assertEqual(blocked.block_reason, "launch_reconciliation_open")

    def test_rt37_cross_tenant_launch_read_not_found(self) -> None:
        """Budget PK remains launch_id-global until composite migration; verify tenant-scoped protocol."""
        self.controller.request_reserve("launch-tenant-a", "tenant-a", 10)
        _record_intent(self.dsn, "tenant-a", "launch-tenant-a")
        recovery = _connect(self.dsn, _RECOVERY_LOGIN, _RECOVERY_LOGIN)
        try:
            with self.assertRaises(Exception) as raised:
                recovery.execute(
                    "SELECT autopilot_queue.launch_protocol_recovery_lease_acquire(%s, %s)",
                    ("tenant-b", "launch-tenant-a"),
                )
            self.assertIn("not_found", str(raised.exception).lower())
            recovery.rollback()
        finally:
            recovery.close()

    def test_rt34_estop_blocks_send_started(self) -> None:
        self.controller.request_reserve("launch-estop", "tenant-a", 15)
        _record_intent(self.dsn, "tenant-a", "launch-estop")
        owner = _owner_connect(self.dsn)
        try:
            owner.execute(
                "UPDATE autopilot_queue.autopilot_launch_control SET autopilot_estop = true WHERE id = 1"
            )
            owner.commit()
        finally:
            owner.close()
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            with self.assertRaises(Exception) as raised:
                adapter.execute(
                    "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                    ("tenant-a", "launch-estop"),
                )
            self.assertIn("estop_active", str(raised.exception).lower())
            adapter.rollback()
        finally:
            adapter.close()

    def test_rt42_concurrent_release_and_intent(self) -> None:
        self.controller.request_reserve("launch-race", "tenant-a", 20)
        outcomes: list[str] = []
        barrier = threading.Barrier(2)

        def releaser() -> None:
            barrier.wait(timeout=5)
            try:
                self.controller.release("launch-race")
                outcomes.append("released")
            except BudgetControllerError as exc:
                outcomes.append(exc.code)

        def intent_writer() -> None:
            barrier.wait(timeout=5)
            try:
                _record_intent(self.dsn, "tenant-a", "launch-race")
                outcomes.append("intent")
            except Exception:
                outcomes.append("intent_fail")

        t1 = threading.Thread(target=releaser)
        t2 = threading.Thread(target=intent_writer)
        t1.start()
        t2.start()
        t1.join(10)
        t2.join(10)
        if "intent" in outcomes:
            self.assertIn("launch_intent_recorded", outcomes)


if __name__ == "__main__":
    unittest.main()
