"""Daily and monthly budget gate.

The spend API is lagged, so a zero reading is not treated as proof that the
next launch fits. A configured reserve is added before either cap is checked.
A personal Ultra plan cannot call the Team Admin spend API; that stays fail-closed.

Team usage events are aggregated hourly and can arrive after the launch
decision. The sum in this process is not a provider-side hard cap. Cursor's
documented spend-limit enforcement is not instant, and the personal plans do
not expose the Team Admin API. Without a confirmed provider cap, launch stays
blocked. The test harness flag is not read from JSON.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone

from bintrans_autopilot.config import AutopilotConfig
from bintrans_autopilot.cursor_api import CursorApiError


@dataclass(frozen=True)
class BudgetWindows:
    daily_start_ms: int
    daily_end_ms: int
    monthly_start_ms: int
    monthly_end_ms: int


@dataclass(frozen=True)
class BudgetDecision:
    allowed: bool
    reason: str
    charged_cents: float | None = None
    preserve_task_state: bool = False


def _ms(moment: datetime) -> int:
    return int(moment.timestamp() * 1000)


def budget_windows(now: datetime) -> BudgetWindows:
    """Monthly start is the first instant of the UTC month, not midnight today."""
    if now.tzinfo is None:
        now = now.replace(tzinfo=timezone.utc)
    now = now.astimezone(timezone.utc)
    day = datetime(now.year, now.month, now.day, tzinfo=timezone.utc)
    month = datetime(now.year, now.month, 1, tzinfo=timezone.utc)
    return BudgetWindows(_ms(day), _ms(now), _ms(month), _ms(now))


def windows_match(now: datetime, windows: BudgetWindows) -> bool:
    return windows == budget_windows(now)


def assess_budget(
    config: AutopilotConfig,
    cursor,
    windows: BudgetWindows,
    now: datetime,
    active_agents: int,
) -> BudgetDecision:
    if not windows_match(now, windows):
        return BudgetDecision(False, "budget_window_mismatch", preserve_task_state=True)
    if windows.monthly_start_ms > windows.daily_start_ms:
        return BudgetDecision(False, "monthly_window_resets_early", preserve_task_state=True)
    if config.budget_max_charged_cents_daily <= 0 or config.budget_max_charged_cents_monthly <= 0:
        return BudgetDecision(False, "budget_not_authorized", 0.0)
    if config.budget_launch_reserve_cents <= 0 or config.budget_reserve_per_active_agent_cents <= 0:
        return BudgetDecision(False, "budget_reserve_not_configured", preserve_task_state=True)
    if config.require_admin_spend_check and not getattr(cursor, "has_admin_key", True):
        if config.cursor_plan == "ultra":
            return BudgetDecision(False, "personal_ultra_no_team_admin_api", preserve_task_state=True)
        return BudgetDecision(False, "admin_api_key_missing", preserve_task_state=True)
    if not config.test_budget_harness and not config.cursor_plan:
        return BudgetDecision(False, "cursor_plan_unverified", preserve_task_state=True)
    try:
        daily = float(cursor.cloud_charged_cents(windows.daily_start_ms, windows.daily_end_ms))
        monthly = float(cursor.cloud_charged_cents(windows.monthly_start_ms, windows.monthly_end_ms))
    except (CursorApiError, TypeError, ValueError):
        return BudgetDecision(False, "spend_api_error", preserve_task_state=True)
    if monthly + 1e-9 < daily:
        return BudgetDecision(False, "spend_inconsistent", preserve_task_state=True)
    if getattr(cursor, "spend_events_lagged", False):
        return BudgetDecision(False, "spend_observation_stale", preserve_task_state=True)
    reserve = config.budget_launch_reserve_cents + (
        config.budget_reserve_per_active_agent_cents * max(active_agents, 0)
    )
    projected_daily = daily + reserve
    projected_monthly = monthly + reserve
    if projected_daily >= config.budget_max_charged_cents_daily:
        return BudgetDecision(False, "daily_budget_exceeded", projected_daily)
    if projected_monthly >= config.budget_max_charged_cents_monthly:
        return BudgetDecision(False, "monthly_budget_exceeded", projected_monthly)
    if config.budget_max_charged_cents > 0 and projected_daily >= config.budget_max_charged_cents:
        return BudgetDecision(False, "budget_exceeded", projected_daily)
    if not config.test_budget_harness:
        return BudgetDecision(False, "provider_spend_cap_unconfirmed", projected_monthly, preserve_task_state=True)
    return BudgetDecision(True, "within_budget", projected_monthly)
