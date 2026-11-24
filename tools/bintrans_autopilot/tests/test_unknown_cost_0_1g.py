"""Negative tests T-1..T-17 for unknown Cursor cost. Mock payloads and an isolated database."""

from __future__ import annotations

import datetime
import os
import threading
import unittest
from pathlib import Path

from bintrans_autopilot.budget import assess_budget, budget_windows
from bintrans_autopilot.budget_controller import (
    BLOCKED,
    COMMITTED,
    RELEASED,
    RESERVED,
    BudgetController,
    BudgetControllerError,
)
from bintrans_autopilot.contract import write_state
from bintrans_autopilot.cursor_api import CursorApiError, CursorClient, HttpResult
from bintrans_autopilot.dispatcher import Dispatcher
from bintrans_autopilot.postgres_queue import MigrationRunner
from bintrans_autopilot.tests.test_budget_controller_03c2 import (
    _AGENT,
    _RUN,
    _TENANTS,
    _arm_confirmed_launch,
    _clear_budget_fixtures,
    _enabled_config,
    _ensure_reconciler_role,
    _ensure_tenant_roles,
    _mark_unknown_verified,
    _owner_connect,
    _reconciler_connect,
    _reseed_tenant_ledgers,
    _tenant_connect,
)
from bintrans_autopilot.tests.test_dispatcher import (
    NOW,
    WINDOW,
    FakeCursor,
    FakeGithub,
    enabled_config,
    env,
    ready_issue,
    state_of,
)
from bintrans_autopilot.usage_verification import usage_verification_refs

_AGENT_B = "bc-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeee11"
_RUN_B = "run-bbbbbbbb-cccc-dddd-eeee-ffffffffffff"


def _dsn() -> str:
    return os.environ.get("BINTRANS_AUTOPILOT_QUEUE_DSN", "").strip()


class _Pages:
    def __init__(self, pages: list[dict]):
        self.pages = pages

    def request(self, method, url, headers, payload):
        page = int((payload or {}).get("page") or 1)
        return HttpResult(200, self.pages[page - 1])


def _client(pages: list[dict]) -> CursorClient:
    return CursorClient("test-key", _Pages(pages), admin_api_key="admin-key")


def _page(events: list[dict], more: bool = False) -> dict:
    return {"usageEvents": events, "pagination": {"hasNextPage": more}}


