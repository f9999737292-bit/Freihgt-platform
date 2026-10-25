#!/usr/bin/env bash
# Fail closed unless every Compose project selector is bintrans-e2e-stand-v01a.
# This script does not invoke Docker.
set -euo pipefail

expected="bintrans-e2e-stand-v01a"
root="$(cd "$(dirname "$0")/../../.." && pwd)"
file="$root/infrastructure/docker-compose/docker-compose.bintrans-e2e-stand-v0.1a.yml"

if [ ! -f "$file" ]; then
  echo "stand compose file missing" >&2
  exit 1
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
done < "$file"

explicit=""
while [ $# -gt 0 ]; do
  case "$1" in
    --project)
      shift
      explicit=${1:-}
      if [ -z "$explicit" ]; then
        echo "compose project name is required" >&2
        exit 1
      fi
      shift
      ;;
    *)
      echo "unknown project guard argument" >&2
      exit 1
      ;;
  esac
done

if [ "$file_project" != "$expected" ]; then
  echo "compose file project is not the isolated stand" >&2
  exit 1
fi

env_project=${COMPOSE_PROJECT_NAME:-}
if [ -n "$env_project" ] && [ "$env_project" != "$expected" ]; then
  echo "COMPOSE_PROJECT_NAME is not the isolated stand" >&2
  exit 1
fi

if [ -n "$explicit" ] && [ "$explicit" != "$expected" ]; then
  echo "compose project flag is not the isolated stand" >&2
  exit 1
fi

resolved=$file_project
if [ -n "$env_project" ]; then
  resolved=$env_project
fi
if [ -n "$explicit" ]; then
  resolved=$explicit
fi

if [ "$resolved" != "$expected" ]; then
  echo "selected compose project is not the isolated stand" >&2
  exit 1
fi
