#!/usr/bin/env bash
# Offline self-check for pipefail-safe running-container digest verification.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

fail() { echo "digest-verification-selfcheck: $*" >&2; exit 1; }

FAKE="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
IMAGE_REF="cr.selcloud.ru/bintrans-staging/api-gateway@sha256:${FAKE}"

# Empty RepoDigests pipeline must not abort under pipefail (regression for R4-OPS-002).
set -o pipefail
empty_match=""
if empty_match="$(printf '' | grep -E '@sha256:' | head -1 || true)"; then
  :
fi
[[ -z "${empty_match}" ]] || fail "empty RepoDigests grep must yield no match"

hex="$(bintrans_digest_hex_from_image_ref "${IMAGE_REF}")"
[[ "${hex}" == "${FAKE}" ]] || fail "digest hex extraction failed"

# Mock docker inspect: empty RepoDigests, digest-pinned Config.Image fallback.
docker() {
  case "$*" in
    *'-f {{range .RepoDigests}}'*)
      return 0
      ;;
    *'-f {{.Config.Image}}'*)
      echo "${IMAGE_REF}"
      return 0
      ;;
  esac
  command docker "$@"
}
export -f docker

digest_ref="$(bintrans_container_image_digest_ref "fixture-cid")"
[[ "${digest_ref}" == "sha256:${FAKE}" ]] || fail "Config.Image fallback failed: ${digest_ref}"

if bintrans_container_digest_matches_expected "fixture-cid" "${FAKE}"; then
  echo "OK: digest match via Config.Image fallback"
else
  fail "expected digest match"
fi

if bintrans_container_digest_matches_expected "fixture-cid" "deadbeef0123456789abcdef0123456789abcdef0123456789abcdef0123456789" 2>/dev/null; then
  fail "digest mismatch must fail"
fi
echo "OK: digest mismatch rejected"

if ( bintrans_container_image_digest_ref "" ) >/dev/null 2>&1; then
  fail "empty container id must fail"
fi
echo "OK: empty container id rejected"

unset -f docker

echo "bintrans-ct-staging-digest-verification-selfcheck: PASS"
