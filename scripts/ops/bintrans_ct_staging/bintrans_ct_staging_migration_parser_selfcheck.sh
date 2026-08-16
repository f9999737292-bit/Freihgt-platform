#!/usr/bin/env bash
# Offline self-check for golang-migrate version output parsing.
# Canonical name alias; implementation shared with migrate_version_parser_selfcheck.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
exec "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_migrate_version_parser_selfcheck.sh" "$@"
