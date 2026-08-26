#!/usr/bin/env bash
# Validate all 13 BINTRANS runtime digest-pinned image references in protected env.
# Offline: does not contact registry or start containers.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

expected_count="${#bintrans_runtime_image_vars[@]}"
pinned=0
invalid=()

for var in "${bintrans_runtime_image_vars[@]}"; do
  value="$(bintrans_env_value "${var}")"
  if [[ -z "${value}" ]]; then
    invalid+=("${var}:missing")
    continue
  fi
  if bintrans_digest_image_ref_ok "${var}" "${value}"; then
    pinned=$((pinned + 1))
    echo "OK: ${var} -> $(bintrans_expected_image_repo "${var}")"
  else
    invalid+=("${var}:invalid")
  fi
done

echo "RUNTIME_IMAGE_COUNT=${expected_count}"
echo "DIGEST_PINNED_COUNT=${pinned}"

if [[ ${#invalid[@]} -eq 0 && "${pinned}" -eq "${expected_count}" ]]; then
  echo "ALL_RUNTIME_IMAGES_PINNED=YES"
  echo "bintrans-ct-staging-runtime-images-validate: PASS"
  exit 0
fi

echo "ALL_RUNTIME_IMAGES_PINNED=NO"
echo "INVALID_OR_MISSING=${invalid[*]}"
echo "bintrans-ct-staging-runtime-images-validate: FAIL"
exit 1
