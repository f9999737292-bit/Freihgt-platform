#!/usr/bin/env bash
# BINTRANS dedicated staging — PostgreSQL backup (pg_dump custom format).
# Manual operator path — validates dump but does NOT auto-publish BACKUP_VERIFIED.
# For automated daily backup use bintrans_pilot_daily_backup.sh.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_pilot_backup_lib.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_pilot_backup_lib.sh"

bintrans_pilot_create_validated_backup no

echo "Creating backup: ${BINTRANS_LAST_BACKUP_PATH}"
echo "BACKUP_PATH=${BINTRANS_LAST_BACKUP_PATH}"
echo "BACKUP_SHA256=${BINTRANS_LAST_BACKUP_SHA256}"
echo "BACKUP_SIZE_BYTES=${BINTRANS_LAST_BACKUP_SIZE_BYTES}"
echo
echo "Operator must set in protected env after manual verification:"
echo "  BACKUP_VERIFIED=YES"
echo "  BACKUP_PATH=${BINTRANS_LAST_BACKUP_PATH}"
echo "  BACKUP_SHA256=${BINTRANS_LAST_BACKUP_SHA256}"
echo
echo "Or run: bintrans_pilot_backup_metadata_update.sh ${BINTRANS_LAST_BACKUP_SHA256}"
echo
echo "bintrans-ct-staging-backup: PASS (operator must confirm BACKUP_VERIFIED=YES)"
