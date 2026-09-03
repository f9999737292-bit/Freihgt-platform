#!/usr/bin/env bash
# Publish backup freshness metrics for node-exporter textfile collector.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_pilot_backup_lib.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_pilot_backup_lib.sh"

BACKUP_DIR="$(bintrans_pilot_backup_dir)"
TEXTFILE_DIR="${ROOT}/infrastructure/monitoring/prometheus-textfile"
OUT="${TEXTFILE_DIR}/bintrans_backup.prom"
META_FILE="$(bintrans_pilot_backup_metadata_file)"
RUN_STATE="$(bintrans_pilot_backup_run_state_file)"
mkdir -p "${TEXTFILE_DIR}"

ts=0
if [[ -f "${META_FILE}" ]]; then
  ts="$(python3 - <<'PY' "${META_FILE}"
import json, sys, calendar, datetime
with open(sys.argv[1], encoding="utf-8") as f:
    doc = json.load(f)
validated = doc.get("BACKUP_VALIDATED_UTC") or doc.get("BACKUP_CREATED_UTC") or ""
if validated.endswith("Z"):
    validated = validated[:-1] + "+00:00"
if validated:
    dt = datetime.datetime.fromisoformat(validated)
    print(int(dt.timestamp()))
else:
    print(0)
PY
)"
fi
if [[ "${ts}" == "0" ]]; then
  latest="$(ls -t "${BACKUP_DIR}"/freight_platform_*.dump 2>/dev/null | head -1 || true)"
  if [[ -n "${latest}" ]]; then
    ts="$(stat -c '%Y' "${latest}" 2>/dev/null || stat -f '%m' "${latest}")"
  fi
fi

run_success=0
if [[ -f "${RUN_STATE}" ]]; then
  run_success="$(python3 - <<'PY' "${RUN_STATE}"
import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    print(int(json.load(f).get("BACKUP_RUN_SUCCESS", 0)))
PY
)"
fi

tmp="$(mktemp)"
cat > "${tmp}" <<EOF
# TYPE bintrans_backup_last_success_timestamp_seconds gauge
# HELP bintrans_backup_last_success_timestamp_seconds Unix timestamp of latest validated BINTRANS pg_dump backup
bintrans_backup_last_success_timestamp_seconds ${ts}
# TYPE bintrans_pilot_backup_last_run_success gauge
# HELP bintrans_pilot_backup_last_run_success 1 if last scheduled/manual automated backup run succeeded, else 0
bintrans_pilot_backup_last_run_success ${run_success}
EOF
mv "${tmp}" "${OUT}"
chmod 644 "${OUT}"
echo "BACKUP_METRIC_TIMESTAMP=${ts}"
echo "BACKUP_LAST_RUN_SUCCESS=${run_success}"
echo "bintrans-pilot-backup-metrics: PASS"
