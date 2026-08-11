#!/usr/bin/env bash
# BINTRANS dedicated staging — start observability stack ONLY (separate from runtime phase).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh
source "${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_ct_staging_common.sh"

bintrans_require_env_file

gw_cid="$(bintrans_compose --profile messaging --profile read-model ps -q api-gateway 2>/dev/null | head -n1)"
[[ -n "${gw_cid}" ]] || bintrans_fail "api-gateway must be running before observability_up"

echo "Starting BINTRANS observability (prometheus + grafana)"
BINTRANS_INCLUDE_SHADOW=1 BINTRANS_INCLUDE_IMAGES=1 \
  bintrans_compose --profile messaging --profile read-model --profile observability \
  up -d --no-build prometheus grafana

echo "Observability requested. Prometheus: 127.0.0.1:$(bintrans_env_value PROMETHEUS_PORT || echo 9090), Grafana: 127.0.0.1:$(bintrans_env_value GRAFANA_PORT || echo 3001)"
