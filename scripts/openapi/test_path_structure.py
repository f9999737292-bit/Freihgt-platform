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


def assert_aggregate_payment_parity(payment_spec: dict, unified_spec: dict) -> None:
    for path in (
        "/api/v1/payments",
        "/api/v1/payments/{id}/allocations",
        "/api/v1/payments/{id}/audit-events",
        "/api/v1/payments/{id}/eligible-obligations",
    ):
        payment_get = payment_spec.get("paths", {}).get(path, {}).get("get")
        unified_get = unified_spec.get("paths", {}).get(path, {}).get("get")
        if payment_get != unified_get:
            raise AssertionError(f"openapi.yaml GET {path} does not match payment-service.yaml")


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
    assert_payment_read_operation_contracts(payment_spec, "payment-service.yaml")
    assert_payment_read_operation_contracts(unified_spec, "openapi.yaml")
    assert_aggregate_payment_parity(payment_spec, unified_spec)

    print("OPENAPI_PATH_STRUCTURE_TEST=PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
