#!/usr/bin/env python3
"""Cross-platform k6 runner with a clear error when k6 is missing."""

from __future__ import annotations

import shutil
import subprocess
import sys


def main() -> int:
    if len(sys.argv) < 2:
        print("Usage: run_k6.py <k6-script.js>")
        return 2

    k6 = shutil.which("k6")
    if not k6:
        print("k6 not found. Install from https://k6.io/docs/get-started/installation/")
        return 1

    script = sys.argv[1]
    return subprocess.call([k6, "run", script])


if __name__ == "__main__":
    sys.exit(main())
