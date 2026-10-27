"""Exact-SHA CI gate. skipped and pending are not success."""

from __future__ import annotations

GO_MODULES = (
    "packages/shared-go",
    "services/analytics-service",
    "services/api-gateway",
    "services/billing-register-service",
    "services/payment-service",
    "services/freight-cost-service",
    "services/contract-rate-service",
    "services/company-service",
    "services/control-tower-read-model-service",
    "services/document-service",
    "services/identity-service",
    "services/localization-service",
    "services/rfx-service",
    "services/shipment-service",
    "services/tracking-service",
    "services/transport-order-service",
    "services/network-optimizer-service",
)

INTEGRATION_BY_PREFIX = {
    "services/shipment-service/": ("shipment-source-authority-postgres",),
    "services/company-service/": ("company-relationship-foundation-postgres",),
    "services/identity-service/": ("tms-postgres-security-foundation",),
    "services/api-gateway/": ("system-security-wave1",),
    "services/rfx-service/": ("rfx-source-authority-postgres",),
    "services/network-optimizer-service/": ("network-optimizer-race-gate",),
}

ALWAYS_REQUIRED = ("repository-safety", "bintrans-autopilot-unit")


def required_checks(paths: list[str] | tuple[str, ...]) -> tuple[str, ...]:
    names = list(ALWAYS_REQUIRED)
    for path in paths:
        normalized = path.replace("\\", "/")
        for module in GO_MODULES:
            if normalized == module or normalized.startswith(module + "/"):
                names.append(f"backend-go-check ({module})")
        for prefix, extras in INTEGRATION_BY_PREFIX.items():
            if normalized.startswith(prefix):
                names.extend(extras)
    return tuple(dict.fromkeys(names))


def _latest(runs: list[dict]) -> dict:
    """The newest attempt wins. Equal timestamps keep the later list entry."""

    def sort_key(pair: tuple[int, dict]) -> tuple[str, str, int]:
        index, run = pair
        return (
            str(run.get("completed_at") or ""),
            str(run.get("started_at") or ""),
            index,
        )

    return max(enumerate(runs), key=sort_key)[1]


def evaluate_ci(checks: list[dict], required: tuple[str, ...], expected_sha: str) -> tuple[bool, str]:
    if len(expected_sha) != 40:
        return False, "missing_head_sha"
    grouped: dict[str, list[dict]] = {}
    for check in checks:
        if check.get("head_sha") != expected_sha:
            return False, "check_sha_mismatch"
        grouped.setdefault(str(check.get("name") or ""), []).append(check)
    for name in required:
        runs = grouped.get(name) or []
        if not runs:
            return False, f"check_missing:{name}"
        latest = _latest(runs)
        status = str(latest.get("status") or "").lower()
        conclusion = str(latest.get("conclusion") or "").lower()
        if status != "completed":
            return False, f"check_pending:{name}"
        if conclusion != "success":
            return False, f"check_not_success:{name}"
    return True, "ci_pass"
