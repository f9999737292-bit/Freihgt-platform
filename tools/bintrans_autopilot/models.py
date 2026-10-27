"""Choose a Cloud Agent model from an approved route and a live catalog.

Model ids come from GET /v1/models. The committed policy names none of them.
Omitting `model` is Cursor's documented default: user default, then team
default, then system default. Issue text cannot select an id.
"""

from __future__ import annotations

from dataclasses import dataclass

from bintrans_autopilot.config import MODEL_ROLES, ModelRoute, ModelRouting
from bintrans_autopilot.contract import TaskContract

# "default" and "unknown" are not prices. They are not free and not the cheap rank.
COST_RANK = {"standard": 1, "expensive": 2}
UNKNOWN_COST = "UNKNOWN"
PROVIDER_UNVERIFIED = "PROVIDER_UNVERIFIED"
_UNPRICED = {"", "default", "unknown"}
FINANCE_PREFIXES = (
    "services/billing-register-service/",
    "services/payment-service/",
    "services/freight-cost-service/",
    "services/contract-rate-service/",
)
FINANCE_MODULES = frozenset({"finance", "billing", "billing-register", "payment", "freight-cost", "contract-rate"})


class ModelPolicyError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


@dataclass(frozen=True)
class CatalogModel:
    id: str
    aliases: tuple[str, ...]
    parameters: dict[str, tuple[str, ...]]
    variants: tuple[tuple[tuple[str, str], ...], ...]
    default_params: tuple[tuple[str, str], ...]

    def accepts(self, params: tuple[tuple[str, str], ...]) -> bool:
        wanted = tuple(sorted(params))
        if self.variants:
            return any(tuple(sorted(variant)) == wanted for variant in self.variants)
        for key, value in params:
            allowed = self.parameters.get(key)
            if allowed is None or value not in allowed:
                return False
        return True


@dataclass(frozen=True)
class ModelCatalog:
    by_id: dict[str, CatalogModel]

    def find(self, model_id: str) -> CatalogModel | None:
        wanted = model_id.strip()
        if not wanted:
            return None
        direct = self.by_id.get(wanted)
        if direct is not None:
            return direct
        for model in self.by_id.values():
            if wanted in model.aliases:
                return model
        return None


@dataclass(frozen=True)
class ModelChoice:
    omit: bool
    model_id: str
    params: tuple[tuple[str, str], ...]
    provider: str
    cost_class: str
    source: str
    cost_status: str = UNKNOWN_COST
    cost_rank: int | None = None
    provider_status: str = PROVIDER_UNVERIFIED
    verified_provider: str = ""

    def as_api(self) -> dict | None:
        if self.omit:
            return None
        body: dict = {"id": self.model_id}
        if self.params:
            body["params"] = [{"id": key, "value": value} for key, value in self.params]
        return body

    def pin(self) -> dict:
        return {
            "omit": self.omit,
            "id": self.model_id,
            "params": [{"id": key, "value": value} for key, value in self.params],
            "provider_status": self.provider_status,
            "verified_provider": self.verified_provider,
            "cost_class": self.cost_class,
            "cost_status": self.cost_status,
        }


def _pairs(raw) -> tuple[tuple[str, str], ...]:
    pairs = []
    for item in raw or []:
        if not isinstance(item, dict):
            raise ModelPolicyError("model_catalog_malformed")
        key = str(item.get("id") or "")
        value = str(item.get("value") or "")
        if not key:
            raise ModelPolicyError("model_catalog_malformed")
        pairs.append((key, value))
    return tuple(pairs)


def parse_model_catalog(body: dict) -> ModelCatalog:
    """Parse the documented GET /v1/models object. Ids are taken only from `items`."""
    if not isinstance(body, dict) or not isinstance(body.get("items"), list):
        raise ModelPolicyError("model_catalog_malformed")
    found: dict[str, CatalogModel] = {}
    for item in body["items"]:
        if not isinstance(item, dict) or not str(item.get("id") or "").strip():
            raise ModelPolicyError("model_catalog_malformed")
        model_id = str(item["id"]).strip()
        parameters: dict[str, tuple[str, ...]] = {}
        for param in item.get("parameters") or []:
            if not isinstance(param, dict) or not str(param.get("id") or "").strip():
                raise ModelPolicyError("model_catalog_malformed")
            values = []
            for entry in param.get("values") or []:
                if isinstance(entry, dict) and str(entry.get("value") or ""):
                    values.append(str(entry["value"]))
            parameters[str(param["id"])] = tuple(values)
        variants = []
        default_params: tuple[tuple[str, str], ...] = ()
        for variant in item.get("variants") or []:
            if not isinstance(variant, dict):
                raise ModelPolicyError("model_catalog_malformed")
            params = _pairs(variant.get("params"))
            variants.append(params)
            if variant.get("isDefault") is True:
                default_params = params
        aliases = tuple(str(alias) for alias in (item.get("aliases") or []) if str(alias).strip())
        found[model_id] = CatalogModel(
            id=model_id,
            aliases=aliases,
            parameters=parameters,
            variants=tuple(variants),
            default_params=default_params,
        )
    return ModelCatalog(found)


