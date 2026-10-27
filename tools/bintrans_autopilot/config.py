"""Committed autopilot limits. The committed flag defaults to off."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path

PACKAGE_DIR = Path(__file__).resolve().parent
CONFIG_PATH = PACKAGE_DIR / "autopilot.config.json"
EMERGENCY_STOP_FILE = PACKAGE_DIR / "EMERGENCY_STOP"


@dataclass(frozen=True)
class DomainLimit:
    name: str
    max_parallel: int
    prefixes: tuple[str, ...]


MODEL_ROLES = (
    "platform_backend",
    "tms",
    "nlo_optimizer",
    "edo",
    "finance",
    "analytics",
    "frontend",
    "independent_qa",
)
MODEL_TASK_CLASSES = (
    "ROUTINE",
    "STANDARD",
    "COMPLEX",
    "SECURITY_CRITICAL",
    "FINANCIAL_CRITICAL",
    "INDEPENDENT_QA",
)
COST_CLASSES = ("default", "standard", "expensive")


@dataclass(frozen=True)
class ModelRoute:
    """One approved selection. An empty model_id means Cursor's configured default."""

    model_id: str = ""
    params: tuple[tuple[str, str], ...] = ()
    fallback: str = "cursor_default"
    cost_class: str = "default"
    cost_approved: bool = False
    provider: str = ""
    fallback_cost_class: str = "default"
    fallback_cost_approved: bool = False
    require_distinct_provider: bool = False


def _cursor_default_route() -> ModelRoute:
    return ModelRoute()


@dataclass(frozen=True)
class ModelRouting:
    cursor_default_allowed: bool = True
    routes: dict[str, ModelRoute] = field(default_factory=dict)

    def route_for(self, role: str, task_class: str) -> ModelRoute:
        return self.routes.get(f"{role}.{task_class}", self.routes.get("default", _cursor_default_route()))


def _param_pairs(raw) -> tuple[tuple[str, str], ...]:
    pairs = []
    for item in raw or []:
        if not isinstance(item, dict):
            raise ValueError("model param must be an object")
        pairs.append((str(item.get("id") or ""), str(item.get("value") or "")))
    return tuple(pairs)


def _route_from(raw: dict | None, base: ModelRoute) -> ModelRoute:
    item = raw or {}
    cost_class = str(item.get("cost_class", base.cost_class))
    fallback_cost = str(item.get("fallback_cost_class", base.fallback_cost_class))
    if cost_class not in COST_CLASSES or fallback_cost not in COST_CLASSES:
        raise ValueError("unknown model cost class")
    fallback = str(item.get("fallback", base.fallback))
    return ModelRoute(
        model_id=str(item["model_id"]).strip() if "model_id" in item else base.model_id,
        params=_param_pairs(item["params"]) if "params" in item else base.params,
        fallback=fallback,
        cost_class=cost_class,
        cost_approved=bool(item.get("cost_approved", base.cost_approved)),
        provider=str(item.get("provider", base.provider)).strip(),
        fallback_cost_class=fallback_cost,
        fallback_cost_approved=bool(item.get("fallback_cost_approved", base.fallback_cost_approved)),
        require_distinct_provider=bool(item.get("require_distinct_provider", base.require_distinct_provider)),
    )


def load_model_routing(raw: dict | None) -> ModelRouting:
    """Explicit model ids are off until a controller writes an approved route."""
    payload = raw or {}
    default = _route_from(payload.get("default_route"), _cursor_default_route())
    routes = {"default": default}
    for key, value in (payload.get("routes") or {}).items():
        role, _, task_class = str(key).partition(".")
        if role not in MODEL_ROLES or task_class not in MODEL_TASK_CLASSES:
            raise ValueError(f"unknown model route {key}")
        routes[str(key)] = _route_from(value if isinstance(value, dict) else {}, default)
    return ModelRouting(
        cursor_default_allowed=bool(payload.get("cursor_default_allowed", True)),
        routes=routes,
    )


