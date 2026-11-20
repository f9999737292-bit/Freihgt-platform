#!/usr/bin/env bash
# Remove only lab resources whose Compose labels match this directory.
# Refuses foreign, missing, or contradictory labels before any deletion.
# Does not accept another project, another compose file, prune, or --remove-orphans.
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lab-common.sh
. "$script_dir/lab-common.sh"

expected_project="$anchor_lab_project"
expected_network="$anchor_lab_network"
expected_volume="$anchor_lab_volume"
compose_file="$(anchor_lab_compose_file)"

fail() {
  echo "LAB_DOWN=FAIL $*" >&2
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
    -p|--project|--project-name|-f|--file|--remove-orphans)
      fail "lab-down.sh does not accept a project, file, or orphan override"
      ;;
    *)
      fail "unauthorized argument"
      ;;
  esac
done

[ -n "$env_file" ] || fail "env file is required"

bash "$script_dir/env-file-guard.sh" --env-file "$env_file"
bash "$script_dir/project-guard.sh" --env-file "$env_file" -p "$expected_project" -f "$compose_file"

endpoint=$(anchor_lab_prepare_docker) || fail "docker endpoint is not an allowed local daemon"

owned_or_absent() {
  kind=$1
  name=$2
  record=$3
  if [ "$record" = "absent" ]; then
    printf '%s\n' "absent"
    return 0
  fi
  project=${record%%|*}
  config=${record#*|}
  if [ "$project" != "$expected_project" ] || [ "$config" != "$compose_file" ]; then
    fail "$kind $name is not owned by the lab compose file; not deleting it"
  fi
  printf '%s\n' "owned"
}

network_record=$(anchor_lab_inspect_resource "$endpoint" network "$expected_network") || fail "network inspection failed"
network_state=$(owned_or_absent network "$expected_network" "$network_record") || exit 1
volume_record=$(anchor_lab_inspect_resource "$endpoint" volume "$expected_volume") || fail "volume inspection failed"
volume_state=$(owned_or_absent volume "$expected_volume" "$volume_record") || exit 1

ps_rows=$(docker --host "$endpoint" ps -a --format '{{.Label "com.docker.compose.project"}}|{{.Label "com.docker.compose.project.config_files"}}') || fail "container label inspection failed"
project_containers=0
while IFS= read -r row || [ -n "$row" ]; do
  row=${row%$'\r'}
  [ -n "$row" ] || continue
  project=${row%%|*}
  config=${row#*|}
  [ -n "$project" ] || [ -n "$config" ] || continue
  case "$config" in
    "$compose_file")
      if [ "$project" != "$expected_project" ]; then
        fail "container labels contradict the lab project; not deleting resources"
      fi
      project_containers=1
      ;;
    *)
      if [ "$project" = "$expected_project" ]; then
        fail "container labels contradict the lab compose file; not deleting resources"
      fi
      ;;
  esac
done <<EOF
$ps_rows
EOF

if [ "$network_state" = "absent" ] && [ "$volume_state" = "absent" ] && [ "$project_containers" -eq 0 ]; then
  echo "LAB_DOWN=NOTHING_TO_REMOVE"
  exit 0
fi

echo "LAB_DOWN_WARNING=-v deletes the lab test volume $expected_volume" >&2
docker --host "$endpoint" compose --env-file "$env_file" -p "$expected_project" -f "$compose_file" down -v
echo "LAB_DOWN=PASS"
