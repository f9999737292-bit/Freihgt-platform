#!/usr/bin/env python3
"""Regression test for OpenAPI path nesting and v1.9.2B void routes."""

from __future__ import annotations

import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("Please install PyYAML: pip install pyyaml", file=sys.stderr)
    raise SystemExit(1)

ROOT = Path(__file__).resolve().parents[2]
OPENAPI_DIR = ROOT / "packages" / "openapi"
ROOT_FORBIDDEN = frozenset({"get", "post", "put", "patch", "delete"})
PAYMENT_WORKSPACE_SCHEMAS = frozenset({
    "PaymentRecord",
    "PaymentListResponse",
    "PaymentAllocationReadRecord",
    "PaymentAllocationListResponse",
    "PaymentAuditEventRecord",
    "PaymentAuditEventListResponse",
    "PaymentObligationRecord",
    "EligiblePaymentObligationListResponse",
})

from validate_spec import validate_openapi_document  # noqa: E402

COMPANY_ID_QUERY_DESCRIPTION_FRAGMENT = "gateway validates membership"
PAYMENT_GUARD_ROUTES = (
    ("get", "/api/v1/payment-obligations"),
    ("get", "/api/v1/payment-obligations/{id}"),
    ("patch", "/api/v1/payment-obligations/{id}/due-date"),
    ("post", "/api/v1/payments"),
    ("get", "/api/v1/payments"),
    ("get", "/api/v1/payments/{id}"),
    ("get", "/api/v1/payments/{id}/allocations"),
    ("get", "/api/v1/payments/{id}/audit-events"),
    ("get", "/api/v1/payments/{id}/eligible-obligations"),
    ("post", "/api/v1/payments/{id}/allocations"),
    ("post", "/api/v1/payments/{id}/reconcile"),
    ("post", "/api/v1/payment-allocations/{id}/void"),
    ("post", "/api/v1/payments/{id}/void"),
)

FREIGHT_COST_PUBLIC_ROUTES = (
    ("get", "/api/v1/freight-costs"),
    ("get", "/api/v1/freight-costs/summary"),
    ("get", "/api/v1/freight-costs/transport-orders/{transportOrderId}"),
    ("get", "/api/v1/freight-costs/transport-orders/{transportOrderId}/variance-detail"),
    ("get", "/api/v1/freight-costs/accessorials/summary"),
    ("get", "/api/v1/freight-costs/carriers/performance"),
    ("get", "/api/v1/freight-costs/lanes/performance"),
)


def load_yaml(path: Path) -> dict:
    with path.open("r", encoding="utf-8") as handle:
        spec = yaml.safe_load(handle)
    if not isinstance(spec, dict):
        raise AssertionError(f"{path}: root must be a mapping")
    return spec


def assert_void_route(spec: dict, path: str, label: str) -> None:
    paths = spec.get("paths", {})
    if path not in paths:
        raise AssertionError(f"{label}: missing path {path}")
    path_item = paths[path]
    if not isinstance(path_item, dict):
        raise AssertionError(f"{label}: path item for {path} must be an object")
    if "post" not in path_item or not isinstance(path_item["post"], dict):
        raise AssertionError(f"{label}: paths['{path}']['post'] must exist")


def assert_no_root_http_methods(path: Path, spec: dict) -> None:
    for key in ROOT_FORBIDDEN:
        if key in spec:
            raise AssertionError(f"{path}: root-level HTTP method '{key}' detected")


def schema_names(spec: dict) -> set[str]:
    schemas = spec.get("components", {}).get("schemas", {})
    if not isinstance(schemas, dict):
        return set()
    return set(schemas.keys())


def operation_parameter_names(operation: dict) -> set[str]:
    params = operation.get("parameters", [])
    if not isinstance(params, list):
        return set()
    names: set[str] = set()
    for param in params:
        if isinstance(param, dict) and "name" in param:
            names.add(param["name"])
    return names


def success_response_schema_ref(operation: dict) -> str | None:
    responses = operation.get("responses", {})
    if not isinstance(responses, dict):
        return None
    success = responses.get("200")
    if not isinstance(success, dict):
        return None
    content = success.get("content", {})
    if not isinstance(content, dict):
        return None
    json_content = content.get("application/json", {})
    if not isinstance(json_content, dict):
        return None
    schema = json_content.get("schema", {})
    if not isinstance(schema, dict):
        return None
    ref = schema.get("$ref")
    return ref if isinstance(ref, str) else None


def company_id_query_parameter(operation: dict) -> dict | None:
    params = operation.get("parameters", [])
    if not isinstance(params, list):
        return None
    for param in params:
        if isinstance(param, dict) and param.get("name") == "company_id" and param.get("in") == "query":
            return param
    return None


