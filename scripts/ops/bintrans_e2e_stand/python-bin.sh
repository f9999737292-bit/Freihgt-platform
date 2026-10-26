#!/usr/bin/env bash
# Print the path of a working Python 3 interpreter.
set -euo pipefail

for candidate in python3 python; do
  if command -v "$candidate" >/dev/null 2>&1; then
    if "$candidate" -c 'import sys; raise SystemExit(0 if sys.version_info[0] == 3 else 1)' >/dev/null 2>&1; then
      command -v "$candidate"
      exit 0
    fi
  fi
done

echo "required command python3 is missing" >&2
exit 1
