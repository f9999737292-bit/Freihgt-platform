#!/usr/bin/env bash
# Validate digest-pinned runtime image references in protected staging env.
# Does not contact registry or start containers.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

expected_count="${#bintrans_runtime_image_vars[@]}"
pinned=0
missing=()

for var in "${bintrans_runtime_image_vars[@]}"; do
  value="$(bintrans_env_value "${var}")"
  if [[ -z "${value}" ]]; then
    missing+=("${var}")
    continue
  fi
  if [[ "${value}" =~ ^cr\.selcloud\.ru/bintrans-staging/[a-z0-9-]+@sha256:[0-9a-f]{64}$ ]]; then
    pinned=$((pinned + 1))
  else
    missing+=("${var}")
  fi
done

echo "RUNTIME_IMAGE_COUNT=${expected_count}"
echo "DIGEST_PINNED_COUNT=${pinned}"

if [[ ${#missing[@]} -eq 0 && "${pinned}" -eq "${expected_count}" ]]; then
  echo "ALL_RUNTIME_IMAGES_PINNED=YES"
  echo "bintrans-ct-staging-registry-digest-validate: PASS"
  exit 0
fi

echo "ALL_RUNTIME_IMAGES_PINNED=NO"
echo "MISSING_OR_INVALID=${missing[*]}"
echo "bintrans-ct-staging-registry-digest-validate: FAIL"
exit 1
