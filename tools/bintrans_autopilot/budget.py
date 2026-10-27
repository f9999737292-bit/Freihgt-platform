"""Budget gate. Missing spend data fails closed and does not launch agents."""

from __future__ import annotations

from dataclasses import dataclass

from bintrans_autopilot.config import AutopilotConfig
from bintrans_autopilot.cursor_api import CursorApiError


@dataclass(frozen=True)
class BudgetDecision:
    allowed: bool
    reason: str
    charged_cents: float | None = None
    preserve_task_state: bool = False


def assess_budget(config: AutopilotConfig, cursor, start_ms: int, end_ms: int) -> BudgetDecision:
    if config.budget_max_charged_cents <= 0:
        return BudgetDecision(False, "budget_not_authorized", 0.0)
    if config.require_admin_spend_check and not getattr(cursor, "has_admin_key", True):
        return BudgetDecision(False, "admin_api_key_missing", preserve_task_state=True)
    try:
        charged = cursor.cloud_charged_cents(start_ms, end_ms)
    except CursorApiError:
        return BudgetDecision(False, "spend_api_error", preserve_task_state=True)
    if charged >= config.budget_max_charged_cents:
        return BudgetDecision(False, "budget_exceeded", charged)
    return BudgetDecision(True, "within_budget", charged)
