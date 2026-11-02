"""R6B-1-R1 security regression tests (Agent J findings)."""

from __future__ import annotations

import os
import unittest
from urllib.parse import urlsplit

from bintrans_autopilot.budget_controller import BudgetControllerError
from bintrans_autopilot.postgres_queue import MigrationRunner
from bintrans_autopilot.tests.test_budget_controller_03c2 import (
    _AGENT,
    _RUN,
    _ensure_tenant_roles,
    _owner_connect,
    _reseed_tenant_ledgers,
    _tenant_controller,
)
from bintrans_autopilot.tests.test_launch_lifecycle_r6b import (
    _ADAPTER_LOGIN,
    _RECOVERY_LOGIN,
    _connect,
    _dsn,
    _ensure_launch_roles,
    _record_intent,
)

_TENANT_A_RUNTIME = ("tenant_a_runtime", "tenant_a_test")
_TENANT_B_RUNTIME = ("tenant_b_runtime", "tenant_b_test")
_PROOF_DIGEST = "a" * 64


def _runtime_connect(dsn: str, role_password: tuple[str, str]):
    role, password = role_password
    return _connect(dsn, role, password)


class LaunchLifecycleR6B1R1SecurityTests(unittest.TestCase):
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
        if cls.runner.version() < 14:
            cls.dsn = ""
            return
        if cls.runner.version() >= 2:
            cls.runner.down_budget_migration(allow_destructive_test=True)
            cls.runner.up()
        _ensure_tenant_roles(cls.dsn, cls.runner)
        _ensure_launch_roles(cls.dsn)
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
        if not self.dsn or self.runner.version() < 14:
            self.skipTest("launch_r6b1r1_migration_unavailable")
        owner = _owner_connect(self.dsn)
        try:
            owner.execute("DELETE FROM autopilot_queue.manual_reconciliation_audit")
            owner.execute("DELETE FROM autopilot_queue.recovery_poll_audit")
            owner.execute("DELETE FROM autopilot_queue.launch_intents")
            owner.execute("DELETE FROM autopilot_queue.budget_verified_usage")
            owner.execute("DELETE FROM autopilot_queue.budget_reservations")
            owner.execute("DELETE FROM autopilot_queue.budget_window_usage")
            owner.execute("DELETE FROM autopilot_queue.budget_tenant_ledger")
            owner.execute(
                """
                UPDATE autopilot_queue.budget_global_ledger
                SET daily_reserved_cents = 0, monthly_reserved_cents = 0,
                    daily_committed_cents = 0, monthly_committed_cents = 0
                """
            )
            owner.execute(
                """
                UPDATE autopilot_queue.autopilot_launch_control
                SET autopilot_launch_mode = 'MOCK_ONLY', autopilot_estop = false,
                    recovery_lease_seconds = 300, send_stale_seconds = 3600,
                    intent_stale_seconds = 3600, max_recovery_polls = 3
                WHERE id = 1
                """
            )
            owner.commit()
        finally:
            owner.close()
        _reseed_tenant_ledgers(self.dsn, self.runner)
        self.controller_a = _tenant_controller(self.dsn, "tenant-a")
        self.controller_b = _tenant_controller(self.dsn, "tenant-b")

    def test_r6b1_002_runtime_cannot_select_launch_intents(self) -> None:
        shared = "launch-shared-x"
        self.controller_a.request_reserve(shared, "tenant-a", 10)
        _record_intent(self.dsn, "tenant-a", shared)
        self.controller_b.request_reserve(shared, "tenant-b", 10)
        _record_intent(self.dsn, "tenant-b", shared)
        conn = _runtime_connect(self.dsn, _TENANT_A_RUNTIME)
        try:
            with self.assertRaises(Exception):
                conn.execute(
                    "SELECT launch_id, tenant_id FROM autopilot_queue.launch_intents"
                )
            with self.assertRaises(Exception):
                conn.execute(
                    """
                    SELECT recovery_lease_token FROM autopilot_queue.launch_intents
                    WHERE tenant_id = %s AND launch_id = %s
                    """,
                    ("tenant-b", shared),
                )
        finally:
            conn.close()

    def test_r6b1_001_cancel_send_started_does_not_release(self) -> None:
        launch_id = "launch-no-cancel-send"
        self.controller_a.request_reserve(launch_id, "tenant-a", 12)
        _record_intent(self.dsn, "tenant-a", launch_id)
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", launch_id),
            )
            with self.assertRaises(Exception) as raised:
                adapter.execute(
                    "SELECT autopilot_queue.launch_protocol_cancel_before_send(%s, %s)",
                    ("tenant-a", launch_id),
                )
            self.assertIn("invalid_caller_or_state", str(raised.exception).lower())
            adapter.rollback()
        finally:
            adapter.close()
        self.assertEqual(self.controller_a.get_reservation(launch_id).state, "RESERVED")

    def test_r6b1_004_adapter_cannot_confirm_unknown(self) -> None:
        launch_id = "launch-no-adapter-unknown-confirm"
        self.controller_a.request_reserve(launch_id, "tenant-a", 11)
        _record_intent(self.dsn, "tenant-a", launch_id)
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", launch_id),
            )
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_unknown(%s, %s)",
                ("tenant-a", launch_id),
            )
            with self.assertRaises(Exception) as raised:
                adapter.execute(
                    "SELECT autopilot_queue.launch_protocol_confirm(%s, %s, %s, %s)",
                    ("tenant-a", launch_id, _AGENT, _RUN),
                )
            self.assertIn("invalid_caller_or_state", str(raised.exception).lower())
            adapter.rollback()
        finally:
            adapter.close()

    def test_r6b1_005_confirm_rejects_null_run_id(self) -> None:
        launch_id = "launch-null-run"
        self.controller_a.request_reserve(launch_id, "tenant-a", 10)
        _record_intent(self.dsn, "tenant-a", launch_id)
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", launch_id),
            )
            with self.assertRaises(Exception) as raised:
                adapter.execute(
                    "SELECT autopilot_queue.launch_protocol_confirm(%s, %s, %s, %s)",
                    ("tenant-a", launch_id, _AGENT, None),
                )
            self.assertIn("invalid_input", str(raised.exception).lower())
            adapter.rollback()
        finally:
            adapter.close()

    def test_r6b1_008_same_launch_id_two_tenants(self) -> None:
        shared = "launch-dup-id"
        self.controller_a.request_reserve(shared, "tenant-a", 8)
        self.controller_b.request_reserve(shared, "tenant-b", 9)
        self.assertEqual(self.controller_a.get_reservation(shared).state, "RESERVED")
        self.assertEqual(self.controller_b.get_reservation(shared).state, "RESERVED")
        _record_intent(self.dsn, "tenant-b", shared)
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-b", shared),
            )
            adapter.commit()
        finally:
            adapter.close()
        self.controller_a.release(shared)
        self.assertEqual(self.controller_b.get_reservation(shared).state, "RESERVED")
        with self.assertRaises(BudgetControllerError):
            self.controller_b.release(shared)

    def test_r6b1_009_definer_owner_not_superuser(self) -> None:
        owner = _owner_connect(self.dsn)
        try:
            row = owner.execute(
                """
                SELECT rolname, rolsuper
                FROM pg_roles
                WHERE rolname = 'autopilot_queue_definer_owner'
                """
            ).fetchone()
            self.assertIsNotNone(row)
            self.assertFalse(bool(row[1]))
        finally:
            owner.close()

    def test_r6b1_recovery_confirm_requires_valid_fence(self) -> None:
        launch_id = "launch-fence-confirm"
        self.controller_a.request_reserve(launch_id, "tenant-a", 10)
        _record_intent(self.dsn, "tenant-a", launch_id)
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", launch_id),
            )
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_unknown(%s, %s)",
                ("tenant-a", launch_id),
            )
            adapter.commit()
        finally:
            adapter.close()
        recovery = _connect(self.dsn, _RECOVERY_LOGIN, _RECOVERY_LOGIN)
        try:
            token = recovery.execute(
                "SELECT autopilot_queue.launch_protocol_recovery_lease_acquire(%s, %s)",
                ("tenant-a", launch_id),
            ).fetchone()[0]
            recovery.commit()
            owner = _owner_connect(self.dsn)
            try:
                owner.execute(
                    """
                    UPDATE autopilot_queue.launch_intents
                    SET recovery_lease_expires_at = clock_timestamp() - interval '1 minute'
                    WHERE tenant_id = %s AND launch_id = %s
                    """,
                    ("tenant-a", launch_id),
                )
                owner.commit()
            finally:
                owner.close()
            with self.assertRaises(Exception) as raised:
                recovery.execute(
                    "SELECT autopilot_queue.launch_protocol_confirm(%s, %s, %s, %s, %s)",
                    ("tenant-a", launch_id, _AGENT, _RUN, token),
                )
            self.assertIn("invalid_lease", str(raised.exception).lower())
            recovery.rollback()
        finally:
            recovery.close()

    def test_r6b1_010_escalate_before_max_polls_fails(self) -> None:
        launch_id = "launch-escalate-early"
        self.controller_a.request_reserve(launch_id, "tenant-a", 10)
        _record_intent(self.dsn, "tenant-a", launch_id)
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", launch_id),
            )
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_unknown(%s, %s)",
                ("tenant-a", launch_id),
            )
            adapter.commit()
        finally:
            adapter.close()
        recovery = _connect(self.dsn, _RECOVERY_LOGIN, _RECOVERY_LOGIN)
        try:
            token = recovery.execute(
                "SELECT autopilot_queue.launch_protocol_recovery_lease_acquire(%s, %s)",
                ("tenant-a", launch_id),
            ).fetchone()[0]
            with self.assertRaises(Exception) as raised:
                recovery.execute(
                    "SELECT autopilot_queue.launch_protocol_escalate_manual(%s, %s, %s)",
                    ("tenant-a", launch_id, token),
                )
            self.assertIn("invalid_caller_or_state", str(raised.exception).lower())
            recovery.rollback()
        finally:
            recovery.close()

    def test_launch_intents_rls_enabled_and_forced(self) -> None:
        owner = _owner_connect(self.dsn)
        try:
            flags = owner.execute(
                """
                SELECT c.relrowsecurity, c.relforcerowsecurity
                FROM pg_class c
                JOIN pg_namespace n ON n.oid = c.relnamespace
                WHERE n.nspname = 'autopilot_queue' AND c.relname = 'launch_intents'
                """
            ).fetchone()
            self.assertTrue(bool(flags[0]))
            self.assertTrue(bool(flags[1]))
        finally:
            owner.close()

    def test_r2_001_runtime_commit_and_mark_unknown_absent(self) -> None:
        import psycopg

        owner = _owner_connect(self.dsn)
        try:
            names = owner.execute(
                """
                SELECT p.proname, pg_get_function_identity_arguments(p.oid) AS args
                FROM pg_proc p
                JOIN pg_namespace n ON n.oid = p.pronamespace
                WHERE n.nspname = 'autopilot_queue'
                  AND p.proname IN ('budget_protocol_commit', 'budget_protocol_mark_unknown')
                """
            ).fetchall()
            self.assertEqual(names, [])
        finally:
            owner.close()
        conn = _runtime_connect(self.dsn, _TENANT_A_RUNTIME)
        try:
            conn.execute("SET ROLE autopilot_queue_runtime")
            owner = _owner_connect(self.dsn)
            try:
                rows = owner.execute(
                    """
                    SELECT p.oid::regprocedure::text
                    FROM pg_proc p
                    JOIN pg_namespace n ON n.oid = p.pronamespace
                    WHERE n.nspname = 'autopilot_queue'
                      AND p.proname IN (
                          'budget_protocol_commit',
                          'budget_protocol_mark_unknown',
                          '_budget_commit_from_verified'
                      )
                    """
                ).fetchall()
            finally:
                owner.close()
            for (regproc,) in rows:
                with self.assertRaises(psycopg.Error):
                    conn.execute(f"SELECT {regproc}")
                conn.rollback()
        finally:
            conn.close()

    def test_r2_001_public_execute_denied_on_internal_budget_helpers(self) -> None:
        owner = _owner_connect(self.dsn)
        try:
            rows = owner.execute(
                """
                SELECT p.oid::regprocedure::text
                FROM pg_proc p
                JOIN pg_namespace n ON n.oid = p.pronamespace
                WHERE n.nspname = 'autopilot_queue'
                  AND p.proname LIKE '\\_budget%' ESCAPE '\\'
                """
            ).fetchall()
            for (regproc,) in rows:
                allowed = owner.execute(
                    "SELECT has_function_privilege('public', %s::regprocedure, 'EXECUTE')",
                    (regproc,),
                ).fetchone()[0]
                self.assertFalse(bool(allowed), regproc)
        finally:
            owner.close()

    def test_r6b1r4_definer_owner_d1_no_superuser_secdef_owners(self) -> None:
        owner = _owner_connect(self.dsn)
        try:
            super_count = owner.execute(
                """
                SELECT COUNT(*)::int
                FROM pg_proc p
                JOIN pg_namespace n ON n.oid = p.pronamespace
                JOIN pg_roles r ON r.oid = p.proowner
                WHERE n.nspname = 'autopilot_queue'
                  AND p.prosecdef
                  AND r.rolsuper
                """
            ).fetchone()[0]
            self.assertEqual(super_count, 0)
            owned = owner.execute(
                """
                SELECT COUNT(*)::int
                FROM pg_proc p
                JOIN pg_namespace n ON n.oid = p.pronamespace
                JOIN pg_roles r ON r.oid = p.proowner
                WHERE n.nspname = 'autopilot_queue'
                  AND p.prosecdef
                  AND r.rolname = 'autopilot_queue_definer_owner'
                """
            ).fetchone()[0]
            self.assertGreater(owned, 0)
            definer = owner.execute(
                """
                SELECT rolname, rolsuper, rolbypassrls
                FROM pg_roles
                WHERE rolname = 'autopilot_queue_definer_owner'
                """
            ).fetchone()
            self.assertIsNotNone(definer)
            self.assertFalse(bool(definer[1]))
            self.assertFalse(bool(definer[2]))
        finally:
            owner.close()

    def test_r1_003_lt06_reject_requires_whitelist_and_matching_report(self) -> None:
        launch_id = "launch-lt06-proof"
        self.controller_a.request_reserve(launch_id, "tenant-a", 10)
        _record_intent(self.dsn, "tenant-a", launch_id)
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", launch_id),
            )
            with self.assertRaises(Exception) as raised:
                adapter.execute(
                    "SELECT autopilot_queue.launch_protocol_reject_definite(%s, %s, %s)",
                    ("tenant-a", launch_id, "HTTP_500"),
                )
            self.assertIn("invalid_caller_or_state", str(raised.exception).lower())
            adapter.rollback()
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", launch_id),
            )
            with self.assertRaises(Exception) as raised:
                adapter.execute(
                    """
                    SELECT autopilot_queue.launch_protocol_report_outbound_result(%s, %s, %s, %s)
                    """,
                    ("tenant-a", launch_id, "HTTP_401", _PROOF_DIGEST),
                )
            self.assertIn("invalid_caller_or_state", str(raised.exception).lower())
            adapter.rollback()
        finally:
            adapter.close()
        self.assertEqual(self.controller_a.get_reservation(launch_id).state, "RESERVED")

    def test_r6b1r4_lt06_mock_trusted_happy_path_releases(self) -> None:
        launch_id = "launch-lt06-mock-ok"
        self.controller_a.request_reserve(launch_id, "tenant-a", 10)
        _record_intent(self.dsn, "tenant-a", launch_id)
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", launch_id),
            )
            adapter.execute(
                """
                SELECT autopilot_queue.launch_protocol_report_outbound_result(%s, %s, %s, %s)
                """,
                ("tenant-a", launch_id, "MOCK_TRUSTED", _PROOF_DIGEST),
            )
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_reject_definite(%s, %s, %s)",
                ("tenant-a", launch_id, "MOCK_TRUSTED"),
            )
            adapter.commit()
        finally:
            adapter.close()
        self.assertEqual(self.controller_a.get_reservation(launch_id).state, "RELEASED")

    def test_r6b1r4_http_classes_rejected_in_mock_only(self) -> None:
        for proof_class in ("HTTP_401", "HTTP_403", "HTTP_404", "HTTP_409", "HTTP_500", "TIMEOUT"):
            launch_id = f"launch-http-{proof_class.lower()}"
            self.controller_a.request_reserve(launch_id, "tenant-a", 9)
            _record_intent(self.dsn, "tenant-a", launch_id)
            adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
            try:
                adapter.execute(
                    "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                    ("tenant-a", launch_id),
                )
                with self.assertRaises(Exception):
                    adapter.execute(
                        """
                        SELECT autopilot_queue.launch_protocol_report_outbound_result(%s, %s, %s, %s)
                        """,
                        ("tenant-a", launch_id, proof_class, _PROOF_DIGEST),
                    )
                adapter.rollback()
            finally:
                adapter.close()
            self.assertEqual(self.controller_a.get_reservation(launch_id).state, "RESERVED")

    def test_r6b1r4_ambiguous_unknown_blocks_lt06_release(self) -> None:
        launch_id = "launch-unknown-no-lt06"
        self.controller_a.request_reserve(launch_id, "tenant-a", 10)
        _record_intent(self.dsn, "tenant-a", launch_id)
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", launch_id),
            )
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_unknown(%s, %s)",
                ("tenant-a", launch_id),
            )
            with self.assertRaises(Exception):
                adapter.execute(
                    "SELECT autopilot_queue.launch_protocol_reject_definite(%s, %s, %s)",
                    ("tenant-a", launch_id, "MOCK_TRUSTED"),
                )
            adapter.rollback()
        finally:
            adapter.close()
        self.assertEqual(self.controller_a.get_reservation(launch_id).state, "RESERVED")

    def test_r1_003_lt06_mock_trusted_requires_mock_mode(self) -> None:
        launch_id = "launch-lt06-real-mode"
        self.controller_a.request_reserve(launch_id, "tenant-a", 10)
        _record_intent(self.dsn, "tenant-a", launch_id)
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            adapter.execute(
                "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                ("tenant-a", launch_id),
            )
            adapter.commit()
        finally:
            adapter.close()
        owner = _owner_connect(self.dsn)
        try:
            owner.execute(
                """
                UPDATE autopilot_queue.autopilot_launch_control
                SET autopilot_launch_mode = 'DISABLED'
                WHERE id = 1
                """
            )
            owner.commit()
        finally:
            owner.close()
        adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
        try:
            with self.assertRaises(Exception) as raised:
                adapter.execute(
                    """
                    SELECT autopilot_queue.launch_protocol_report_outbound_result(%s, %s, %s, %s)
                    """,
                    ("tenant-a", launch_id, "MOCK_TRUSTED", _PROOF_DIGEST),
                )
            lowered = str(raised.exception).lower()
            self.assertTrue(
                "invalid_caller_or_state" in lowered or "launch_mode_blocked" in lowered,
                lowered,
            )
            adapter.rollback()
        finally:
            adapter.close()

    def test_r1_005_functional_v9_cancel_and_reject_then_restore_head(self) -> None:
        runner = self.runner
        launch_cancel = "launch-v9-cancel"
        launch_reject = "launch-v9-reject"
        self.controller_a.request_reserve(launch_cancel, "tenant-a", 7)
        self.controller_a.request_reserve(launch_reject, "tenant-a", 8)
        _record_intent(self.dsn, "tenant-a", launch_cancel)
        _record_intent(self.dsn, "tenant-a", launch_reject)
        try:
            while runner.version() > 9:
                runner.down(allow_destructive_test=True)
            self.assertEqual(runner.version(), 9)
            adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
            try:
                adapter.execute(
                    "SELECT autopilot_queue.launch_protocol_cancel_before_send(%s, %s)",
                    ("tenant-a", launch_cancel),
                )
                adapter.execute(
                    "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
                    ("tenant-a", launch_reject),
                )
                adapter.commit()
            finally:
                adapter.close()
            owner = _owner_connect(self.dsn)
            try:
                owner.execute(
                    """
                    UPDATE autopilot_queue.launch_intents
                    SET outbound_attempt_count = 1
                    WHERE tenant_id = %s AND launch_id = %s
                    """,
                    ("tenant-a", launch_reject),
                )
                owner.commit()
            finally:
                owner.close()
            adapter = _connect(self.dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
            try:
                adapter.execute(
                    "SELECT autopilot_queue.launch_protocol_reject_definite(%s, %s, %s)",
                    ("tenant-a", launch_reject, "MOCK_TRUSTED"),
                )
                adapter.commit()
            finally:
                adapter.close()
            self.assertEqual(self.controller_a.get_reservation(launch_cancel).state, "RELEASED")
            self.assertEqual(self.controller_a.get_reservation(launch_reject).state, "RELEASED")
        finally:
            while runner.version() < 14:
                runner.up()
            _reseed_tenant_ledgers(self.dsn, runner)

    def test_r6b1r3_up_down_up_commit_still_absent(self) -> None:
        runner = self.runner
        try:
            self.assertEqual(runner.down(allow_destructive_test=True), 13)
            self.assertEqual(runner.up(), 14)
            owner = _owner_connect(self.dsn)
            try:
                count = owner.execute(
                    """
                    SELECT COUNT(*)::int
                    FROM pg_proc p
                    JOIN pg_namespace n ON n.oid = p.pronamespace
                    WHERE n.nspname = 'autopilot_queue'
                      AND p.proname = 'budget_protocol_commit'
                    """
                ).fetchone()[0]
                self.assertEqual(count, 0)
            finally:
                owner.close()
        finally:
            if runner.version() < 14:
                runner.up()


if __name__ == "__main__":
    unittest.main()
