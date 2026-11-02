"""R6B-2 launch-gated settlement. Isolated PostgreSQL only."""

from __future__ import annotations

import os
import unittest
import uuid

from bintrans_autopilot.postgres_queue import MigrationRunner
from bintrans_autopilot.tests.test_budget_controller_03c2 import (
    _AGENT,
    _RUN,
    _RECONCILER_LOGIN,
    _RECONCILER_PASSWORD,
    _ensure_reconciler_role,
    _ensure_tenant_roles,
    _owner_connect,
    _reseed_tenant_ledgers,
    _tenant_controller,
)
from bintrans_autopilot.tests.test_launch_lifecycle_r6b import (
    _ADAPTER_LOGIN,
    _connect,
    _dsn,
    _ensure_launch_roles,
    _record_intent,
)
_DIGEST = "b" * 64


def _confirm(dsn: str, tenant: str, launch_id: str) -> None:
    adapter = _connect(dsn, _ADAPTER_LOGIN, _ADAPTER_LOGIN)
    try:
        adapter.execute(
            "SELECT autopilot_queue.launch_protocol_mark_send_started(%s, %s)",
            (tenant, launch_id),
        )
        adapter.execute(
            "SELECT autopilot_queue.launch_protocol_confirm(%s, %s, %s, %s)",
            (tenant, launch_id, _AGENT, _RUN),
        )
        adapter.commit()
    finally:
        adapter.close()


def _settle(conn, tenant: str, launch_id: str, *, charged: str = "4", proof: str = "MOCK_TRUSTED", lease=None, verification: str = "verify-r6b2-01"):
    conn.execute(
        """
        SELECT autopilot_queue.launch_budget_settle(
            %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
        )
        """,
        (
            tenant,
            launch_id,
            _AGENT,
            _RUN,
            charged,
            charged,
            True,
            verification,
            _DIGEST,
            proof,
            lease,
            "cursor_usage_api",
        ),
    )


