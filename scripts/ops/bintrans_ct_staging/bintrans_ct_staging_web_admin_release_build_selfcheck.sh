#!/usr/bin/env bash
# Selfcheck for bintrans_ct_staging_web_admin_release_build.sh (dry-run only).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
export DEPLOYED_GIT_SHA="${DEPLOYED_GIT_SHA:-3c7c6505445b922f0553be9a71b707c0fdd249e5}"
"${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_web_admin_release_build.sh" --dry-run
echo "bintrans-ct-staging-web-admin-release-build-selfcheck: PASS"