def assert_required_company_id_query(operation: dict, label: str, method: str, path: str) -> None:
    param = company_id_query_parameter(operation)
    if param is None:
        raise AssertionError(f"{label}: {method.upper()} {path} missing required company_id query parameter")
    if param.get("required") is not True:
        raise AssertionError(f"{label}: {method.upper()} {path} company_id must be required")
    schema = param.get("schema", {})
    if not isinstance(schema, dict):
        raise AssertionError(f"{label}: {method.upper()} {path} company_id schema must be an object")
    if schema.get("type") != "string" or schema.get("format") != "uuid":
        raise AssertionError(f"{label}: {method.upper()} {path} company_id must be string/uuid")
    description = param.get("description", "")
    if not isinstance(description, str) or COMPANY_ID_QUERY_DESCRIPTION_FRAGMENT not in description.lower():
        raise AssertionError(f"{label}: {method.upper()} {path} company_id description must document gateway membership validation")


def assert_all_payment_guard_routes_require_company_id(spec: dict, label: str) -> None:
    paths = spec.get("paths", {})
    for method, path in PAYMENT_GUARD_ROUTES:
        path_item = paths.get(path)
        if not isinstance(path_item, dict):
            raise AssertionError(f"{label}: missing path {path}")
        operation = path_item.get(method)
        if not isinstance(operation, dict):
            raise AssertionError(f"{label}: missing {method.upper()} {path}")
        assert_required_company_id_query(operation, label, method, path)


def assert_payment_detail_contract(spec: dict, label: str) -> None:
    operation = spec.get("paths", {}).get("/api/v1/payments/{id}", {}).get("get")
    if not isinstance(operation, dict):
        raise AssertionError(f"{label}: missing GET /api/v1/payments/{{id}}")
    if success_response_schema_ref(operation) != "#/components/schemas/PaymentRecord":
        raise AssertionError(f"{label}: GET /api/v1/payments/{{id}} must reference PaymentRecord")
    assert_required_company_id_query(operation, label, "get", "/api/v1/payments/{id}")


def assert_payment_read_operation_contracts(spec: dict, label: str) -> None:
    paths = spec.get("paths", {})
    payment_list = paths.get("/api/v1/payments", {}).get("get")
    if not isinstance(payment_list, dict):
        raise AssertionError(f"{label}: missing GET /api/v1/payments")
    if success_response_schema_ref(payment_list) != "#/components/schemas/PaymentListResponse":
        raise AssertionError(f"{label}: GET /api/v1/payments must reference PaymentListResponse")
    list_params = operation_parameter_names(payment_list)
    for name in ("company_id", "status", "currency_code", "from_date", "to_date", "q", "limit", "offset"):
        if name not in list_params:
            raise AssertionError(f"{label}: GET /api/v1/payments missing query parameter {name}")

    detail_ops = {
        "/api/v1/payments/{id}/allocations": "PaymentAllocationListResponse",
        "/api/v1/payments/{id}/audit-events": "PaymentAuditEventListResponse",
        "/api/v1/payments/{id}/eligible-obligations": "EligiblePaymentObligationListResponse",
    }
    for path, schema_name in detail_ops.items():
        operation = paths.get(path, {}).get("get")
        if not isinstance(operation, dict):
            raise AssertionError(f"{label}: missing GET {path}")
        expected_ref = f"#/components/schemas/{schema_name}"
        if success_response_schema_ref(operation) != expected_ref:
            raise AssertionError(f"{label}: GET {path} must reference {schema_name}")
        params = operation_parameter_names(operation)
        for name in ("company_id", "limit", "offset", "id"):
            if name not in params:
                raise AssertionError(f"{label}: GET {path} missing parameter {name}")


def assert_freight_cost_public_routes_present(spec: dict, label: str) -> None:
    paths = spec.get("paths", {})
    for method, path in FREIGHT_COST_PUBLIC_ROUTES:
        path_item = paths.get(path)
        if not isinstance(path_item, dict):
            raise AssertionError(f"{label}: missing path {path}")
        operation = path_item.get(method)
        if not isinstance(operation, dict):
            raise AssertionError(f"{label}: missing {method.upper()} {path}")
        params = operation.get("parameters", [])
        if not isinstance(params, list):
            raise AssertionError(f"{label}: {method.upper()} {path} parameters must be a list")
        header_names = {
            param.get("name")
            for param in params
            if isinstance(param, dict) and param.get("in") == "header"
        }
        ref_headers = {
            param.get("$ref", "").split("/")[-1]
            for param in params
            if isinstance(param, dict) and "$ref" in param
        }
        if "XCompanyID" not in ref_headers and "X-Company-ID" not in header_names:
            raise AssertionError(f"{label}: {method.upper()} {path} must document X-Company-ID header")