def _under_prefix(path: str, prefix: str) -> bool:
    root = prefix.rstrip("/")
    return path == root or path.startswith(root + "/")


def _finance_path(path: str) -> bool:
    return any(_under_prefix(path, prefix) for prefix in FINANCE_PREFIXES)


def role_for(contract: TaskContract) -> str:
    """Role comes from the approved contract, never from free text in the issue."""
    paths = tuple(path.replace("\\", "/").lower() for path in contract.allowed_paths)
    module = contract.module.strip().lower()
    if contract.owner_agent == "frontend-engineer" or any(path.startswith("apps/") for path in paths):
        return "frontend"
    if any("network-optimizer" in path for path in paths) or module in {"nlo", "nlo-optimizer", "optimizer"}:
        return "nlo_optimizer"
    if any("document-service" in path for path in paths) or module in {"edo", "documents"}:
        return "edo"
    if any(_finance_path(path) for path in paths) or module in FINANCE_MODULES:
        return "finance"
    if any("analytics-service" in path for path in paths) or module == "analytics":
        return "analytics"
    if any(token in path for path in paths for token in ("shipment-service", "transport-order", "tracking-service")) or module in {"tms", "shipment", "transport"}:
        return "tms"
    return "platform_backend"


def task_class_for(contract: TaskContract, kind: str) -> str:
    if kind == "qa":
        return "INDEPENDENT_QA"
    paths = tuple(path.replace("\\", "/").lower() for path in contract.allowed_paths)
    if any(path.startswith("services/identity-service/") or path.startswith("services/api-gateway/") for path in paths):
        return "SECURITY_CRITICAL"
    if any(_finance_path(path) for path in paths):
        return "FINANCIAL_CRITICAL"
    if contract.risk_level in {"HIGH", "CRITICAL"}:
        return "COMPLEX"
    if contract.risk_level == "MEDIUM":
        return "STANDARD"
    return "ROUTINE"


def _known_rank(cost_class: str) -> int | None:
    """None means the class has no price. That is not rank 0 and not a free class."""
    if cost_class in _UNPRICED:
        return None
    if cost_class not in COST_RANK:
        raise ModelPolicyError("model_cost_unknown")
    return COST_RANK[cost_class]


def _cursor_default(source: str) -> ModelChoice:
    return ModelChoice(True, "", (), "", "unknown", source, UNKNOWN_COST, None, PROVIDER_UNVERIFIED, "")


def _explicit(model: CatalogModel, params, cost_class: str, source: str) -> ModelChoice:
    return ModelChoice(
        False,
        model.id,
        params,
        "",
        cost_class,
        source,
        UNKNOWN_COST,
        _known_rank(cost_class),
        PROVIDER_UNVERIFIED,
        "",
    )


def _pin_params(pin: dict) -> tuple[tuple[str, str], ...]:
    return _pairs(pin.get("params"))


def _explicit_fallback(route: ModelRoute) -> bool:
    return bool(route.fallback) and route.fallback not in {"cursor_default", "fail_closed"}


def choice_needs_catalog(routing: ModelRouting, contract: TaskContract, kind: str, state: dict) -> bool:
    pin = _active_pin(kind, state)
    if pin is not None:
        return not pin.get("omit") and bool(str(pin.get("id") or ""))
    if kind != "qa" and state.get("agent_id"):
        return False
    route = _route(routing, contract, kind)
    return bool(route.model_id) or _explicit_fallback(route)


def _active_pin(kind: str, state: dict) -> dict | None:
    if kind == "qa" and isinstance(state.get("qa_model_pin"), dict):
        return state["qa_model_pin"]
    if kind != "qa" and isinstance(state.get("model_pin"), dict):
        return state["model_pin"]
    return None


def _route(routing: ModelRouting, contract: TaskContract, kind: str) -> ModelRoute:
    role = "independent_qa" if kind == "qa" else role_for(contract)
    if role not in MODEL_ROLES:
        raise ModelPolicyError("model_role_unknown")
    return routing.route_for(role, task_class_for(contract, kind))


