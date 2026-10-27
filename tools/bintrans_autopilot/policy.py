"""Pure eligibility rules. The dispatcher cannot widen a Task Contract."""

from __future__ import annotations

from bintrans_autopilot.config import AutopilotConfig
from bintrans_autopilot.contract import TaskContract
from bintrans_autopilot.paths import any_match, domains_for, normalize, under_prefix
from bintrans_autopilot.states import DEPENDENCY_SATISFIED, STATUSES


def validate_scope(contract: TaskContract, config: AutopilotConfig) -> str:
    for path in contract.allowed_paths:
        normalized = normalize(path)
        if normalized.startswith("/") or ".." in normalized.split("/"):
            return "path_escape"
        if any_match(config.hard_forbidden_prefixes, normalized) or under_prefix(config.hard_forbidden_prefixes, normalized):
            return "hard_forbidden_path"
        if not under_prefix(config.approved_path_prefixes, normalized):
            return "outside_approved_catalog"
        if any_match(contract.forbidden_paths, normalized):
            return "allowed_path_also_forbidden"
    if contract.max_retries > config.max_retries_cap:
        return "retries_above_cap"
    if contract.status not in STATUSES:
        return "invalid_status"
    return ""


def dependencies_closed(contract: TaskContract, status_by_task: dict[str, str]) -> str:
    for dependency in contract.dependencies:
        status = status_by_task.get(dependency)
        if status not in DEPENDENCY_SATISFIED:
            return "dependency_blocked"
    return ""


def base_is_current(contract: TaskContract, origin_main_sha: str) -> str:
    if contract.base_sha != origin_main_sha:
        return "stale_base_sha"
    return ""


def feature_branch(config: AutopilotConfig, task_id: str) -> str:
    branch = f"{config.feature_branch_prefix}{task_id.lower()}"
    if branch in config.protected_branches or branch.split("/")[-1] in config.protected_branches:
        raise ValueError("protected branch")
    return branch


def assert_feature_branch(config: AutopilotConfig, branch: str) -> None:
    if branch in config.protected_branches or branch.endswith("/main") or branch.endswith("/master"):
        raise ValueError("refusing protected branch")
    prefix = config.feature_branch_prefix
    if not branch.startswith(prefix) or branch == prefix:
        raise ValueError("refusing branch outside the autopilot prefix")
    rest = branch[len(prefix) :]
    if "/" in rest or rest in config.protected_branches:
        raise ValueError("refusing nested or protected feature branch")


def critical_domains(contract: TaskContract, config: AutopilotConfig) -> set[str]:
    return domains_for(contract.allowed_paths, config)


def shared_path_allowed(contract: TaskContract, config: AutopilotConfig, labels: list[str]) -> str:
    if critical_domains(contract, config) and "autopilot-shared-path-approved" not in labels:
        return "shared_path_not_approved"
    if critical_domains(contract, config) and contract.risk_level not in {"HIGH", "CRITICAL"}:
        return "shared_path_risk_too_low"
    return ""
