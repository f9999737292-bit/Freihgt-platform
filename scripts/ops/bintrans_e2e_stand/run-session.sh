#!/usr/bin/env bash
# One Bash process for a dedicated-host BINTRANS E2E session.
# Default mode is CHECK_ONLY. --execute is used only after a separate
# controller approval and is not part of readiness stage 0.1F.
set -euo pipefail

root="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$root"
ops="$root/scripts/ops/bintrans_e2e_stand"
# shellcheck disable=SC1091
. "$ops/session-common.sh"

mode="check"
sha=""
env_file=""
env_dev=""
env_ino=""
resources_started=no
child_pid=""
child_pgid=""
up_pid=""

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

bash "$ops/checkout-guard.sh" --sha "$sha"

preflight_log=$(mktemp)
set +e
bash "$ops/host-preflight.sh" >"$preflight_log" 2>&1
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

process_fields() {
  pid=$1
  if ps -o pid= -p "$$" >/dev/null 2>&1; then
    ps -o pid=,ppid=,pgid= -p "$pid" | awk 'NF>=3 {print $1, $2, $3; exit}'
  else
    ps -p "$pid" | awk 'NR>1 && NF>=3 {print $1, $2, $3; exit}'
  fi
}

list_group() {
  pgid=$1
  if ps -o pid= -p "$$" >/dev/null 2>&1; then
    ps -eo pid=,ppid=,pgid= | awk -v g="$pgid" '$3==g {print $1, $2}'
  else
    ps | awk -v g="$pgid" 'NR>1 && $3==g {print $1, $2}'
  fi
}

in_started_tree() {
  cursor=$1
  steps=0
  while [ "$cursor" != "$child_pid" ]; do
    if [ -z "$cursor" ] || [ "$cursor" = "0" ] || [ "$cursor" = "1" ]; then
      return 1
    fi
    fields=$(process_fields "$cursor") || return 1
    cursor=$(printf '%s\n' "$fields" | awk '{print $2}')
    steps=$((steps + 1))
    if [ "$steps" -gt 20 ]; then
      return 1
    fi
  done
  return 0
}

group_is_safe() {
  if [ -z "${child_pid:-}" ] || [ -z "${child_pgid:-}" ]; then
    return 1
  fi
  session_fields=$(process_fields "$$") || return 1
  session_pgid=$(printf '%s\n' "$session_fields" | awk '{print $3}')
  if [ -z "$session_pgid" ] || [ "$child_pgid" = "$session_pgid" ] || [ "$child_pgid" = "$$" ]; then
    return 1
  fi
  if [ "$child_pgid" != "$child_pid" ]; then
    return 1
  fi
  fields=$(process_fields "$child_pid") || return 1
  current_pid=$(printf '%s\n' "$fields" | awk '{print $1}')
  current_ppid=$(printf '%s\n' "$fields" | awk '{print $2}')
  current_pgid=$(printf '%s\n' "$fields" | awk '{print $3}')
  if [ "$current_pid" != "$child_pid" ] || [ "$current_pgid" != "$child_pgid" ] || [ "$current_ppid" != "$$" ]; then
    return 1
  fi
  members=$(list_group "$child_pgid") || return 1
  if [ -z "$members" ]; then
    return 1
  fi
  while read -r member_pid member_ppid; do
    if [ -z "${member_pid:-}" ]; then
      continue
    fi
    if [ "$member_pid" = "$$" ]; then
      return 1
    fi
    if ! in_started_tree "$member_pid"; then
      return 1
    fi
    unset member_ppid
  done <<EOF
$members
EOF
  return 0
}

stop_child_group() {
  if [ -z "${child_pid:-}" ]; then
    return 0
  fi
  if ! kill -0 "$child_pid" 2>/dev/null; then
    child_pid=""
    return 0
  fi
  if ! group_is_safe; then
    echo "refusing to signal an unverified process group" >&2
    return 1
  fi
  kill -TERM "-$child_pgid" || true
  i=0
  while kill -0 "$child_pid" 2>/dev/null; do
    i=$((i + 1))
    if [ "$i" -eq 30 ]; then
      if ! group_is_safe; then
        return 1
      fi
      kill -KILL "-$child_pgid" || return 1
    fi
    if [ "$i" -gt 50 ]; then
      return 1
    fi
    sleep 0.1
  done
  if [ -n "${up_pid:-}" ]; then
    wait "$up_pid" 2>/dev/null || true
  fi
  child_pid=""
  echo "CHILD_STOP=DONE"
  return 0
}

