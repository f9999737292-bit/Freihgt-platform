#!/usr/bin/env bash
# Fail closed unless every lab project source is bintrans-anchor-lab.
# Static mismatches exit before Docker. A matching set is confirmed with
# `docker compose config`, whose rendered document is not printed.
#
# Calling docker compose directly does not run this guard. Only the lab
# wrapper scripts in this directory do.
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lab-common.sh
. "$script_dir/lab-common.sh"

expected="$anchor_lab_project"
compose_file="$(anchor_lab_compose_file)"

fail() {
  echo "PROJECT_GUARD=FAIL $*" >&2
  exit 1
}

env_file=""
file_arg=""
project_arg=""

while [ $# -gt 0 ]; do
  case "$1" in
    --env-file)
      [ $# -ge 2 ] || fail "env file path is required"
      [ -z "$env_file" ] || fail "env file repeated"
      env_file=$2
      shift 2
      ;;
    -f|--file)
      [ $# -ge 2 ] || fail "compose file path is required"
      [ -z "$file_arg" ] || fail "compose file repeated"
      file_arg=$2
      shift 2
      ;;
    -p|--project|--project-name)
      [ $# -ge 2 ] || fail "project selector is empty"
      if [ "$2" != "$expected" ]; then
        fail "project selector is not $expected"
      fi
      if [ -n "$project_arg" ] && [ "$project_arg" != "$2" ]; then
        fail "project selectors conflict"
      fi
      project_arg=$2
      shift 2
      ;;
    *)
      fail "unauthorized project selector"
      ;;
  esac
done

if [ -z "$env_file" ]; then
  fail "env file is required"
fi
if [ ! -f "$env_file" ] || [ ! -r "$env_file" ]; then
  fail "env file is missing or unreadable"
fi

if [ -n "$file_arg" ]; then
  if [ ! -f "$file_arg" ]; then
    fail "compose file path is not the lab compose file"
  fi
  requested="$(cd "$(dirname "$file_arg")" && pwd)/$(basename "$file_arg")"
  if [ "$requested" != "$compose_file" ]; then
    fail "compose file path is not the lab compose file"
  fi
fi

if [ ! -f "$compose_file" ]; then
  fail "lab compose file is missing"
fi

file_project=""
while IFS= read -r line || [ -n "$line" ]; do
  line=${line%$'\r'}
  case "$line" in
    name:*)
      file_project=${line#name:}
      file_project=$(printf '%s' "$file_project" | tr -d '[:space:]"'"'"'')
      break
      ;;
  esac
done < "$compose_file"

if [ "$file_project" != "$expected" ]; then
  fail "compose file name is not $expected"
fi

if [ "${COMPOSE_PROJECT_NAME+set}" = "set" ] && [ "$COMPOSE_PROJECT_NAME" != "$expected" ]; then
  fail "COMPOSE_PROJECT_NAME is not $expected"
fi

env_project_set=0
env_project=""
while IFS= read -r line || [ -n "$line" ]; do
  line=${line%$'\r'}
  case "$line" in
    COMPOSE_PROJECT_NAME=*)
      env_project=${line#COMPOSE_PROJECT_NAME=}
      env_project=${env_project#\"}
      env_project=${env_project%\"}
      env_project=${env_project#\'}
      env_project=${env_project%\'}
      env_project_set=1
      ;;
  esac
done < "$env_file"

if [ "$env_project_set" -eq 1 ] && [ "$env_project" != "$expected" ]; then
  fail "env file COMPOSE_PROJECT_NAME is not $expected"
fi

endpoint=$(anchor_lab_prepare_docker) || fail "docker endpoint is not an allowed local daemon"
rendered=$(docker --host "$endpoint" compose --env-file "$env_file" -p "$expected" -f "$compose_file" config) || fail "docker compose config rejected the lab project"
rendered_name=$(printf '%s\n' "$rendered" | awk '/^name:/ { gsub(/\r/, "", $2); print $2; exit }')
unset rendered

if [ "$rendered_name" != "$expected" ]; then
  fail "rendered compose project is not $expected"
fi

echo "PROJECT_GUARD=PASS"
