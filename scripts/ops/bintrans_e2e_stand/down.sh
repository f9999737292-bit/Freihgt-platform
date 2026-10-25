#!/usr/bin/env bash
# Removes only Compose project bintrans-e2e-stand-v01a.
# Usage: down.sh --env-file PATH
# The env file is passed to Compose and is never printed.
set -euo pipefail

usage() {
  echo "usage: down.sh --env-file PATH" >&2
  exit 1
}

env_file=""
while [ $# -gt 0 ]; do
  case "$1" in
    --env-file)
      if [ $# -lt 2 ] || [ -n "$env_file" ]; then
        usage
      fi
      env_file=$2
      shift 2
      ;;
    -p|--project-name|--project)
      echo "down.sh does not accept a project override" >&2
      exit 1
      ;;
    *)
      echo "unknown argument" >&2
      exit 1
      ;;
  esac
done

if [ -z "$env_file" ] || [ ! -f "$env_file" ] || [ ! -r "$env_file" ]; then
  echo "ephemeral env file is missing or unreadable" >&2
  exit 1
fi

require_key() {
  key=$1
  value=$(grep -E "^${key}=" "$env_file" | tail -n 1 | cut -d= -f2- || true)
  value=${value%$'\r'}
  if [ -z "$value" ]; then
    echo "ephemeral env file is incomplete" >&2
    exit 1
  fi
}

require_key BINTRANS_E2E_STAND_POSTGRES_USER
require_key BINTRANS_E2E_STAND_POSTGRES_PASSWORD
require_key BINTRANS_E2E_STAND_POSTGRES_DB

root="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$root"

project="bintrans-e2e-stand-v01a"
file="infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml"

bash "$root/scripts/ops/bintrans_e2e_stand/project-guard.sh" --project "$project"

exec docker compose --env-file "$env_file" -p "$project" -f "$file" down -v --remove-orphans
