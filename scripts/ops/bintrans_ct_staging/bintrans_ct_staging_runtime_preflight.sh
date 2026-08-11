#!/usr/bin/env bash
# BINTRANS dedicated staging — runtime deploy preflight (no containers started).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

MIGRATION_TARGET="${MIGRATION_TARGET:-000019}"
MIGRATION_VERSION=19

echo "=== BINTRANS dedicated staging runtime preflight ==="

bintrans_require_env_file

required_files=(
  "${BINTRANS_COMPOSE_BASE}"
  "${BINTRANS_COMPOSE_BINTRANS}"
  "${BINTRANS_COMPOSE_SHADOW}"
  "${BINTRANS_COMPOSE_IMAGES}"
)
for f in "${required_files[@]}"; do
  [[ -f "${f}" ]] || bintrans_fail "missing required file: ${f}"
done

env_check() {
  local key="$1" expected="$2"
  local actual
  actual="$(bintrans_env_value "${key}")"
  [[ "${actual}" == "${expected}" ]] || bintrans_fail "${key} must be ${expected} (found: ${actual:-<unset>})"
}

env_check AUTH_ENABLED true
env_check CONTROL_TOWER_READ_MODEL_MODE shadow
env_check CONTROL_TOWER_CONSUMER_ENABLED true
env_check SHIPMENT_OUTBOX_ENABLED true
env_check MIGRATION_TARGET "${MIGRATION_TARGET}"
env_check BACKUP_VERIFIED YES
echo "OK: runtime safety env fields"

bintrans_require_nonplaceholder_jwt_secret
echo "OK: JWT_SECRET present and non-placeholder"

for var in "${bintrans_runtime_image_vars[@]}"; do
  value="$(bintrans_env_value "${var}")"
  [[ -n "${value}" ]] || bintrans_fail "${var} must be set to a digest-pinned image reference for runtime deploy"
  if [[ "${value}" == *":git-"* ]] || [[ "${value}" == *":${BINTRANS_IMAGE_TAG:-git-b75eb3d}" ]]; then
    bintrans_fail "${var} must be digest-pinned (@sha256:...), not mutable tag-only (${value})"
  fi
  if [[ ! "${value}" =~ ^cr\.selcloud\.ru/bintrans-staging/[a-z0-9-]+@sha256:[0-9a-f]{64}$ ]]; then
    bintrans_fail "${var} must match cr.selcloud.ru/bintrans-staging/<service>@sha256:<64-hex> (got invalid form)"
  fi
done
echo "OK: all runtime services use digest-pinned registry references"

cohort_approved="$(bintrans_env_value COHORT_APPROVED || echo NO)"
if [[ "${cohort_approved}" != "YES" ]]; then
  echo "NOTE: COHORT_APPROVED!=YES — runtime smoke may proceed; Day 0 observation remains blocked"
fi

render_runtime="$(mktemp)"
render_observability="$(mktemp)"
trap 'rm -f "${render_runtime}" "${render_observability}"' EXIT

BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model config > "${render_runtime}"

BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model --profile observability config > "${render_observability}"

gateway_mode="$(bintrans_extract_gateway_mode "${render_runtime}")"
[[ "${gateway_mode}" == "shadow" ]] \
  || bintrans_fail "effective api-gateway CONTROL_TOWER_READ_MODEL_MODE must be shadow (found: ${gateway_mode:-<unset>})"
[[ "${gateway_mode}" == "primary" ]] && bintrans_fail "PRIMARY mode is forbidden on BINTRANS staging"
echo "OK: effective api-gateway mode=shadow"

if grep -q 'JWT_SECRET: dev_secret_change_me' "${render_runtime}"; then
  bintrans_fail "effective runtime config must not use dev JWT_SECRET default"
fi
if grep -qE 'JWT_SECRET: ""' "${render_runtime}"; then
  bintrans_fail "effective runtime JWT_SECRET must not be empty"
fi
if ! grep -q 'JWT_SECRET:' "${render_runtime}"; then
  bintrans_fail "effective runtime config must include JWT_SECRET from protected env"
fi
echo "OK: JWT_SECRET externalized in effective runtime config"

check_no_wide_bind() {
  local cfg="$1"
  local label="$2"
  if grep -E 'published: "(5432|19092|9090|8080|8081|8082|8083|8084|8085|8086|8087|8088|3000|3001)"' "${cfg}" >/dev/null; then
    while IFS= read -r pub_line; do
      port="${pub_line#*published: \"}"
      port="${port%%\"*}"
      block="$(grep -B6 "published: \"${port}\"" "${cfg}" | tail -n7)"
      if ! echo "${block}" | grep -q 'host_ip: 127.0.0.1'; then
        bintrans_fail "dangerous host bind ${port} without 127.0.0.1 in ${label}"
      fi
    done < <(grep -E 'published: "(5432|19092|9090|8080|8081|8082|8083|8084|8085|8086|8087|8088|3000|3001)"' "${cfg}" || true)
  fi
}

check_no_wide_bind "${render_runtime}" "runtime"
check_no_wide_bind "${render_observability}" "observability"
echo "OK: runtime port isolation verified"

echo "--- Published ports: runtime shadow ---"
grep -B3 'published:' "${render_runtime}" | grep -E '^(  [a-z0-9-]+:|        published:|        host_ip:)' || echo "(none)"
echo "--- Published ports: observability ---"
grep -B3 'published:' "${render_observability}" | grep -E '^(  [a-z0-9-]+:|        published:|        host_ip:)' || echo "(none)"

echo "bintrans-ct-staging-runtime-preflight: PASS"
