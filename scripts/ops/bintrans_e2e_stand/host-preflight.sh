#!/usr/bin/env bash
# Read-only preflight for a dedicated BINTRANS E2E Linux host.
# Exit 0 PASS, 1 FAIL, 2 BLOCKED. This script does not start services,
# pull images, create containers, or delete resources.
set -euo pipefail

expected_project="bintrans-e2e-stand-v01a"
min_vcpu=4
# Controller thresholds. Decimal GB for total RAM, GiB for available RAM and Docker disk.
min_total_ram_bytes=16000000000
min_available_ram_bytes=6442450944
min_disk_bytes=53687091200
root="$(cd "$(dirname "$0")/../../.." && pwd)"
compose="$root/infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml"

fail() {
  echo "PREFLIGHT_RESULT=FAIL" >&2
  echo "PREFLIGHT_FAIL: $*" >&2
  exit 1
}

block() {
  echo "PREFLIGHT_RESULT=BLOCKED" >&2
  echo "PREFLIGHT_BLOCKED: $*" >&2
  exit 2
}

require_uint() {
  case "$2" in
    ""|*[!0-9]*) fail "$1 is not a decimal integer" ;;
  esac
}

# Run a Docker metadata or daemon command and keep its output only after a zero status.
# A failing command substitution must not be hidden inside a heredoc.
capture_lines() {
  dest=$1
  label=$2
  shift 2
  output=""
  if ! output=$("$@"); then
    fail "$label failed"
  fi
  printf -v "$dest" '%s' "$output"
}

reject_remote_endpoint() {
  label=$1
  value=${2%$'\r'}
  case "$value" in
    unix:///*) ;;
    tcp://*|ssh://*) fail "$label is remote" ;;
    *) fail "$label is not a local unix socket" ;;
  esac
}

echo "PREFLIGHT_MODE=READ_ONLY"
echo "EXPECTED_PROJECT=$expected_project"
echo "MIN_VCPU=$min_vcpu"
echo "MIN_TOTAL_RAM_BYTES=$min_total_ram_bytes"
echo "MIN_AVAILABLE_RAM_BYTES=$min_available_ram_bytes"
echo "MIN_DOCKER_DISK_FREE_BYTES=$min_disk_bytes"
echo "RECOMMENDED_DISK_GB=100"

kernel=$(uname -s)
kernel=${kernel%$'\r'}
echo "KERNEL=$kernel"
if [ "$kernel" != "Linux" ]; then
  fail "kernel $kernel is not Linux"
fi
if [ -r /etc/os-release ]; then
  os_id=$(awk -F= '$1=="ID" {gsub(/"/, "", $2); print $2}' /etc/os-release)
  os_version=$(awk -F= '$1=="VERSION_ID" {gsub(/"/, "", $2); print $2}' /etc/os-release)
  echo "OS_ID=${os_id:-unknown}"
  echo "OS_VERSION_ID=${os_version:-unknown}"
else
  echo "OS_ID=unknown"
  echo "OS_VERSION_ID=unknown"
fi

arch=$(uname -m)
arch=${arch%$'\r'}
echo "ARCH=$arch"
if [ "$arch" != "x86_64" ]; then
  fail "architecture $arch is not x86_64"
fi

cpus=$(nproc)
cpus=${cpus%$'\r'}
echo "VCPU=$cpus"
require_uint "vCPU count" "$cpus"
if [ "$cpus" -lt "$min_vcpu" ]; then
  fail "vCPU $cpus is below $min_vcpu"
fi

ram_total=$(free -b | awk '/^Mem:/ {print $2}')
ram_avail=$(free -b | awk '/^Mem:/ {print $7}')
ram_total=${ram_total%$'\r'}
ram_avail=${ram_avail%$'\r'}
require_uint "total RAM" "$ram_total"
require_uint "available RAM" "$ram_avail"
echo "RAM_TOTAL_BYTES=$ram_total"
echo "RAM_AVAILABLE_BYTES=$ram_avail"
if [ "$ram_total" -lt "$min_total_ram_bytes" ]; then
  fail "total RAM is below ${min_total_ram_bytes} bytes"
fi
if [ "$ram_avail" -lt "$min_available_ram_bytes" ]; then
  fail "available RAM is below ${min_available_ram_bytes} bytes"
fi
echo "RAM_THRESHOLDS=PASS"

for cmd in git openssl awk; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    fail "required command $cmd is missing"
  fi
done
echo "HOST_TOOLS=PRESENT"

# Docker CLI v28 uses a set DOCKER_HOST to force the default context, which
# hides DOCKER_CONTEXT. This preflight does the opposite selection: a set
# DOCKER_CONTEXT names the endpoint, and DOCKER_HOST must be empty or the same
# unix socket. Remote tcp:// and ssh:// values fail before any daemon call.
# Later commands pass --host with that socket so they reach the same daemon.
host_env=${DOCKER_HOST-}
context_env=${DOCKER_CONTEXT-}
if [ -n "$host_env" ]; then
  reject_remote_endpoint "DOCKER_HOST" "$host_env"
fi

context_name=""
if [ -n "$context_env" ]; then
  context_name=$context_env
else
  capture_lines context_name "docker context show" docker context show
fi
context_name=$(printf '%s' "$context_name" | tr -d '[:space:]')
if [ -z "$context_name" ]; then
  fail "docker context name is empty"
fi

