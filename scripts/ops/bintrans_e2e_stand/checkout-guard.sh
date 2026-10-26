#!/usr/bin/env bash
# Confirm the worktree is the controller-approved SHA and contains host-preflight.
# This script does not fetch, switch revisions, or invoke Docker.
set -euo pipefail

expected=""
while [ $# -gt 0 ]; do
  case "$1" in
    --sha)
      shift
      expected=${1:-}
      if [ -z "$expected" ]; then
        echo "approved sha is required" >&2
        exit 1
      fi
      shift
      ;;
    *)
      echo "unknown checkout guard argument" >&2
      exit 1
      ;;
  esac
done

if [ -z "$expected" ]; then
  echo "approved sha is required" >&2
  exit 1
fi

head=""
if ! head=$(git rev-parse HEAD); then
  echo "git rev-parse failed" >&2
  exit 1
fi
head=$(printf '%s' "$head" | tr -d '[:space:]')
if [ "$head" != "$expected" ]; then
  echo "HEAD is not the approved sha" >&2
  exit 1
fi

status=""
if ! status=$(git status --porcelain); then
  echo "git status failed" >&2
  exit 1
fi
if [ -n "$status" ]; then
  echo "worktree is not clean" >&2
  exit 1
fi

root="$(cd "$(dirname "$0")/../../.." && pwd)"
if [ ! -f "$root/scripts/ops/bintrans_e2e_stand/host-preflight.sh" ]; then
  echo "host-preflight.sh is missing from this checkout" >&2
  exit 1
fi

echo "CHECKOUT_GUARD=PASS"
