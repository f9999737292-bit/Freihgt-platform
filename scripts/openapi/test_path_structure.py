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

    print("OPENAPI_PATH_STRUCTURE_TEST=PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
