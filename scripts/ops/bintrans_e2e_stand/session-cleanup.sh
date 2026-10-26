#!/usr/bin/env bash
# EXIT-trap helper for one dedicated-host attempt.
# When resources were started, down.sh runs before the env file is removed.
# A failed down.sh prints CLEANUP_FAILED, keeps the env file, and leaves every other Docker object untouched.
# The env file is removed only when its device and inode still match the file this session created.
set -euo pipefail

env_file=""
started="no"
env_dev=""
env_ino=""
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
    --env-dev)
      shift
      env_dev=${1:-}
      if [ -z "$env_dev" ]; then
        echo "env device is required" >&2
        exit 1
      fi
      shift
      ;;
    --env-ino)
      shift
      env_ino=${1:-}
      if [ -z "$env_ino" ]; then
        echo "env inode is required" >&2
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
ops="$(cd "$(dirname "$0")" && pwd)"
# shellcheck disable=SC1091
. "$ops/session-common.sh"

cleanup_failed() {
  echo "CLEANUP_FAILED" >&2
  exit 1
}

if [ "$started" = "yes" ]; then
  if [ -z "$env_dev" ] || [ -z "$env_ino" ]; then
    echo "ephemeral env identity is missing" >&2
    cleanup_failed
  fi
  python=$(stand_python)
  native_env=$(to_native_path "$env_file")
  if ! "$python" "$ops/private-env.py" verify "$native_env" "$env_dev" "$env_ino"; then
    cleanup_failed
  fi
  if ! bash "$root/scripts/ops/bintrans_e2e_stand/down.sh" --env-file "$env_file"; then
    cleanup_failed
  fi
fi

if [ -z "$env_dev" ] || [ -z "$env_ino" ]; then
  if [ "$started" = "yes" ]; then
    cleanup_failed
  fi
  exit 0
fi

python=${python:-$(stand_python)}
native_env=${native_env:-$(to_native_path "$env_file")}
if ! "$python" "$ops/private-env.py" remove "$native_env" "$env_dev" "$env_ino"; then
  cleanup_failed
fi
echo "EPHEMERAL_ENV=REMOVED"
