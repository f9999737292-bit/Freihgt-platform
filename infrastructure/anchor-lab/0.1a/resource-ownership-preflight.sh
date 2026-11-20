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

endpoint=$(anchor_lab_prepare_docker) || fail "docker endpoint is not an allowed local daemon"

reject_existing() {
  kind=$1
  name=$2
  record=$3
  if [ "$record" = "ABSENT" ]; then
    return 0
  fi
  case "$record" in
    PRESENT$'\t'*) ;;
    *)
      fail "$kind inspection returned unrecognized metadata"
      ;;
  esac
  rest=${record#PRESENT$'\t'}
  IFS=$'\t' read -r got_name got_project got_type got_config <<EOF
$rest
EOF
  if [ "$got_project" != "$expected_project" ] || [ -z "$got_project" ] || [ "$got_name" != "$name" ]; then
    fail "$kind $name already exists under another owner; not adopting or deleting it"
  fi
  if [ -n "$got_config" ] && [ "$got_config" != "$compose_file" ]; then
    fail "$kind $name has contradictory metadata; not adopting or deleting it"
  fi
  if [ -z "$got_type" ]; then
    fail "$kind $name already exists with incomplete metadata; not adopting or deleting it"
  fi
  fail "$kind $name already exists; first-start preflight does not adopt it"
}

network_record=$(anchor_lab_inspect_resource "$endpoint" network "$expected_network") || fail "network inspection failed"
reject_existing network "$expected_network" "$network_record"

volume_record=$(anchor_lab_inspect_resource "$endpoint" volume "$expected_volume") || fail "volume inspection failed"
reject_existing volume "$expected_volume" "$volume_record"

ps_rows=$(anchor_lab_list_containers "$endpoint") || fail "container label inspection failed"
while IFS= read -r row || [ -n "$row" ]; do
  row=${row%$'\r'}
  [ -n "$row" ] || continue
  IFS=$'\t' read -r got_name got_project got_service got_config <<EOF
$row
EOF
  [ -n "$got_name" ] || [ -n "$got_project" ] || [ -n "$got_service" ] || [ -n "$got_config" ] || continue
  claims=0
  case "$got_name" in
    bintrans-anchor-lab-postgres|bintrans-anchor-lab-postgres-*)
      claims=1
      ;;
  esac
  if [ "$got_project" = "$expected_project" ] || [ "$got_config" = "$compose_file" ]; then
    claims=1
  fi
  if [ "$claims" -eq 0 ]; then
    continue
  fi
  if [ "$got_project" != "$expected_project" ] || [ -z "$got_project" ]; then
    fail "container ${got_name:-unknown} already exists under another owner; not adopting or deleting it"
  fi
  fail "a container already carries project $expected_project; first-start preflight does not adopt it"
done <<EOF
$ps_rows
EOF

echo "OWNERSHIP_PREFLIGHT=PASS"
echo "OWNERSHIP_MODEL=FIRST_START_ONLY"
