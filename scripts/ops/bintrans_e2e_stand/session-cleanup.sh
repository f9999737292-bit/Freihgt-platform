#!/usr/bin/env bash
# EXIT-trap helper for one dedicated-host attempt.
# When resources were started, down.sh runs before the env file is removed.
# A failed down.sh prints CLEANUP_FAILED, keeps the env file, and leaves every other Docker object untouched.
set -euo pipefail

env_file=""
started="no"
while [ $# -gt 0 ]; do
  case "$1" in
    --env-file)
      shift
      env_file=${1:-}
      if [ -z "$env_file" ]; then
        echo "env file is required" >&2
        exit 1
      fi
      shift
      ;;
    --resources-started)
      shift
      started=${1:-}
      if [ -z "$started" ]; then
        echo "resources-started value is required" >&2
        exit 1
      fi
      shift
      ;;
    *)
      echo "unknown session cleanup argument" >&2
      exit 1
      ;;
  esac
done

if [ -z "$env_file" ]; then
  echo "env file is required" >&2
  exit 1
fi

root="$(cd "$(dirname "$0")/../../.." && pwd)"
if [ "$started" = "yes" ]; then
  if [ ! -f "$env_file" ]; then
    echo "CLEANUP_FAILED" >&2
    echo "ephemeral env file is missing" >&2
    exit 1
  fi
  if ! bash "$root/scripts/ops/bintrans_e2e_stand/down.sh" --env-file "$env_file"; then
    echo "CLEANUP_FAILED" >&2
    exit 1
  fi
fi

rm -f "$env_file"
echo "EPHEMERAL_ENV=REMOVED"
