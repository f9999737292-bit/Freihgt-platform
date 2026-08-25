#!/usr/bin/env bash
# Static self-check for target-driven migration gate helpers (no DB/containers).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

fail() { echo "migrate-gate-selfcheck: $*" >&2; exit 1; }

assert_version_from_target() {
  local target="$1"
  local expect="$2"
  local actual
  actual="$(bintrans_migration_version_from_target "${target}")"
  [[ "${actual}" == "${expect}" ]] \
    || fail "target ${target}: expected version ${expect}, got ${actual}"
  echo "OK: ${target} -> ${actual}"
}

assert_format_fail() {
  local label="$1"
  local target="$2"
  if ( bintrans_validate_migration_target_format "${target}" >/dev/null 2>&1 ); then
    fail "${label}: expected format failure for ${target:-<empty>}"
  fi
  echo "OK: ${label} rejected"
}

assert_resolve_pair() {
  local target="$1"
  local migrations_dir="${ROOT}/infrastructure/migrations"
  local up_count
  up_count="$(find "${migrations_dir}" -maxdepth 1 -type f -name "${target}_*.up.sql" | wc -l | tr -d ' ')"
  if [[ "${up_count}" -eq 0 ]]; then
    echo "SKIP: ${target} migration files not present in checkout"
    return 0
  fi
  local -a files
  mapfile -t files < <(bintrans_resolve_migration_file_pair "${target}")
  [[ ${#files[@]} -eq 2 ]] || fail "${target}: expected 2 migration files, got ${#files[@]}"
  [[ -f "${files[0]}" ]] || fail "${target}: missing up file ${files[0]}"
  [[ -f "${files[1]}" ]] || fail "${target}: missing down file ${files[1]}"
  echo "OK: ${target} resolves ${files[0]##*/} + ${files[1]##*/}"
}

assert_resolve_ambiguous_fail() {
  local label="$1"
  local ambig_dir="$2"
  local target="$3"
  mkdir -p "${ambig_dir}"
  cp "${ROOT}/infrastructure/migrations/${target}_"*.up.sql "${ambig_dir}/"
  cp "${ROOT}/infrastructure/migrations/${target}_"*.down.sql "${ambig_dir}/"
  cp "${ambig_dir}/"*.up.sql "${ambig_dir}/${target}_duplicate_ambiguous.up.sql"
  cp "${ambig_dir}/"*.down.sql "${ambig_dir}/${target}_duplicate_ambiguous.down.sql"
  if (
    BINTRANS_MIGRATIONS_DIR="${ambig_dir}"
    bintrans_resolve_migration_file_pair "${target}" >/dev/null
  ); then
    fail "${label}: expected ambiguous resolution failure"
  fi
  echo "OK: ${label} rejected ambiguous files"
}

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

assert_version_from_target 000019 19
assert_version_from_target 000036 36
assert_version_from_target 000041 41
assert_version_from_target 000064 64

assert_format_fail INVALID_EMPTY ""
assert_format_fail INVALID_SHORT 36
assert_format_fail INVALID_OCTAL_LIKE 000019x
assert_format_fail INVALID_TRAVERSAL ../0036

assert_resolve_pair 000036
assert_resolve_pair 000041

# Protected env contract checks (disposable env files only)
write_env() {
  local file="$1"
  shift
  : > "${file}"
  while [[ $# -gt 0 ]]; do
    echo "$1" >> "${file}"
    shift
  done
}

env_ok="${tmpdir}/target_ok.env"
write_env "${env_ok}" "MIGRATION_TARGET=000036"
BINTRANS_STAGING_ENV="${env_ok}" bintrans_read_protected_migration_target >/dev/null \
  || fail "valid protected target rejected"
echo "OK: protected env accepts MIGRATION_TARGET=000036"

env_missing="${tmpdir}/target_missing.env"
write_env "${env_missing}" "DEPLOYED_GIT_SHA=abc123"
if ( BINTRANS_STAGING_ENV="${env_missing}" bintrans_read_protected_migration_target >/dev/null 2>&1 ); then
  fail "missing MIGRATION_TARGET should fail"
fi
echo "OK: missing MIGRATION_TARGET rejected"

env_dup="${tmpdir}/target_dup.env"
write_env "${env_dup}" "MIGRATION_TARGET=000036" "MIGRATION_TARGET=000041"
if ( BINTRANS_STAGING_ENV="${env_dup}" bintrans_read_protected_migration_target >/dev/null 2>&1 ); then
  fail "duplicate MIGRATION_TARGET should fail"
fi
echo "OK: duplicate MIGRATION_TARGET rejected"

env_confirm="${tmpdir}/confirm_in_env.env"
write_env "${env_confirm}" "MIGRATION_TARGET=000036" "CONFIRM_MIGRATION_TARGET=true"
if ( BINTRANS_STAGING_ENV="${env_confirm}" bintrans_require_migration_target_contract >/dev/null 2>&1 ); then
  fail "CONFIRM_MIGRATION_TARGET in protected env should fail"
fi
echo "OK: persistent CONFIRM_MIGRATION_TARGET rejected"

env_old_confirm="${tmpdir}/old_confirm.env"
write_env "${env_old_confirm}" "MIGRATION_TARGET=000036" "CONFIRM_MIGRATION_000019=true"
if ( BINTRANS_STAGING_ENV="${env_old_confirm}" bintrans_require_migration_target_contract >/dev/null 2>&1 ); then
  fail "CONFIRM_MIGRATION_000019 in protected env should fail"
fi
echo "OK: persistent CONFIRM_MIGRATION_000019 rejected"

# Gate decision matrix (pure numeric logic, no DB)
gate_decision() {
  local current="$1"
  local target="$2"
  if [[ "${current}" -gt "${target}" ]]; then
    echo FAIL
  elif [[ "${current}" -eq "${target}" ]]; then
    echo ALREADY_AT_TARGET
  else
    echo GATE_ONLY
  fi
}

[[ "$(gate_decision 36 36)" == "ALREADY_AT_TARGET" ]] || fail "36/36 should be ALREADY_AT_TARGET"
[[ "$(gate_decision 36 41)" == "GATE_ONLY" ]] || fail "36/41 should be GATE_ONLY"
[[ "$(gate_decision 36 19)" == "FAIL" ]] || fail "36/19 should be FAIL"
echo "OK: gate decision matrix"

# Backup requirement matrix (logic only)
backup_required_for_mutation() {
  local confirm="$1"
  local backup="$2"
  [[ "${confirm}" == "true" && "${backup}" != "YES" ]] && echo FAIL || echo OK
}

[[ "$(backup_required_for_mutation true NO)" == "FAIL" ]] || fail "mutation without backup should fail"
[[ "$(backup_required_for_mutation false NO)" == "OK" ]] || fail "gate-only without backup should not fail backup gate"
[[ "$(backup_required_for_mutation true YES)" == "OK" ]] || fail "mutation with backup should pass backup gate"
echo "OK: backup gate matrix"

ambig_dir="$(mktemp -d)"
assert_resolve_ambiguous_fail "AMBIGUOUS_000036" "${ambig_dir}" "000036"
rm -rf "${ambig_dir}"
trap 'rm -rf "${tmpdir}"' EXIT

echo "bintrans-ct-staging-migrate-gate-selfcheck: PASS"
