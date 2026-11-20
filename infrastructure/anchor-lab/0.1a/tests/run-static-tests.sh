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
  mkdir -p "$work/bin" "$work/lab"
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
  network)
    [ "${2:-}" = "inspect" ]
    if [ "${MOCK_NETWORK_STATE:-absent}" = "absent" ]; then
      echo "Error: No such network" >&2
      exit 1
    fi
    printf '%s\n' "$MOCK_NETWORK_STATE"
    ;;
  volume)
    [ "${2:-}" = "inspect" ]
    if [ "${MOCK_VOLUME_STATE:-absent}" = "absent" ]; then
      echo "Error: No such volume" >&2
      exit 1
    fi
    printf '%s\n' "$MOCK_VOLUME_STATE"
    ;;
  ps)
    printf '%s\n' "${MOCK_PS:-}"
    exit "${MOCK_PS_STATUS:-0}"
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
if [ "${1:-}" = "-c" ] && [ "${2:-}" = "%a" ]; then
  printf '%s\n' "${MOCK_STAT_MODE:-600}"
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
  ( "$@" )
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

test_foreign_network() {
  work=$(new_workspace)
  log="$work/docker.log"
  : > "$log"
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" \
    MOCK_NETWORK_STATE="bintrans-e2e-stand-v01a|/opt/bintrans/e2e-stand/compose.yaml" \
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
    MOCK_VOLUME_STATE="bintrans-e2e-stand-v01a|bintrans-e2e-stand-postgres" \
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
    MOCK_NETWORK_STATE="bintrans-e2e-stand-v01a|/opt/bintrans/e2e-stand/compose.yaml" \
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
  set +e
  out=$(PATH="$work/bin:$PATH" ANCHOR_LAB_MOCK_LOG="$log" MOCK_STAT_MODE=600 \
    MOCK_NETWORK_STATE="bintrans-anchor-lab|$compose" \
    MOCK_VOLUME_STATE="bintrans-anchor-lab|$compose" \
    MOCK_PS="bintrans-anchor-lab|/opt/bintrans/e2e-stand/compose.yaml" \
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
    MOCK_NETWORK_STATE="bintrans-anchor-lab|$compose" \
    MOCK_VOLUME_STATE="bintrans-anchor-lab|$compose" \
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

say "STATIC_TESTS_PASSED=$pass"
say "STATIC_TESTS_FAILED=$fail_count"
[ "$fail_count" -eq 0 ]
