#!/usr/bin/env python3
"""Verify Python version is available and >= 3.10."""

from __future__ import annotations

import sys

MIN_VERSION = (3, 10)


def main() -> int:
    version = sys.version_info
    version_text = f"{version.major}.{version.minor}.{version.micro}"
    print(f"Python version: {version_text}")

    if version[:2] < MIN_VERSION:
        print(
            f"ERROR: Python {MIN_VERSION[0]}.{MIN_VERSION[1]}+ is required "
            f"(found {version.major}.{version.minor})"
        )
        return 1

    print("Python check: OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
