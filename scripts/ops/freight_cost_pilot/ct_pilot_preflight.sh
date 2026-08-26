#!/usr/bin/env bash
# CT VM operator preflight for Freight Cost Intelligence v2.2 pilot.
# Run ON the dedicated non-prod VM (161.104.57.152) after SSH access is available.
# Does NOT mutate production. Does NOT print secrets.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
ENV_FILE="${BINTRANS_STAGING_ENV:-/protected/bintrans/control-tower-observation/staging.env}"
PILOT_ENV="${FREIGHT_PILOT_ENV:-/protected/bintrans/freight-cost-pilot/pilot.env}"

fail() { echo "freight-cost-pilot-preflight: $*" >&2; exit 1; }

echo "=== Freight Cost v2.2 CT pilot preflight ==="
echo "Host: $(hostname -f 2>/dev/null || hostname)"
echo "Protected env: ${ENV_FILE}"

[[ -f "${ENV_FILE}" ]] || fail "missing protected staging env"

grep -qE '^STAGING_ENVIRONMENT=selectel-staging' "${ENV_FILE}" \
  || fail "STAGING_ENVIRONMENT must be selectel-staging"

MODE="$(grep -E '^CONTROL_TOWER_READ_MODEL_MODE=' "${ENV_FILE}" | tail -1 | cut -d= -f2- || true)"
[[ "${MODE}" == "shadow" || "${MODE}" == "disabled" ]] \
  || fail "CONTROL_TOWER_READ_MODEL_MODE must be shadow or disabled (not primary)"

echo "Docker: $(docker --version)"
echo "Compose: $(docker compose version)"

echo "--- Running containers (names only) ---"
docker ps --format '{{.Names}}\t{{.Status}}'

echo "--- Schema version (requires postgres running) ---"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"
BINTRANS_STAGING_ENV="${ENV_FILE}"
OUT="$(bintrans_compose --profile messaging run --rm migrate version 2>&1 || true)"
echo "${OUT}"
PARSED="$(bintrans_parse_migrate_version "${OUT}" || true)"
read -r SCHEMA VER_DIRTY <<< "${PARSED:-0 no}"
echo "Parsed schema: ${SCHEMA} dirty=${VER_DIRTY}"

if [[ "${SCHEMA}" -lt 64 ]]; then
  fail "schema ${SCHEMA} < 64 — run migration drill then CT migrate up to 000064 before pilot"
fi

for var in BINTRANS_FREIGHT_COST_IMAGE BINTRANS_API_GATEWAY_IMAGE BINTRANS_WEB_PROCUREMENT_IMAGE; do
  if ! grep -qE "^${var}=cr\\.selcloud\\.ru/bintrans-staging/" "${ENV_FILE}" 2>/dev/null; then
    if [[ -f "${PILOT_ENV}" ]] && grep -qE "^${var}=" "${PILOT_ENV}"; then
      echo "OK: ${var} in pilot env"
    else
      fail "${var} digest pin missing — publish v2.2 images first"
    fi
  fi
done

echo "TARGET_NON_PROD_CONFIRMED=YES"
echo "CT_MIGRATION_GATE=PASS (schema ${SCHEMA})"
echo "Preflight OK — proceed with PILOT_RUNBOOK.md Phase 1"
