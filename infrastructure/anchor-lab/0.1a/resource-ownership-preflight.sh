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
  project=${record%%|*}
  config=${record#*|}
  if [ "$project" != "$expected_project" ] || [ -z "$project" ]; then
    fail "$kind $name already exists under another owner; not adopting or deleting it"
  fi
  case "$config" in
    "$compose_file") ;;
    *)
      fail "$kind $name has a conflicting compose label; not adopting or deleting it"
      ;;
  esac
  fail "$kind $name already exists; first-start preflight does not adopt it"
}

network_record=$(anchor_lab_inspect_resource "$endpoint" network "$expected_network") || fail "network inspection failed"
if [ "$network_record" != "absent" ]; then
  reject_existing network "$expected_network" "$network_record"
fi

volume_record=$(anchor_lab_inspect_resource "$endpoint" volume "$expected_volume") || fail "volume inspection failed"
if [ "$volume_record" != "absent" ]; then
  reject_existing volume "$expected_volume" "$volume_record"
fi

ps_rows=$(docker --host "$endpoint" ps -a --format '{{.Label "com.docker.compose.project"}}|{{.Label "com.docker.compose.project.config_files"}}') || fail "container label inspection failed"
while IFS= read -r row || [ -n "$row" ]; do
  row=${row%$'\r'}
  [ -n "$row" ] || continue
  project=${row%%|*}
  config=${row#*|}
  [ -n "$project" ] || [ -n "$config" ] || continue
  if [ "$project" = "$expected_project" ]; then
    fail "a container already carries project $expected_project; first-start preflight does not adopt it"
  fi
  case "$config" in
    "$compose_file"|*"/anchor-lab/"*)
      fail "container labels conflict with the lab compose file; not adopting or deleting them"
      ;;
  esac
done <<EOF
$ps_rows
EOF

echo "OWNERSHIP_PREFLIGHT=PASS"
echo "OWNERSHIP_MODEL=FIRST_START_ONLY"
