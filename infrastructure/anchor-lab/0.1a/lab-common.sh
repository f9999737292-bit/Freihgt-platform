#!/usr/bin/env bash
# Shared read-only helpers for the anchor lab wrappers.
# These helpers do not create or delete Docker resources.
set -euo pipefail

anchor_lab_project="bintrans-anchor-lab"
anchor_lab_network="bintrans-anchor-lab-internal"
anchor_lab_volume="bintrans-anchor-lab-postgres"

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

# Inspect one named network or volume.
# Prints "absent" or "project|config_files". The caller decides ownership.
# A missing resource is not created. An existing resource is not deleted.
anchor_lab_inspect_resource() {
  endpoint=$1
  kind=$2
  name=$3
  err_file=$(mktemp)
  if output=$(docker --host "$endpoint" "$kind" inspect "$name" --format '{{index .Labels "com.docker.compose.project"}}|{{index .Labels "com.docker.compose.project.config_files"}}' 2>"$err_file"); then
    rm -f "$err_file"
    printf '%s\n' "$output"
    return 0
  fi
  err=$(cat "$err_file" 2>/dev/null || true)
  rm -f "$err_file"
  case "$err" in
    *[Nn]o\ such*|*[Nn]ot\ found*)
      printf '%s\n' "absent"
      return 0
      ;;
  esac
  echo "$kind inspect failed for $name" >&2
  return 1
}
