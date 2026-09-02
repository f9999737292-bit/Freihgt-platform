#!/usr/bin/env bash
# Automated daily validated PostgreSQL backup for BINTRANS staging.
# Uses flock, atomic .partial → .dump, fail-closed validation, metadata + metrics.
set -euo pipefail
set +x

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_pilot_backup_lib.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_pilot_backup_lib.sh"

LOCK_FILE="$(bintrans_pilot_backup_lock_file)"
RUN_UTC="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

exec 9>"${LOCK_FILE}"
if ! flock -n 9; then
  echo "bintrans-pilot-daily-backup: another backup is running — exiting safely" >&2
  exit 0
fi

backup_file=""
backup_sha256=""
backup_size_bytes=""

_on_fail() {
  local msg="${1:-backup failed}"
  bintrans_pilot_backup_write_run_state 0 "${msg}" "${RUN_UTC}" || true
  bintrans_pilot_backup_publish_metrics || true
}
trap '_on_fail "unexpected error"' ERR

bintrans_pilot_create_validated_backup yes

backup_file="${BINTRANS_LAST_BACKUP_PATH}"
backup_sha256="${BINTRANS_LAST_BACKUP_SHA256}"
backup_size_bytes="${BINTRANS_LAST_BACKUP_SIZE_BYTES}"

bintrans_pilot_backup_write_run_state 1 "validated backup complete" "${RUN_UTC}"
bintrans_pilot_backup_publish_metrics
trap - ERR

echo "BACKUP_PATH=${backup_file}"
echo "BACKUP_SHA256=${backup_sha256}"
echo "BACKUP_SIZE_BYTES=${backup_size_bytes}"
echo "BACKUP_VERIFIED=YES"
echo "BACKUP_CREATED_UTC=${RUN_UTC}"
echo "BACKUP_VALIDATED_UTC=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "bintrans-pilot-daily-backup: PASS"