context_endpoint=""
capture_lines context_endpoint "docker context inspect" docker context inspect "$context_name" --format '{{.Endpoints.docker.Host}}'
context_endpoint=$(printf '%s' "$context_endpoint" | tr -d '[:space:]')
reject_remote_endpoint "docker context endpoint" "$context_endpoint"
if [ -n "$host_env" ] && [ "$host_env" != "$context_endpoint" ]; then
  fail "DOCKER_HOST contradicts the selected docker context endpoint"
fi

effective=$context_endpoint
sock=${effective#unix://}
if ! env test -S "$sock"; then
  block "docker unix socket is not available"
fi
echo "DOCKER_CONTEXT_NAME=$context_name"
echo "DOCKER_ENDPOINT=$effective"
scheme=${effective%%://*}
echo "DOCKER_ENDPOINT_SCHEME=$scheme"

if ! docker --host "$effective" info >/dev/null 2>&1; then
  block "docker daemon is not available"
fi
echo "DOCKER_DAEMON=AVAILABLE"

engine=""
capture_lines engine "docker version" docker --host "$effective" version --format '{{.Server.Version}}'
engine=$(printf '%s' "$engine" | tr -d '[:space:]')
if [ -z "$engine" ]; then
  fail "docker engine version is empty"
fi
echo "DOCKER_ENGINE_VERSION=$engine"

compose_version=""
capture_lines compose_version "docker compose version" docker --host "$effective" compose version
compose_version=${compose_version%$'\r'}
if [ -z "$compose_version" ]; then
  fail "docker compose plugin is not available"
fi
echo "DOCKER_COMPOSE_VERSION=$compose_version"
echo "DOCKER_COMPOSE_PLUGIN=PRESENT"

docker_root=""
capture_lines docker_root "docker info" docker --host "$effective" info --format '{{.DockerRootDir}}'
docker_root=$(printf '%s' "$docker_root" | tr -d '[:space:]')
if [ -z "$docker_root" ]; then
  fail "docker data root is empty"
fi
echo "DOCKER_DATA_ROOT=$docker_root"
disk_free=$(df -B1 -P "$docker_root" | awk 'NR==2 {print $4}')
disk_free=${disk_free%$'\r'}
require_uint "docker filesystem free bytes" "$disk_free"
echo "DOCKER_DISK_FREE_BYTES=$disk_free"
if [ "$disk_free" -lt "$min_disk_bytes" ]; then
  fail "docker filesystem free space is below ${min_disk_bytes} bytes"
fi

compose_projects=""
capture_lines compose_projects "docker compose ls" docker --host "$effective" compose ls -a -q
while IFS= read -r name || [ -n "$name" ]; do
  name=${name%$'\r'}
  [ -n "$name" ] || continue
  case "$name" in
    "$expected_project"|bintrans-e2e-stand-v0.1a|bintrans-ct-staging)
      fail "compose project $name conflicts with the isolated stand"
      ;;
  esac
done <<EOF
$compose_projects
EOF
echo "COMPOSE_PROJECT_CONFLICT=NONE"

networks=""
capture_lines networks "docker network ls" docker --host "$effective" network ls --format '{{.Name}}'
while IFS= read -r name || [ -n "$name" ]; do
  name=${name%$'\r'}
  [ -n "$name" ] || continue
  case "$name" in
    freight-platform-network)
      fail "docker network freight-platform-network is present"
      ;;
  esac
done <<EOF
$networks
EOF

volumes=""
capture_lines volumes "docker volume ls" docker --host "$effective" volume ls --format '{{.Name}}'
while IFS= read -r name || [ -n "$name" ]; do
  name=${name%$'\r'}
  [ -n "$name" ] || continue
  case "$name" in
    *freight_postgres_data*)
      fail "docker volume $name is a shared postgres volume"
      ;;
  esac
done <<EOF
$volumes
EOF
echo "SHARED_DOCKER_RESOURCES=ABSENT"

ps_rows=""
capture_lines ps_rows "docker ps" docker --host "$effective" ps -a --format '{{.Label "com.docker.compose.project"}}|{{.Ports}}'
while IFS= read -r row || [ -n "$row" ]; do
  row=${row%$'\r'}
  [ -n "$row" ] || continue
  project=${row%%|*}
  ports=${row#*|}
  if [ "$project" = "$expected_project" ] && [ -n "$ports" ]; then
    fail "stand project publishes host ports"
  fi
done <<EOF
$ps_rows
EOF

if [ ! -f "$compose" ]; then
  fail "stand compose file is missing"
fi
file_project=""
while IFS= read -r line || [ -n "$line" ]; do
  line=${line%$'\r'}
  case "$line" in
    name:*)
      file_project=${line#name:}
      file_project=$(printf '%s' "$file_project" | tr -d '[:space:]"'"'"'')
      break
      ;;
  esac
done < "$compose"
echo "COMPOSE_FILE_PROJECT=$file_project"
if [ "$file_project" != "$expected_project" ]; then
  fail "compose file project is not $expected_project"
fi
if grep -E '^[[:space:]]*ports:' "$compose" >/dev/null 2>&1; then
  fail "stand compose file contains a host port binding"
fi
echo "COMPOSE_HOST_PORTS=ABSENT"

if ! bash "$root/scripts/ops/bintrans_e2e_stand/project-guard.sh" --project "$expected_project"; then
  fail "project guard rejected the canonical project"
fi
echo "PROJECT_GUARD=PASS"
echo "PREFLIGHT_RESULT=PASS"
