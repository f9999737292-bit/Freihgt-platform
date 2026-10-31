"""PostgreSQL-backed budget reservation for BINTRANS Autopilot phase 0.3C2."""

from __future__ import annotations

import logging
import os
import re
import threading
from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import ROUND_CEILING, Decimal, InvalidOperation
from contextlib import contextmanager
from typing import Callable, Iterator

from bintrans_autopilot.config import (
    EMERGENCY_STOP_FILE,
    AutopilotConfig,
    load_config,
    runtime_enabled,
)
from bintrans_autopilot.postgres_queue import RECONCILER_ROLE, RUNTIME_ROLE

logger = logging.getLogger("bintrans_autopilot.budget_controller")

_LAUNCH_ID = re.compile(r"^[A-Za-z0-9._:-]{1,120}$")
_TENANT = re.compile(r"^[A-Za-z0-9._:-]{1,80}$")
_AGENT_ID = re.compile(r"^bc-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
_RUN_ID = re.compile(r"^run-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")

RESERVED = "RESERVED"
COMMITTED = "COMMITTED"
RELEASED = "RELEASED"
BLOCKED = "BLOCKED"
REQUESTED = "REQUESTED"
OVERRUN = "OVERRUN"


class BudgetControllerError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


@dataclass(frozen=True)
class BudgetLimits:
    per_launch_cap_cents: int
    daily_limit_cents: int
    monthly_limit_cents: int


@dataclass(frozen=True)
class KillSwitchDecision:
    allowed: bool
    reason: str


@dataclass(frozen=True)
class ReservationView:
    launch_id: str
    tenant_id: str
    state: str
    reserve_cents: int
    raw_cost_text: str | None
    charged_text: str | None
    charged_known: bool
    agent_id: str | None
    run_id: str | None
    block_reason: str | None
    usage_unknown: bool = False
    ledger_utc_day: object | None = None
    ledger_utc_month: object | None = None


def limits_from_config(config: AutopilotConfig) -> BudgetLimits:
    return BudgetLimits(
        per_launch_cap_cents=_config_cents(config.budget_max_charged_cents),
        daily_limit_cents=_config_cents(config.budget_max_charged_cents_daily),
        monthly_limit_cents=_config_cents(config.budget_max_charged_cents_monthly),
    )


def _config_cents(value: float) -> int:
    if value is None:
        return 0
    amount = Decimal(str(value))
    if amount <= 0:
        return 0
    return int(amount.to_integral_value())


def _decimal_text(value: object | None) -> str | None:
    if value is None:
        return None
    if isinstance(value, bool):
        raise BudgetControllerError("invalid_amount")
    try:
        amount = Decimal(str(value).strip())
    except (InvalidOperation, AttributeError) as exc:
        raise BudgetControllerError("invalid_amount") from exc
    if not amount.is_finite() or amount < 0:
        raise BudgetControllerError("invalid_amount")
    text = format(amount, "f")
    if "." in text:
        text = text.rstrip("0").rstrip(".")
    return text or "0"


def _utc_windows(now: datetime) -> tuple[datetime, datetime]:
    if now.tzinfo is None:
        now = now.replace(tzinfo=timezone.utc)
    now = now.astimezone(timezone.utc)
    day = datetime(now.year, now.month, now.day, tzinfo=timezone.utc).date()
    month = datetime(now.year, now.month, 1, tzinfo=timezone.utc).date()
    return day, month


def _rollover_counters(ledger: dict, day, month) -> dict:
    updated = dict(ledger)
    if updated["utc_month"] != month:
        updated["utc_month"] = month
        updated["monthly_reserved_cents"] = 0
        updated["monthly_committed_cents"] = 0
    if updated["utc_day"] != day:
        updated["utc_day"] = day
        updated["daily_reserved_cents"] = 0
        updated["daily_committed_cents"] = 0
    return updated


class BudgetController:
    """Dry-run budget controller. Mutations require a tenant-bound runtime connection."""

    def __init__(
        self,
        connect: Callable,
        *,
        admin_connect: Callable | None = None,
        reconcile_connect: Callable | None = None,
        mode: str = "dry_run",
        config: AutopilotConfig | None = None,
        now: Callable[[], datetime] | None = None,
    ):
        if mode != "dry_run":
            raise BudgetControllerError("controller_disabled")
        for name in (
            "PAID_AUTOMATION_ENABLED",
            "EXTERNAL_TOOLS_ENABLED",
            "AUTO_MERGE_ENABLED",
            "AUTO_DEPLOY_ENABLED",
            "WORKER_ENABLED",
        ):
            if os.environ.get(name, "").strip().lower() == "true":
                raise BudgetControllerError("controller_disabled")
        self._connect = connect
        self._admin_connect = admin_connect
        self._reconcile_connect = reconcile_connect
        self._config = config or load_config()
        self._limits = limits_from_config(self._config)
        self._now = now or (lambda: datetime.now(timezone.utc))
        self._lock = threading.Lock()
        self.cloud_agents_started = 0
        self.autopilot_enabled = False
        self.paid_automation_enabled = False

    def evaluate_kill_switch(self) -> KillSwitchDecision:
        try:
            with self._cursor() as cursor:
                return self._kill_switch_for_cursor(cursor)
        except BudgetControllerError as exc:
            if exc.code == "store_unavailable":
                return KillSwitchDecision(False, "budget_store_unavailable")
            raise

    def sync_tenant_limits(self, tenant_id: str) -> None:
        if self._admin_connect is None:
            raise BudgetControllerError("admin_required")
        tenant_id = _tenant_id(tenant_id)
        day, month = _utc_windows(self._now())
        with self._cursor(self._admin_connect) as cursor:
            self._ensure_ledger_row(cursor, tenant_id, day, month)
            cursor.execute(
                """
                UPDATE autopilot_queue.budget_tenant_ledger
                SET daily_limit_cents = %s,
                    monthly_limit_cents = %s,
                    utc_day = %s,
                    utc_month = %s
                WHERE tenant_id = %s
                """,
                (
                    self._limits.daily_limit_cents,
                    self._limits.monthly_limit_cents,
                    day,
                    month,
                    tenant_id,
                ),
            )
            cursor.execute(
                """
                UPDATE autopilot_queue.budget_global_ledger
                SET daily_limit_cents = %s,
                    monthly_limit_cents = %s,
                    per_launch_cap_cents = %s,
                    utc_day = %s,
                    utc_month = %s
                WHERE id = 1
                """,
                (
                    self._limits.daily_limit_cents,
                    self._limits.monthly_limit_cents,
                    self._limits.per_launch_cap_cents,
                    day,
                    month,
                ),
            )

    def request_reserve(
        self,
        launch_id: str,
        tenant_id: str,
        reserve_cents: int,
        *,
        agent_id: str | None = None,
        run_id: str | None = None,
    ) -> ReservationView:
        launch_id = _launch_id(launch_id)
        tenant_id = _tenant_id(tenant_id)
        reserve_cents = _reserve_cents(reserve_cents)
        agent_id = _optional_agent_id(agent_id)
        run_id = _optional_run_id(run_id)
        with self._lock:
            with self._cursor() as cursor:
                _assert_budget_runtime_binding(cursor, tenant_id)
                existing = self._fetch_reservation(cursor, launch_id)
                if existing is not None:
                    if existing.tenant_id == tenant_id and existing.reserve_cents == reserve_cents:
                        return existing
                    raise BudgetControllerError("idempotency_conflict")
                gate = self._kill_switch_for_cursor(cursor)
                if not gate.allowed:
                    return self._block_in_tx(cursor, launch_id, tenant_id, reserve_cents, gate.reason)
                cursor.execute("SAVEPOINT budget_reserve")
                try:
                    cursor.execute(
                        """
                        SELECT autopilot_queue.budget_protocol_reserve(
                            %s, %s, %s, %s
                        )
                        """,
                        (launch_id, reserve_cents, agent_id, run_id),
                    )
                except Exception as exc:
                    cursor.execute("ROLLBACK TO SAVEPOINT budget_reserve")
                    code = _exception_code(exc)
                    if code in {
                        "global_budget_exceeded",
                        "tenant_budget_exceeded",
                        "per_launch_cap_exceeded",
                    }:
                        return self._insert_blocked(cursor, launch_id, tenant_id, reserve_cents, code)
                    if code in {"overrun_open", "unknown_usage_open"}:
                        return self._insert_blocked(cursor, launch_id, tenant_id, reserve_cents, code)
                    raise _budget_error_from_exception(exc) from exc
                logger.info(
                    "event=budget_reserved tenant=%s launch=%s reserve_cents=%s",
                    tenant_id,
                    launch_id,
                    reserve_cents,
                )
                return self._require_fetch(cursor, launch_id)

    def commit_usage(
        self,
        launch_id: str,
        *,
        raw_cost_text: str | None,
        charged_text: str | None,
        charged_known: bool,
        agent_id: str | None = None,
        run_id: str | None = None,
        verification_id: str | None = None,
        payload_digest: str | None = None,
        source: str = "cursor_usage_api",
    ) -> ReservationView:
        launch_id = _launch_id(launch_id)
        if not charged_known:
            raise BudgetControllerError("unknown_usage")
        agent_id = _optional_agent_id(agent_id)
        run_id = _optional_run_id(run_id)
        if agent_id is None or run_id is None:
            raise BudgetControllerError("invalid_identity")
        if not verification_id or not payload_digest:
            raise BudgetControllerError("verified_usage_required")
        raw_cost_text = _decimal_text(raw_cost_text) if raw_cost_text is not None else None
        charged_text = _decimal_text(charged_text) if charged_text is not None else None
        if charged_text is None:
            raise BudgetControllerError("unknown_usage")
        with self._cursor() as cursor:
            row = self._fetch_reservation(cursor, launch_id)
            if row is None:
                raise BudgetControllerError("not_found")
            if row.state == COMMITTED:
                if (
                    row.raw_cost_text == raw_cost_text
                    and row.charged_text == charged_text
                    and row.charged_known == charged_known
                    and row.agent_id == agent_id
                    and row.run_id == run_id
                ):
                    return row
                raise BudgetControllerError("idempotency_conflict")
            if row.state == OVERRUN:
                if row.charged_text == charged_text and row.agent_id == agent_id and row.run_id == run_id:
                    return row
                raise BudgetControllerError("idempotency_conflict")
            if row.agent_id and agent_id and row.agent_id != agent_id:
                raise BudgetControllerError("identity_mismatch")
            if row.run_id and run_id and row.run_id != run_id:
                raise BudgetControllerError("identity_mismatch")
            reserve = Decimal(row.reserve_cents)
            charged_amount = Decimal(charged_text)
            over_reserve = charged_amount > reserve
        with self._reconciler_cursor() as cursor:
            try:
                cursor.execute(
                    """
                    SELECT autopilot_queue.budget_reconciler_apply_usage(
                        %s, %s, %s, %s, %s, %s, %s, %s, %s
                    )
                    """,
                    (
                        launch_id,
                        agent_id,
                        run_id,
                        raw_cost_text,
                        charged_text,
                        True,
                        verification_id,
                        payload_digest,
                        source,
                    ),
                )
            except Exception as exc:
                raise _budget_error_from_exception(exc) from exc
        if over_reserve:
            logger.warning(
                "event=budget_overrun tenant=%s launch=%s reserve=%s charged=%s",
                row.tenant_id,
                launch_id,
                row.reserve_cents,
                charged_text,
            )
        with self._cursor() as cursor:
            return self._require_fetch(cursor, launch_id)

    def release(self, launch_id: str) -> ReservationView:
        launch_id = _launch_id(launch_id)
        with self._cursor() as cursor:
            row = self._fetch_reservation(cursor, launch_id)
            if row is None:
                raise BudgetControllerError("not_found")
            _assert_budget_runtime_binding(cursor, row.tenant_id)
            if row.state == RELEASED:
                return row
            if row.state == OVERRUN:
                raise BudgetControllerError("overrun_open")
            if row.state != RESERVED:
                raise BudgetControllerError("invalid_state")
            try:
                cursor.execute(
                    "SELECT autopilot_queue.budget_protocol_release(%s)",
                    (launch_id,),
                )
            except Exception as exc:
                raise _budget_error_from_exception(exc) from exc
            return self._require_fetch(cursor, launch_id)

    def mark_usage_unknown(
        self,
        launch_id: str,
        *,
        agent_id: str,
        run_id: str,
        raw_cost_text: str | None,
        verification_id: str,
        payload_digest: str,
        source: str = "cursor_usage_api",
    ) -> None:
        launch_id = _launch_id(launch_id)
        agent_id = _optional_agent_id(agent_id)
        run_id = _optional_run_id(run_id)
        if agent_id is None or run_id is None:
            raise BudgetControllerError("invalid_identity")
        if not verification_id or not payload_digest:
            raise BudgetControllerError("verified_usage_required")
        raw_cost_text = _decimal_text(raw_cost_text) if raw_cost_text is not None else None
        with self._cursor() as cursor:
            row = self._fetch_reservation(cursor, launch_id)
            if row is None:
                raise BudgetControllerError("not_found")
            if row.state != RESERVED and not row.usage_unknown:
                raise BudgetControllerError("invalid_state")
            if row.agent_id and row.agent_id != agent_id:
                raise BudgetControllerError("identity_mismatch")
            if row.run_id and row.run_id != run_id:
                raise BudgetControllerError("identity_mismatch")
        with self._reconciler_cursor() as cursor:
            try:
                cursor.execute(
                    """
                    SELECT autopilot_queue.budget_reconciler_apply_usage(
                        %s, %s, %s, %s, %s, %s, %s, %s, %s
                    )
                    """,
                    (
                        launch_id,
                        agent_id,
                        run_id,
                        raw_cost_text,
                        None,
                        False,
                        verification_id,
                        payload_digest,
                        source,
                    ),
                )
            except Exception as exc:
                raise _budget_error_from_exception(exc) from exc

    def get_reservation(self, launch_id: str) -> ReservationView | None:
        launch_id = _launch_id(launch_id)
        with self._cursor() as cursor:
            return self._fetch_reservation(cursor, launch_id)

    def _kill_switch_for_cursor(self, cursor) -> KillSwitchDecision:
        if os.environ.get("PAID_AUTOMATION_ENABLED", "").strip().lower() == "true":
            return KillSwitchDecision(False, "paid_automation_disabled")
        if os.environ.get("AUTOPILOT_EMERGENCY_STOP", "").strip().lower() == "true":
            return KillSwitchDecision(False, "emergency_stop_env")
        if self._config.emergency_stop or EMERGENCY_STOP_FILE.exists():
            return KillSwitchDecision(False, "emergency_stop")
        if (
            self._limits.per_launch_cap_cents <= 0
            or self._limits.daily_limit_cents <= 0
            or self._limits.monthly_limit_cents <= 0
        ):
            return KillSwitchDecision(False, "zero_budget")
        enabled, reason = runtime_enabled(self._config, os.environ)
        if not enabled:
            return KillSwitchDecision(False, reason)
        reason = self._global_safety_block_reason(cursor)
        if reason:
            return KillSwitchDecision(False, reason)
        return KillSwitchDecision(True, "allowed")

    def _block_in_tx(
        self,
        cursor,
        launch_id: str,
        tenant_id: str,
        reserve_cents: int,
        reason: str,
    ) -> ReservationView:
        existing = self._fetch_reservation(cursor, launch_id)
        if existing is not None:
            if existing.state == BLOCKED and existing.block_reason == reason:
                return existing
            raise BudgetControllerError("idempotency_conflict")
        return self._insert_blocked(cursor, launch_id, tenant_id, reserve_cents, reason)

    def _insert_blocked(
        self,
        cursor,
        launch_id: str,
        tenant_id: str,
        reserve_cents: int,
        reason: str,
    ) -> ReservationView:
        cursor.execute(
            """
            SELECT autopilot_queue.budget_protocol_insert_blocked(
                %s, %s, %s
            )
            """,
            (launch_id, reserve_cents, reason),
        )
        logger.info(
            "event=budget_blocked tenant=%s launch=%s reason=%s",
            tenant_id,
            launch_id,
            reason,
        )
        return self._require_fetch(cursor, launch_id)

    def _global_safety_block_reason(self, cursor) -> str | None:
        cursor.execute("SELECT autopilot_queue.budget_global_safety_block_reason() AS reason")
        row = cursor.fetchone()
        if row is None:
            return None
        data = _row_dict(row, cursor)
        reason = data.get("reason")
        return str(reason) if reason else None

    def _projected_exceeds(self, ledger: dict, reserve_cents: int) -> bool:
        daily = int(ledger["daily_reserved_cents"]) + int(ledger["daily_committed_cents"]) + reserve_cents
        monthly = int(ledger["monthly_reserved_cents"]) + int(ledger["monthly_committed_cents"]) + reserve_cents
        return daily > int(ledger["daily_limit_cents"]) or monthly > int(ledger["monthly_limit_cents"])

    def _ensure_ledger_row(self, cursor, tenant_id: str, day, month) -> None:
        cursor.execute(
            """
            INSERT INTO autopilot_queue.budget_tenant_ledger (
                tenant_id, utc_day, utc_month, daily_limit_cents, monthly_limit_cents
            ) VALUES (%s, %s, %s, %s, %s)
            ON CONFLICT (tenant_id) DO NOTHING
            """,
            (
                tenant_id,
                day,
                month,
                self._limits.daily_limit_cents,
                self._limits.monthly_limit_cents,
            ),
        )

    def _lock_ledger(self, cursor, tenant_id: str, day, month) -> dict:
        self._ensure_ledger_row(cursor, tenant_id, day, month)
        cursor.execute(
            """
            SELECT tenant_id, utc_day, utc_month,
                   daily_limit_cents, monthly_limit_cents,
                   daily_reserved_cents, monthly_reserved_cents,
                   daily_committed_cents, monthly_committed_cents
            FROM autopilot_queue.budget_tenant_ledger
            WHERE tenant_id = %s
            FOR UPDATE
            """,
            (tenant_id,),
        )
        row = cursor.fetchone()
        if row is None:
            raise BudgetControllerError("store_unavailable")
        ledger = _row_dict(row, cursor)
        rolled = _rollover_counters(ledger, day, month)
        if rolled != ledger:
            cursor.execute(
                """
                UPDATE autopilot_queue.budget_tenant_ledger
                SET utc_day = %s,
                    utc_month = %s,
                    daily_reserved_cents = %s,
                    monthly_reserved_cents = %s,
                    daily_committed_cents = %s,
                    monthly_committed_cents = %s,
                    daily_limit_cents = %s,
                    monthly_limit_cents = %s
                WHERE tenant_id = %s
                """,
                (
                    rolled["utc_day"],
                    rolled["utc_month"],
                    rolled["daily_reserved_cents"],
                    rolled["monthly_reserved_cents"],
                    rolled["daily_committed_cents"],
                    rolled["monthly_committed_cents"],
                    self._limits.daily_limit_cents,
                    self._limits.monthly_limit_cents,
                    tenant_id,
                ),
            )
            ledger = rolled
        return ledger

    def _lock_global_ledger(self, cursor, day, month) -> dict:
        ledger = self._lock_global_ledger_window(cursor, day, month)
        rolled = _rollover_counters(ledger, day, month)
        if rolled != ledger:
            self._write_global_ledger(cursor, rolled)
            return rolled
        return ledger

    def _lock_global_ledger_window(self, cursor, day, month) -> dict:
        cursor.execute("SELECT * FROM autopilot_queue.budget_global_lock_row()")
        row = cursor.fetchone()
        if row is None:
            raise BudgetControllerError("store_unavailable")
        ledger = _row_dict(row, cursor)
        if ledger["utc_day"] != day or ledger["utc_month"] != month:
            ledger = {**ledger, "utc_day": day, "utc_month": month}
        return ledger

    def _lock_ledger_window(self, cursor, tenant_id: str, day, month) -> dict:
        self._ensure_ledger_row(cursor, tenant_id, day, month)
        cursor.execute(
            """
            SELECT tenant_id, utc_day, utc_month,
                   daily_limit_cents, monthly_limit_cents,
                   daily_reserved_cents, monthly_reserved_cents,
                   daily_committed_cents, monthly_committed_cents
            FROM autopilot_queue.budget_tenant_ledger
            WHERE tenant_id = %s
            FOR UPDATE
            """,
            (tenant_id,),
        )
        row = cursor.fetchone()
        if row is None:
            raise BudgetControllerError("store_unavailable")
        ledger = _row_dict(row, cursor)
        if ledger["utc_day"] != day or ledger["utc_month"] != month:
            cursor.execute(
                """
                UPDATE autopilot_queue.budget_tenant_ledger
                SET utc_day = %s, utc_month = %s
                WHERE tenant_id = %s
                """,
                (day, month, tenant_id),
            )
            ledger["utc_day"] = day
            ledger["utc_month"] = month
        return ledger

    def _write_global_ledger(self, cursor, ledger: dict) -> None:
        try:
            cursor.execute(
                """
                SELECT autopilot_queue.budget_global_write_counters(
                    %s, %s, %s, %s, %s, %s
                )
                """,
                (
                    ledger["utc_day"],
                    ledger["utc_month"],
                    ledger["daily_reserved_cents"],
                    ledger["monthly_reserved_cents"],
                    ledger["daily_committed_cents"],
                    ledger["monthly_committed_cents"],
                ),
            )
        except Exception as exc:
            if _sqlstate(exc) == "global_budget_exceeded":
                raise BudgetControllerError("global_budget_exceeded") from exc
            raise BudgetControllerError("store_unavailable") from exc

    def _add_global_reserved(self, cursor, ledger: dict, day, month, cents: int) -> None:
        rolled = _rollover_counters(ledger, day, month)
        rolled["daily_reserved_cents"] = int(rolled["daily_reserved_cents"]) + cents
        rolled["monthly_reserved_cents"] = int(rolled["monthly_reserved_cents"]) + cents
        self._write_global_ledger(cursor, rolled)

    def _release_global_reserved(self, cursor, ledger: dict, day, month, cents: int) -> None:
        rolled = _rollover_counters(ledger, day, month)
        rolled["daily_reserved_cents"] = int(rolled["daily_reserved_cents"]) - cents
        rolled["monthly_reserved_cents"] = int(rolled["monthly_reserved_cents"]) - cents
        self._write_global_ledger(cursor, rolled)

    def _move_reserved_to_committed(
        self,
        cursor,
        global_ledger: dict,
        day,
        month,
        tenant_id: str,
        reserve_cents: int,
        charged_cents: int,
    ) -> None:
        global_next = _rollover_counters(global_ledger, day, month)
        global_next["daily_reserved_cents"] = int(global_next["daily_reserved_cents"]) - reserve_cents
        global_next["monthly_reserved_cents"] = int(global_next["monthly_reserved_cents"]) - reserve_cents
        global_next["daily_committed_cents"] = int(global_next["daily_committed_cents"]) + charged_cents
        global_next["monthly_committed_cents"] = int(global_next["monthly_committed_cents"]) + charged_cents
        self._write_global_ledger(cursor, global_next)
        cursor.execute(
            """
            UPDATE autopilot_queue.budget_tenant_ledger
            SET daily_reserved_cents = daily_reserved_cents - %s,
                monthly_reserved_cents = monthly_reserved_cents - %s,
                daily_committed_cents = daily_committed_cents + %s,
                monthly_committed_cents = monthly_committed_cents + %s
            WHERE tenant_id = %s
            """,
            (reserve_cents, reserve_cents, charged_cents, charged_cents, tenant_id),
        )

    def _lock_reservation(self, cursor, launch_id: str) -> ReservationView:
        cursor.execute(
            """
            SELECT launch_id, tenant_id, state, reserve_cents,
                   raw_cost_text, charged_text, charged_known,
                   agent_id, run_id, block_reason,
                   usage_unknown, ledger_utc_day, ledger_utc_month
            FROM autopilot_queue.budget_reservations
            WHERE launch_id = %s
            FOR UPDATE
            """,
            (launch_id,),
        )
        row = cursor.fetchone()
        if row is None:
            raise BudgetControllerError("not_found")
        return _reservation_from_row(row, cursor)

    def _fetch_reservation(self, cursor, launch_id: str) -> ReservationView | None:
        cursor.execute(
            """
            SELECT launch_id, tenant_id, state, reserve_cents,
                   raw_cost_text, charged_text, charged_known,
                   agent_id, run_id, block_reason,
                   usage_unknown, ledger_utc_day, ledger_utc_month
            FROM autopilot_queue.budget_reservations
            WHERE launch_id = %s
            """,
            (launch_id,),
        )
        row = cursor.fetchone()
        if row is None:
            return None
        return _reservation_from_row(row, cursor)

    def _require_fetch(self, cursor, launch_id: str) -> ReservationView:
        row = self._fetch_reservation(cursor, launch_id)
        if row is None:
            raise BudgetControllerError("store_unavailable")
        return row

    @contextmanager
    def _reconciler_cursor(self) -> Iterator:
        if self._reconcile_connect is None:
            raise BudgetControllerError("reconciler_required")
        with self._cursor(self._reconcile_connect) as cursor:
            _assert_reconciler_session(cursor)
            yield cursor

    @contextmanager
    def _cursor(self, connect: Callable | None = None) -> Iterator:
        from bintrans_autopilot.postgres_queue import _cursor_for

        factory = connect or self._connect
        try:
            connection = factory()
        except Exception as exc:
            raise BudgetControllerError("store_unavailable") from exc
        try:
            with _cursor_for(connection) as cursor:
                yield cursor
            connection.commit()
        except BudgetControllerError:
            connection.rollback()
            raise
        except Exception as exc:
            connection.rollback()
            if _is_unique_violation(exc):
                raise BudgetControllerError("idempotency_conflict") from exc
            raise _budget_error_from_exception(exc) from exc
        finally:
            connection.close()


def _assert_reconciler_session(cursor) -> None:
    cursor.execute(
        """
        SELECT session_user AS session_user,
               EXISTS (
                   SELECT 1
                   FROM pg_auth_members m
                   JOIN pg_roles member ON member.oid = m.member
                   JOIN pg_roles granted ON granted.oid = m.roleid
                   WHERE member.rolname = session_user
                     AND granted.rolname = %s
               ) AS is_reconciler
        """,
        (RECONCILER_ROLE,),
    )
    row = cursor.fetchone()
    if row is not None and not isinstance(row, dict):
        row = _row_dict(row, cursor)
    if row is None or not row["is_reconciler"]:
        raise BudgetControllerError("reconciler_required")


def _assert_budget_runtime_binding(cursor, tenant_id: str) -> None:
    cursor.execute(
        """
        SELECT session_role.rolsuper AS session_super,
               session_role.rolbypassrls AS session_bypass,
               session_role.rolcreaterole AS session_createrole,
               session_role.rolcreatedb AS session_createdb,
               session_role.rolreplication AS session_replication,
               active_role.rolsuper AS current_super,
               active_role.rolbypassrls AS current_bypass,
               active_role.rolcreaterole AS current_createrole,
               active_role.rolcreatedb AS current_createdb,
               active_role.rolreplication AS current_replication,
               session_user AS session_user,
               current_user AS current_user,
               current_setting('row_security') AS row_security,
               pg_get_userbyid(reservations.relowner) AS owner_name,
               reservations.relrowsecurity AS row_level_enabled,
               reservations.relforcerowsecurity AS row_level_forced
        FROM pg_roles AS session_role
        JOIN pg_roles AS active_role ON active_role.rolname = current_user
        JOIN pg_class AS reservations
          ON reservations.relname = 'budget_reservations'
        JOIN pg_namespace AS namespace
          ON namespace.oid = reservations.relnamespace AND namespace.nspname = 'autopilot_queue'
        WHERE session_role.rolname = session_user
        """
    )
    row = cursor.fetchone()
    if row is not None and not isinstance(row, dict):
        row = _row_dict(row, cursor)
    privileged = (
        "session_super",
        "session_bypass",
        "session_createrole",
        "session_createdb",
        "session_replication",
        "current_super",
        "current_bypass",
        "current_createrole",
        "current_createdb",
        "current_replication",
    )
    if (
        row is None
        or row["row_security"] != "on"
        or not row["row_level_enabled"]
        or not row["row_level_forced"]
        or any(row[name] for name in privileged)
        or row["session_user"] in {row["owner_name"], RUNTIME_ROLE}
        or row["current_user"] == row["owner_name"]
    ):
        raise BudgetControllerError("runtime_privileged")
    cursor.execute("SELECT autopilot_queue.bound_tenant() AS tenant_id")
    bound_row = cursor.fetchone()
    if bound_row is not None and not isinstance(bound_row, dict):
        bound_row = _row_dict(bound_row, cursor)
    if bound_row is None or not bound_row["tenant_id"]:
        raise BudgetControllerError("tenant_unbound")
    if bound_row["tenant_id"] != tenant_id:
        raise BudgetControllerError("tenant_mismatch")


def _reserve_cents(value: int) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise BudgetControllerError("invalid_amount")
    if value <= 0:
        raise BudgetControllerError("invalid_amount")
    return value


def _optional_agent_id(value: str | None) -> str | None:
    if value is None:
        return None
    if not _AGENT_ID.fullmatch(value):
        raise BudgetControllerError("invalid_identity")
    return value


def _optional_run_id(value: str | None) -> str | None:
    if value is None:
        return None
    if not _RUN_ID.fullmatch(value):
        raise BudgetControllerError("invalid_identity")
    return value


def _tenant_id(value: str) -> str:
    if not isinstance(value, str) or not _TENANT.fullmatch(value):
        raise BudgetControllerError("invalid_input")
    return value


def _launch_id(value: str) -> str:
    if not isinstance(value, str) or not _LAUNCH_ID.fullmatch(value):
        raise BudgetControllerError("invalid_input")
    return value


def _row_dict(row, cursor) -> dict:
    if isinstance(row, dict):
        return row
    columns = [item[0] if not hasattr(item, "name") else item.name for item in cursor.description]
    return dict(zip(columns, row))


def _reservation_from_row(row, cursor) -> ReservationView:
    data = _row_dict(row, cursor)
    return ReservationView(
        launch_id=str(data["launch_id"]),
        tenant_id=str(data["tenant_id"]),
        state=str(data["state"]),
        reserve_cents=int(data["reserve_cents"]),
        raw_cost_text=data.get("raw_cost_text"),
        charged_text=data.get("charged_text"),
        charged_known=bool(data["charged_known"]),
        agent_id=data.get("agent_id"),
        run_id=data.get("run_id"),
        block_reason=data.get("block_reason"),
        usage_unknown=bool(data.get("usage_unknown", False)),
        ledger_utc_day=data.get("ledger_utc_day"),
        ledger_utc_month=data.get("ledger_utc_month"),
    )


def _is_unique_violation(exc: Exception) -> bool:
    try:
        import psycopg.errors

        return isinstance(exc, psycopg.errors.UniqueViolation)
    except Exception:
        return False


def _ledger_cents_ceiling(amount: Decimal) -> int:
    return int(amount.to_integral_value(rounding=ROUND_CEILING))


def _exception_code(exc: Exception) -> str | None:
    message = str(exc).lower()
    tokens = (
        "global_budget_exceeded",
        "tenant_budget_exceeded",
        "per_launch_cap_exceeded",
        "overrun_open",
        "unknown_usage_open",
        "identity_mismatch",
        "invalid_state",
        "invalid_amount",
        "not_found",
        "tenant_mismatch",
        "tenant_unbound",
        "store_unavailable",
        "reconciler_required",
        "verified_usage_missing",
        "verified_usage_required",
        "unknown_usage",
    )
    for token in tokens:
        if token in message:
            return token
    return None


def _budget_error_from_exception(exc: Exception) -> BudgetControllerError:
    if _is_unique_violation(exc):
        return BudgetControllerError("idempotency_conflict")
    code = _exception_code(exc)
    if code:
        return BudgetControllerError(code)
    return BudgetControllerError("store_unavailable")
