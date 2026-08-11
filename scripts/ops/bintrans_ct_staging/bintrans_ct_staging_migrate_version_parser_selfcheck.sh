#!/usr/bin/env bash
# Static self-check for golang-migrate version output parsing (no DB/containers).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

fail() { echo "migrate-version-parser-selfcheck: $*" >&2; exit 1; }

assert_parse() {
  local label="$1"
  local input="$2"
  local expect_version="$3"
  local expect_dirty="$4"
  local parsed rc version dirty

  if ! parsed="$(bintrans_parse_migrate_version "${input}")"; then
    rc=$?
    fail "${label}: expected parse success (version=${expect_version} dirty=${expect_dirty}), got rc=${rc}"
  fi
  version="${parsed%% *}"
  dirty="${parsed#* }"
  [[ "${version}" == "${expect_version}" ]] \
    || fail "${label}: expected VERSION=${expect_version}, got ${version}"
  [[ "${dirty}" == "${expect_dirty}" ]] \
    || fail "${label}: expected DIRTY=${expect_dirty}, got ${dirty}"
  echo "OK: ${label} VERSION=${version} DIRTY=${dirty}"
}

assert_parse_fail() {
  local label="$1"
  local input="$2"
  local expect_rc="${3:-1}"
  local rc=0
  set +e
  bintrans_parse_migrate_version "${input}" >/dev/null 2>&1
  rc=$?
  set -e
  [[ "${rc}" -ne 0 ]] || fail "${label}: expected parse failure"
  [[ "${rc}" -eq "${expect_rc}" ]] \
    || fail "${label}: expected rc=${expect_rc}, got ${rc}"
  echo "OK: ${label} rejected (rc=${rc})"
}

# CASE A
assert_parse "CASE_A_plain_19" $'19\n' 19 no

# CASE B
assert_parse "CASE_B_compose_noise_before_19" $'Container bintrans-ct-staging-migrate-1 Creating\nContainer bintrans-ct-staging-migrate-1 Created\n19\n' 19 no

# CASE C
assert_parse "CASE_C_compose_noise_dirty" $'Container foo Running\n19 (dirty)\n' 19 yes

# CASE D
assert_parse "CASE_D_no_migration" $'error: no migration\n' 0 no

# CASE E
assert_parse_fail "CASE_E_noise_only" $'Container bintrans-ct-staging-migrate-1 Creating\nContainer bintrans-ct-staging-migrate-1 Created\n' 1

# CASE F
assert_parse_fail "CASE_F_conflicting_versions" $'18\n19\n' 2

# ALREADY_AT_19 parser path
assert_parse "ALREADY_AT_19" $'Container bintrans-ct-staging-migrate-run-b59dd298c0e3 Created\n19\n' 19 no

echo "bintrans-ct-staging-migrate-version-parser-selfcheck: PASS"
