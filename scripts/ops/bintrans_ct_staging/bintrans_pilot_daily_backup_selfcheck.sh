#!/usr/bin/env bash
# Static self-check for daily backup automation contract.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
fail() { echo "bintrans-pilot-daily-backup-selfcheck: $*" >&2; exit 1; }

daily="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_pilot_daily_backup.sh"
lib="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_pilot_backup_lib.sh"
metrics="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_pilot_backup_metrics.sh"
service="${ROOT}/infrastructure/systemd/bintrans-pilot-backup.service"
timer="${ROOT}/infrastructure/systemd/bintrans-pilot-backup.timer"

for f in "${daily}" "${lib}" "${metrics}" "${service}" "${timer}"; do
  [[ -f "${f}" ]] || fail "missing ${f}"
done

bash -n "${daily}" || fail "daily backup syntax"
bash -n "${lib}" || fail "backup lib syntax"
bash -n "${metrics}" || fail "metrics syntax"

grep -q 'flock' "${daily}" || fail "daily backup must use flock"
grep -q '\.partial' "${lib}" || fail "lib must use atomic partial file"
grep -q 'bintrans_pilot_backup_write_metadata_json' "${lib}" || fail "lib must write metadata"
grep -q 'publish.*yes' "${lib}" || fail "lib must support publish mode"
grep -q 'bintrans_pilot_backup_last_run_success' "${metrics}" || fail "metrics must expose last run success"
grep -q 'Europe/Moscow' "${timer}" || fail "timer must use Europe/Moscow"
grep -q 'RandomizedDelaySec=0' "${timer}" || fail "timer must not add random delay"
grep -q 'UMask=0077' "${service}" || fail "service must set UMask 0077"
grep -q 'POSTGRES_PASSWORD' "${daily}" && fail "must not reference POSTGRES_PASSWORD in daily script"
grep -q 'POSTGRES_PASSWORD' "${lib}" && fail "must not reference POSTGRES_PASSWORD in lib"

manual="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_backup.sh"
grep -q 'bintrans_pilot_create_validated_backup no' "${manual}" \
  || fail "manual backup must remain non-auto-publish"

echo "bintrans-pilot-daily-backup-selfcheck: PASS"
