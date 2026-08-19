"""Shared OpenAPI structural validation helpers."""

from __future__ import annotations

from typing import Any

HTTP_METHODS = frozenset({"get", "post", "put", "patch", "delete", "options", "head", "trace"})
ROOT_FORBIDDEN_HTTP_METHODS = frozenset({"get", "post", "put", "patch", "delete"})


def validate_openapi_document(spec: Any) -> list[str]:
    errors: list[str] = []

    if not isinstance(spec, dict):
        return ["Invalid OpenAPI document: root must be a mapping"]

    if spec.get("openapi") != "3.0.3":
        errors.append("Invalid or missing openapi version (expected 3.0.3)")

    info = spec.get("info")
    if not isinstance(info, dict) or not info.get("title") or not info.get("version"):
        errors.append("Missing info.title or info.version")

    paths = spec.get("paths")
    if not isinstance(paths, dict) or not paths:
        errors.append("Missing or empty paths")
        return errors

    for root_key in ROOT_FORBIDDEN_HTTP_METHODS:
        if root_key in spec:
            errors.append(f"Root-level HTTP method key '{root_key}' is forbidden")

    for path, path_item in paths.items():
        if path_item is None:
            errors.append(f"Path item for '{path}' must not be null")
            continue
        if not isinstance(path_item, dict):
            errors.append(f"Path item for '{path}' must be a mapping/object")
            continue
        operations = [key for key in path_item if key in HTTP_METHODS]
        if not operations:
            errors.append(f"Path '{path}' must contain at least one HTTP operation")
            continue
        for operation_key in operations:
            operation = path_item.get(operation_key)
            if not isinstance(operation, dict):
                errors.append(f"Operation '{path}.{operation_key}' must be a mapping/object")

    components = spec.get("components")
    if not isinstance(components, dict):
        errors.append("Missing components section")

    return errors