@dataclass(frozen=True)
class AutopilotConfig:
    autopilot_enabled: bool
    emergency_stop: bool
    max_parallel_agents: int
    max_launches_per_day: int
    max_retries_cap: int
    max_qa_fix_cycles: int
    budget_max_charged_cents: float
    budget_max_charged_cents_daily: float
    budget_max_charged_cents_monthly: float
    budget_launch_reserve_cents: float
    budget_reserve_per_active_agent_cents: float
    cursor_plan: str
    contract_approvers: tuple[str, ...]
    require_admin_spend_check: bool
    repo_url: str
    protected_branches: tuple[str, ...]
    feature_branch_prefix: str
    approved_path_prefixes: tuple[str, ...]
    hard_forbidden_prefixes: tuple[str, ...]
    critical_domains: tuple[DomainLimit, ...] = field(default_factory=tuple)
    model_routing: ModelRouting = field(default_factory=load_model_routing)

    def domain_for_path(self, path: str) -> str | None:
        normalized = path.replace("\\", "/").lstrip("./")
        for domain in self.critical_domains:
            for prefix in domain.prefixes:
                if normalized == prefix.rstrip("/") or normalized.startswith(prefix):
                    return domain.name
        return None


def load_config(path: Path | None = None) -> AutopilotConfig:
    raw = json.loads((path or CONFIG_PATH).read_text(encoding="utf-8"))
    domains = tuple(
        DomainLimit(
            name=name,
            max_parallel=int(item["max_parallel"]),
            prefixes=tuple(item["prefixes"]),
        )
        for name, item in raw.get("critical_domains", {}).items()
    )
    return AutopilotConfig(
        autopilot_enabled=bool(raw["autopilot_enabled"]),
        emergency_stop=bool(raw.get("emergency_stop", False)),
        max_parallel_agents=int(raw["max_parallel_agents"]),
        max_launches_per_day=int(raw["max_launches_per_day"]),
        max_retries_cap=int(raw["max_retries_cap"]),
        max_qa_fix_cycles=int(raw["max_qa_fix_cycles"]),
        budget_max_charged_cents=float(raw["budget_max_charged_cents"]),
        budget_max_charged_cents_daily=float(raw.get("budget_max_charged_cents_daily", 0)),
        budget_max_charged_cents_monthly=float(raw.get("budget_max_charged_cents_monthly", 0)),
        budget_launch_reserve_cents=float(raw.get("budget_launch_reserve_cents", 0)),
        budget_reserve_per_active_agent_cents=float(raw.get("budget_reserve_per_active_agent_cents", 0)),
        cursor_plan=str(raw.get("cursor_plan", "")).strip().lower(),
        contract_approvers=tuple(
            item for item in (str(name).strip() for name in raw.get("contract_approvers", [])) if item
        ),
        require_admin_spend_check=bool(raw.get("require_admin_spend_check", True)),
        repo_url=str(raw["repo_url"]),
        protected_branches=tuple(raw["protected_branches"]),
        feature_branch_prefix=str(raw["feature_branch_prefix"]),
        approved_path_prefixes=tuple(raw["approved_path_prefixes"]),
        hard_forbidden_prefixes=tuple(raw["hard_forbidden_prefixes"]),
        critical_domains=domains,
        model_routing=load_model_routing(raw.get("model_routing")),
    )


def runtime_enabled(config: AutopilotConfig, env: dict[str, str]) -> tuple[bool, str]:
    """Both the committed flag and AUTOPILOT_ENABLED=true are required."""
    if env.get("AUTOPILOT_EMERGENCY_STOP", "").strip().lower() == "true":
        return False, "emergency_stop_env"
    if config.emergency_stop or EMERGENCY_STOP_FILE.exists():
        return False, "emergency_stop"
    if not config.autopilot_enabled:
        return False, "config_disabled"
    if env.get("AUTOPILOT_ENABLED", "").strip() != "true":
        return False, "env_disabled"
    return True, "enabled"
