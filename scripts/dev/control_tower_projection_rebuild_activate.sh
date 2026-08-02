#!/usr/bin/env bash
set -euo pipefail

if [[ "${CONFIRM_PROJECTION_REBUILD_ACTIVATION:-}" != "true" ]]; then
  echo "CONFIRM_PROJECTION_REBUILD_ACTIVATION=true is required" >&2
  exit 2
fi

echo "Activation is not implemented in v0.1 core infrastructure." >&2
exit 2
