#!/usr/bin/env bash
# Static self-check for generic BINTRANS release contract + topology (A–P).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

fail() { echo "release-contract-selfcheck: $*" >&2; exit 1; }

assert_pass() {
  local label="$1"
  shift
  if ! "$@" >/dev/null 2>&1; then
    fail "${label}: expected PASS"
  fi
  echo "OK: ${label}"
}

assert_fail() {
  local label="$1"
  shift
  if "$@"; then
    fail "${label}: expected FAIL"
  fi
  echo "OK: ${label} rejected"
}

FIXTURE_SHA="9c334f86ece87e63aedc282984bb8e90b10f2b49"
FIXTURE_TAG="git-9c334f8"
DIGEST="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

write_env() {
  local file="$1"
  shift
  : > "${file}"
  while [[ $# -gt 0 ]]; do
    echo "$1" >> "${file}"
    shift
  done
}

digest_for() {
  printf 'cr.selcloud.ru/bintrans-staging/%s@sha256:%s' "$1" "${DIGEST}"
}

populate_digest_env() {
  local file="$1" sha="$2" tag="$3"
  write_env "${file}" \
    "DEPLOYED_GIT_SHA=${sha}" \
    "BINTRANS_IMAGE_TAG=${tag}" \
    "MIGRATION_TARGET=000064" \
    "JWT_SECRET=fixture_jwt_secret_value_32chars_minimum_required" \
    "INTERNAL_SERVICE_TOKEN=fixture_internal_service_token_32chars" \
    "POSTGRES_PASSWORD=fixture_postgres_password_not_real_32chars"
  local svc var
  for svc in "${bintrans_runtime_service_names[@]}"; do
    var="$(bintrans_runtime_image_var_for_service "${svc}")"
    echo "${var}=$(digest_for "${svc}")" >> "${file}"
  done
}

# A. VALID GENERIC RELEASE
valid_env="${tmpdir}/valid.env"
write_env "${valid_env}" \
  "DEPLOYED_GIT_SHA=${FIXTURE_SHA}" \
  "BINTRANS_IMAGE_TAG=${FIXTURE_TAG}"
BINTRANS_STAGING_ENV="${valid_env}" bintrans_validate_release_contract

# B. MISMATCHED TAG
mismatch_env="${tmpdir}/mismatch.env"
write_env "${mismatch_env}" \
  "DEPLOYED_GIT_SHA=${FIXTURE_SHA}" \
  "BINTRANS_IMAGE_TAG=git-deadbeef"
assert_fail "B_MISMATCHED_TAG" bash -c 'source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; BINTRANS_STAGING_ENV="'"${mismatch_env}"'" bintrans_validate_release_contract'

# C. LATEST IMAGE TAG
latest_env="${tmpdir}/latest.env"
write_env "${latest_env}" \
  "DEPLOYED_GIT_SHA=${FIXTURE_SHA}" \
  "BINTRANS_IMAGE_TAG=latest"
assert_fail "C_LATEST_TAG" bash -c 'source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; BINTRANS_STAGING_ENV="'"${latest_env}"'" bintrans_validate_release_contract'

# D. PLACEHOLDER IMAGE
assert_fail "D_PLACEHOLDER_DIGEST" bash -c 'source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; bintrans_validate_digest_image_ref BINTRANS_API_GATEWAY_IMAGE "cr.selcloud.ru/bintrans-staging/api-gateway@sha256:REPLACE_WITH_VERIFIED_DIGEST"'

# E. VALID DIGEST MODE
digest_env="${tmpdir}/digest.env"
populate_digest_env "${digest_env}" "${FIXTURE_SHA}" "${FIXTURE_TAG}"
BINTRANS_STAGING_ENV="${digest_env}" bintrans_validate_all_runtime_digest_images
echo "OK: E_VALID_DIGEST_MODE"

# F. MISSING REQUIRED SERVICE IMAGE
missing_env="${tmpdir}/missing.env"
grep -v 'BINTRANS_PAYMENT_IMAGE=' "${digest_env}" > "${missing_env}"
if ( BINTRANS_STAGING_ENV="${missing_env}" bintrans_validate_all_runtime_digest_images >/dev/null 2>&1 ); then
  fail "F_MISSING_SERVICE: expected FAIL"
fi
echo "OK: F_MISSING_REQUIRED_SERVICE rejected"

# G. MIXED RELEASE REVISION
mixed_env="${tmpdir}/mixed.env"
cp "${digest_env}" "${mixed_env}"
echo "BINTRANS_PAYMENT_IMAGE=cr.selcloud.ru/bintrans-staging/payment-service:git-deadbeef" >> "${mixed_env}"
assert_fail "G_MIXED_RELEASE" bash -c 'source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; BINTRANS_STAGING_ENV="'"${mixed_env}"'" bintrans_validate_no_mixed_release_tags'

# G2. ALL DIGEST PINNED (distinct digests must not false-positive as mixed tags)
all_digest_env="${tmpdir}/all_digest.env"
write_env "${all_digest_env}" \
  "DEPLOYED_GIT_SHA=${FIXTURE_SHA}" \
  "BINTRANS_IMAGE_TAG=${FIXTURE_TAG}"
for svc in "${bintrans_runtime_service_names[@]}"; do
  var="$(bintrans_runtime_image_var_for_service "${svc}")"
  svc_digest="$(printf '%064x' "${#svc}")"
  echo "${var}=cr.selcloud.ru/bintrans-staging/${svc}@sha256:${svc_digest}" >> "${all_digest_env}"
done
assert_pass "G2_ALL_DIGEST_PINNED" bash -c 'source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; BINTRANS_STAGING_ENV="'"${all_digest_env}"'" bintrans_validate_no_mixed_release_tags'

# H. OCI REVISION MISMATCH (mock via function contract)
if ( bintrans_validate_running_image_revision "nonexistent:ref" "${FIXTURE_SHA}" >/dev/null 2>&1 ); then
  fail "H_OCI_MISMATCH: expected missing label failure"
fi
echo "OK: H_OCI_REVISION_MISMATCH rejected (missing label)"

# I. ALL REQUIRED SERVICES IN STAGING OVERLAY
bintrans_validate_staging_topology_files
echo "OK: I_STAGING_OVERLAY_TOPOLOGY"

# J. ALL REQUIRED SERVICES IN RUNTIME-UP
runtime_up="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_up.sh"
grep -q 'bintrans_runtime_service_names' "${runtime_up}" \
  || fail "J: runtime_up must start canonical bintrans_runtime_service_names set"
echo "OK: J_RUNTIME_UP_TOPOLOGY"

# K. ALL REQUIRED SERVICES IN HEALTH CHECK
health_sh="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_health.sh"
grep -q 'bintrans_runtime_service_names' "${health_sh}" \
  || fail "K: runtime_health must validate canonical bintrans_runtime_service_names set"
grep -q 'bintrans_validate_project_service_names' "${health_sh}" \
  || fail "K: runtime_health must use project service validation"
echo "OK: K_RUNTIME_HEALTH_TOPOLOGY"

# L. NO UNEXPECTED PUBLIC PORTS (compose static render)
fixture_env="${ROOT}/scripts/ops/bintrans_ct_staging/fixtures/compose-static.env"
[[ -f "${fixture_env}" ]] || fail "compose fixture missing"
export BINTRANS_STAGING_ENV="${fixture_env}"
render_runtime="$(mktemp)"
BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model config > "${render_runtime}"
bintrans_check_no_wide_bind "${render_runtime}" "runtime-fixture"
echo "OK: L_NO_UNEXPECTED_PUBLIC_PORTS"

# M. MIGRATION TARGET EXISTS
assert_pass "M_MIGRATION_TARGET_EXISTS" bintrans_resolve_migration_file_pair 000064

# N. MIGRATION TARGET MISSING
if ( bintrans_resolve_migration_file_pair 999999 >/dev/null 2>&1 ); then
  fail "N: missing migration target should fail"
fi
echo "OK: N_MIGRATION_TARGET_MISSING rejected"

# O. BACKUP REQUIRED BEFORE CONFIRMED MIGRATION
[[ "$(grep -c 'BACKUP_VERIFIED=YES' "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_migrate_gate.sh")" -ge 1 ]] \
  || fail "O: migrate gate must require BACKUP_VERIFIED=YES"
echo "OK: O_BACKUP_GATE_PRESERVED"

# P. CONTROL_TOWER_READ_MODEL_MODE=shadow
gateway_mode="$(bintrans_extract_gateway_mode "${render_runtime}")"
[[ "${gateway_mode}" == "shadow" ]] || fail "P: gateway mode must be shadow (found ${gateway_mode:-<unset>})"
echo "OK: P_CONTROL_TOWER_SHADOW_MODE"

# Synthetic 36 -> 64 target resolution
assert_pass "MIGRATION_36_RESOLVES" bintrans_resolve_migration_file_pair 000036
assert_pass "MIGRATION_64_RESOLVES" bintrans_resolve_migration_file_pair 000064
assert_pass "MIGRATION_65_RESOLVES" bintrans_resolve_migration_file_pair 000065
max_target="$(bintrans_max_migration_target)"
[[ "${max_target}" == "000065" ]] || fail "expected max migration 000065, got ${max_target}"
BINTRANS_STAGING_ENV="${valid_env}" write_env "${valid_env}" \
  "DEPLOYED_GIT_SHA=${FIXTURE_SHA}" \
  "BINTRANS_IMAGE_TAG=${FIXTURE_TAG}" \
  "MIGRATION_TARGET=000065"
BINTRANS_STAGING_ENV="${valid_env}" bintrans_validate_migration_target_bounded 000065
assert_fail "MIGRATION_TARGET_ABOVE_MAX" bash -c 'source "'"${ROOT}"'/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"; bintrans_validate_migration_target_bounded 999999'
echo "OK: synthetic 36->65 bounded migration contract"

echo "bintrans-ct-staging-release-contract-selfcheck: PASS"