def _fallback(route: ModelRoute, routing: ModelRouting, catalog: ModelCatalog, attempted_cost: str) -> ModelChoice:
    if route.require_distinct_provider:
        raise ModelPolicyError("provider_unverified")
    fallback_rank = _known_rank(route.fallback_cost_class)
    attempted_rank = _known_rank(attempted_cost)
    if fallback_rank is not None and attempted_rank is not None and fallback_rank > attempted_rank:
        raise ModelPolicyError("expensive_fallback_rejected")
    if route.fallback == "cursor_default":
        if attempted_rank is not None and fallback_rank is None:
            raise ModelPolicyError("unknown_cost")
        if not routing.cursor_default_allowed:
            raise ModelPolicyError("cursor_default_not_allowed")
        return _cursor_default("fallback_cursor_default")
    if route.fallback == "fail_closed" or not route.fallback:
        raise ModelPolicyError("model_not_in_catalog")
    if route.fallback_cost_class == "expensive" and not route.fallback_cost_approved:
        raise ModelPolicyError("expensive_fallback_rejected")
    model = catalog.find(route.fallback)
    if model is None:
        raise ModelPolicyError("model_not_in_catalog")
    params = route.params or model.default_params
    if not model.accepts(params):
        raise ModelPolicyError("model_params_rejected")
    return _explicit(model, params, route.fallback_cost_class, "fallback")


def _from_route(route: ModelRoute, routing: ModelRouting, catalog: ModelCatalog, implementation_pin: dict | None) -> ModelChoice:
    del implementation_pin
    if route.require_distinct_provider:
        raise ModelPolicyError("provider_unverified")
    if not route.model_id:
        if not routing.cursor_default_allowed:
            raise ModelPolicyError("cursor_default_not_allowed")
        return _cursor_default("cursor_default")
    if route.cost_class == "expensive" and not route.cost_approved:
        return _fallback(route, routing, catalog, route.cost_class)
    model = catalog.find(route.model_id)
    if model is None:
        return _fallback(route, routing, catalog, route.cost_class)
    params = route.params or model.default_params
    if not model.accepts(params):
        raise ModelPolicyError("model_params_rejected")
    return _explicit(model, params, route.cost_class, "policy")


def _from_pin(pin: dict, route: ModelRoute, routing: ModelRouting, catalog: ModelCatalog) -> ModelChoice:
    """A later config change cannot raise the model that was already pinned."""
    del route, routing
    if pin.get("omit") or not str(pin.get("id") or ""):
        return _cursor_default("pinned_cursor_default")
    model = catalog.find(str(pin["id"]))
    if model is None:
        raise ModelPolicyError("pinned_model_unavailable")
    params = _pin_params(pin)
    if not model.accepts(params):
        raise ModelPolicyError("model_params_rejected")
    pinned_cost = str(pin.get("cost_class") or "unknown")
    return _explicit(model, params, pinned_cost, "pinned")


def distinct_provider_status(left_id: str, right_id: str, left_verified: str = "", right_verified: str = "") -> str:
    """Different model ids are not different providers. An unverified side cannot pass."""
    del left_id, right_id
    if not left_verified or not right_verified:
        return PROVIDER_UNVERIFIED
    if left_verified == right_verified:
        return "SAME_PROVIDER"
    return "DISTINCT"


def unpaid_model_reason(config, choice: ModelChoice) -> str:
    """Selecting a model does not authorize its spend. Catalog JSON is not a price."""
    if getattr(config, "test_model_spend_harness", False):
        return ""
    source = getattr(getattr(config, "pricing", None), "source", "unverified")
    if source != "verified_pricing_api" or choice.cost_status != "VERIFIED":
        return "unknown_cost"
    return ""


def resolve_model(
    routing: ModelRouting,
    contract: TaskContract,
    kind: str,
    catalog: ModelCatalog,
    state: dict | None = None,
) -> ModelChoice:
    state = state or {}
    route = _route(routing, contract, kind)
    pin = _active_pin(kind, state)
    if pin is None and kind != "qa" and state.get("agent_id"):
        return _cursor_default("pinned_cursor_default")
    if pin is not None:
        return _from_pin(pin, route, routing, catalog)
    return _from_route(route, routing, catalog, state.get("model_pin") if isinstance(state.get("model_pin"), dict) else None)


def prepare_model_choice(routing: ModelRouting, contract: TaskContract, kind: str, state: dict, fetch_catalog) -> ModelChoice:
    catalog = ModelCatalog({})
    if choice_needs_catalog(routing, contract, kind, state):
        catalog = parse_model_catalog(fetch_catalog())
    return resolve_model(routing, contract, kind, catalog, state)
