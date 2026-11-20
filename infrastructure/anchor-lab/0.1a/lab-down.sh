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

anchor_lab_require_json_parser || fail "docker metadata parser is unavailable"
endpoint=$(anchor_lab_prepare_docker) || fail "docker endpoint is not an allowed local daemon"

owned_or_absent() {
  kind=$1
  name=$2
  expected_type=$3
  record=$4
  if [ "$record" = "ABSENT" ]; then
    printf '%s\n' "absent"
    return 0
  fi
  anchor_lab_json_is_object "$record" || fail "$kind inspection returned unrecognized metadata; not deleting it"
  anchor_lab_resource_is shape "$kind" "$record" "$name" "$expected_type" "$compose_file" || fail "$kind inspection returned unrecognized metadata; not deleting it"
  if ! anchor_lab_resource_is identity "$kind" "$record" "$name" "$expected_type" "$compose_file" \
    || ! anchor_lab_resource_is type "$kind" "$record" "$name" "$expected_type" "$compose_file"; then
    fail "$kind $name is not owned by the lab; not deleting it"
  fi
  anchor_lab_resource_is config "$kind" "$record" "$name" "$expected_type" "$compose_file" || fail "$kind $name has contradictory metadata; not deleting it"
  printf '%s\n' "owned"
}

network_record=$(anchor_lab_inspect_resource "$endpoint" network "$expected_network") || fail "network inspection failed"
network_state=$(owned_or_absent network "$expected_network" "$anchor_lab_network_key" "$network_record") || exit 1
volume_record=$(anchor_lab_inspect_resource "$endpoint" volume "$expected_volume") || fail "volume inspection failed"
volume_state=$(owned_or_absent volume "$expected_volume" "$anchor_lab_volume_key" "$volume_record") || exit 1

container_ids=$(anchor_lab_list_container_ids "$endpoint") || fail "container list failed"
project_containers=0
while IFS= read -r id || [ -n "$id" ]; do
  [ -n "$id" ] || continue
  record=$(anchor_lab_inspect_json "$endpoint" container "$id") || fail "container inspection failed"
  [ "$record" != "ABSENT" ] || continue
  anchor_lab_json_is_object "$record" || fail "container inspection returned unrecognized metadata"
  anchor_lab_container_is shape "$record" "$compose_file" || fail "container inspection returned unrecognized metadata"
  anchor_lab_container_is claims "$record" "$compose_file" || continue
  if anchor_lab_container_is owned "$record" "$compose_file"; then
    project_containers=1
    continue
  fi
  if anchor_lab_container_is ours "$record" "$compose_file"; then
    fail "container labels contradict the lab service or compose file; not deleting resources"
  fi
  fail "container is not owned by the lab; not deleting resources"
done <<EOF
$container_ids
EOF

if [ "$network_state" = "absent" ] && [ "$volume_state" = "absent" ] && [ "$project_containers" -eq 0 ]; then
  echo "LAB_DOWN=NOTHING_TO_REMOVE"
  exit 0
fi

if [ "$network_state" = "owned" ]; then
  echo "NETWORK_OWNERSHIP=OWNED"
fi
if [ "$volume_state" = "owned" ]; then
  echo "VOLUME_OWNERSHIP=OWNED"
fi
if [ "$project_containers" -eq 1 ]; then
  echo "CONTAINER_OWNERSHIP=OWNED"
fi

echo "LAB_DOWN_WARNING=-v deletes the lab test volume $expected_volume" >&2
docker --host "$endpoint" compose --env-file "$env_file" -p "$expected_project" -f "$compose_file" down -v
echo "LAB_DOWN=PASS"