class LaunchGatedSettlementR6B2Tests(unittest.TestCase):
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
        if cls.runner.version() < 14:
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
            self.skipTest("r6b2_postgres_unavailable")
        owner = _owner_connect(self.dsn)
        try:
            owner.execute("DELETE FROM autopilot_queue.launch_intents")
            owner.execute("DELETE FROM autopilot_queue.budget_verified_usage")
            owner.execute("DELETE FROM autopilot_queue.budget_reservations")
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

    def test_valid_mock_settlement_is_idempotent(self) -> None:
        launch_id = "r6b2-ok"
        self._ready(launch_id)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            _settle(recon, "tenant-a", launch_id)
            _settle(recon, "tenant-a", launch_id)
            recon.commit()
        finally:
            recon.close()
        self.assertEqual(self.controller.get_reservation(launch_id).state, "COMMITTED")

    def test_conflicting_retry_rejected(self) -> None:
        launch_id = "r6b2-conflict"
        self._ready(launch_id)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            _settle(recon, "tenant-a", launch_id, charged="4")
            with self.assertRaises(Exception):
                _settle(recon, "tenant-a", launch_id, charged="9", verification="verify-r6b2-other")
            recon.rollback()
        finally:
            recon.close()

    def test_missing_intent_denied(self) -> None:
        launch_id = "r6b2-no-intent"
        self.controller.request_reserve(launch_id, "tenant-a", 10)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception) as raised:
                _settle(recon, "tenant-a", launch_id)
            self.assertIn("not_found", str(raised.exception).lower())
            recon.rollback()
        finally:
            recon.close()
        self.assertEqual(self.controller.get_reservation(launch_id).state, "RESERVED")

    def test_wrong_tenant_denied(self) -> None:
        launch_id = "r6b2-cross"
        self._ready(launch_id)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception):
                _settle(recon, "tenant-b", launch_id)
            recon.rollback()
        finally:
            recon.close()
        self.assertEqual(self.controller.get_reservation(launch_id).state, "RESERVED")

    def test_unknown_proof_and_estop_and_lease_denied(self) -> None:
        launch_id = "r6b2-deny"
        self._ready(launch_id)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception):
                _settle(recon, "tenant-a", launch_id, proof="HTTP_409")
            recon.rollback()
            with self.assertRaises(Exception):
                _settle(recon, "tenant-a", launch_id, proof="TIMEOUT")
            recon.rollback()
        finally:
            recon.close()
        owner = _owner_connect(self.dsn)
        try:
            owner.execute(
                """
                UPDATE autopilot_queue.autopilot_launch_control
                SET autopilot_estop = true WHERE id = 1
                """
            )
            owner.commit()
        finally:
            owner.close()
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception) as raised:
                _settle(recon, "tenant-a", launch_id)
            self.assertIn("estop_active", str(raised.exception).lower())
            recon.rollback()
        finally:
            recon.close()
        owner = _owner_connect(self.dsn)
        try:
            token = str(uuid.uuid4())
            owner.execute(
                """
                UPDATE autopilot_queue.autopilot_launch_control
                SET autopilot_estop = false WHERE id = 1
                """
            )
            owner.execute(
                """
                UPDATE autopilot_queue.launch_intents
                SET recovery_lease_token = %s,
                    recovery_lease_expires_at = clock_timestamp() - interval '1 minute'
                WHERE tenant_id = 'tenant-a' AND launch_id = %s
                """,
                (token, launch_id),
            )
            owner.commit()
        finally:
            owner.close()
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception) as raised:
                _settle(recon, "tenant-a", launch_id, lease=token)
            self.assertIn("invalid_lease", str(raised.exception).lower())
            recon.rollback()
        finally:
            recon.close()
        self.assertEqual(self.controller.get_reservation(launch_id).state, "RESERVED")

    def test_rollback_does_not_publish_settlement(self) -> None:
        launch_id = "r6b2-rollback"
        self._ready(launch_id)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            _settle(recon, "tenant-a", launch_id)
            recon.rollback()
        finally:
            recon.close()
        self.assertEqual(self.controller.get_reservation(launch_id).state, "RESERVED")

    def test_public_and_runtime_cannot_settle(self) -> None:
        owner = _owner_connect(self.dsn)
        try:
            allowed = owner.execute(
                """
                SELECT has_function_privilege(
                    'public',
                    'autopilot_queue.launch_budget_settle(text,text,text,text,text,text,boolean,text,text,text,uuid,text)'::regprocedure,
                    'EXECUTE'
                )
                """
            ).fetchone()[0]
            self.assertFalse(bool(allowed))
            owner_name = owner.execute(
                """
                SELECT r.rolname, r.rolsuper, r.rolbypassrls, r.rolcanlogin
                FROM pg_proc p
                JOIN pg_namespace n ON n.oid = p.pronamespace
                JOIN pg_roles r ON r.oid = p.proowner
                WHERE n.nspname = 'autopilot_queue' AND p.proname = 'launch_budget_settle'
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
                _settle(runtime, "tenant-a", "missing")
            runtime.rollback()
        finally:
            runtime.close()

    def test_unconfirmed_launch_denied(self) -> None:
        launch_id = "r6b2-intent-only"
        self.controller.request_reserve(launch_id, "tenant-a", 8)
        _record_intent(self.dsn, "tenant-a", launch_id)
        recon = _connect(self.dsn, _RECONCILER_LOGIN, _RECONCILER_PASSWORD)
        try:
            with self.assertRaises(Exception) as raised:
                _settle(recon, "tenant-a", launch_id)
            self.assertIn("invalid_caller_or_state", str(raised.exception).lower())
            recon.rollback()
        finally:
            recon.close()
        self.assertEqual(self.controller.get_reservation(launch_id).state, "RESERVED")


if __name__ == "__main__":
    unittest.main()
