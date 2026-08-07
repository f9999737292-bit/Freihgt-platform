#!/usr/bin/env bash
# Operational rollback drill wrapper — delegates to live acceptance tenant B path.
set -euo pipefail
export MSYS_NO_PATHCONV=1
export MSYS2_ARG_CONV_EXCL="*"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"
: "${GATEWAY_URL:?GATEWAY_URL required}"
: "${READ_MODEL_URL:?READ_MODEL_URL required}"
export ROLLBACK_ONLY=1
exec "${ROOT}/scripts/dev/control_tower_projection_rebuild_rollback_acceptance.sh"
