#!/usr/bin/env bash
# One Bash process for a dedicated-host BINTRANS E2E session.
# Default mode is CHECK_ONLY. --execute is used only after a separate
# controller approval and is not part of readiness stage 0.1F.
set -euo pipefail

root="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$root"

mode="check"
sha=""
env_file="/var/tmp/bintrans-e2e-stand/stand.env"
resources_started=no

usage() {
  echo "usage: run-session.sh --sha SHA [--execute] [--env-file PATH]" >&2
  exit 1
}

while [ $# -gt 0 ]; do
  case "$1" in
    --sha)
      shift
      sha=${1:-}
      if [ -z "$sha" ]; then
        usage
      fi
      shift
      ;;
    --execute)
      mode="execute"
      shift
      ;;
    --env-file)
      shift
      env_file=${1:-}
      if [ -z "$env_file" ]; then
        usage
      fi
      shift
      ;;
    *)
      usage
      ;;
  esac
done

if [ -z "$sha" ]; then
  usage
fi
case "$sha" in
  *[!0-9a-f]*)
    echo "approved sha must be a full 40-character hexadecimal sha" >&2
    exit 1
    ;;
esac
if [ "${#sha}" -ne 40 ]; then
  echo "approved sha must be a full 40-character hexadecimal sha" >&2
  exit 1
fi

echo "SESSION_PID=$$"
if [ "$mode" = "execute" ]; then
  echo "SESSION_MODE=EXECUTE"
else
  echo "SESSION_MODE=CHECK_ONLY"
fi

bash "$root/scripts/ops/bintrans_e2e_stand/checkout-guard.sh" --sha "$sha"

preflight_log=$(mktemp)
set +e
bash "$root/scripts/ops/bintrans_e2e_stand/host-preflight.sh" >"$preflight_log" 2>&1
preflight_status=$?
set -e
cat "$preflight_log" >&2
if [ "$preflight_status" -ne 0 ]; then
  rm -f "$preflight_log"
  exit "$preflight_status"
fi
verified_endpoint=$(awk -F= '$1=="DOCKER_ENDPOINT" {print substr($0, index($0, "=")+1)}' "$preflight_log" | tail -n 1)
verified_endpoint=$(printf '%s' "$verified_endpoint" | tr -d '[:space:]')
rm -f "$preflight_log"
case "$verified_endpoint" in
  unix:///*) ;;
  *)
    echo "verified docker endpoint is not a local unix socket" >&2
    exit 1
    ;;
esac

export DOCKER_HOST="$verified_endpoint"
unset DOCKER_CONTEXT
echo "DOCKER_HOST=$DOCKER_HOST"
echo "DOCKER_CONTEXT_UNSET=YES"

cleanup() {
  session_status=$?
  trap - EXIT INT TERM
  echo "SESSION_CLEANUP_PID=$$"
  if [ -n "${env_file:-}" ]; then
    if ! bash "$root/scripts/ops/bintrans_e2e_stand/session-cleanup.sh" --env-file "$env_file" --resources-started "$resources_started"; then
      exit 1
    fi
  fi
  exit "$session_status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

umask 077
mkdir -p "$(dirname "$env_file")"
bash "$root/scripts/ops/bintrans_e2e_stand/ephemeral-env.sh" >"$env_file"
chmod 0600 "$env_file"

if ! bash "$root/scripts/ops/bintrans_e2e_stand/project-guard.sh" --project bintrans-e2e-stand-v01a; then
  echo "project guard failed" >&2
  exit 1
fi

compose_file="infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml"
project="bintrans-e2e-stand-v01a"
if ! docker --host "$DOCKER_HOST" compose --env-file "$env_file" -p "$project" -f "$compose_file" config --quiet; then
  echo "compose config failed" >&2
  exit 1
fi
docker --host "$DOCKER_HOST" compose --env-file "$env_file" -p "$project" -f "$compose_file" config | awk '
  /^name:/ {print; seen=1}
  /^[[:space:]]*ports:/ {bad=1}
  END {if (!seen || bad) exit 1}
'

if [ "$mode" != "execute" ]; then
  echo "SESSION_RESULT=CHECK_ONLY_PASS"
  exit 0
fi

resources_started=yes
set +e
docker --host "$DOCKER_HOST" compose --env-file "$env_file" -p "$project" -f "$compose_file" up --build --abort-on-container-exit --exit-code-from runner
up_status=$?
set -e
exit "$up_status"
