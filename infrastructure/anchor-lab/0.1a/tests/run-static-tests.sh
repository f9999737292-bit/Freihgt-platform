#!/usr/bin/env bash
# Static tests for the anchor lab guards. Docker is a mock. No resource is created or deleted.
set -euo pipefail

lab="$(cd "$(dirname "$0")/.." && pwd)"
pass=0
fail_count=0
canary="CANARY_PASSWORD_SHOULD_NOT_LEAK"

say() {
  printf '%s\n' "$*"
}

record() {
  name=$1
  status=$2
  if [ "$status" -eq 0 ]; then
    pass=$((pass + 1))
    say "PASS $name"
  else
    fail_count=$((fail_count + 1))
    say "FAIL $name"
  fi
}

new_workspace() {
  work=$(mktemp -d)
  mkdir -p "$work/bin" "$work/lab" "$work/containers"
  cp "$lab/lab-common.sh" "$lab/project-guard.sh" "$lab/env-file-guard.sh" \
    "$lab/resource-ownership-preflight.sh" "$lab/lab-down.sh" "$lab/lab-up.sh" \
    "$lab/compose.yaml" "$work/lab/"
  cat > "$work/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log=${ANCHOR_LAB_MOCK_LOG:?}
printf '%s\n' "$*" >> "$log"
args=()
skip=0
for arg in "$@"; do
  if [ "$skip" -eq 1 ]; then
    skip=0
    continue
  fi
  if [ "$arg" = "--host" ]; then
    skip=1
    continue
  fi
  args+=("$arg")
done
set -- "${args[@]}"
op=${1:-}
case "$op" in
  context)
    case ${2:-} in
      show)
        printf '%s\n' "${MOCK_CONTEXT_NAME:-default}"
        ;;
      inspect)
        printf '%s\n' "${MOCK_ENDPOINT:-unix:///var/run/docker.sock}"
        ;;
      *)
        echo "unexpected context command" >&2
        exit 1
        ;;
    esac
    ;;
  info)
    exit "${MOCK_INFO_STATUS:-0}"
    ;;
  network|volume|container)
    if [ "${2:-}" != "inspect" ] || [ "${3:-}" != "--format" ] || [ "${4:-}" != '{{json .}}' ] || [ $# -ne 5 ]; then
      echo "unexpected $op command or format" >&2
      exit 2
    fi
    case "$op" in
      network) state=${MOCK_NETWORK_STATE:-absent} ;;
      volume) state=${MOCK_VOLUME_STATE:-absent} ;;
      container)
        state=${MOCK_CONTAINER_INSPECT_STATE:-file}
        if [ "$state" = "file" ]; then
          if [ -f "$(dirname "$log")/containers/$5.json" ]; then
            state=$(cat "$(dirname "$log")/containers/$5.json")
          else
            state=absent
          fi
        fi
        ;;
    esac
    case "$state" in
      absent)
        echo "Error: No such $op: $5" >&2
        exit 1
        ;;
      error)
        echo "Error: permission denied" >&2
        exit 1
        ;;
      *)
        printf '%s\n' "$state"
        ;;
    esac
    ;;
  ps)
    if [ "$*" != "ps -a --no-trunc --format {{.ID}}" ]; then
      echo "unexpected ps format" >&2
      exit 2
    fi
    if [ "${MOCK_PS_STATUS:-0}" -ne 0 ]; then
      echo "Error: permission denied" >&2
      exit "$MOCK_PS_STATUS"
    fi
    for f in "$(dirname "$log")"/containers/*.json; do
      [ -f "$f" ] || continue
      basename "$f" .json
    done
    ;;
  compose)
    action=""
    for arg in "$@"; do
      case "$arg" in
        config|down|up|create|pull) action=$arg ;;
      esac
    done
    case "$action" in
      config)
        printf '%s\n' "${MOCK_CONFIG:-name: bintrans-anchor-lab}"
        ;;
      down|up|create|pull)
        printf '%s\n' "MOCK_${action}" >> "$log"
        ;;
      *)
        echo "unexpected compose command" >&2
        exit 1
        ;;
    esac
    ;;
  *)
    echo "unexpected docker command" >&2
    exit 1
    ;;
esac
EOF
  cat > "$work/bin/stat" <<'EOF'
#!/usr/bin/env bash
if [ "${1:-}" = "-c" ] && [ "${2:-}" = "%a %u" ]; then
  printf '%s %s\n' "${MOCK_STAT_MODE:-600}" "${MOCK_STAT_UID:-$(id -u)}"
  exit 0
fi
echo "unexpected stat command" >&2
exit 1
EOF
  chmod +x "$work/bin/docker" "$work/bin/stat" "$work/lab/"*.sh
  printf '%s\n' "$work"
}

write_env() {
  dest=$1
  password=$2
  project=${3:-bintrans-anchor-lab}
  cat > "$dest" <<EOF
COMPOSE_PROJECT_NAME=$project
ANCHOR_LAB_POSTGRES_USER=anchor_lab
ANCHOR_LAB_POSTGRES_PASSWORD=$password
ANCHOR_LAB_POSTGRES_DB=anchor_lab
EOF
}

assert_no_mutation() {
  log=$1
  if [ -f "$log" ] && grep -E 'network rm|volume rm|container rm|system prune|volume prune|network prune|compose .* down|compose .* up|MOCK_down|MOCK_up|MOCK_create|MOCK_pull' "$log" >/dev/null; then
    return 1
  fi
  return 0
}

run_one() {
  name=$1
  shift
  set +e
  # errexit must be on inside the test so every assertion line counts.
  ( set -e; "$@" )
  status=$?
  set -e
  record "$name" "$status"
}

test_wrong_project_selector() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  write_env "$work/env" "static-secret-value"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" \
    bash "$work/lab/project-guard.sh" -p bintrans-e2e-stand-v01a --env-file "$work/env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "project selector is not"
  assert_no_mutation "$log"
  [ ! -s "$log" ]
  rm -rf "$work"
}

test_wrong_compose_project_name() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  write_env "$work/env" "static-secret-value"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" COMPOSE_PROJECT_NAME=bintrans-ct-staging \
    bash "$work/lab/project-guard.sh" --env-file "$work/env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "COMPOSE_PROJECT_NAME"
  assert_no_mutation "$log"
  [ ! -s "$log" ]
  rm -rf "$work"
}

test_wrong_compose_path() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  write_env "$work/env" "static-secret-value"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" \
    bash "$work/lab/project-guard.sh" -f "$lab/../../../infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml" --env-file "$work/env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "compose file path"
  assert_no_mutation "$log"
  [ ! -s "$log" ]
  rm -rf "$work"
}

test_wrong_name_key() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  write_env "$work/env" "static-secret-value"
  sed -i 's/^name: bintrans-anchor-lab$/name: bintrans-e2e-stand-v01a/' "$work/lab/compose.yaml"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" \
    bash "$work/lab/project-guard.sh" --env-file "$work/env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "compose file name"
  assert_no_mutation "$log"
  [ ! -s "$log" ]
  rm -rf "$work"
}

test_conflicting_project_sources() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  write_env "$work/env" "static-secret-value" "bintrans-ct-staging"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" COMPOSE_PROJECT_NAME=bintrans-anchor-lab \
    bash "$work/lab/project-guard.sh" -p bintrans-anchor-lab --env-file "$work/env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "env file COMPOSE_PROJECT_NAME"
  assert_no_mutation "$log"
  [ ! -s "$log" ]
  rm -rf "$work"
}

test_remote_endpoint_guard() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  write_env "$work/env" "static-secret-value"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" DOCKER_HOST=tcp://203.0.113.8:2375 \
    bash "$work/lab/project-guard.sh" --env-file "$work/env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "not an allowed local docker endpoint"
  [ ! -s "$log" ]
  rm -rf "$work"
}

test_daemon_unavailable_guard() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  write_env "$work/env" "static-secret-value"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_INFO_STATUS=1 \
    bash "$work/lab/project-guard.sh" --env-file "$work/env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "docker daemon is unavailable"
  grep -q " info" "$log"
  if grep -q " config" "$log"; then
    return 1
  fi
  assert_no_mutation "$log"
  rm -rf "$work"
}

test_guard_hides_rendered_secret() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  write_env "$work/env" "$canary"
  config_file="$work/config.yml"
  cat > "$config_file" <<EOF
name: bintrans-anchor-lab
services:
  postgres:
    environment:
      POSTGRES_PASSWORD: $canary
EOF
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_CONFIG="$(cat "$config_file")" \
    bash "$work/lab/project-guard.sh" --env-file "$work/env" 2>&1)
  status=$?
  set -e
  [ "$status" -eq 0 ]
  printf '%s\n' "$out" | grep -q "PROJECT_GUARD=PASS"
  if printf '%s\n' "$out" | grep -q "$canary"; then
    return 1
  fi
  rm -rf "$work"
}

test_placeholder_password() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  git -C "$work/lab" init -q
  write_env "$work/lab/.env" "change-me"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    bash "$work/lab/env-file-guard.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "placeholder password is forbidden"
  if printf '%s\n' "$out" | grep -q "ANCHOR_LAB_POSTGRES_PASSWORD"; then
    return 1
  fi
  [ ! -s "$log" ]
  rm -rf "$work"
}

test_empty_password() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  git -C "$work/lab" init -q
  write_env "$work/lab/.env" ""
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    bash "$work/lab/env-file-guard.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "password is empty"
  [ ! -s "$log" ]
  rm -rf "$work"
}

test_tracked_env_rejected() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  git -C "$work/lab" init -q
  write_env "$work/lab/.env" "static-secret-value"
  git -C "$work/lab" add -f .env >/dev/null
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    bash "$work/lab/env-file-guard.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "tracked by git"
  if printf '%s\n' "$out" | grep -q "static-secret-value"; then
    return 1
  fi
  rm -rf "$work"
}

test_mode_rejected_by_real_stat() {
  work=$(new_workspace)
  git -C "$work/lab" init -q
  write_env "$work/lab/.env" "static-secret-value"
  set +e
  out=$(PATH="/usr/bin" bash "$work/lab/env-file-guard.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "mode is not 0600"
  rm -rf "$work"
}

test_env_guard_hides_password() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  git -C "$work/lab" init -q
  write_env "$work/lab/.env" "static-secret-value"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    bash "$work/lab/env-file-guard.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -eq 0 ]
  printf '%s\n' "$out" | grep -q "ENV_FILE_GUARD=PASS"
  if printf '%s\n' "$out" | grep -q "static-secret-value"; then
    return 1
  fi
  [ ! -s "$log" ]
  rm -rf "$work"
}

prepare_owned_env() {
  work=$1
  git -C "$work/lab" init -q
  write_env "$work/lab/.env" "static-secret-value"
}

# Build Docker-shaped JSON. Each label argument is key=value; omitted keys are missing.
# MSYS2_ARG_CONV_EXCL keeps Git Bash from rewriting /paths passed to a native jq.
labels_json() {
  MSYS2_ARG_CONV_EXCL='*' jq -cn '$ARGS.positional | map(capture("^(?<key>[^=]*)=(?<value>[\\s\\S]*)$")) | from_entries' --args "$@"
}

resource_json() {
  name=$1
  shift
  MSYS2_ARG_CONV_EXCL='*' jq -cn --arg n "$name" --argjson l "$(labels_json "$@")" '{Name: $n, Driver: "local", Labels: $l}'
}

container_json() {
  name=$1
  shift
  MSYS2_ARG_CONV_EXCL='*' jq -cn --arg n "/$name" --argjson l "$(labels_json "$@")" '{Id: "x", Name: $n, Config: {Labels: $l}}'
}

add_container() {
  printf '%s\n' "$3" > "$1/containers/$2.json"
}

owned_network_record() {
  resource_json bintrans-anchor-lab-internal com.docker.compose.project=bintrans-anchor-lab com.docker.compose.network=anchor-lab
}

owned_volume_record() {
  resource_json bintrans-anchor-lab-postgres com.docker.compose.project=bintrans-anchor-lab com.docker.compose.volume=anchor-lab-postgres
}

foreign_network_record() {
  resource_json bintrans-anchor-lab-internal com.docker.compose.project=bintrans-e2e-stand-v01a com.docker.compose.network=bintrans-e2e-stand com.docker.compose.project.config_files=/opt/bintrans/e2e-stand/compose.yaml
}

foreign_volume_record() {
  resource_json bintrans-anchor-lab-postgres com.docker.compose.project=bintrans-e2e-stand-v01a com.docker.compose.volume=bintrans-e2e-stand-postgres com.docker.compose.project.config_files=/opt/bintrans/e2e-stand/compose.yaml
}

test_foreign_network() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" \
    MOCK_NETWORK_STATE="$(foreign_network_record)" \
    bash "$work/lab/resource-ownership-preflight.sh" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "another owner"
  assert_no_mutation "$log"
  rm -rf "$work"
}

test_foreign_volume() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" \
    MOCK_VOLUME_STATE="$(foreign_volume_record)" \
    bash "$work/lab/resource-ownership-preflight.sh" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "another owner"
  assert_no_mutation "$log"
  rm -rf "$work"
}

test_remote_preflight() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" DOCKER_HOST=ssh://example.invalid/var/run/docker.sock \
    bash "$work/lab/resource-ownership-preflight.sh" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "not an allowed local docker endpoint"
  [ ! -s "$log" ]
  rm -rf "$work"
}

test_daemon_unavailable_preflight() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_INFO_STATUS=1 \
    bash "$work/lab/resource-ownership-preflight.sh" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "docker daemon is unavailable"
  if grep -E 'network inspect|volume inspect' "$log" >/dev/null; then
    return 1
  fi
  assert_no_mutation "$log"
  rm -rf "$work"
}

test_cleanup_foreign_resources() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    MOCK_NETWORK_STATE="$(foreign_network_record)" \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "not owned"
  assert_no_mutation "$log"
  if printf '%s\n' "$out" | grep -q "static-secret-value"; then
    return 1
  fi
  rm -rf "$work"
}

test_cleanup_contradictory_labels() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  compose="$(cd "$work/lab" && pwd)/compose.yaml"
  add_container "$work" a1 "$(container_json bintrans-anchor-lab-postgres-1 com.docker.compose.project=bintrans-anchor-lab com.docker.compose.service=postgres com.docker.compose.project.config_files=/opt/bintrans/e2e-stand/compose.yaml)"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    MOCK_NETWORK_STATE="$(owned_network_record)" \
    MOCK_VOLUME_STATE="$(owned_volume_record)" \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "contradict"
  assert_no_mutation "$log"
  rm -rf "$work"
}

test_cleanup_wrong_project_before_docker() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    COMPOSE_PROJECT_NAME=bintrans-e2e-stand-v01a \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "COMPOSE_PROJECT_NAME"
  [ ! -s "$log" ]
  rm -rf "$work"
}

test_cleanup_owned_is_explicit_and_mocked() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  compose="$(cd "$work/lab" && pwd)/compose.yaml"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    MOCK_NETWORK_STATE="$(owned_network_record)" \
    MOCK_VOLUME_STATE="$(owned_volume_record)" \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -eq 0 ]
  printf '%s\n' "$out" | grep -q "LAB_DOWN_WARNING=-v deletes the lab test volume"
  grep -q -- "-p bintrans-anchor-lab" "$log"
  grep -q -- "-f $compose" "$log"
  grep -q " down -v" "$log"
  if grep -E 'remove-orphans|bintrans-e2e-stand|prune' "$log" >/dev/null; then
    return 1
  fi
  rm -rf "$work"
}

test_down_rejects_override_flags() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" --remove-orphans 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "does not accept"
  [ ! -s "$log" ]
  rm -rf "$work"
}

test_owned_network_and_volume_recognized() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  compose="$(cd "$work/lab" && pwd)/compose.yaml"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    MOCK_NETWORK_STATE="$(owned_network_record)" \
    MOCK_VOLUME_STATE="$(owned_volume_record)" \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -eq 0 ]
  printf '%s\n' "$out" | grep -q "NETWORK_OWNERSHIP=OWNED"
  printf '%s\n' "$out" | grep -q "VOLUME_OWNERSHIP=OWNED"
  printf '%s\n' "$out" | grep -q " down -v" || grep -q " down -v" "$log"
  rm -rf "$work"
}

test_owned_container_recognized() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  compose="$(cd "$work/lab" && pwd)/compose.yaml"
  add_container "$work" a1 "$(container_json bintrans-anchor-lab-postgres-1 com.docker.compose.project=bintrans-anchor-lab com.docker.compose.service=postgres "com.docker.compose.project.config_files=$compose")"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    MOCK_NETWORK_STATE="$(owned_network_record)" \
    MOCK_VOLUME_STATE="$(owned_volume_record)" \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -eq 0 ]
  printf '%s\n' "$out" | grep -q "CONTAINER_OWNERSHIP=OWNED"
  grep -q " down -v" "$log"
  if grep -E 'remove-orphans|prune|bintrans-e2e-stand' "$log" >/dev/null; then
    return 1
  fi
  rm -rf "$work"
}

test_cleanup_absent_does_nothing() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -eq 0 ]
  printf '%s\n' "$out" | grep -q "LAB_DOWN=NOTHING_TO_REMOVE"
  assert_no_mutation "$log"
  rm -rf "$work"
}

test_wrong_network_label() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    MOCK_NETWORK_STATE="$(resource_json bintrans-anchor-lab-internal com.docker.compose.project=bintrans-anchor-lab com.docker.compose.network=bintrans-e2e-stand)" \
    MOCK_VOLUME_STATE="$(owned_volume_record)" \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "not owned"
  assert_no_mutation "$log"
  echo "DOCKER_MUTATION=NO" >/dev/null
  rm -rf "$work"
}

test_wrong_volume_label() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    MOCK_NETWORK_STATE="$(owned_network_record)" \
    MOCK_VOLUME_STATE="$(resource_json bintrans-anchor-lab-postgres com.docker.compose.project=bintrans-anchor-lab com.docker.compose.volume=bintrans-e2e-stand-postgres)" \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "not owned"
  assert_no_mutation "$log"
  rm -rf "$work"
}

test_missing_labels() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    MOCK_NETWORK_STATE="$(resource_json bintrans-anchor-lab-internal)" \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "not owned"
  assert_no_mutation "$log"
  rm -rf "$work"
}

test_inspect_error_is_not_absence() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    MOCK_NETWORK_STATE=error \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "inspection failed"
  if printf '%s\n' "$out" | grep -q "NOTHING_TO_REMOVE"; then
    return 1
  fi
  assert_no_mutation "$log"
  rm -rf "$work"
}

test_foreign_container_under_lab_name() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  add_container "$work" a1 "$(container_json bintrans-anchor-lab-postgres-1 com.docker.compose.project=bintrans-e2e-stand-v01a com.docker.compose.service=postgres com.docker.compose.project.config_files=/opt/bintrans/e2e-stand/compose.yaml)"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    MOCK_NETWORK_STATE="$(owned_network_record)" \
    MOCK_VOLUME_STATE="$(owned_volume_record)" \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "not owned"
  assert_no_mutation "$log"
  rm -rf "$work"
}

test_cleanup_remote_endpoint() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    DOCKER_HOST=tcp://203.0.113.9:2375 \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "not an allowed local docker endpoint"
  [ ! -s "$log" ]
  rm -rf "$work"
}

test_partial_unknown_owner() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  prepare_owned_env "$work"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    MOCK_NETWORK_STATE="$(owned_network_record)" \
    MOCK_VOLUME_STATE="$(foreign_volume_record)" \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "not owned"
  assert_no_mutation "$log"
  if printf '%s\n' "$out" | grep -q "NETWORK_OWNERSHIP=OWNED"; then
    return 1
  fi
  rm -rf "$work"
}

test_first_start_rejects_labeled_resources() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" \
    MOCK_NETWORK_STATE="$(owned_network_record)" \
    MOCK_VOLUME_STATE="$(owned_volume_record)" \
    bash "$work/lab/resource-ownership-preflight.sh" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "does not adopt"
  assert_no_mutation "$log"
  rm -rf "$work"
}

run_down() {
  work=$1
  shift
  set +e
  out=$(env PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$work/docker.log" MOCK_STAT_MODE=600 "$@" \
    bash "$work/lab/lab-down.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
}

expect_down_refused() {
  work=$1
  message=$2
  [ "$status" -ne 0 ] || return 1
  printf '%s\n' "$out" | grep -q -- "$message" || return 1
  assert_no_mutation "$work/docker.log" || return 1
  if printf '%s\n' "$out" | grep -E "_OWNERSHIP=OWNED|static-secret-value" >/dev/null; then
    return 1
  fi
}

run_env_guard() {
  work=$1
  shift
  set +e
  out=$(env PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$work/docker.log" MOCK_STAT_MODE=600 "$@" \
    bash "$work/lab/env-file-guard.sh" --env-file "$work/lab/.env" 2>&1)
  status=$?
  set -e
}

expect_env_refused() {
  work=$1
  message=$2
  [ "$status" -ne 0 ] || return 1
  printf '%s\n' "$out" | grep -q -- "$message" || return 1
  if printf '%s\n' "$out" | grep -E "ENV_FILE_GUARD=PASS|static-secret-value|change-me" >/dev/null; then
    return 1
  fi
  assert_no_mutation "$work/docker.log" || return 1
  [ ! -s "$work/docker.log" ]
}

test_empty_project_label_shift() {
  work=$(new_workspace)
  : > "$work/docker.log"
  prepare_owned_env "$work"
  # A tab template read with IFS would shift these into an owned-looking record.
  run_down "$work" \
    MOCK_NETWORK_STATE="$(resource_json bintrans-anchor-lab-internal com.docker.compose.project= com.docker.compose.network=bintrans-anchor-lab com.docker.compose.project.config_files=anchor-lab)" \
    MOCK_VOLUME_STATE="$(owned_volume_record)"
  expect_down_refused "$work" "not owned"
  rm -rf "$work"
}

test_empty_network_label() {
  work=$(new_workspace)
  : > "$work/docker.log"
  prepare_owned_env "$work"
  run_down "$work" \
    MOCK_NETWORK_STATE="$(resource_json bintrans-anchor-lab-internal com.docker.compose.project=bintrans-anchor-lab com.docker.compose.network=)" \
    MOCK_VOLUME_STATE="$(owned_volume_record)"
  expect_down_refused "$work" "not owned"
  rm -rf "$work"
}

test_empty_volume_label() {
  work=$(new_workspace)
  : > "$work/docker.log"
  prepare_owned_env "$work"
  run_down "$work" \
    MOCK_NETWORK_STATE="$(owned_network_record)" \
    MOCK_VOLUME_STATE="$(resource_json bintrans-anchor-lab-postgres com.docker.compose.project=bintrans-anchor-lab com.docker.compose.volume=)"
  expect_down_refused "$work" "not owned"
  rm -rf "$work"
}

test_container_missing_service_label() {
  work=$(new_workspace)
  : > "$work/docker.log"
  prepare_owned_env "$work"
  compose="$(cd "$work/lab" && pwd)/compose.yaml"
  add_container "$work" a1 "$(container_json bintrans-anchor-lab-postgres-1 com.docker.compose.project=bintrans-anchor-lab "com.docker.compose.project.config_files=$compose")"
  run_down "$work" MOCK_NETWORK_STATE="$(owned_network_record)" MOCK_VOLUME_STATE="$(owned_volume_record)"
  expect_down_refused "$work" "contradict"
  rm -rf "$work"
}

test_container_missing_config_label() {
  work=$(new_workspace)
  : > "$work/docker.log"
  prepare_owned_env "$work"
  add_container "$work" a1 "$(container_json bintrans-anchor-lab-postgres-1 com.docker.compose.project=bintrans-anchor-lab com.docker.compose.service=postgres)"
  run_down "$work" MOCK_NETWORK_STATE="$(owned_network_record)" MOCK_VOLUME_STATE="$(owned_volume_record)"
  expect_down_refused "$work" "contradict"
  rm -rf "$work"
}

test_shifted_metadata_fields() {
  work=$(new_workspace)
  : > "$work/docker.log"
  prepare_owned_env "$work"
  run_down "$work" \
    MOCK_NETWORK_STATE="$(resource_json bintrans-anchor-lab-internal com.docker.compose.project=anchor-lab com.docker.compose.network=bintrans-anchor-lab)" \
    MOCK_VOLUME_STATE="$(owned_volume_record)"
  expect_down_refused "$work" "not owned"
  run_down "$work" \
    MOCK_NETWORK_STATE="$(resource_json bintrans-anchor-lab-internal "com.docker.compose.project=$(printf '\t')bintrans-anchor-lab" com.docker.compose.network=anchor-lab)" \
    MOCK_VOLUME_STATE="$(owned_volume_record)"
  expect_down_refused "$work" "not owned"
  rm -rf "$work"
}

test_malformed_metadata() {
  work=$(new_workspace)
  : > "$work/docker.log"
  prepare_owned_env "$work"
  owned=$(owned_network_record)
  for payload in \
    '{"Name": "bintrans-anchor-lab-internal", "Labels": ' \
    "[$owned]" \
    "$owned$owned" \
    "$(printf '%s\t%s\t%s\t' bintrans-anchor-lab-internal bintrans-anchor-lab anchor-lab)" \
    '{"Name":"bintrans-anchor-lab-internal","Labels":"com.docker.compose.project=bintrans-anchor-lab"}' \
    '{"Name":7,"Labels":{"com.docker.compose.project":"bintrans-anchor-lab","com.docker.compose.network":"anchor-lab"}}' \
    '{"Name":"bintrans-anchor-lab-internal","Labels":{"com.docker.compose.project":"bintrans-e2e-stand-v01a","com.docker.compose.project":"bintrans-anchor-lab","com.docker.compose.network":"anchor-lab"}}'; do
    run_down "$work" MOCK_NETWORK_STATE="$payload" MOCK_VOLUME_STATE="$(owned_volume_record)"
    expect_down_refused "$work" "unrecognized metadata" || return 1
  done
  rm -rf "$work"
}

test_extra_fields_keep_required_values() {
  work=$(new_workspace)
  : > "$work/docker.log"
  prepare_owned_env "$work"
  net=$(owned_network_record | jq -c '. + {Project: "bintrans-e2e-stand-v01a", Scope: "local"} | .Labels += {"com.docker.compose.version": "5.7.0", "com.docker.compose.project.extra": "bintrans-e2e-stand-v01a"}')
  run_down "$work" MOCK_NETWORK_STATE="$net" MOCK_VOLUME_STATE="$(owned_volume_record)"
  [ "$status" -eq 0 ]
  printf '%s\n' "$out" | grep -q "NETWORK_OWNERSHIP=OWNED"
  grep -q " down -v" "$work/docker.log"
  if grep -E 'remove-orphans|prune|bintrans-e2e-stand' "$work/docker.log" >/dev/null; then
    return 1
  fi
  rm -rf "$work"
}

test_similar_name_foreign_resource() {
  work=$(new_workspace)
  : > "$work/docker.log"
  prepare_owned_env "$work"
  run_down "$work" \
    MOCK_NETWORK_STATE="$(resource_json bintrans-anchor-lab-internal-e2e com.docker.compose.project=bintrans-anchor-lab com.docker.compose.network=anchor-lab)" \
    MOCK_VOLUME_STATE="$(owned_volume_record)"
  expect_down_refused "$work" "not owned"
  add_container "$work" b2 "$(container_json bintrans-anchor-lab-postgres-old com.docker.compose.project=bintrans-e2e-stand-v01a com.docker.compose.service=postgres)"
  run_down "$work" MOCK_NETWORK_STATE="$(owned_network_record)" MOCK_VOLUME_STATE="$(owned_volume_record)"
  expect_down_refused "$work" "not owned"
  rm -rf "$work"
}

test_inspect_errors_fail_closed() {
  work=$(new_workspace)
  : > "$work/docker.log"
  prepare_owned_env "$work"
  run_down "$work" MOCK_NETWORK_STATE="$(owned_network_record)" MOCK_VOLUME_STATE=error
  expect_down_refused "$work" "volume inspection failed"
  add_container "$work" c3 "$(container_json bintrans-anchor-lab-postgres-1 com.docker.compose.project=bintrans-anchor-lab)"
  run_down "$work" MOCK_NETWORK_STATE="$(owned_network_record)" MOCK_VOLUME_STATE="$(owned_volume_record)" MOCK_CONTAINER_INSPECT_STATE=error
  expect_down_refused "$work" "container inspection failed"
  run_down "$work" MOCK_NETWORK_STATE="$(owned_network_record)" MOCK_VOLUME_STATE="$(owned_volume_record)" MOCK_PS_STATUS=1
  expect_down_refused "$work" "container list failed"
  if printf '%s\n' "$out" | grep -q "NOTHING_TO_REMOVE"; then
    return 1
  fi
  rm -rf "$work"
}

test_standalone_safe_env() {
  work=$(new_workspace)
  : > "$work/docker.log"
  write_env "$work/lab/.env" "static-secret-value"
  [ ! -e "$work/lab/.git" ]
  run_env_guard "$work"
  [ "$status" -eq 0 ]
  printf '%s\n' "$out" | grep -q "ENV_FILE_GUARD_PLACEMENT=STANDALONE"
  printf '%s\n' "$out" | grep -q "ENV_FILE_GUARD=PASS"
  if printf '%s\n' "$out" | grep -q "static-secret-value"; then
    return 1
  fi
  [ ! -s "$work/docker.log" ]
  rm -rf "$work"
}

test_standalone_owned_cleanup() {
  work=$(new_workspace)
  : > "$work/docker.log"
  write_env "$work/lab/.env" "static-secret-value"
  run_down "$work" MOCK_NETWORK_STATE="$(owned_network_record)" MOCK_VOLUME_STATE="$(owned_volume_record)"
  [ "$status" -eq 0 ]
  printf '%s\n' "$out" | grep -q "ENV_FILE_GUARD_PLACEMENT=STANDALONE"
  grep -q -- "-p bintrans-anchor-lab" "$work/docker.log"
  grep -q " down -v" "$work/docker.log"
  if grep -E 'remove-orphans|prune|bintrans-e2e-stand' "$work/docker.log" >/dev/null; then
    return 1
  fi
  rm -rf "$work"
}

test_standalone_unsafe_env() {
  work=$(new_workspace)
  : > "$work/docker.log"
  write_env "$work/lab/.env" "change-me"
  run_env_guard "$work"
  expect_env_refused "$work" "placeholder password is forbidden"
  rm -rf "$work"
}

test_env_symlink_rejected() {
  work=$(new_workspace)
  : > "$work/docker.log"
  write_env "$work/real.env" "static-secret-value"
  MSYS=winsymlinks:nativestrict ln -s "$work/real.env" "$work/lab/.env"
  [ -L "$work/lab/.env" ]
  run_env_guard "$work"
  expect_env_refused "$work" "symlink"
  rm -rf "$work"
}

test_env_untrusted_owner() {
  work=$(new_workspace)
  : > "$work/docker.log"
  git -C "$work/lab" init -q
  write_env "$work/lab/.env" "static-secret-value"
  run_env_guard "$work" MOCK_STAT_UID=4242
  expect_env_refused "$work" "owner is not the trusted lab user"
  rm -rf "$work"
}

test_env_wrong_mode_standalone() {
  work=$(new_workspace)
  : > "$work/docker.log"
  write_env "$work/lab/.env" "static-secret-value"
  run_env_guard "$work" MOCK_STAT_MODE=640
  expect_env_refused "$work" "mode is not 0600"
  rm -rf "$work"
}

test_parser_unavailable_fails_closed() {
  work=$(new_workspace)
  : > "$work/docker.log"
  prepare_owned_env "$work"
  printf '%s\n' '#!/usr/bin/env bash' 'exit 1' > "$work/bin/jq"
  chmod +x "$work/bin/jq"
  run_down "$work" MOCK_NETWORK_STATE="$(owned_network_record)" MOCK_VOLUME_STATE="$(owned_volume_record)"
  expect_down_refused "$work" "parser is unavailable"
  if grep -E 'network inspect|volume inspect|container inspect| ps ' "$work/docker.log" >/dev/null; then
    return 1
  fi
  : > "$work/docker.log"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$work/docker.log" \
    bash "$work/lab/resource-ownership-preflight.sh" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ]
  printf '%s\n' "$out" | grep -q "parser is unavailable"
  [ ! -s "$work/docker.log" ]
  rm -rf "$work"
}

test_scripts_do_not_prune() {
  if grep -E '^[^#]*docker .*(system prune|volume prune|network prune|remove-orphans)' \
    "$lab/project-guard.sh" "$lab/env-file-guard.sh" "$lab/resource-ownership-preflight.sh" \
    "$lab/lab-down.sh" "$lab/lab-up.sh" "$lab/lab-common.sh" >/dev/null; then
    return 1
  fi
  return 0
}

say "STATIC_TEST_DOCKER=MOCK"
run_one "wrong project selector" test_wrong_project_selector
run_one "wrong COMPOSE_PROJECT_NAME" test_wrong_compose_project_name
run_one "wrong compose path" test_wrong_compose_path
run_one "wrong compose name" test_wrong_name_key
run_one "conflicting project sources" test_conflicting_project_sources
run_one "remote endpoint before docker" test_remote_endpoint_guard
run_one "daemon unavailable before config" test_daemon_unavailable_guard
run_one "rendered secret is not printed" test_guard_hides_rendered_secret
run_one "placeholder password" test_placeholder_password
run_one "empty password" test_empty_password
run_one "tracked env file" test_tracked_env_rejected
run_one "mode other than 0600" test_mode_rejected_by_real_stat
run_one "accepted env hides password" test_env_guard_hides_password
run_one "foreign network" test_foreign_network
run_one "foreign volume" test_foreign_volume
run_one "remote preflight" test_remote_preflight
run_one "daemon unavailable preflight" test_daemon_unavailable_preflight
run_one "cleanup foreign resources" test_cleanup_foreign_resources
run_one "cleanup contradictory labels" test_cleanup_contradictory_labels
run_one "cleanup wrong project" test_cleanup_wrong_project_before_docker
run_one "mocked owned cleanup" test_cleanup_owned_is_explicit_and_mocked
run_one "cleanup rejects orphan override" test_down_rejects_override_flags
run_one "scripts do not prune" test_scripts_do_not_prune
run_one "owned network and volume recognized" test_owned_network_and_volume_recognized
run_one "owned container recognized" test_owned_container_recognized
run_one "cleanup absent does nothing" test_cleanup_absent_does_nothing
run_one "wrong network label" test_wrong_network_label
run_one "wrong volume label" test_wrong_volume_label
run_one "missing labels" test_missing_labels
run_one "inspect error is not absence" test_inspect_error_is_not_absence
run_one "foreign container under lab name" test_foreign_container_under_lab_name
run_one "cleanup remote endpoint" test_cleanup_remote_endpoint
run_one "partial unknown owner" test_partial_unknown_owner
run_one "first start rejects labeled resources" test_first_start_rejects_labeled_resources
run_one "empty project label does not shift" test_empty_project_label_shift
run_one "empty network label" test_empty_network_label
run_one "empty volume label" test_empty_volume_label
run_one "container missing service label" test_container_missing_service_label
run_one "container missing config-files label" test_container_missing_config_label
run_one "shifted metadata fields" test_shifted_metadata_fields
run_one "malformed metadata" test_malformed_metadata
run_one "extra fields keep required values" test_extra_fields_keep_required_values
run_one "similar-name foreign resource" test_similar_name_foreign_resource
run_one "inspect errors fail closed" test_inspect_errors_fail_closed
run_one "standalone safe env" test_standalone_safe_env
run_one "standalone owned cleanup" test_standalone_owned_cleanup
run_one "standalone unsafe env" test_standalone_unsafe_env
run_one "env symlink rejected" test_env_symlink_rejected
run_one "env untrusted owner" test_env_untrusted_owner
run_one "env wrong mode standalone" test_env_wrong_mode_standalone
run_one "parser unavailable fails closed" test_parser_unavailable_fails_closed

say "STATIC_TESTS_PASSED=$pass"
say "STATIC_TESTS_FAILED=$fail_count"
[ "$fail_count" -eq 0 ]
