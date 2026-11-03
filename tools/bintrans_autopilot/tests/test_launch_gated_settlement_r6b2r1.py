"""R6B-2-R1 remediation. Isolated PostgreSQL only. Synthetic amounts."""

from __future__ import annotations

import os
import threading
import unittest
import uuid

from bintrans_autopilot.budget_controller import BudgetControllerError
from bintrans_autopilot.postgres_queue import MigrationRunner
from bintrans_autopilot.tests.test_budget_controller_03c2 import (
    _AGENT,
    _RUN,
    _RECONCILER_LOGIN,
    _RECONCILER_PASSWORD,
    _commit_verified,
    _ensure_reconciler_role,
    _ensure_tenant_roles,
    _owner_connect,
    _reseed_tenant_ledgers,
    _tenant_controller,
)
from bintrans_autopilot.tests.test_launch_gated_settlement_r6b2 import _confirm, _settle
from bintrans_autopilot.tests.test_launch_lifecycle_r6b import (
    _ADAPTER_LOGIN,
    _RECOVERY_LOGIN,
    _connect,
    _dsn,
    _ensure_launch_roles,
    _record_intent,
)

_DIGEST = "c" * 64


def _committed(dsn: str) -> int:
    owner = _owner_connect(dsn)
    try:
        row = owner.execute(
            """
            SELECT COALESCE(SUM(committed_cents), 0)::bigint
            FROM autopilot_queue.budget_window_usage
            WHERE scope = 'tenant' AND tenant_id = 'tenant-a' AND bucket = 'day'
            """
        ).fetchone()
        return int(row[0])
    finally:
        owner.close()


