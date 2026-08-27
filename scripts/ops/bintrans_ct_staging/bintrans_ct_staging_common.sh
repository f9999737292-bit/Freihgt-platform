#!/usr/bin/env bash
# Shared helpers for BINTRANS dedicated Control Tower staging operator scripts.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

export BINTRANS_COMPOSE_PROJECT="${BINTRANS_COMPOSE_PROJECT:-bintrans-ct-staging}"
export BINTRANS_STAGING_ENV="${BINTRANS_STAGING_ENV:-/protected/bintrans/control-tower-observation/staging.env}"
export BINTRANS_COMPOSE_BASE="${ROOT}/infrastructure/docker-compose/docker-compose.yml"
export BINTRANS_COMPOSE_BINTRANS="${ROOT}/infrastructure/docker-compose/docker-compose.bintrans-ct-staging.yml"
export BINTRANS_COMPOSE_SHADOW="${ROOT}/infrastructure/docker-compose/docker-compose.staging-shadow.yml"
export BINTRANS_COMPOSE_IMAGES="${ROOT}/infrastructure/docker-compose/docker-compose.bintrans-ct-staging-images.yml"
export BINTRANS_COMPOSE_POOL="${ROOT}/infrastructure/docker-compose/docker-compose.bintrans-ct-staging-pool.yml"

# Staging-specific PostgreSQL pool budget contract (v0.5B6.2 remediation).
bintrans_staging_postgres_effective_app_capacity() { echo 97; }
bintrans_staging_expected_aggregate_pool_budget() { echo 80; }
bintrans_staging_default_pool_max_open() { echo 25; }

bintrans_db_pool_using_service_names=(
  identity-service
  company-service
  transport-order-service
  rfx-service
  shipment-service
  document-service
  billing-register-service
  low-code-service
  payment-service
  contract-rate-service
  freight-cost-service
  control-tower-read-model-service
)

bintrans_db_pool_light_service_names=(
  shipment-service
  control-tower-read-model-service
)

bintrans_compose() {
  local -a files=(
    -f "${BINTRANS_COMPOSE_BASE}"
    -f "${BINTRANS_COMPOSE_BINTRANS}"
  )
  if [[ "${BINTRANS_INCLUDE_POOL:-1}" == "1" ]]; then
    [[ -f "${BINTRANS_COMPOSE_POOL}" ]] \
      || bintrans_fail "missing required pool overlay: ${BINTRANS_COMPOSE_POOL}"
    files+=(-f "${BINTRANS_COMPOSE_POOL}")
  fi
  if [[ "${BINTRANS_INCLUDE_SHADOW:-0}" == "1" ]]; then
    files+=(-f "${BINTRANS_COMPOSE_SHADOW}")
  fi
  if [[ "${BINTRANS_INCLUDE_IMAGES:-0}" == "1" ]]; then
    files+=(-f "${BINTRANS_COMPOSE_IMAGES}")
  fi
  docker compose \
    --env-file "${BINTRANS_STAGING_ENV}" \
    -p "${BINTRANS_COMPOSE_PROJECT}" \
    "${files[@]}" \
    "$@"
}

bintrans_fail() {
  echo "bintrans-ct-staging: $*" >&2
  exit 1
}

bintrans_require_env_file() {
  [[ -f "${BINTRANS_STAGING_ENV}" ]] || bintrans_fail "protected env missing: ${BINTRANS_STAGING_ENV}"
}

bintrans_postgres_container() {
  bintrans_compose --profile messaging ps -q postgres 2>/dev/null | head -n1
}

bintrans_redpanda_container() {
  bintrans_compose --profile messaging ps -q redpanda 2>/dev/null | head -n1
}

bintrans_env_value() {
  grep -E "^${1}=" "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2- || true
}

bintrans_migration_target_format_ok() {
  [[ "${1}" =~ ^[0-9]{6}$ ]]
}

bintrans_validate_migration_target_format() {
  local target="$1"
  bintrans_migration_target_format_ok "${target}" \
    || bintrans_fail "MIGRATION_TARGET must be exactly 6 decimal digits (got: ${target:-<empty>})"
}

bintrans_read_protected_migration_target() {
  local count target
  count="$(grep -cE '^MIGRATION_TARGET=' "${BINTRANS_STAGING_ENV}" || true)"
  [[ "${count}" -eq 1 ]] \
    || bintrans_fail "MIGRATION_TARGET must appear exactly once in protected env (found ${count})"
  target="$(bintrans_env_value MIGRATION_TARGET)"
  [[ -n "${target}" ]] || bintrans_fail "MIGRATION_TARGET must be set in protected env"
  bintrans_validate_migration_target_format "${target}"
  printf '%s\n' "${target}"
}

