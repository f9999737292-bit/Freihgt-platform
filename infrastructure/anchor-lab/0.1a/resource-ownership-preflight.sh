#!/usr/bin/env bash
# Read-only check for a future first start.
# If the lab network, volume, or a lab container already exists, stop.
# Do not adopt an existing resource and do not delete a conflicting one.
# A model for a later start is not implemented here.
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lab-common.sh
. "$script_dir/lab-common.sh"

expected_project="$anchor_lab_project"
expected_network="$anchor_lab_network"
expected_volume="$anchor_lab_volume"
compose_file="$(anchor_lab_compose_file)"

fail() {
  echo "OWNERSHIP_PREFLIGHT=FAIL $*" >&2
  exit 1
}

if [ $# -ne 0 ]; then
  fail "this preflight accepts no arguments"
fi

anchor_lab_require_json_parser || fail "docker metadata parser is unavailable"
endpoint=$(anchor_lab_prepare_docker) || fail "docker endpoint is not an allowed local daemon"

reject_existing() {
  kind=$1
  name=$2
  expected_type=$3
  record=$4
  if [ "$record" = "ABSENT" ]; then
    return 0
  fi
  anchor_lab_json_is_object "$record" || fail "$kind inspection returned unrecognized metadata"
  anchor_lab_resource_is shape "$kind" "$record" "$name" "$expected_type" "$compose_file" || fail "$kind inspection returned unrecognized metadata"
  if ! anchor_lab_resource_is identity "$kind" "$record" "$name" "$expected_type" "$compose_file"; then
    fail "$kind $name already exists under another owner; not adopting or deleting it"
  fi
  if ! anchor_lab_resource_is config "$kind" "$record" "$name" "$expected_type" "$compose_file"; then
    fail "$kind $name has contradictory metadata; not adopting or deleting it"
  fi
  if ! anchor_lab_resource_is type "$kind" "$record" "$name" "$expected_type" "$compose_file"; then
    fail "$kind $name already exists with unexpected or incomplete metadata; not adopting or deleting it"
  fi
  fail "$kind $name already exists; first-start preflight does not adopt it"
}

network_record=$(anchor_lab_inspect_resource "$endpoint" network "$expected_network") || fail "network inspection failed"
reject_existing network "$expected_network" "$anchor_lab_network_key" "$network_record"

volume_record=$(anchor_lab_inspect_resource "$endpoint" volume "$expected_volume") || fail "volume inspection failed"
reject_existing volume "$expected_volume" "$anchor_lab_volume_key" "$volume_record"

container_ids=$(anchor_lab_list_container_ids "$endpoint") || fail "container list failed"
while IFS= read -r id || [ -n "$id" ]; do
  [ -n "$id" ] || continue
  record=$(anchor_lab_inspect_json "$endpoint" container "$id") || fail "container inspection failed"
  [ "$record" != "ABSENT" ] || continue
  anchor_lab_json_is_object "$record" || fail "container inspection returned unrecognized metadata"
  anchor_lab_container_is shape "$record" "$compose_file" || fail "container inspection returned unrecognized metadata"
  anchor_lab_container_is claims "$record" "$compose_file" || continue
  if ! anchor_lab_container_is ours "$record" "$compose_file"; then
    fail "container already exists under another owner; not adopting or deleting it"
  fi
  fail "a container already carries project $expected_project; first-start preflight does not adopt it"
done <<EOF
$container_ids
EOF

echo "OWNERSHIP_PREFLIGHT=PASS"
echo "OWNERSHIP_MODEL=FIRST_START_ONLY"