class LaunchGatedSettlementR6B2R1Tests(unittest.TestCase):
    dsn = ""

    @classmethod
    def setUpClass(cls) -> None:
        cls.dsn = _dsn()
        if not cls.dsn:
            return
        cls._prior_autopilot = os.environ.get("AUTOPILOT_ENABLED")
        os.environ["AUTOPILOT_ENABLED"] = "true"
        try:
            cls.runner = MigrationRunner(lambda: _owner_connect(cls.dsn))
            cls.runner.up()
        except Exception:
            cls.dsn = ""
            return
        if cls.runner.version() < 15:
            cls.dsn = ""
            return
        _ensure_tenant_roles(cls.dsn, cls.runner)
        _ensure_launch_roles(cls.dsn)
        _ensure_reconciler_role(cls.dsn)
        _reseed_tenant_ledgers(cls.dsn, cls.runner)

    @classmethod
    def tearDownClass(cls) -> None:
        prior = getattr(cls, "_prior_autopilot", None)
        if prior is None:
            os.environ.pop("AUTOPILOT_ENABLED", None)
        else:
            os.environ["AUTOPILOT_ENABLED"] = prior

    def setUp(self) -> None:
        if not self.dsn:
            self.skipTest("r6b2r1_postgres_unavailable")
        owner = _owner_connect(self.dsn)
        try:
            owner.execute("DELETE FROM autopilot_queue.launch_intents")
            owner.execute("DELETE FROM autopilot_queue.budget_verified_usage")
            owner.execute("DELETE FROM autopilot_queue.budget_reservations")
            owner.execute("DELETE FROM autopilot_queue.budget_window_usage")
            owner.execute(
                """
                UPDATE autopilot_queue.autopilot_launch_control
                SET autopilot_launch_mode = 'MOCK_ONLY', autopilot_estop = false
                WHERE id = 1
                """
            )
            owner.commit()
        finally:
            owner.close()
        _reseed_tenant_ledgers(self.dsn, self.runner)
        self.controller = _tenant_controller(self.dsn, "tenant-a")

    def _ready(self, launch_id: str, cents: int = 10) -> None:
        self.controller.request_reserve(launch_id, "tenant-a", cents)
        _record_intent(self.dsn, "tenant-a", launch_id)
        _confirm(self.dsn, "tenant-a", launch_id)

    def _apply(self, conn, tenant: str, launch_id: str, *, charged: str = "4", verification: str = "verify-r6b2r1") -> None:
        conn.execute(
            """
            SELECT autopilot_queue.budget_reconciler_apply_usage(
                %s, %s, %s, %s, %s, %s, true, %s, %s, 'cursor_usage_api'
            )
            """,
            (tenant, launch_id, _AGENT, _RUN, charged, charged, verification, _DIGEST),
        )

    def test_missing_intent_denied_on_sql_and_python(self) -> None:
        launch_id = "r6b2r1-no-intent"
        self.controller.request_reserve(launch_id, "tenant-a", 10)
        before = _committed(self.dsn)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception) as legacy:
                self._apply(recon, "tenant-a", launch_id)
            self.assertIn("not_found", str(legacy.exception).lower())
            recon.rollback()
            with self.assertRaises(Exception) as gated:
                _settle(recon, "tenant-a", launch_id, verification="verify-r6b2r1-b")
            self.assertIn("not_found", str(gated.exception).lower())
            recon.rollback()
        finally:
            recon.close()
        with self.assertRaises(BudgetControllerError) as product:
            self.controller.commit_usage(
                launch_id,
                raw_cost_text="4",
                charged_text="4",
                charged_known=True,
                agent_id=_AGENT,
                run_id=_RUN,
                verification_id="verify-r6b2r1-c",
                payload_digest=_DIGEST,
            )
        self.assertEqual(product.exception.code, "not_found")
        self.assertEqual(self.controller.get_reservation(launch_id).state, "RESERVED")
        self.assertEqual(_committed(self.dsn), before)

    def test_estop_and_disabled_deny_product_and_legacy(self) -> None:
        launch_id = "r6b2r1-stop"
        self._ready(launch_id)
        owner = _owner_connect(self.dsn)
        try:
            owner.execute(
                "UPDATE autopilot_queue.autopilot_launch_control SET autopilot_estop = true WHERE id = 1"
            )
            owner.commit()
        finally:
            owner.close()
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception) as raised:
                self._apply(recon, "tenant-a", launch_id)
            self.assertIn("estop_active", str(raised.exception).lower())
            recon.rollback()
        finally:
            recon.close()
        with self.assertRaises(BudgetControllerError) as product:
            self.controller.commit_usage(
                launch_id,
                raw_cost_text="4",
                charged_text="4",
                charged_known=True,
                agent_id=_AGENT,
                run_id=_RUN,
                verification_id="verify-r6b2r1-estop",
                payload_digest=_DIGEST,
            )
        self.assertEqual(product.exception.code, "estop_active")
        owner = _owner_connect(self.dsn)
        try:
            owner.execute(
                """
                UPDATE autopilot_queue.autopilot_launch_control
                SET autopilot_estop = false, autopilot_launch_mode = 'DISABLED'
                WHERE id = 1
                """
            )
            owner.commit()
        finally:
            owner.close()
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception) as raised:
                self._apply(recon, "tenant-a", launch_id)
            self.assertIn("launch_mode_blocked", str(raised.exception).lower())
            recon.rollback()
        finally:
            recon.close()
        self.assertEqual(self.controller.get_reservation(launch_id).state, "RESERVED")
        self.assertEqual(_committed(self.dsn), 0)

    def test_concurrent_settlement_charges_once(self) -> None:
        launch_id = "r6b2r1-race"
        self._ready(launch_id, 10)
        barrier = threading.Barrier(2)
        errors: list[BaseException] = []

        def worker() -> None:
            conn = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
            try:
                barrier.wait(5)
                self._apply(conn, "tenant-a", launch_id, charged="4", verification="verify-r6b2r1-race")
                conn.commit()
            except BaseException as exc:
                errors.append(exc)
                conn.rollback()
            finally:
                conn.close()

        threads = [threading.Thread(target=worker), threading.Thread(target=worker)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(10)
        self.assertEqual(errors, [])
        self.assertEqual(self.controller.get_reservation(launch_id).state, "COMMITTED")
        self.assertEqual(self.controller.get_reservation(launch_id).charged_text, "4")
        self.assertEqual(_committed(self.dsn), 4)

    def test_conflicting_key_wrong_reservation_and_tenant(self) -> None:
        self._ready("r6b2r1-conflict", 10)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            self._apply(recon, "tenant-a", "r6b2r1-conflict", charged="4", verification="verify-r6b2r1-ok")
            with self.assertRaises(Exception) as raised:
                self._apply(recon, "tenant-a", "r6b2r1-conflict", charged="9", verification="verify-r6b2r1-other")
            self.assertIn("idempotency_conflict", str(raised.exception).lower())
            recon.rollback()
        finally:
            recon.close()
        self.assertEqual(self.controller.get_reservation("r6b2r1-conflict").state, "RESERVED")
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception) as raised:
                self._apply(recon, "tenant-b", "r6b2r1-conflict")
            self.assertIn("not_found", str(raised.exception).lower())
            recon.rollback()
        finally:
            recon.close()
        self.controller.request_reserve("r6b2r1-unbound", "tenant-a", 8)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception) as raised:
                self._apply(recon, "tenant-a", "r6b2r1-unbound")
            self.assertIn("not_found", str(raised.exception).lower())
            recon.rollback()
        finally:
            recon.close()
        self.assertEqual(self.controller.get_reservation("r6b2r1-conflict").state, "RESERVED")
        self.assertEqual(self.controller.get_reservation("r6b2r1-unbound").state, "RESERVED")
        self.assertEqual(_committed(self.dsn), 0)

    def test_null_wrong_expired_and_replaced_lease(self) -> None:
        launch_id = "r6b2r1-fence"
        self.controller.request_reserve(launch_id, "tenant-a", 10)
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
        first = _connect(self.dsn, _RECOVERY_LOGIN, _RECOVERY_LOGIN)
        second = _connect(self.dsn, _RECOVERY_LOGIN, _RECOVERY_LOGIN)
        try:
            token = first.execute(
                "SELECT autopilot_queue.launch_protocol_recovery_lease_acquire(%s, %s)",
                ("tenant-a", launch_id),
            ).fetchone()[0]
            again = first.execute(
                "SELECT autopilot_queue.launch_protocol_recovery_lease_acquire(%s, %s)",
                ("tenant-a", launch_id),
            ).fetchone()[0]
            self.assertEqual(token, again)
            first.commit()
            with self.assertRaises(Exception) as raised:
                second.execute(
                    "SELECT autopilot_queue.launch_protocol_recovery_lease_acquire(%s, %s)",
                    ("tenant-a", launch_id),
                )
            self.assertIn("lease_held_by_other", str(raised.exception).lower())
            second.rollback()
            with self.assertRaises(Exception) as raised:
                second.execute(
                    "SELECT decision FROM autopilot_queue.launch_protocol_recovery_poll_attempt(%s, %s, %s)",
                    ("tenant-a", launch_id, None),
                )
            self.assertIn("invalid_lease", str(raised.exception).lower())
            second.rollback()
            with self.assertRaises(Exception) as raised:
                first.execute(
                    "SELECT decision FROM autopilot_queue.launch_protocol_recovery_poll_attempt(%s, %s, %s)",
                    ("tenant-a", launch_id, str(uuid.uuid4())),
                )
            self.assertIn("invalid_lease", str(raised.exception).lower())
            first.rollback()
            token = first.execute(
                "SELECT autopilot_queue.launch_protocol_recovery_lease_acquire(%s, %s)",
                ("tenant-a", launch_id),
            ).fetchone()[0]
            with self.assertRaises(Exception) as raised:
                first.execute(
                    "SELECT autopilot_queue.launch_protocol_confirm(%s, %s, %s, %s, %s)",
                    ("tenant-a", launch_id, _AGENT, _RUN, None),
                )
            self.assertIn("invalid_lease", str(raised.exception).lower())
            first.rollback()
            owner = _owner_connect(self.dsn)
            try:
                owner.execute(
                    """
                    UPDATE autopilot_queue.launch_intents
                    SET recovery_lease_expires_at = clock_timestamp() - interval '1 minute'
                    WHERE tenant_id = 'tenant-a' AND launch_id = %s
                    """,
                    (launch_id,),
                )
                owner.commit()
            finally:
                owner.close()
            with self.assertRaises(Exception) as raised:
                first.execute(
                    "SELECT decision FROM autopilot_queue.launch_protocol_recovery_poll_attempt(%s, %s, %s)",
                    ("tenant-a", launch_id, token),
                )
            self.assertIn("invalid_lease", str(raised.exception).lower())
            first.rollback()
            replaced = second.execute(
                "SELECT autopilot_queue.launch_protocol_recovery_lease_acquire(%s, %s)",
                ("tenant-a", launch_id),
            ).fetchone()[0]
            self.assertNotEqual(replaced, token)
            second.commit()
            with self.assertRaises(Exception) as raised:
                first.execute(
                    "SELECT decision FROM autopilot_queue.launch_protocol_recovery_poll_attempt(%s, %s, %s)",
                    ("tenant-a", launch_id, token),
                )
            self.assertIn("invalid_lease", str(raised.exception).lower())
            first.rollback()
        finally:
            first.close()
            second.close()
        owner = _owner_connect(self.dsn)
        try:
            count = owner.execute(
                "SELECT recovery_poll_count FROM autopilot_queue.launch_intents WHERE launch_id = %s",
                (launch_id,),
            ).fetchone()[0]
            state = owner.execute(
                "SELECT lifecycle_state FROM autopilot_queue.launch_intents WHERE launch_id = %s",
                (launch_id,),
            ).fetchone()[0]
        finally:
            owner.close()
        self.assertEqual(count, 0)
        self.assertEqual(state, "LAUNCH_UNKNOWN")

    def test_null_mark_unknown_denied(self) -> None:
        launch_id = "r6b2r1-unknown"
        self.controller.request_reserve(launch_id, "tenant-a", 10)
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
                UPDATE autopilot_queue.launch_intents
                SET send_started_at = clock_timestamp() - interval '2 days'
                WHERE launch_id = %s
                """,
                (launch_id,),
            )
            owner.commit()
        finally:
            owner.close()
        recovery = _connect(self.dsn, _RECOVERY_LOGIN, _RECOVERY_LOGIN)
        try:
            recovery.execute(
                "SELECT autopilot_queue.launch_protocol_recovery_lease_acquire(%s, %s)",
                ("tenant-a", launch_id),
            )
            with self.assertRaises(Exception) as raised:
                recovery.execute(
                    "SELECT autopilot_queue.launch_protocol_mark_unknown(%s, %s, %s)",
                    ("tenant-a", launch_id, None),
                )
            self.assertIn("invalid_lease", str(raised.exception).lower())
            recovery.rollback()
        finally:
            recovery.close()
        owner = _owner_connect(self.dsn)
        try:
            state = owner.execute(
                "SELECT lifecycle_state FROM autopilot_queue.launch_intents WHERE launch_id = %s",
                (launch_id,),
            ).fetchone()[0]
        finally:
            owner.close()
        self.assertEqual(state, "SEND_STARTED")

    def test_active_lease_does_not_block_settlement_or_disclose_token(self) -> None:
        launch_id = "r6b2r1-lease-settle"
        self._ready(launch_id)
        owner = _owner_connect(self.dsn)
        try:
            owner.execute(
                """
                UPDATE autopilot_queue.launch_intents
                SET recovery_lease_token = %s,
                    recovery_lease_holder = 'other-worker',
                    recovery_lease_expires_at = clock_timestamp() + interval '5 minutes'
                WHERE tenant_id = 'tenant-a' AND launch_id = %s
                """,
                (str(uuid.uuid4()), launch_id),
            )
            owner.commit()
        finally:
            owner.close()
        row = _commit_verified(self.controller, launch_id, raw_cost_text="4", charged_text="4")
        self.assertEqual(row.state, "COMMITTED")
        self.assertEqual(_committed(self.dsn), 4)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception):
                recon.execute("SELECT recovery_lease_token FROM autopilot_queue.launch_intents")
            recon.rollback()
        finally:
            recon.close()

    def test_over_reserve_and_over_cap_stay_visible_overrun(self) -> None:
        reserved = self.controller.request_reserve("r6b2r1-over-cap", "tenant-a", 5)
        self.assertEqual(reserved.state, "RESERVED")
        huge = _commit_verified(
            self.controller,
            "r6b2r1-over-cap",
            raw_cost_text="999999",
            charged_text="999999",
        )
        self.assertEqual(huge.state, "OVERRUN")
        self.assertEqual(huge.charged_text, "999999")
        self.assertEqual(_committed(self.dsn), 999999)
        blocked = self.controller.request_reserve("r6b2r1-after-cap", "tenant-a", 5)
        self.assertEqual(blocked.state, "BLOCKED")
        self.assertEqual(blocked.block_reason, "overrun_open")

    def test_rollback_then_retry_commits_once(self) -> None:
        launch_id = "r6b2r1-retry"
        self._ready(launch_id)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            self._apply(recon, "tenant-a", launch_id)
            recon.rollback()
        finally:
            recon.close()
        self.assertEqual(self.controller.get_reservation(launch_id).state, "RESERVED")
        self.assertEqual(_committed(self.dsn), 0)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            self._apply(recon, "tenant-a", launch_id)
            recon.commit()
        finally:
            recon.close()
        self.assertEqual(self.controller.get_reservation(launch_id).state, "COMMITTED")
        self.assertEqual(_committed(self.dsn), 4)

    def test_runtime_public_and_set_role_denied(self) -> None:
        owner = _owner_connect(self.dsn)
        try:
            for name in ("budget_reconciler_apply_usage", "launch_budget_settle"):
                if name == "launch_budget_settle":
                    signature = (
                        "autopilot_queue.launch_budget_settle(text,text,text,text,text,text,boolean,text,text,text,uuid,text)"
                    )
                else:
                    signature = (
                        "autopilot_queue.budget_reconciler_apply_usage(text,text,text,text,text,text,boolean,text,text,text)"
                    )
                allowed = owner.execute(
                    "SELECT has_function_privilege('public', %s::regprocedure, 'EXECUTE')",
                    (signature,),
                ).fetchone()[0]
                self.assertFalse(bool(allowed))
            owner_name = owner.execute(
                """
                SELECT r.rolname, r.rolsuper, r.rolbypassrls, r.rolcanlogin
                FROM pg_proc p
                JOIN pg_namespace n ON n.oid = p.pronamespace
                JOIN pg_roles r ON r.oid = p.proowner
                WHERE n.nspname = 'autopilot_queue' AND p.proname = 'budget_reconciler_apply_usage'
                """
            ).fetchone()
            self.assertEqual(owner_name[0], "autopilot_queue_definer_owner")
            self.assertFalse(bool(owner_name[1]))
            self.assertFalse(bool(owner_name[2]))
            self.assertFalse(bool(owner_name[3]))
        finally:
            owner.close()
        runtime = _connect(self.dsn, "tenant_a_runtime", "tenant_a_test")
        try:
            with self.assertRaises(Exception):
                runtime.execute("SET ROLE autopilot_queue_definer_owner")
            runtime.rollback()
            with self.assertRaises(Exception):
                self._apply(runtime, "tenant-a", "missing")
            runtime.rollback()
        finally:
            runtime.close()

    def test_http_proof_still_denied(self) -> None:
        launch_id = "r6b2r1-http"
        self._ready(launch_id)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception) as raised:
                _settle(recon, "tenant-a", launch_id, proof="HTTP_409")
            self.assertIn("invalid_caller_or_state", str(raised.exception).lower())
            recon.rollback()
        finally:
            recon.close()
        self.assertEqual(self.controller.get_reservation(launch_id).state, "RESERVED")


if __name__ == "__main__":
    unittest.main()
