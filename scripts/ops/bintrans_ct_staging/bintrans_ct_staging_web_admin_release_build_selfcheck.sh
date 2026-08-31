#!/usr/bin/env bash
# Selfcheck for bintrans_ct_staging_web_admin_release_build.sh (dry-run + reference validation).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

export DEPLOYED_GIT_SHA="${DEPLOYED_GIT_SHA:-3c7c6505445b922f0553be9a71b707c0fdd249e5}"
"${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_web_admin_release_build.sh" --dry-run

# Reject incomplete digest-only references.
if grep -q 'BINTRANS_WEB_ADMIN_IMAGE=@sha256:' "${ROOT}/docs/ops/BINTRANS_CT_STAGING_WEB_ADMIN_DEPLOY_RUNBOOK.md"; then
  echo "bintrans-ct-staging-web-admin-release-build-selfcheck: FAIL incomplete digest reference in runbook" >&2
  exit 1
fi

# Require full immutable runtime reference example in runbook.
grep -q 'cr.selcloud.ru/bintrans-staging/web-admin@sha256:' \
  "${ROOT}/docs/ops/BINTRANS_CT_STAGING_WEB_ADMIN_DEPLOY_RUNBOOK.md" \
  || { echo "bintrans-ct-staging-web-admin-release-build-selfcheck: FAIL missing full digest reference example" >&2; exit 1; }

# Container must listen on all interfaces inside namespace; host publication loopback-only.
grep -q 'HOST: 0.0.0.0' "${ROOT}/infrastructure/docker-compose/docker-compose.bintrans-ct-staging-web-admin.yml" \
  || { echo "bintrans-ct-staging-web-admin-release-build-selfcheck: FAIL container listen not 0.0.0.0" >&2; exit 1; }
grep -q '127.0.0.1:\${WEB_ADMIN_REMOTE_HOST_PORT' "${ROOT}/infrastructure/docker-compose/docker-compose.bintrans-ct-staging-web-admin.yml" \
  || { echo "bintrans-ct-staging-web-admin-release-build-selfcheck: FAIL host publication not loopback-only" >&2; exit 1; }

echo "HOST_PUBLICATION_LOOPBACK_ONLY=YES"
echo "CONTAINER_LISTEN_ALL_INTERFACES=YES"
echo "bintrans-ct-staging-web-admin-release-build-selfcheck: PASS"
