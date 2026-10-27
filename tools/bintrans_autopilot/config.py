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


@dataclass(frozen=True)
class AutopilotConfig:
    autopilot_enabled: bool
    emergency_stop: bool
    max_parallel_agents: int
    max_launches_per_day: int
    max_retries_cap: int
    max_qa_fix_cycles: int
    budget_max_charged_cents: float
    require_admin_spend_check: bool
    repo_url: str
    protected_branches: tuple[str, ...]
    feature_branch_prefix: str
    approved_path_prefixes: tuple[str, ...]
    hard_forbidden_prefixes: tuple[str, ...]
    critical_domains: tuple[DomainLimit, ...] = field(default_factory=tuple)

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
        require_admin_spend_check=bool(raw.get("require_admin_spend_check", True)),
        repo_url=str(raw["repo_url"]),
        protected_branches=tuple(raw["protected_branches"]),
        feature_branch_prefix=str(raw["feature_branch_prefix"]),
        approved_path_prefixes=tuple(raw["approved_path_prefixes"]),
        hard_forbidden_prefixes=tuple(raw["hard_forbidden_prefixes"]),
        critical_domains=domains,
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
