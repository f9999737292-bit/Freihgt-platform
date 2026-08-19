#!/usr/bin/env python3
"""Minimal OpenAPI validation with semantic path-structure checks."""

from __future__ import annotations

import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("Please install PyYAML: pip install pyyaml", file=sys.stderr)
    raise SystemExit(1)

from validate_spec import validate_openapi_document


def validate_file(path: Path) -> int:
    if not path.is_file():
        print(f"File not found: {path}", file=sys.stderr)
        return 1

    with path.open("r", encoding="utf-8") as handle:
        spec = yaml.safe_load(handle)

    errors = validate_openapi_document(spec)
    if errors:
        print(f"OpenAPI validation failed for {path}:", file=sys.stderr)
        for err in errors:
            print(f"  - {err}", file=sys.stderr)
        return 1

    print(f"OpenAPI validation passed: {path}")
    return 0


def main() -> int:
    root = Path(__file__).resolve().parents[2]
    openapi_dir = root / "packages" / "openapi"

    if len(sys.argv) == 2 and sys.argv[1] == "--all":
        paths = [openapi_dir / "openapi.yaml", *sorted(openapi_dir.glob("*-service.yaml"))]
    elif len(sys.argv) == 2:
        paths = [Path(sys.argv[1])]
    else:
        print("Usage: validate_openapi.py <openapi.yaml>|--all", file=sys.stderr)
        return 1

    for path in paths:
        if validate_file(path) != 0:
            return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