bintrans_migration_version_from_target() {
  local target="$1"
  bintrans_validate_migration_target_format "${target}"
  printf '%s\n' "$((10#${target}))"
}

bintrans_resolve_migration_file_pair() {
  local target="$1"
  local migrations_dir="${BINTRANS_MIGRATIONS_DIR:-${ROOT}/infrastructure/migrations}"
  local -a up_files down_files
  mapfile -t up_files < <(find "${migrations_dir}" -maxdepth 1 -type f -name "${target}_*.up.sql" | sort)
  mapfile -t down_files < <(find "${migrations_dir}" -maxdepth 1 -type f -name "${target}_*.down.sql" | sort)
  [[ ${#up_files[@]} -eq 1 ]] \
    || bintrans_fail "expected exactly one ${target}_*.up.sql in ${migrations_dir}, found ${#up_files[@]}"
  [[ ${#down_files[@]} -eq 1 ]] \
    || bintrans_fail "expected exactly one ${target}_*.down.sql in ${migrations_dir}, found ${#down_files[@]}"
  printf '%s\n' "${up_files[0]}"
  printf '%s\n' "${down_files[0]}"
}

bintrans_reject_persistent_migration_confirm() {
  if grep -qE '^CONFIRM_MIGRATION_TARGET=' "${BINTRANS_STAGING_ENV}"; then
    bintrans_fail "CONFIRM_MIGRATION_TARGET must not be stored in protected env (invocation-local only)"
  fi
  if grep -qE '^CONFIRM_MIGRATION_000019=' "${BINTRANS_STAGING_ENV}"; then
    bintrans_fail "CONFIRM_MIGRATION_000019 must not be stored in protected env (use CONFIRM_MIGRATION_TARGET invocation-local only)"
  fi
}

bintrans_require_migration_target_contract() {
  bintrans_read_protected_migration_target >/dev/null
  bintrans_reject_persistent_migration_confirm
}

# Parse golang-migrate "version" output that may include Docker Compose lifecycle noise.
# Prints "<version> <dirty>" (dirty: yes|no). Exit 0 ok, 1 unparseable, 2 conflicting.
bintrans_parse_migrate_version() {
  local output="$1"
  local -a versions=()
  local dirty=no line

  while IFS= read -r line; do
    line="$(printf '%s' "$line" | tr -d '\r' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    [[ -z "$line" ]] && continue
    if [[ "$line" =~ ^([0-9]+)[[:space:]]*\(dirty\)[[:space:]]*$ ]]; then
      versions+=("${BASH_REMATCH[1]}")
      dirty=yes
    elif [[ "$line" =~ ^([0-9]+)[[:space:]]*$ ]]; then
      versions+=("${BASH_REMATCH[1]}")
    fi
  done <<< "$(printf '%s\n' "$output")"

  if [[ ${#versions[@]} -eq 0 ]]; then
    if echo "$output" | grep -qi 'no migration'; then
      printf '0 no\n'
      return 0
    fi
    return 1
  fi

  local first="${versions[0]}"
  for v in "${versions[@]}"; do
    [[ "$v" == "$first" ]] || return 2
  done

  printf '%s %s\n' "$first" "$dirty"
  return 0
}

bintrans_jwt_secret_placeholder() {
  case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
    dev_secret_change_me|changeme|change_me|replace_me|example|example_secret|secret|your_secret|your_password) return 0 ;;
  esac
  return 1
}

bintrans_require_nonplaceholder_jwt_secret() {
  local count val
  count="$(grep -cE '^[[:space:]]*JWT_SECRET=' "${BINTRANS_STAGING_ENV}" || true)"
  [[ "${count}" -eq 1 ]] || bintrans_fail "JWT_SECRET must appear exactly once in protected env"
  val="$(bintrans_env_value JWT_SECRET)"
  [[ -n "${val}" ]] || bintrans_fail "JWT_SECRET must be non-empty in protected env"
  if bintrans_jwt_secret_placeholder "${val}"; then
    bintrans_fail "JWT_SECRET must not use an obvious placeholder value"
  fi
  if [[ "${#val}" -lt 32 ]]; then
    bintrans_fail "JWT_SECRET must be at least 32 characters for staging runtime"
  fi
}

bintrans_digest_image_pattern='^cr\.selcloud\.ru/bintrans-staging/[a-z0-9-]+@sha256:[0-9a-f]{64}$'

# ---------------------------------------------------------------------------
# Canonical BINTRANS staging application service contract (single source).
# Keep bintrans_runtime_service_names and bintrans_runtime_image_vars aligned.
# ---------------------------------------------------------------------------
bintrans_runtime_service_names=(
  identity-service
  company-service
  transport-order-service
  rfx-service
  shipment-service
  document-service
  billing-register-service
  low-code-service
  payment-service
  contract-rate-service
  freight-cost-service
  control-tower-read-model-service
  api-gateway
)

bintrans_runtime_image_vars=(
  BINTRANS_IDENTITY_IMAGE
  BINTRANS_COMPANY_IMAGE
  BINTRANS_TRANSPORT_ORDER_IMAGE
  BINTRANS_RFX_IMAGE
  BINTRANS_SHIPMENT_IMAGE
  BINTRANS_DOCUMENT_IMAGE
  BINTRANS_BILLING_REGISTER_IMAGE
  BINTRANS_LOW_CODE_IMAGE
  BINTRANS_PAYMENT_IMAGE
  BINTRANS_CONTRACT_RATE_IMAGE
  BINTRANS_FREIGHT_COST_IMAGE
  BINTRANS_CONTROL_TOWER_READ_MODEL_IMAGE
  BINTRANS_API_GATEWAY_IMAGE
)

bintrans_runtime_image_var_for_service() {
  local svc="$1" i
  for i in "${!bintrans_runtime_service_names[@]}"; do
    if [[ "${bintrans_runtime_service_names[$i]}" == "${svc}" ]]; then
      printf '%s\n' "${bintrans_runtime_image_vars[$i]}"
      return 0
    fi
  done
  return 1
}

bintrans_service_for_image_var() {
  local var="$1" i
  for i in "${!bintrans_runtime_image_vars[@]}"; do
    if [[ "${bintrans_runtime_image_vars[$i]}" == "${var}" ]]; then
      printf '%s\n' "${bintrans_runtime_service_names[$i]}"
      return 0
    fi
  done
  return 1
}

bintrans_assert_service_contract_aligned() {
  [[ "${#bintrans_runtime_service_names[@]}" -eq "${#bintrans_runtime_image_vars[@]}" ]] \
    || bintrans_fail "internal: service/image var contract length mismatch"
}

# Expected registry repository suffix per env var (must match digest ref path).
bintrans_expected_image_repo() {
  local svc
  svc="$(bintrans_service_for_image_var "$1")" || return 1
  printf '%s' "${svc}"
}

# ---------------------------------------------------------------------------
# Generic immutable release contract (no historical SHA hardcoding).
# ---------------------------------------------------------------------------
bintrans_is_full_git_sha() {
  [[ "${1}" =~ ^[0-9a-f]{40}$ ]]
}

bintrans_short_git_sha() {
  printf '%s' "${1:0:7}"
}

bintrans_expected_image_tag_for_sha() {
  printf 'git-%s' "$(bintrans_short_git_sha "$1")"
}

bintrans_release_placeholder_sha() {
  case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
    changeme|change_me|replace_me|example|0000000000000000000000000000000000000000) return 0 ;;
  esac
  [[ "$1" == *REPLACE* || "$1" == *CHANGEME* || "$1" == *TODO* ]]
}

bintrans_validate_deployed_git_sha() {
  local sha="$1"
  [[ -n "${sha}" ]] || bintrans_fail "DEPLOYED_GIT_SHA must be set"
  bintrans_is_full_git_sha "${sha}" \
    || bintrans_fail "DEPLOYED_GIT_SHA must be a full 40-character lowercase Git SHA (got: ${sha})"
  if bintrans_release_placeholder_sha "${sha}"; then
    bintrans_fail "DEPLOYED_GIT_SHA must not use a placeholder value"
  fi
}

bintrans_validate_image_tag_for_sha() {
  local sha="$1" tag="$2"
  [[ -n "${tag}" ]] || bintrans_fail "BINTRANS_IMAGE_TAG must be set"
  [[ "${tag}" != "latest" ]] || bintrans_fail "BINTRANS_IMAGE_TAG must not be 'latest'"
  if [[ "${tag}" == *REPLACE* || "${tag}" == *CHANGEME* || "${tag}" == *TODO* ]]; then
    bintrans_fail "BINTRANS_IMAGE_TAG must not use placeholder values"
  fi
  [[ "${tag}" =~ ^git-[0-9a-f]{7}$ ]] \
    || bintrans_fail "BINTRANS_IMAGE_TAG must be git-<7-char SHA> (got: ${tag})"
  local expected
  expected="$(bintrans_expected_image_tag_for_sha "${sha}")"
  [[ "${tag}" == "${expected}" ]] \
    || bintrans_fail "BINTRANS_IMAGE_TAG ${tag} does not match DEPLOYED_GIT_SHA (expected ${expected})"
}

bintrans_validate_release_contract() {
  local sha="${1:-$(bintrans_env_value DEPLOYED_GIT_SHA)}"
  local tag="${2:-$(bintrans_env_value BINTRANS_IMAGE_TAG)}"
  bintrans_validate_deployed_git_sha "${sha}"
  bintrans_validate_image_tag_for_sha "${sha}" "${tag}"
}

# Canonical OCI source for BINTRANS application images.
export BINTRANS_OCI_IMAGE_SOURCE="https://github.com/f9999737292-bit/Freihgt-platform"

bintrans_release_build_services() {
  bintrans_assert_service_contract_aligned
  printf '%s\n' "${bintrans_runtime_service_names[@]}"
}

bintrans_resolve_release_build_sha() {
  local requested="${BINTRANS_RELEASE_GIT_SHA:-}"
  local head_sha
  head_sha="$(git -C "${ROOT}" rev-parse HEAD 2>/dev/null || true)"
  [[ -n "${head_sha}" ]] || bintrans_fail "unable to resolve git HEAD for release build"
  if [[ -z "${requested}" ]]; then
    requested="${head_sha}"
  fi
  bintrans_validate_deployed_git_sha "${requested}"
  [[ "${head_sha}" == "${requested}" ]] \
    || bintrans_fail "release build requires checkout at ${requested} (HEAD is ${head_sha})"
  printf '%s\n' "${requested}"
}

bintrans_validate_release_build_version_for_sha() {
  local sha="$1" version="$2"
  local expected
  expected="$(bintrans_expected_image_tag_for_sha "${sha}")"
  [[ -n "${version}" ]] || bintrans_fail "release build requires BINTRANS_IMAGE_VERSION"
  [[ "${version}" != "latest" ]] || bintrans_fail "BINTRANS_IMAGE_VERSION must not be latest"
  [[ "${version}" == "${expected}" ]] \
    || bintrans_fail "BINTRANS_IMAGE_VERSION must be ${expected} for release SHA ${sha} (got: ${version})"
}

bintrans_validate_release_build_args() {
  local sha="${1:-}" version="${2:-}"
  [[ -n "${sha}" ]] || bintrans_fail "release build requires BINTRANS_GIT_SHA"
  bintrans_validate_deployed_git_sha "${sha}"
  bintrans_validate_release_build_version_for_sha "${sha}" "${version}"
}

bintrans_image_ref_uses_latest() {
  [[ "${1}" == *:latest ]] || [[ "${1}" == */latest ]]
}

bintrans_image_ref_is_placeholder() {
  local value="$1"
  [[ "${value}" == *REPLACE* || "${value}" == *CHANGEME* || "${value}" == *"<digest>"* || "${value}" == *"<verified_digest>"* ]]
}

bintrans_image_ref_is_digest_pinned() {
  [[ "${1}" =~ @sha256:[0-9a-f]{64}$ ]]
}

bintrans_validate_runtime_image_ref() {
  local var="$1" value="$2" tag="${3:-$(bintrans_env_value BINTRANS_IMAGE_TAG)}"
  [[ -n "${value}" ]] || bintrans_fail "${var} must be set for runtime deploy"
  if bintrans_image_ref_uses_latest "${value}"; then
    bintrans_fail "${var} must not use mutable 'latest' tag"
  fi
  if bintrans_image_ref_is_placeholder "${value}"; then
    bintrans_fail "${var} placeholder must be replaced before runtime deploy"
  fi
  if bintrans_image_ref_is_digest_pinned "${value}"; then
    bintrans_validate_digest_image_ref "${var}" "${value}"
    return 0
  fi
  if [[ "${value}" == *@sha256:* ]]; then
    bintrans_fail "${var} digest reference is malformed (expected full cr.selcloud.ru/bintrans-staging/<service>@sha256:<64-hex>)"
  fi
  local expected_repo actual_ref
  expected_repo="$(bintrans_expected_image_repo "${var}")" \
    || bintrans_fail "internal: unknown runtime image var ${var}"
  if [[ "${value}" =~ ^cr\.selcloud\.ru/bintrans-staging/([a-z0-9-]+):(.+)$ ]]; then
    actual_ref="${BASH_REMATCH[1]}"
    local actual_tag="${BASH_REMATCH[2]}"
    [[ "${actual_ref}" == "${expected_repo}" ]] \
      || bintrans_fail "${var} repository must be ${expected_repo} (found ${actual_ref})"
    [[ "${actual_tag}" == "${tag}" ]] \
      || bintrans_fail "${var} tag ${actual_tag} must match BINTRANS_IMAGE_TAG ${tag}"
    return 0
  fi
  bintrans_fail "${var} must be digest-pinned or registry tag reference cr.selcloud.ru/bintrans-staging/${expected_repo}:<tag>"
}

bintrans_collect_runtime_image_tags() {
  local var value tags=()
  for var in "${bintrans_runtime_image_vars[@]}"; do
    value="$(bintrans_env_value "${var}")"
    if bintrans_image_ref_is_digest_pinned "${value}"; then
      tags+=("@digest")
    elif [[ "${value}" =~ :([^@]+)$ ]]; then
      tags+=("${BASH_REMATCH[1]}")
    else
      tags+=("<unknown>")
    fi
  done
  printf '%s\n' "${tags[@]}"
}

bintrans_validate_no_mixed_release_tags() {
  local -a tags unique=()
  mapfile -t tags < <(bintrans_collect_runtime_image_tags)
  local tag
  for tag in "${tags[@]}"; do
    local seen=0 u
    for u in "${unique[@]:-}"; do
      [[ "${u}" == "${tag}" ]] && seen=1 && break
    done
    [[ "${seen}" -eq 0 ]] && unique+=("${tag}")
  done
  [[ "${#unique[@]}" -le 1 ]] \
    || bintrans_fail "mixed release image tags/digests across services: ${unique[*]}"
}

bintrans_oci_revision_label() {
  local ref="$1"
  docker image inspect --format='{{index .Config.Labels "org.opencontainers.image.revision"}}' "${ref}" 2>/dev/null || true
}

bintrans_validate_running_image_revision() {
  local ref="$1" expected_sha="$2" label
  label="$(bintrans_oci_revision_label "${ref}")"
  [[ -n "${label}" ]] || bintrans_fail "missing org.opencontainers.image.revision on ${ref}"
  [[ "${label}" == "${expected_sha}" ]] \
    || bintrans_fail "OCI revision ${label} on ${ref} != DEPLOYED_GIT_SHA ${expected_sha}"
}

bintrans_migrations_dir() {
  printf '%s\n' "${BINTRANS_MIGRATIONS_DIR:-${ROOT}/infrastructure/migrations}"
}

bintrans_max_migration_target() {
  local migrations_dir file base max=0 num
  migrations_dir="$(bintrans_migrations_dir)"
  while IFS= read -r file; do
    base="$(basename "${file}")"
    num="${base%%_*}"
    if bintrans_migration_target_format_ok "${num}"; then
      if [[ $((10#${num})) -gt "${max}" ]]; then
        max=$((10#${num}))
      fi
    fi
  done < <(find "${migrations_dir}" -maxdepth 1 -type f -name '*.up.sql' | sort)
  printf '%06d\n' "${max}"
}

bintrans_validate_migration_target_bounded() {
  local target="$1" max_target max_version target_version
  bintrans_validate_migration_target_format "${target}"
  max_target="$(bintrans_max_migration_target)"
  max_version="$(bintrans_migration_version_from_target "${max_target}")"
  target_version="$(bintrans_migration_version_from_target "${target}")"
  [[ "${target_version}" -le "${max_version}" ]] \
    || bintrans_fail "MIGRATION_TARGET ${target} exceeds repository max ${max_target}"
}

bintrans_extract_gateway_mode() {
  awk '
    /^  api-gateway:/ { in_gw=1; next }
    in_gw && /^  [a-zA-Z0-9_-]+:/ { exit }
    in_gw && $1 == "CONTROL_TOWER_READ_MODEL_MODE:" { print $2; exit }
  ' "$1"
}

bintrans_foundation_service_names=(
  postgres
  redpanda
)

bintrans_observability_service_names=(
  prometheus
  grafana
)

# Canonical approved running services for the full BINTRANS staging compose project.
bintrans_full_stack_service_names=(
  "${bintrans_foundation_service_names[@]}"
  "${bintrans_runtime_service_names[@]}"
  "${bintrans_observability_service_names[@]}"
)

bintrans_forbidden_project_services=(
  migrate
)

bintrans_is_approved_project_service() {
  local svc="$1"
  local approved
  for approved in "${bintrans_full_stack_service_names[@]}"; do
    [[ "${svc}" == "${approved}" ]] && return 0
  done
  return 1
}

# Validate project-wide compose ps service names: forbid migrate/unknown; allow full approved set.
bintrans_validate_project_service_names() {
  local running="$1"
  local svc
  while IFS= read -r svc; do
    [[ -z "${svc}" ]] && continue
    for forbidden in "${bintrans_forbidden_project_services[@]}"; do
      [[ "${svc}" == "${forbidden}" ]] \
        && bintrans_fail "forbidden project service running: ${forbidden}"
    done
    if ! bintrans_is_approved_project_service "${svc}"; then
      bintrans_fail "unknown project service running: ${svc}"
    fi
  done <<< "${running}"
}

bintrans_assert_services_listed() {
  local running="$1"
  shift
  local svc
  for svc in "$@"; do
    echo "${running}" | grep -qx "${svc}" \
      || bintrans_fail "expected service not running: ${svc}"
    echo "OK: service running: ${svc}"
  done
}

# Read sorted unique running service names from compose ps (project-wide enumeration).
bintrans_compose_running_service_names() {
  bintrans_compose "$@" ps --format '{{.Service}}' 2>/dev/null | sort -u
}

bintrans_runtime_forbidden_up_services=(
  migrate
  prometheus
  grafana
  postgres
  redpanda
)

bintrans_validate_digest_image_ref() {
  local var="$1"
  local value="$2"
  local expected_repo actual_repo
  [[ -n "${value}" ]] || bintrans_fail "${var} must be set to a digest-pinned image reference for runtime deploy"
  if [[ "${value}" == *":git-"* ]] || [[ "${value}" == *":${BINTRANS_IMAGE_TAG:-}" && "${value}" != *@sha256:* ]]; then
    bintrans_fail "${var} must be digest-pinned (@sha256:...), not mutable tag-only"
  fi
  if [[ "${value}" =~ ^@sha256: ]]; then
    bintrans_fail "${var} must include full registry/repository path, not bare @sha256:..."
  fi
  if [[ "${value}" == *REPLACE_WITH_VERIFIED_DIGEST* ]] || [[ "${value}" == *"<digest>"* ]] || [[ "${value}" == *"<verified_digest>"* ]]; then
    bintrans_fail "${var} placeholder digest must be replaced before runtime deploy"
  fi
  if [[ "${value}" =~ @sha256:[A-F] ]]; then
    bintrans_fail "${var} digest must use lowercase hex sha256"
  fi
  if [[ ! "${value}" =~ ^cr\.selcloud\.ru/bintrans-staging/([a-z0-9-]+)@sha256:[0-9a-f]{64}$ ]]; then
    bintrans_fail "${var} must match cr.selcloud.ru/bintrans-staging/<service>@sha256:<64-hex>"
  fi
  expected_repo="$(bintrans_expected_image_repo "${var}")" \
    || bintrans_fail "internal: unknown runtime image var ${var}"
  actual_repo="${BASH_REMATCH[1]}"
  [[ "${actual_repo}" == "${expected_repo}" ]] \
    || bintrans_fail "${var} repository must be ${expected_repo} (found ${actual_repo})"
}

bintrans_digest_image_ref_ok() {
  local var="$1" value="$2"
  ( bintrans_validate_digest_image_ref "${var}" "${value}" ) >/dev/null 2>&1
}

bintrans_validate_all_runtime_digest_images() {
  local var value
  for var in "${bintrans_runtime_image_vars[@]}"; do
    value="$(bintrans_env_value "${var}")"
    bintrans_validate_digest_image_ref "${var}" "${value}"
  done
}

bintrans_require_runtime_env_contract() {
  local key expected actual
  local -a pairs=(
    AUTH_ENABLED true
    CONTROL_TOWER_READ_MODEL_MODE shadow
    CONTROL_TOWER_CONSUMER_ENABLED true
    SHIPMENT_OUTBOX_ENABLED true
    BINTRANS_REGISTRY cr.selcloud.ru/bintrans-staging
  )
  local i=0
  while [[ $i -lt ${#pairs[@]} ]]; do
    key="${pairs[$i]}"
    expected="${pairs[$((i + 1))]}"
    actual="$(bintrans_env_value "${key}")"
    [[ "${actual}" == "${expected}" ]] \
      || bintrans_fail "${key} must be ${expected} (found: ${actual:-<unset>})"
    i=$((i + 2))
  done
  bintrans_validate_release_contract
  bintrans_validate_no_mixed_release_tags
  bintrans_require_migration_target_contract
  local migration_target
  migration_target="$(bintrans_read_protected_migration_target)"
  bintrans_validate_migration_target_bounded "${migration_target}"
  bintrans_require_nonplaceholder_jwt_secret
  local pg internal_token
  pg="$(bintrans_env_value POSTGRES_PASSWORD)"
  [[ -n "${pg}" ]] || bintrans_fail "POSTGRES_PASSWORD must be set in protected env"
  [[ "${pg}" != "freight_password" ]] || bintrans_fail "POSTGRES_PASSWORD must not use dev default freight_password"
  internal_token="$(bintrans_env_value INTERNAL_SERVICE_TOKEN)"
  [[ -n "${internal_token}" ]] || bintrans_fail "INTERNAL_SERVICE_TOKEN must be set in protected env for S2S services"
  if bintrans_jwt_secret_placeholder "${internal_token}"; then
    bintrans_fail "INTERNAL_SERVICE_TOKEN must not use an obvious placeholder value"
  fi
}

bintrans_topology_services_present_in_file() {
  local file="$1"
  shift
  local svc
  for svc in "$@"; do
    grep -qE "^  ${svc}:" "${file}" \
      || bintrans_fail "required service missing from ${file}: ${svc}"
  done
}

bintrans_validate_staging_topology_files() {
  bintrans_assert_service_contract_aligned
  bintrans_topology_services_present_in_file "${BINTRANS_COMPOSE_IMAGES}" "${bintrans_runtime_service_names[@]}"
}

bintrans_check_no_wide_bind() {
  local cfg="$1"
  local label="$2"
  if grep -E 'published: "(5432|19092|9090|8080|8081|8082|8083|8084|8085|8086|8087|8088|8090|8091|8092|3000|3001)"' "${cfg}" >/dev/null; then
    while IFS= read -r pub_line; do
      local port block
      port="${pub_line#*published: \"}"
      port="${port%%\"*}"
      block="$(grep -B6 "published: \"${port}\"" "${cfg}" | tail -n7)"
      if ! echo "${block}" | grep -q 'host_ip: 127.0.0.1'; then
        bintrans_fail "dangerous host bind ${port} without 127.0.0.1 in ${label}"
      fi
    done < <(grep -E 'published: "(5432|19092|9090|8080|8081|8082|8083|8084|8085|8086|8087|8088|8090|8091|8092|3000|3001)"' "${cfg}" || true)
  fi
}

bintrans_is_positive_int() {
  [[ "${1}" =~ ^[0-9]+$ ]] && [[ "${1}" -gt 0 ]]
}

bintrans_env_int_or_default() {
  local key="$1" fallback="$2" raw
  raw="$(bintrans_env_value "${key}")"
  if [[ -z "${raw}" ]]; then
    echo "${fallback}"
    return 0
  fi
  if ! bintrans_is_positive_int "${raw}"; then
    return 1
  fi
  echo "${raw}"
}

bintrans_service_is_light_pool() {
  local svc="$1" light
  for light in "${bintrans_db_pool_light_service_names[@]}"; do
    [[ "${svc}" == "${light}" ]] && return 0
  done
  return 1
}

bintrans_validate_pool_env_contract() {
  local max_open max_idle max_light capacity
  max_open="$(bintrans_env_int_or_default DB_MAX_OPEN_CONNS 7)" \
    || bintrans_fail "DB_MAX_OPEN_CONNS must be a positive integer"
  max_idle="$(bintrans_env_int_or_default DB_MAX_IDLE_CONNS 3)" \
    || bintrans_fail "DB_MAX_IDLE_CONNS must be a positive integer"
  max_light="$(bintrans_env_int_or_default DB_MAX_OPEN_LIGHT 5)" \
    || bintrans_fail "DB_MAX_OPEN_LIGHT must be a positive integer"
  [[ "${max_idle}" -le "${max_open}" ]] \
    || bintrans_fail "DB_MAX_IDLE_CONNS (${max_idle}) must be <= DB_MAX_OPEN_CONNS (${max_open})"
  [[ "${max_light}" -le "${max_open}" ]] \
    || bintrans_fail "DB_MAX_OPEN_LIGHT (${max_light}) must be <= DB_MAX_OPEN_CONNS (${max_open})"
  capacity="$(bintrans_staging_postgres_effective_app_capacity)"
  local expected budget
  expected="$(bintrans_staging_expected_aggregate_pool_budget)"
  budget=$(( (12 - ${#bintrans_db_pool_light_service_names[@]}) * max_open + ${#bintrans_db_pool_light_service_names[@]} * max_light ))
  [[ "${budget}" -le "${capacity}" ]] \
    || bintrans_fail "aggregate pool budget ${budget} exceeds PostgreSQL effective app capacity ${capacity}"
  [[ "${budget}" -eq "${expected}" ]] \
    || bintrans_fail "aggregate pool budget ${budget} != staging contract ${expected} (check DB_MAX_OPEN_CONNS/DB_MAX_OPEN_LIGHT)"
}

bintrans_rendered_service_env_value() {
  local cfg="$1" svc="$2" key="$3"
  awk -v svc="${svc}" -v key="${key}" '
    $0 ~ "^  " svc ":" { in_svc=1; next }
    in_svc && $0 ~ "^  [a-z0-9-]+:" && $0 !~ "^  " svc ":" { in_svc=0 }
    in_svc && $0 ~ "^      " key ":" {
      line=$0
      sub(/^      [^:]+: /, "", line)
      gsub(/"/, "", line)
      print line
      exit
    }
  ' "${cfg}"
}

bintrans_calculate_rendered_aggregate_pool_budget() {
  local cfg="$1" svc max_open sum=0
  for svc in "${bintrans_db_pool_using_service_names[@]}"; do
    max_open="$(bintrans_rendered_service_env_value "${cfg}" "${svc}" "DB_MAX_OPEN_CONNS")"
    if [[ -z "${max_open}" ]]; then
      max_open="$(bintrans_staging_default_pool_max_open)"
    fi
    if ! bintrans_is_positive_int "${max_open}"; then
      bintrans_fail "invalid rendered DB_MAX_OPEN_CONNS for ${svc}: ${max_open:-<unset>}"
    fi
    sum=$((sum + max_open))
  done
  echo "${sum}"
}

bintrans_validate_rendered_pool_budget() {
  local cfg="$1" svc max_open max_idle default_open light_open capacity aggregate
  default_open="$(bintrans_env_int_or_default DB_MAX_OPEN_CONNS 7)" \
    || bintrans_fail "DB_MAX_OPEN_CONNS must be a positive integer"
  light_open="$(bintrans_env_int_or_default DB_MAX_OPEN_LIGHT 5)" \
    || bintrans_fail "DB_MAX_OPEN_LIGHT must be a positive integer"
  capacity="$(bintrans_staging_postgres_effective_app_capacity)"
  for svc in "${bintrans_db_pool_using_service_names[@]}"; do
    max_open="$(bintrans_rendered_service_env_value "${cfg}" "${svc}" "DB_MAX_OPEN_CONNS")"
    max_idle="$(bintrans_rendered_service_env_value "${cfg}" "${svc}" "DB_MAX_IDLE_CONNS")"
    [[ -n "${max_open}" ]] \
      || bintrans_fail "rendered config missing DB_MAX_OPEN_CONNS for ${svc} (pool overlay omitted?)"
    [[ -n "${max_idle}" ]] \
      || bintrans_fail "rendered config missing DB_MAX_IDLE_CONNS for ${svc} (pool overlay omitted?)"
    if bintrans_service_is_light_pool "${svc}"; then
      [[ "${max_open}" == "${light_open}" ]] \
        || bintrans_fail "rendered ${svc} DB_MAX_OPEN_CONNS=${max_open}, expected light pool ${light_open}"
    else
      [[ "${max_open}" == "${default_open}" ]] \
        || bintrans_fail "rendered ${svc} DB_MAX_OPEN_CONNS=${max_open}, expected ${default_open}"
    fi
    [[ "${max_idle}" -le "${max_open}" ]] \
      || bintrans_fail "rendered ${svc} DB_MAX_IDLE_CONNS (${max_idle}) > DB_MAX_OPEN_CONNS (${max_open})"
    [[ "${max_open}" -ne "$(bintrans_staging_default_pool_max_open)" ]] \
      || bintrans_fail "rendered ${svc} still uses unsafe default DB_MAX_OPEN_CONNS=${max_open}"
  done
  aggregate="$(bintrans_calculate_rendered_aggregate_pool_budget "${cfg}")"
  [[ "${aggregate}" -le "${capacity}" ]] \
    || bintrans_fail "rendered aggregate pool budget ${aggregate} exceeds PostgreSQL effective capacity ${capacity}"
  [[ "${aggregate}" -eq "$(bintrans_staging_expected_aggregate_pool_budget)" ]] \
    || bintrans_fail "rendered aggregate pool budget ${aggregate} != staging contract $(bintrans_staging_expected_aggregate_pool_budget)"
}

bintrans_assert_service_contract_aligned