class UnknownCostClientTests(unittest.TestCase):
    def test_t1_missing_charged_cents_rejects_the_observation(self) -> None:
        client = _client([_page([{"inputTokens": 1}])])
        with self.assertRaises(CursorApiError) as raised:
            client.cloud_charged_cents(0, 1)
        self.assertEqual(raised.exception.code, "spend_amount_absent")
        now = datetime.datetime.fromisoformat("2026-10-27T12:00:00+00:00")
        decision = assess_budget(enabled_config(), client, budget_windows(now), now, 0)
        self.assertEqual(decision.reason, "spend_api_error")
        self.assertTrue(decision.preserve_task_state)
        cursor = FakeCursor()
        cursor.spend_error = CursorApiError(0, "spend_amount_absent", "chargedCents is absent")
        github = FakeGithub([ready_issue()])
        result = Dispatcher(
            enabled_config(),
            github,
            cursor,
            env(),
            NOW,
            WINDOW,
            ledger_block_reason=lambda: "",
        ).run()
        self.assertEqual(result.reason, "spend_api_error")
        self.assertEqual(cursor.creates, [])
        self.assertNotEqual(state_of(github.issues[0]).get("status"), "BUDGET_HOLD")

    def test_t2_null_charged_cents_rejects_the_observation(self) -> None:
        client = _client([_page([{"chargedCents": None}])])
        with self.assertRaises(CursorApiError) as raised:
            client.cloud_charged_cents(0, 1)
        self.assertEqual(raised.exception.code, "spend_amount_absent")

    def test_t3_blank_charged_cents_rejects_the_observation(self) -> None:
        client = _client([_page([{"chargedCents": ""}])])
        with self.assertRaises(CursorApiError) as raised:
            client.cloud_charged_cents(0, 1)
        self.assertEqual(raised.exception.code, "spend_amount_absent")

    def test_t4_zero_plus_missing_field_rejects_the_whole_page(self) -> None:
        client = _client([_page([{"chargedCents": 0}, {"inputTokens": 3}])])
        with self.assertRaises(CursorApiError) as raised:
            client.cloud_charged_cents(0, 1)
        self.assertEqual(raised.exception.code, "spend_amount_absent")

    def test_t5_explicit_zero_is_an_observation(self) -> None:
        client = _client([_page([{"chargedCents": 0}])])
        self.assertEqual(client.cloud_charged_cents(0, 1), 0.0)
        self.assertTrue(client.spend_events_lagged)

    def test_t14_missing_field_on_the_next_page_cancels_the_sum(self) -> None:
        duplicate = _client(
            [
                _page([{"chargedCents": 5}], more=True),
                _page([{"chargedCents": 5}]),
            ]
        )
        self.assertEqual(duplicate.cloud_charged_cents(0, 1), 10.0)
        missing = _client(
            [
                _page([{"chargedCents": 5}], more=True),
                _page([{"inputTokens": 1}]),
            ]
        )
        with self.assertRaises(CursorApiError) as raised:
            missing.cloud_charged_cents(0, 1)
        self.assertEqual(raised.exception.code, "spend_amount_absent")

    def test_t15_emergency_stop_does_not_cancel_or_create(self) -> None:
        issue = ready_issue()
        issue.body = write_state(
            issue.body,
            {"status": "IMPLEMENTING", "agent_id": _AGENT, "run_id": _RUN, "task_id": "BT-100"},
        )
        cursor = FakeCursor()
        cursor.cancel_run = lambda agent_id, run_id: (_ for _ in ()).throw(
            CursorApiError(0, "cancel_ambiguous", "not approved")
        )
        result = Dispatcher(
            enabled_config(),
            FakeGithub([issue]),
            cursor,
            env(AUTOPILOT_EMERGENCY_STOP="true"),
            NOW,
            WINDOW,
        ).run()
        self.assertEqual(result.reason, "emergency_stop_env")
        self.assertEqual(cursor.creates, [])
        self.assertEqual(cursor.cancels, [])

    def test_t17_dispatcher_closes_when_the_ledger_reason_cannot_be_read(self) -> None:
        source = (Path(__file__).resolve().parents[1] / "dispatcher.py").read_text(encoding="utf-8")
        self.assertNotIn("psycopg", source)
        cursor = FakeCursor()
        github = FakeGithub([ready_issue()])
        blocked = Dispatcher(enabled_config(), github, cursor, env(), NOW, WINDOW).run()
        self.assertEqual(blocked.reason, "ledger_unreadable")
        self.assertEqual(cursor.creates, [])
        self.assertNotEqual(state_of(github.issues[0]).get("status"), "BUDGET_HOLD")
        open_unknown = Dispatcher(
            enabled_config(),
            github,
            cursor,
            env(),
            NOW,
            WINDOW,
            ledger_block_reason=lambda: "unknown_usage_open",
        ).run()
        self.assertEqual(open_unknown.reason, "unknown_usage_open")
        self.assertEqual(cursor.creates, [])

    def test_unproven_hour_stays_stale(self) -> None:
        client = _client([_page([{"chargedCents": 4}])])
        now = datetime.datetime.fromisoformat("2026-10-27T12:00:00+00:00")
        decision = assess_budget(enabled_config(cursor_plan="ultra"), client, budget_windows(now), now, 0)
        self.assertEqual(decision.reason, "spend_observation_stale")
        self.assertTrue(decision.preserve_task_state)
        self.assertFalse(decision.allowed)


