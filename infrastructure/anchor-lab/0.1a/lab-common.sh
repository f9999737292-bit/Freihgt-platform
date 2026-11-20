#!/usr/bin/env bash
# Shared read-only helpers for the anchor lab wrappers.
# These helpers do not create or delete Docker resources.
set -euo pipefail

anchor_lab_project="bintrans-anchor-lab"
anchor_lab_network="bintrans-anchor-lab-internal"
anchor_lab_volume="bintrans-anchor-lab-postgres"
anchor_lab_network_key="anchor-lab"
anchor_lab_volume_key="anchor-lab-postgres"
anchor_lab_service="postgres"

# Compose v5.1.4 and v5.7.0 stamp networks with the project label and
# com.docker.compose.network set to the compose key. Volumes receive the
# project label and com.docker.compose.volume. Those versions do not stamp
# com.docker.compose.project.config_files on networks or volumes.
# Containers carry the project label, com.docker.compose.service, and
# com.docker.compose.project.config_files. Those rules stay separate.

anchor_lab_root() {
  cd "$(dirname "${BASH_SOURCE[0]}")" && pwd
}

anchor_lab_compose_file() {
  printf '%s\n' "$(anchor_lab_root)/compose.yaml"
}

anchor_lab_env_file() {
  printf '%s\n' "$(anchor_lab_root)/.env"
}

anchor_lab_reject_remote_endpoint() {
  label=$1
  value=${2-}
  value=${value%$'\r'}
  case "$value" in
    "")
      return 0
      ;;
    unix:///*)
      return 0
      ;;
    *)
      echo "$label is not an allowed local docker endpoint" >&2
      return 1
      ;;
  esac
}

# Resolve the Docker endpoint and require a reachable local daemon.
# Prints the unix socket endpoint. Fails before any resource mutation.
anchor_lab_prepare_docker() {
  host_env=${DOCKER_HOST-}
  if ! anchor_lab_reject_remote_endpoint "DOCKER_HOST" "$host_env"; then
    return 1
  fi

  context_name=${DOCKER_CONTEXT-}
  if [ -z "$context_name" ]; then
    if ! context_name=$(docker context show 2>/dev/null); then
      echo "docker context is unavailable" >&2
      return 1
    fi
  fi
  context_name=$(printf '%s' "$context_name" | tr -d '[:space:]')
  if [ -z "$context_name" ]; then
    echo "docker context name is empty" >&2
    return 1
  fi

  endpoint=""
  if ! endpoint=$(docker context inspect "$context_name" --format '{{.Endpoints.docker.Host}}' 2>/dev/null); then
    echo "docker context endpoint is unavailable" >&2
    return 1
  fi
  endpoint=$(printf '%s' "$endpoint" | tr -d '[:space:]')
  if ! anchor_lab_reject_remote_endpoint "docker context endpoint" "$endpoint"; then
    return 1
  fi
  if [ -z "$endpoint" ]; then
    echo "docker context endpoint is empty" >&2
    return 1
  fi
  if [ -n "$host_env" ] && [ "$host_env" != "$endpoint" ]; then
    echo "DOCKER_HOST contradicts the selected docker context endpoint" >&2
    return 1
  fi

  if ! docker --host "$endpoint" info >/dev/null 2>&1; then
    echo "docker daemon is unavailable" >&2
    return 1
  fi
  printf '%s\n' "$endpoint"
}

# Docker metadata is parsed as JSON with jq. Tab-separated templates are not
# used because bash read collapses empty fields and shifts later labels.
# Without a working jq the wrappers stop before any Docker call.
anchor_lab_require_json_parser() {
  if ! command -v jq >/dev/null 2>&1; then
    echo "jq is required to parse Docker metadata" >&2
    return 1
  fi
  probe=$(printf '%s' '{"k":"v"}' | jq -r '.k' 2>/dev/null) || probe=""
  if [ "$probe" != "v" ]; then
    echo "jq is not working; Docker metadata cannot be parsed" >&2
    return 1
  fi
}

# Succeeds only when the input is exactly one JSON object without duplicate
# keys. jq keeps the last duplicate silently, so duplicates are ambiguous.
anchor_lab_json_is_object() {
  printf '%s' "$1" | jq -s -e 'length == 1 and (.[0] | type) == "object"' >/dev/null 2>&1 || return 1
  printf '%s' "$1" | jq -n -e --stream '[inputs | select(length == 2) | .[0]] | length == (unique | length)' >/dev/null 2>&1
}

# Read one value from a single JSON object snapshot.
# $2 is a jq path. $3 is available to that path as $k.
# Prints S:<value> for a string and MISSING for an absent key or null.
# Any other type, or a parse error, returns non-zero.
anchor_lab_json_get() {
  printf '%s' "$1" | jq -r --arg k "${3-}" \
    "($2) as \$v | if \$v == null then \"MISSING\" elif (\$v | type) == \"string\" then \"S:\" + \$v else error(\"unexpected type\") end" 2>/dev/null
}

anchor_lab_type_label() {
  case "$1" in
    network) printf '%s\n' "com.docker.compose.network" ;;
    volume) printf '%s\n' "com.docker.compose.volume" ;;
    *)
      echo "inspect kind is not network or volume" >&2
      return 1
      ;;
  esac
}

# Run one read-only inspect and return its JSON object snapshot.
# Prints ABSENT for a not-found error. Any other failure, an empty answer,
# or an answer that is not exactly one JSON object returns 1.
anchor_lab_inspect_json() {
  endpoint=$1
  kind=$2
  name=$3
  err_file=$(mktemp)
  if output=$(docker --host "$endpoint" "$kind" inspect --format '{{json .}}' "$name" 2>"$err_file"); then
    rm -f "$err_file"
    output=${output%$'\r'}
    if [ -z "$output" ] || ! anchor_lab_json_is_object "$output"; then
      echo "$kind inspect returned unrecognized metadata for $name" >&2
      return 1
    fi
    printf '%s\n' "$output"
    return 0
  fi
  err=$(cat "$err_file" 2>/dev/null || true)
  rm -f "$err_file"
  case "$err" in
    *[Nn]o\ such*|*[Nn]ot\ found*)
      printf '%s\n' "ABSENT"
      return 0
      ;;
  esac
  echo "$kind inspect failed for $name" >&2
  return 1
}

# Inspect one named network or volume without changing it.
anchor_lab_inspect_resource() {
  anchor_lab_type_label "$2" >/dev/null || return 1
  anchor_lab_inspect_json "$1" "$2" "$3"
}

# Print full container IDs, one per line. Does not change containers.
anchor_lab_list_container_ids() {
  endpoint=$1
  ids=$(docker --host "$endpoint" ps -a --no-trunc --format '{{.ID}}') || return 1
  while IFS= read -r id || [ -n "$id" ]; do
    id=${id%$'\r'}
    [ -n "$id" ] || continue
    case "$id" in
      *[!0-9a-f]*)
        echo "container list returned an unrecognized id" >&2
        return 1
        ;;
    esac
    printf '%s\n' "$id"
  done <<EOF
$ids
EOF
}
