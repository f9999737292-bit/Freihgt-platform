#!/usr/bin/env bash
# Emit safe rollback baseline manifest (metadata only — no secrets).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

PREVIOUS_DEPLOYED_GIT_SHA="$(bintrans_env_value DEPLOYED_GIT_SHA)"
PREVIOUS_MIGRATION_VERSION="$(bintrans_read_protected_migration_target)"
DEPLOY_TOOLING_SHA="$(git -C "${ROOT}" rev-parse HEAD 2>/dev/null || echo unknown)"

if [[ "${PREVIOUS_DEPLOYED_GIT_SHA}" != "${PREVIOUS_DEPLOYED_GIT_SHA//[^0-9a-f]/}" ]] \
  || [[ "${#PREVIOUS_DEPLOYED_GIT_SHA}" -ne 40 ]]; then
  bintrans_fail "DEPLOYED_GIT_SHA must be full 40-char SHA for baseline manifest"
fi

echo "{"
echo "  \"manifest_version\": \"bintrans-staging-release-baseline-v1\","
echo "  \"generated_at_utc\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\","
echo "  \"deploy_tooling_sha\": \"${DEPLOY_TOOLING_SHA}\","
echo "  \"previous_deployed_git_sha\": \"${PREVIOUS_DEPLOYED_GIT_SHA}\","
echo "  \"previous_migration_version\": \"${PREVIOUS_MIGRATION_VERSION}\","
echo "  \"application_rollback_ready\": \"DIGEST_PINNED_IMAGES_REQUIRED\","
echo "  \"database_rollback_ready\": \"NOT_AUTOMATIC_AFTER_MIGRATION\","
echo "  \"services\": ["

first=1
for var in "${bintrans_runtime_image_vars[@]}"; do
  svc="$(bintrans_service_for_image_var "${var}")"
  ref="$(bintrans_env_value "${var}")"
  digest=""
  if [[ "${ref}" == *@sha256:* ]]; then
    digest="${ref#*@}"
  fi
  [[ "${first}" -eq 1 ]] || echo ","
  first=0
  printf '    {"service": "%s", "image_var": "%s", "previous_image_digest": "%s"}' \
    "${svc}" "${var}" "${digest}"
done
echo
echo "  ]"
echo "}"
