#!/usr/bin/env bash
# Future first start of the anchor lab. This stage does not run this script.
# A second start is refused by the first-start preflight. That later-start
# ownership model is not implemented.
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lab-common.sh
. "$script_dir/lab-common.sh"

expected_project="$anchor_lab_project"
compose_file="$(anchor_lab_compose_file)"

fail() {
  echo "LAB_UP=FAIL $*" >&2
  exit 1
}

env_file=""
while [ $# -gt 0 ]; do
  case "$1" in
    --env-file)
      [ $# -ge 2 ] || fail "env file path is required"
      [ -z "$env_file" ] || fail "env file repeated"
      env_file=$2
      shift 2
      ;;
    -p|--project|--project-name|-f|--file)
      fail "lab-up.sh does not accept a project or file override"
      ;;
    *)
      fail "unauthorized argument"
      ;;
  esac
done

[ -n "$env_file" ] || fail "env file is required"

bash "$script_dir/env-file-guard.sh" --env-file "$env_file"
bash "$script_dir/project-guard.sh" --env-file "$env_file" -p "$expected_project" -f "$compose_file"
bash "$script_dir/resource-ownership-preflight.sh"

endpoint=$(anchor_lab_prepare_docker) || fail "docker endpoint is not an allowed local daemon"
echo "LAB_UP_OWNERSHIP_MODEL=FIRST_START_ONLY" >&2
docker --host "$endpoint" compose --env-file "$env_file" -p "$expected_project" -f "$compose_file" up -d
echo "LAB_UP=PASS"
