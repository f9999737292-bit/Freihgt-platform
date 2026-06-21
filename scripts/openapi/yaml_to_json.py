#!/usr/bin/env python3
"""Convert OpenAPI YAML to JSON."""

import json
import sys
from pathlib import Path


def main() -> int:
    if len(sys.argv) != 3:
        print("Usage: yaml_to_json.py <input.yaml> <output.json>", file=sys.stderr)
        return 1

    try:
        import yaml
    except ImportError:
        print("Please install PyYAML: pip install pyyaml", file=sys.stderr)
        return 1

    input_path = Path(sys.argv[1])
    output_path = Path(sys.argv[2])

    if not input_path.is_file():
        print(f"Input file not found: {input_path}", file=sys.stderr)
        return 1

    with input_path.open("r", encoding="utf-8") as handle:
        data = yaml.safe_load(handle)

    output_path.parent.mkdir(parents=True, exist_ok=True)
    with output_path.open("w", encoding="utf-8") as handle:
        json.dump(data, handle, indent=2, ensure_ascii=False)
        handle.write("\n")

    print(f"Wrote {output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