class UnknownCostLedgerTests(unittest.TestCase):
    dsn = ""

    @classmethod
    def setUpClass(cls) -> None:
        cls.dsn = _dsn()
        if not cls.dsn:
            return
        cls._prior = os.environ.get("AUTOPILOT_ENABLED")
        os.environ["AUTOPILOT_ENABLED"] = "true"
        cls.runner = MigrationRunner(lambda: _owner_connect(cls.dsn))
        cls.runner.up()
        _clear_budget_fixtures(cls.dsn)
        _ensure_tenant_roles(cls.dsn, cls.runner)
        _ensure_reconciler_role(cls.dsn)
        _reseed_tenant_ledgers(cls.dsn, cls.runner, config=_enabled_config())

    @classmethod
    def tearDownClass(cls) -> None:
        if not hasattr(cls, "_prior"):
            return
        if cls._prior is None:
            os.environ.pop("AUTOPILOT_ENABLED", None)
        else:
            os.environ["AUTOPILOT_ENABLED"] = cls._prior

    def setUp(self) -> None:
        if not self.dsn:
            self.skipTest("BINTRANS_AUTOPILOT_QUEUE_DSN is required")
        if self.runner.version() < 16:
            self.skipTest("migration 0016 is required")
        _clear_budget_fixtures(self.dsn)
        _reseed_tenant_ledgers(self.dsn, self.runner, config=_enabled_config())

    def test_t5_explicit_zero_commits_and_is_not_released(self) -> None:
        controller = self._controller()
        controller.request_reserve("launch-zero", "tenant-a", 4)
        row = self._commit(controller, "launch-zero", "0", _AGENT, _RUN)
        self.assertEqual(row.state, COMMITTED)
        self.assertNotEqual(row.state, RELEASED)
        self.assertFalse(row.usage_unknown)
        self.assertEqual(row.charged_text, "0")
        self.assertEqual(self._committed(), 0)

    def test_t6_token_only_payload_stays_reserved(self) -> None:
        controller = self._controller()
        controller.request_reserve("launch-tokens", "tenant-a", 4)
        before = self._committed()
        _mark_unknown_verified(controller, "launch-tokens")
        row = controller.get_reservation("launch-tokens")
        self.assertTrue(row.usage_unknown)
        self.assertEqual(row.state, RESERVED)
        self.assertEqual(self._committed(), before)

    def test_t7_runtime_release_is_refused_while_unknown(self) -> None:
        controller = self._controller()
        controller.request_reserve("launch-rel", "tenant-a", 4)
        _mark_unknown_verified(controller, "launch-rel")
        with self.assertRaises(BudgetControllerError) as raised:
            controller.release("launch-rel")
        self.assertEqual(raised.exception.code, "unknown_usage_open")
        self.assertEqual(controller.get_reservation("launch-rel").state, RESERVED)

    def test_t8_sql_release_is_refused_while_unknown(self) -> None:
        controller = self._controller()
        controller.request_reserve("launch-sql", "tenant-a", 4)
        _mark_unknown_verified(controller, "launch-sql")
        connection = _owner_connect(self.dsn)
        try:
            with self.assertRaises(Exception) as raised:
                connection.execute(
                    "SELECT autopilot_queue._budget_release_unused_reserve(%s, %s)",
                    ("tenant-a", "launch-sql"),
                )
                connection.commit()
            connection.rollback()
        finally:
            connection.close()
        self.assertIn("unknown_usage_open", str(raised.exception).lower())
        self.assertEqual(controller.get_reservation("launch-sql").state, RESERVED)
        self.assertEqual(self._committed(), 0)

    def test_t9_second_reserve_is_blocked_while_unknown(self) -> None:
        controller = self._controller()
        controller.request_reserve("launch-open", "tenant-a", 4)
        _mark_unknown_verified(controller, "launch-open")
        blocked = controller.request_reserve("launch-next", "tenant-a", 4)
        self.assertEqual(blocked.state, BLOCKED)
        self.assertEqual(blocked.block_reason, "unknown_usage_open")
        self.assertNotEqual(blocked.state, RESERVED)

    def test_t10_duplicate_verification_charges_once(self) -> None:
        controller = self._controller()
        controller.request_reserve("launch-dup", "tenant-a", 4)
        first = self._commit(controller, "launch-dup", "4", _AGENT, _RUN)
        second = self._commit(controller, "launch-dup", "4", _AGENT, _RUN)
        self.assertEqual(first.state, COMMITTED)
        self.assertEqual(second.state, COMMITTED)
        self.assertEqual(self._committed(), 4)

    def test_t10_concurrent_same_verification_charges_once(self) -> None:
        controller = self._controller()
        controller.request_reserve("launch-race", "tenant-a", 4)
        barrier = threading.Barrier(2)
        errors: list[str] = []

        def settle() -> None:
            barrier.wait(timeout=5)
            try:
                self._commit(controller, "launch-race", "4", _AGENT, _RUN)
            except BudgetControllerError as exc:
                errors.append(exc.code)

        threads = [threading.Thread(target=settle) for _ in range(2)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(10)
        self.assertTrue(all(code == "idempotency_conflict" for code in errors))
        self.assertEqual(controller.get_reservation("launch-race").state, COMMITTED)
        self.assertEqual(self._committed(), 4)

    def test_t11_late_amount_after_unknown_conflicts(self) -> None:
        controller = self._controller()
        controller.request_reserve("launch-late", "tenant-a", 4)
        _mark_unknown_verified(controller, "launch-late")
        with self.assertRaises(BudgetControllerError) as raised:
            self._commit(controller, "launch-late", "4", _AGENT, _RUN)
        self.assertEqual(raised.exception.code, "idempotency_conflict")
        row = controller.get_reservation("launch-late")
        self.assertTrue(row.usage_unknown)
        self.assertEqual(row.state, RESERVED)
        self.assertNotEqual(row.state, RELEASED)
        self.assertNotEqual(row.charged_text, "0")
        self.assertEqual(self._committed(), 0)

    def test_t12_late_larger_amount_after_commit_conflicts(self) -> None:
        controller = self._controller()
        controller.request_reserve("launch-again", "tenant-a", 4)
        self._commit(controller, "launch-again", "4", _AGENT, _RUN)
        with self.assertRaises(BudgetControllerError) as raised:
            self._commit(controller, "launch-again", "9", _AGENT, _RUN)
        self.assertEqual(raised.exception.code, "idempotency_conflict")
        self.assertEqual(self._committed(), 4)

    def test_t13_charge_after_release_does_not_increase_the_window(self) -> None:
        controller = self._controller()
        controller.request_reserve("launch-gone", "tenant-a", 4)
        released = controller.release("launch-gone")
        self.assertEqual(released.state, RELEASED)
        before = self._committed()
        with self.assertRaises(BudgetControllerError) as raised:
            self._commit(controller, "launch-gone", "7", _AGENT, _RUN)
        self.assertIn(raised.exception.code, {"invalid_state", "invalid_caller_or_state", "not_found"})
        self.assertEqual(controller.get_reservation("launch-gone").state, RELEASED)
        self.assertEqual(self._committed(), before)

    def test_t16_charged_cents_helper_rejects_null_and_blank(self) -> None:
        connection = _owner_connect(self.dsn)
        try:
            self.assertEqual(
                connection.execute("SELECT autopilot_queue._budget_charged_cents('0')").fetchone()[0],
                0,
            )
            connection.rollback()
            for value in (None, ""):
                with self.assertRaises(Exception) as raised:
                    connection.execute("SELECT autopilot_queue._budget_charged_cents(%s)", (value,))
                    connection.commit()
                connection.rollback()
                self.assertIn("invalid_amount", str(raised.exception).lower())
        finally:
            connection.close()

    def _controller(self) -> BudgetController:
        role, password = _TENANTS["tenant-a"]
        return BudgetController(
            lambda: _tenant_connect(self.dsn, role, password),
            reconcile_connect=lambda: _reconciler_connect(self.dsn),
            config=_enabled_config(),
        )

    def _commit(self, controller, launch_id: str, charged: str, agent_id: str, run_id: str):
        _arm_confirmed_launch(self.dsn, "tenant-a", launch_id, agent_id, run_id)
        payload = {"rawCostCents": charged, "chargedCents": charged}
        verification_id, payload_digest = usage_verification_refs(agent_id, run_id, payload)
        return controller.commit_usage(
            launch_id,
            raw_cost_text=charged,
            charged_text=charged,
            charged_known=True,
            agent_id=agent_id,
            run_id=run_id,
            verification_id=verification_id,
            payload_digest=payload_digest,
            source="unknown_cost_test",
        )

    def _committed(self) -> int:
        connection = _owner_connect(self.dsn)
        try:
            row = connection.execute(
                """
                SELECT COALESCE(sum(committed_cents), 0)
                FROM autopilot_queue.budget_window_usage
                WHERE scope = 'tenant' AND tenant_id = 'tenant-a' AND bucket = 'day'
                """
            ).fetchone()
            connection.rollback()
        finally:
            connection.close()
        return int(row[0])
