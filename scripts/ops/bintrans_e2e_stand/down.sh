#!/usr/bin/env bash
# Removes only Compose project bintrans-e2e-stand-v0.1a.
# Do not point this script at the platform Compose file or the staging project.
set -euo pipefail

root="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$root"

project="bintrans-e2e-stand-v0.1a"
file="infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml"

if [ ! -f "$file" ]; then
  echo "stand compose file missing" >&2
  exit 1
fi

exec docker compose -p "$project" -f "$file" down -v --remove-orphans