def assert_aggregate_freight_cost_parity(freight_spec: dict, unified_spec: dict) -> None:
    for method, path in FREIGHT_COST_PUBLIC_ROUTES:
        freight_op = freight_spec.get("paths", {}).get(path, {}).get(method)
        unified_op = unified_spec.get("paths", {}).get(path, {}).get(method)
        if freight_op != unified_op:
            raise AssertionError(f"openapi.yaml {method.upper()} {path} does not match freight-cost-service.yaml")


def assert_aggregate_payment_parity(payment_spec: dict, unified_spec: dict) -> None:
    for method, path in PAYMENT_GUARD_ROUTES:
        payment_op = payment_spec.get("paths", {}).get(path, {}).get(method)
        unified_op = unified_spec.get("paths", {}).get(path, {}).get(method)
        if payment_op != unified_op:
            raise AssertionError(f"openapi.yaml {method.upper()} {path} does not match payment-service.yaml")


def assert_payment_schema_isolation() -> None:
    payment_spec = load_yaml(OPENAPI_DIR / "payment-service.yaml")
    payment_schemas = schema_names(payment_spec)
    for name in PAYMENT_WORKSPACE_SCHEMAS:
        if name not in payment_schemas:
            raise AssertionError(f"payment-service.yaml missing required schema {name}")

    for filename in (
        "rfx-service.yaml",
        "company-service.yaml",
        "identity-service.yaml",
        "document-service.yaml",
        "shipment-service.yaml",
        "transport-order-service.yaml",
        "billing-register-service.yaml",
    ):
        spec = load_yaml(OPENAPI_DIR / filename)
        leaked = PAYMENT_WORKSPACE_SCHEMAS.intersection(schema_names(spec))
        if leaked:
            raise AssertionError(f"{filename} contains payment workspace schemas: {sorted(leaked)}")

    unified_spec = load_yaml(OPENAPI_DIR / "openapi.yaml")
    unified_schemas = schema_names(unified_spec)
    for name in PAYMENT_WORKSPACE_SCHEMAS:
        if name not in unified_schemas:
            raise AssertionError(f"openapi.yaml missing required schema {name}")


def main() -> int:
    targets = [
        OPENAPI_DIR / "payment-service.yaml",
        OPENAPI_DIR / "freight-cost-service.yaml",
        OPENAPI_DIR / "openapi.yaml",
    ]

    for target in targets:
        if not target.is_file():
            print(f"Missing required spec: {target}", file=sys.stderr)
            return 1
        spec = load_yaml(target)
        assert_no_root_http_methods(target, spec)
        errors = validate_openapi_document(spec)
        if errors:
            print(f"{target} validation failed:", file=sys.stderr)
            for err in errors:
                print(f"  - {err}", file=sys.stderr)
            return 1

    payment_spec = load_yaml(OPENAPI_DIR / "payment-service.yaml")
    freight_spec = load_yaml(OPENAPI_DIR / "freight-cost-service.yaml")
    unified_spec = load_yaml(OPENAPI_DIR / "openapi.yaml")

    assert_void_route(payment_spec, "/api/v1/payment-allocations/{id}/void", "payment-service.yaml")
    assert_void_route(payment_spec, "/api/v1/payments/{id}/void", "payment-service.yaml")
    assert_void_route(unified_spec, "/api/v1/payment-allocations/{id}/void", "openapi.yaml")
    assert_void_route(unified_spec, "/api/v1/payments/{id}/void", "openapi.yaml")

    void_request = payment_spec.get("components", {}).get("schemas", {}).get("VoidRequest")
    if not isinstance(void_request, dict):
        print("payment-service.yaml missing VoidRequest schema", file=sys.stderr)
        return 1
    reason = void_request.get("properties", {}).get("reason", {})
    if reason.get("minLength") != 1 or reason.get("maxLength") != 255:
        print("VoidRequest.reason must enforce minLength=1 maxLength=255", file=sys.stderr)
        return 1

    assert_payment_schema_isolation()
    assert_all_payment_guard_routes_require_company_id(payment_spec, "payment-service.yaml")
    assert_all_payment_guard_routes_require_company_id(unified_spec, "openapi.yaml")
    assert_payment_detail_contract(payment_spec, "payment-service.yaml")
    assert_payment_detail_contract(unified_spec, "openapi.yaml")
    assert_payment_read_operation_contracts(payment_spec, "payment-service.yaml")
    assert_payment_read_operation_contracts(unified_spec, "openapi.yaml")
    assert_aggregate_payment_parity(payment_spec, unified_spec)
    assert_freight_cost_public_routes_present(freight_spec, "freight-cost-service.yaml")
    assert_freight_cost_public_routes_present(unified_spec, "openapi.yaml")
    assert_aggregate_freight_cost_parity(freight_spec, unified_spec)

    print("OPENAPI_PATH_STRUCTURE_TEST=PASS")
    print("PAYMENT_COMPANY_CONTEXT_CONTRACT_TEST=PASS")
    print("FREIGHT_COST_PUBLIC_ROUTE_PARITY=PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
