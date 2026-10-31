"""Reconcile Cursor usage with a budget reservation.

This module never creates Cloud Agents. It only reads usage payloads and
updates reservation rows through ``BudgetController``.
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal, InvalidOperation

from bintrans_autopilot.budget_controller import BudgetController, BudgetControllerError
from bintrans_autopilot.cursor_api import CursorApiError, CursorClient


@dataclass(frozen=True)
class UsageAmounts:
    raw_cost_text: str | None
    charged_text: str | None
    charged_known: bool


@dataclass(frozen=True)
class ReconcileResult:
    status: str
    code: str = ""


def parse_usage_payload(payload: dict) -> UsageAmounts:
    raw = _optional_amount(payload.get("rawCostCents"))
    charged = _optional_amount(payload.get("chargedCents"))
    if raw is None and charged is None:
        return UsageAmounts(None, None, False)
    if charged is None:
        return UsageAmounts(raw, None, False)
    return UsageAmounts(raw, charged, True)


def reconcile_launch_usage(
    controller: BudgetController,
    *,
    launch_id: str,
    agent_id: str,
    run_id: str,
    usage_fetcher,
) -> ReconcileResult:
    """Fetch usage once and commit the reservation when charged is known."""
    reservation = controller.get_reservation(launch_id)
    if reservation is None:
        return ReconcileResult("failed", "not_found")
    if reservation.agent_id and reservation.agent_id != agent_id:
        return ReconcileResult("failed", "identity_mismatch")
    if reservation.run_id and reservation.run_id != run_id:
        return ReconcileResult("failed", "identity_mismatch")
    try:
        payload = usage_fetcher(agent_id)
    except CursorApiError as exc:
        if exc.retryable:
            return ReconcileResult("retry", "usage_api_retry")
        return ReconcileResult("blocked", "usage_api_error")
    amounts = parse_usage_payload(payload if isinstance(payload, dict) else {})
    if not amounts.charged_known:
        _attach_probe(controller, launch_id, agent_id, run_id, amounts.raw_cost_text)
        return ReconcileResult("pending", "unknown_usage")
    try:
        controller.commit_usage(
            launch_id,
            raw_cost_text=amounts.raw_cost_text,
            charged_text=amounts.charged_text,
            charged_known=True,
            agent_id=agent_id,
            run_id=run_id,
        )
    except BudgetControllerError as exc:
        return ReconcileResult("failed", exc.code)
    return ReconcileResult("committed", "ok")


def reconcile_from_client(
    controller: BudgetController,
    client: CursorClient,
    *,
    launch_id: str,
    agent_id: str,
    run_id: str,
) -> ReconcileResult:
    return reconcile_launch_usage(
        controller,
        launch_id=launch_id,
        agent_id=agent_id,
        run_id=run_id,
        usage_fetcher=client.usage,
    )


def _attach_probe(
    controller: BudgetController,
    launch_id: str,
    agent_id: str,
    run_id: str,
    raw_cost_text: str | None,
) -> None:
    with controller._cursor() as cursor:
        row = controller._lock_reservation(cursor, launch_id)
        if row.state != "RESERVED":
            raise BudgetControllerError("invalid_state")
        cursor.execute(
            """
            UPDATE autopilot_queue.budget_reservations
            SET agent_id = %s,
                run_id = %s,
                raw_cost_text = %s,
                charged_known = false,
                updated_at = clock_timestamp()
            WHERE launch_id = %s
            """,
            (agent_id, run_id, raw_cost_text, launch_id),
        )


def _optional_amount(value: object | None) -> str | None:
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