cleanup() {
  session_status=$?
  trap - EXIT INT TERM
  echo "SESSION_CLEANUP_PID=$$"
  if [ -n "${child_pid:-}" ]; then
    if ! stop_child_group; then
      echo "CLEANUP_FAILED" >&2
      exit 1
    fi
  fi
  if [ -n "${env_dev:-}" ] || [ "$resources_started" = "yes" ]; then
    cleanup_cmd=(bash "$ops/session-cleanup.sh" --env-file "$env_file" --resources-started "$resources_started")
    if [ -n "${env_dev:-}" ] && [ -n "${env_ino:-}" ]; then
      cleanup_cmd+=(--env-dev "$env_dev" --env-ino "$env_ino")
    fi
    if ! "${cleanup_cmd[@]}"; then
      exit 1
    fi
  fi
  exit "$session_status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [ -z "$env_file" ]; then
  python=$(stand_python)
  token=$("$python" -c 'import secrets; print(secrets.token_hex(8))')
  if [ -d /var/tmp ] && [ ! -L /var/tmp ]; then
    session_base=/var/tmp
  else
    session_base=${TMPDIR:-/tmp}
  fi
  env_file="$session_base/bintrans-e2e-stand-sessions/$$-$token/stand.env"
fi
case "$env_file" in
  /*|[A-Za-z]:*) ;;
  *)
    echo "env path must be absolute" >&2
    exit 1
    ;;
esac

python=$(stand_python)
native_env=$(to_native_path "$env_file")
native_root=$(to_native_path "$root")
identity=$(bash "$ops/ephemeral-env.sh" | "$python" "$ops/private-env.py" create "$native_env" "$native_root")
env_dev=$(printf '%s\n' "$identity" | awk -F= '$1=="ENV_DEV" {print $2}')
env_ino=$(printf '%s\n' "$identity" | awk -F= '$1=="ENV_INO" {print $2}')
env_mode=$(printf '%s\n' "$identity" | awk -F= '$1=="ENV_MODE" {print $2}')
case "$env_dev" in
  ""|*[!0-9]*) echo "env file identity was not recorded" >&2; exit 1 ;;
esac
case "$env_ino" in
  ""|*[!0-9]*) echo "env file identity was not recorded" >&2; exit 1 ;;
esac
echo "ENV_MODE=$env_mode"

if ! bash "$ops/project-guard.sh" --project bintrans-e2e-stand-v01a; then
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
set -m
docker --host "$DOCKER_HOST" compose --env-file "$env_file" -p "$project" -f "$compose_file" up --build --abort-on-container-exit --exit-code-from runner &
up_pid=$!
child_pid=$up_pid
if fields=$(process_fields "$child_pid"); then
  child_pgid=$(printf '%s\n' "$fields" | awk '{print $3}')
  session_fields=$(process_fields "$$")
  session_pgid=$(printf '%s\n' "$session_fields" | awk '{print $3}')
  echo "SESSION_PGID=$session_pgid"
  echo "CHILD_PID=$child_pid"
  echo "CHILD_PGID=$child_pgid"
  if ! group_is_safe; then
    echo "child process group is not isolated" >&2
    child_pgid=""
    if [ "$(printf '%s\n' "$fields" | awk '{print $2}')" = "$$" ]; then
      kill -TERM "$child_pid" 2>/dev/null || true
      wait "$child_pid" 2>/dev/null || true
    fi
    child_pid=""
    exit 1
  fi
  echo "CHILD_GROUP=ISOLATED"
  if [ -n "${SESSION_MARKER:-}" ]; then
    printf 'CHILD_GROUP=ISOLATED\nCHILD_PID=%s\nCHILD_PGID=%s\nSESSION_PGID=%s\n' "$child_pid" "$child_pgid" "$session_pgid" >> "$SESSION_MARKER"
  fi
else
  if kill -0 "$up_pid" 2>/dev/null; then
    echo "child process group is unknown" >&2
    exit 1
  fi
  child_pid=""
fi
set +m
set +e
wait "$up_pid"
up_status=$?
set -e
child_pid=""
exit "$up_status"
