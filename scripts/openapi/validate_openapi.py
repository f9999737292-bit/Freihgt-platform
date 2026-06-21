#!/usr/bin/env python3
"""Minimal OpenAPI validation."""

import sys
from pathlib import Path


def main() -> int:
    if len(sys.argv) != 2:
        print("Usage: validate_openapi.py <openapi.yaml>", file=sys.stderr)
        return 1

    try:
        import yaml
    except ImportError:
        print("Please install PyYAML: pip install pyyaml", file=sys.stderr)
        return 1

    path = Path(sys.argv[1])
    if not path.is_file():
        print(f"File not found: {path}", file=sys.stderr)
        return 1

    with path.open("r", encoding="utf-8") as handle:
        spec = yaml.safe_load(handle)

    if not isinstance(spec, dict):
        print("Invalid OpenAPI document: root must be a mapping", file=sys.stderr)
        return 1

    if spec.get("openapi") != "3.0.3":
        print("Invalid or missing openapi version (expected 3.0.3)", file=sys.stderr)
        return 1

    info = spec.get("info")
    if not isinstance(info, dict) or not info.get("title") or not info.get("version"):
        print("Missing info.title or info.version", file=sys.stderr)
        return 1

    if not isinstance(spec.get("paths"), dict) or not spec["paths"]:
        print("Missing or empty paths", file=sys.stderr)
        return 1

    if not isinstance(spec.get("components"), dict):
        print("Missing components section", file=sys.stderr)
        return 1

    print("OpenAPI validation passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
