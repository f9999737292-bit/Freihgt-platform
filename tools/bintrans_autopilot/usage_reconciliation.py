"""Reconcile Cursor usage with a budget reservation."""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal, InvalidOperation

from bintrans_autopilot.budget_controller import BudgetController, BudgetControllerError
from bintrans_autopilot.cursor_api import CursorApiError, CursorClient
from bintrans_autopilot.usage_verification import usage_verification_refs
import re

_AGENT_ID = re.compile(r"^bc-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
_RUN_ID = re.compile(r"^run-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")


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
    if not isinstance(agent_id, str) or not isinstance(run_id, str):
        return ReconcileResult("failed", "invalid_identity")
    if not _AGENT_ID.fullmatch(agent_id) or not _RUN_ID.fullmatch(run_id):
        return ReconcileResult("failed", "invalid_identity")
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
    except BudgetControllerError as exc:
        return ReconcileResult("failed", exc.code)
    except Exception:
        return ReconcileResult("failed", "usage_fetch_failed")
    if not isinstance(payload, dict):
        payload = {}
    verification_id, payload_digest = usage_verification_refs(agent_id, run_id, payload)
    try:
        amounts = parse_usage_payload(payload)
    except BudgetControllerError as exc:
        return ReconcileResult("failed", exc.code)
    if not amounts.charged_known:
        try:
            controller.mark_usage_unknown(
                launch_id,
                agent_id=agent_id,
                run_id=run_id,
                raw_cost_text=amounts.raw_cost_text,
                verification_id=verification_id,
                payload_digest=payload_digest,
            )
        except BudgetControllerError as exc:
            return ReconcileResult("failed", exc.code)
        return ReconcileResult("pending", "unknown_usage")
    try:
        controller.commit_usage(
            launch_id,
            raw_cost_text=amounts.raw_cost_text,
            charged_text=amounts.charged_text,
            charged_known=True,
            agent_id=agent_id,
            run_id=run_id,
            verification_id=verification_id,
            payload_digest=payload_digest,
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


def _optional_amount(value: object | None) -> str | None:
    if value is None:
        return None
    if isinstance(value, str) and not value.strip():
        return None
    if isinstance(value, bool):
        raise BudgetControllerError("invalid_amount")
    try:
        amount = Decimal(str(value).strip())
    except (InvalidOperation, AttributeError, TypeError, ValueError) as exc:
        raise BudgetControllerError("invalid_amount") from exc
    if not amount.is_finite() or amount < 0:
        raise BudgetControllerError("invalid_amount")
    text = format(amount, "f")
    if "." in text:
        text = text.rstrip("0").rstrip(".")
    return text or "0"
