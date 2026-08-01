#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROMTOOL="${PROMTOOL:-promtool}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://127.0.0.1:9090}"
GRAFANA_DASH="${ROOT}/infrastructure/monitoring/grafana/provisioning/dashboards/control-tower-shadow-rollout.json"

fail() {
  echo "observability-check: $*" >&2
  exit 1
}

if command -v "${PROMTOOL}" >/dev/null 2>&1; then
  "${PROMTOOL}" check rules "${ROOT}/infrastructure/monitoring/prometheus/control_tower_shadow_recording_rules.yml"
  "${PROMTOOL}" check rules "${ROOT}/infrastructure/monitoring/prometheus/control_tower_shadow_alerts.example.yml"
  "${PROMTOOL}" check config "${ROOT}/infrastructure/monitoring/prometheus/prometheus.yml"
else
  docker run --rm -v "${ROOT}/infrastructure/monitoring/prometheus:/etc/prometheus:ro" prom/prometheus:v2.54.1 \
    promtool check rules /etc/prometheus/control_tower_shadow_recording_rules.yml
  docker run --rm -v "${ROOT}/infrastructure/monitoring/prometheus:/etc/prometheus:ro" prom/prometheus:v2.54.1 \
    promtool check rules /etc/prometheus/control_tower_shadow_alerts.example.yml
  docker run --rm -v "${ROOT}/infrastructure/monitoring/prometheus:/etc/prometheus:ro" prom/prometheus:v2.54.1 \
    promtool check config /etc/prometheus/prometheus.yml
fi

command -v jq >/dev/null 2>&1 || fail "jq is required"
jq empty "${GRAFANA_DASH}"

if ! grep -q 'control_tower_shadow_alerts.example.yml' "${ROOT}/infrastructure/monitoring/prometheus/prometheus.yml"; then
  echo "observability-check: example alerts file is not loaded by active prometheus.yml"
else
  fail "example alerts must not be loaded by active prometheus.yml"
fi

if curl -fsS "${PROMETHEUS_URL}/api/v1/status/config" >/dev/null 2>&1; then
  curl -fsS "${PROMETHEUS_URL}/api/v1/rules" | grep -q 'control_tower:shadow_comparison:rate5m' \
    || fail "recording rule control_tower:shadow_comparison:rate5m not loaded"
fi

echo "control-tower-shadow-rollout-observability-check: OK"
