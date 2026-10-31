"""PostgreSQL-backed budget reservation for BINTRANS Autopilot phase 0.3C2.

Limits come from the existing ``autopilot.config.json`` via ``load_config``.
This module does not replace ``budget.py`` spend polling; it adds durable
reservation and reconciliation state in ``autopilot_queue``.

Money comparisons use ``Decimal``. Unknown charges stay unknown; they are not
treated as zero. No Cloud Agent is started from this module.
"""

from __future__ import annotations

import logging
import os
import re
import threading
from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal, InvalidOperation
from contextlib import contextmanager
from typing import Callable, Iterator

from bintrans_autopilot.config import AutopilotConfig, load_config

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
    if amount < 0:
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


class BudgetController:
    """Dry-run budget controller. Construction does not open a connection."""

    def __init__(
        self,
        connect: Callable,
        *,
        mode: str = "dry_run",
        config: AutopilotConfig | None = None,
        now: Callable[[], datetime] | None = None,
    ):
        if mode != "dry_run":
            raise BudgetControllerError("controller_disabled")
        for name in (
            "AUTOPILOT_ENABLED",
            "PAID_AUTOMATION_ENABLED",
            "EXTERNAL_TOOLS_ENABLED",
            "AUTO_MERGE_ENABLED",
            "AUTO_DEPLOY_ENABLED",
            "WORKER_ENABLED",
        ):
            if os.environ.get(name, "").strip().lower() == "true":
                raise BudgetControllerError("controller_disabled")
        self._connect = connect
        self._config = config or load_config()
        self._limits = limits_from_config(self._config)
        self._now = now or (lambda: datetime.now(timezone.utc))
        self._lock = threading.Lock()
        self.cloud_agents_started = 0
        self.autopilot_enabled = False
        self.paid_automation_enabled = False

    def evaluate_kill_switch(self) -> KillSwitchDecision:
        if os.environ.get("AUTOPILOT_ENABLED", "").strip().lower() == "true":
            return KillSwitchDecision(False, "autopilot_disabled")
        if os.environ.get("PAID_AUTOMATION_ENABLED", "").strip().lower() == "true":
            return KillSwitchDecision(False, "paid_automation_disabled")
        if self._config.emergency_stop:
            return KillSwitchDecision(False, "emergency_stop")
        if (
            self._limits.per_launch_cap_cents <= 0
            or self._limits.daily_limit_cents <= 0
            or self._limits.monthly_limit_cents <= 0
        ):
            return KillSwitchDecision(False, "zero_budget")
        try:
            with self._cursor() as cursor:
                if self._has_unknown_open_reservations(cursor):
                    return KillSwitchDecision(False, "unknown_usage_open")
        except BudgetControllerError as exc:
            if exc.code == "store_unavailable":
                return KillSwitchDecision(False, "budget_store_unavailable")
            raise
        return KillSwitchDecision(True, "allowed")

    def sync_tenant_limits(self, tenant_id: str) -> None:
        tenant_id = _tenant_id(tenant_id)
        day, month = _utc_windows(self._now())
        with self._cursor() as cursor:
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

    def request_reserve(self, launch_id: str, tenant_id: str, reserve_cents: int) -> ReservationView:
        launch_id = _launch_id(launch_id)
        tenant_id = _tenant_id(tenant_id)
        if reserve_cents <= 0:
            raise BudgetControllerError("invalid_amount")
        with self._lock:
            with self._cursor() as cursor:
                existing = self._fetch_reservation(cursor, launch_id)
                if existing is not None:
                    if existing.tenant_id == tenant_id and existing.reserve_cents == reserve_cents:
                        return existing
                    raise BudgetControllerError("idempotency_conflict")
        gate = self.evaluate_kill_switch()
        if not gate.allowed:
            return self._block(launch_id, tenant_id, reserve_cents, gate.reason)
        if reserve_cents > self._limits.per_launch_cap_cents:
            return self._block(launch_id, tenant_id, reserve_cents, "per_launch_cap_exceeded")
        with self._lock:
            with self._cursor() as cursor:
                existing = self._fetch_reservation(cursor, launch_id)
                if existing is not None:
                    if (
                        existing.tenant_id == tenant_id
                        and existing.reserve_cents == reserve_cents
                        and existing.state in {RESERVED, COMMITTED, RELEASED, BLOCKED}
                    ):
                        return existing
                    raise BudgetControllerError("idempotency_conflict")
                day, month = _utc_windows(self._now())
                ledger = self._lock_ledger(cursor, tenant_id, day, month)
                if self._projected_exceeds(ledger, reserve_cents):
                    return self._insert_blocked(
                        cursor,
                        launch_id,
                        tenant_id,
                        reserve_cents,
                        "tenant_budget_exceeded",
                    )
                cursor.execute(
                    """
                    UPDATE autopilot_queue.budget_tenant_ledger
                    SET daily_reserved_cents = daily_reserved_cents + %s,
                        monthly_reserved_cents = monthly_reserved_cents + %s
                    WHERE tenant_id = %s
                    """,
                    (reserve_cents, reserve_cents, tenant_id),
                )
                cursor.execute(
                    """
                    INSERT INTO autopilot_queue.budget_reservations (
                        launch_id, tenant_id, state, reserve_cents, block_reason
                    ) VALUES (%s, %s, %s, %s, NULL)
                    """,
                    (launch_id, tenant_id, RESERVED, reserve_cents),
                )
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
    ) -> ReservationView:
        launch_id = _launch_id(launch_id)
        if not charged_known:
            raise BudgetControllerError("unknown_usage")
        if agent_id is not None and not _AGENT_ID.fullmatch(agent_id):
            raise BudgetControllerError("invalid_identity")
        if run_id is not None and not _RUN_ID.fullmatch(run_id):
            raise BudgetControllerError("invalid_identity")
        raw_cost_text = _decimal_text(raw_cost_text) if raw_cost_text is not None else None
        charged_text = _decimal_text(charged_text) if charged_text is not None else None
        with self._cursor() as cursor:
            row = self._lock_reservation(cursor, launch_id)
            if row.state == COMMITTED:
                if (
                    row.raw_cost_text == raw_cost_text
                    and row.charged_text == charged_text
                    and row.charged_known == charged_known
                ):
                    return row
                raise BudgetControllerError("idempotency_conflict")
            if row.state != RESERVED:
                raise BudgetControllerError("invalid_state")
            charged_amount = Decimal(charged_text) if charged_text is not None else Decimal(0)
            reserve = Decimal(row.reserve_cents)
            if charged_amount > reserve:
                raise BudgetControllerError("charge_exceeds_reserve")
            day, month = _utc_windows(self._now())
            self._lock_ledger(cursor, row.tenant_id, day, month)
            cursor.execute(
                """
                UPDATE autopilot_queue.budget_tenant_ledger
                SET daily_reserved_cents = daily_reserved_cents - %s,
                    monthly_reserved_cents = monthly_reserved_cents - %s,
                    daily_committed_cents = daily_committed_cents + %s,
                    monthly_committed_cents = monthly_committed_cents + %s
                WHERE tenant_id = %s
                """,
                (
                    row.reserve_cents,
                    row.reserve_cents,
                    int(charged_amount.to_integral_value()),
                    int(charged_amount.to_integral_value()),
                    row.tenant_id,
                ),
            )
            cursor.execute(
                """
                UPDATE autopilot_queue.budget_reservations
                SET state = %s,
                    raw_cost_text = %s,
                    charged_text = %s,
                    charged_known = %s,
                    agent_id = %s,
                    run_id = %s,
                    updated_at = clock_timestamp()
                WHERE launch_id = %s
                """,
                (
                    COMMITTED,
                    raw_cost_text,
                    charged_text,
                    charged_known,
                    agent_id,
                    run_id,
                    launch_id,
                ),
            )
            return self._require_fetch(cursor, launch_id)

    def release(self, launch_id: str) -> ReservationView:
        launch_id = _launch_id(launch_id)
        with self._cursor() as cursor:
            row = self._lock_reservation(cursor, launch_id)
            if row.state == RELEASED:
                return row
            if row.state != RESERVED:
                raise BudgetControllerError("invalid_state")
            day, month = _utc_windows(self._now())
            self._lock_ledger(cursor, row.tenant_id, day, month)
            cursor.execute(
                """
                UPDATE autopilot_queue.budget_tenant_ledger
                SET daily_reserved_cents = daily_reserved_cents - %s,
                    monthly_reserved_cents = monthly_reserved_cents - %s
                WHERE tenant_id = %s
                """,
                (row.reserve_cents, row.reserve_cents, row.tenant_id),
            )
            cursor.execute(
                """
                UPDATE autopilot_queue.budget_reservations
                SET state = %s, updated_at = clock_timestamp()
                WHERE launch_id = %s
                """,
                (RELEASED, launch_id),
            )
            return self._require_fetch(cursor, launch_id)

    def get_reservation(self, launch_id: str) -> ReservationView | None:
        launch_id = _launch_id(launch_id)
        with self._cursor() as cursor:
            return self._fetch_reservation(cursor, launch_id)

    def _block(self, launch_id: str, tenant_id: str, reserve_cents: int, reason: str) -> ReservationView:
        with self._cursor() as cursor:
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
            INSERT INTO autopilot_queue.budget_reservations (
                launch_id, tenant_id, state, reserve_cents, block_reason
            ) VALUES (%s, %s, %s, %s, %s)
            """,
            (launch_id, tenant_id, BLOCKED, reserve_cents, reason),
        )
        logger.info(
            "event=budget_blocked tenant=%s launch=%s reason=%s",
            tenant_id,
            launch_id,
            reason,
        )
        return self._require_fetch(cursor, launch_id)

    def _has_unknown_open_reservations(self, cursor) -> bool:
        cursor.execute(
            """
            SELECT 1 AS present
            FROM autopilot_queue.budget_reservations
            WHERE state = %s AND charged_known = false
            LIMIT 1
            """,
            (RESERVED,),
        )
        return cursor.fetchone() is not None

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
        if ledger["utc_day"] != day or ledger["utc_month"] != month:
            cursor.execute(
                """
                UPDATE autopilot_queue.budget_tenant_ledger
                SET utc_day = %s,
                    utc_month = %s,
                    daily_reserved_cents = 0,
                    monthly_reserved_cents = 0,
                    daily_committed_cents = 0,
                    monthly_committed_cents = 0,
                    daily_limit_cents = %s,
                    monthly_limit_cents = %s
                WHERE tenant_id = %s
                """,
                (day, month, self._limits.daily_limit_cents, self._limits.monthly_limit_cents, tenant_id),
            )
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
            ledger = _row_dict(row, cursor)
        return ledger

    def _lock_reservation(self, cursor, launch_id: str) -> ReservationView:
        cursor.execute(
            """
            SELECT launch_id, tenant_id, state, reserve_cents,
                   raw_cost_text, charged_text, charged_known,
                   agent_id, run_id, block_reason
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
                   agent_id, run_id, block_reason
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
    def _cursor(self) -> Iterator:
        from bintrans_autopilot.postgres_queue import _cursor_for

        try:
            connection = self._connect()
        except Exception as exc:
            raise BudgetControllerError("store_unavailable") from exc
        try:
            with _cursor_for(connection) as cursor:
                yield cursor
            connection.commit()
        except BudgetControllerError:
            connection.rollback()
            raise
        except Exception:
            connection.rollback()
            raise BudgetControllerError("store_unavailable") from None
        finally:
            connection.close()


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
    )
