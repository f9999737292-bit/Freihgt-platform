#!/usr/bin/env bash
set -euo pipefail

if [ -n "${BINTRANS_E2E_KFAKE_ALLOW:-}" ]; then
  echo "stand runner refuses BINTRANS_E2E_KFAKE_ALLOW" >&2
  exit 1
fi
if [ "${BINTRANS_E2E_STAND_MODE:-}" != "stand" ]; then
  echo "stand runner requires BINTRANS_E2E_STAND_MODE=stand" >&2
  exit 1
fi
if [ "${BINTRANS_E2E_KAFKA_BROKERS:-}" != "redpanda:9092" ]; then
  echo "stand runner requires BINTRANS_E2E_KAFKA_BROKERS=redpanda:9092" >&2
  exit 1
fi
if [ -z "${TEST_DATABASE_URL:-}" ]; then
  echo "stand runner requires TEST_DATABASE_URL" >&2
  exit 1
fi
if [ ! -d "${DOCUMENT_STORAGE_ROOT:-}" ]; then
  echo "stand runner requires DOCUMENT_STORAGE_ROOT" >&2
  exit 1
fi

cd /src/tests/bintrans-e2e-transport-flow
exec go test -tags=integration -count=1 -timeout 25m -run TestBintransE2ETransportFlow -v .
