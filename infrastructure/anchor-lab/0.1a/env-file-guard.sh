#!/usr/bin/env bash
# Check the future lab .env without printing its values.
# The file must be the untracked .env beside this script, mode 0600.
# This script does not read staging or production environment files.
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lab-common.sh
. "$script_dir/lab-common.sh"

expected_env="$(anchor_lab_env_file)"

fail() {
  echo "ENV_FILE_GUARD=FAIL $*" >&2
  exit 1
}

env_file=""
while [ $# -gt 0 ]; do
  case "$1" in
    --env-file)
      [ $# -ge 2 ] || fail "env file path is required"
      [ -z "$env_file" ] || fail "env file repeated"
      env_file=$2
      shift 2
      ;;
    *)
      fail "unauthorized argument"
      ;;
  esac
done

[ -n "$env_file" ] || fail "env file is required"
if [ ! -f "$env_file" ] || [ ! -r "$env_file" ]; then
  fail "env file is missing or unreadable"
fi
requested="$(cd "$(dirname "$env_file")" && pwd)/$(basename "$env_file")"
if [ "$requested" != "$expected_env" ]; then
  fail "env file must be the lab .env"
fi

mode=""
if mode=$(stat -c %a "$expected_env" 2>/dev/null); then
  :
elif mode=$(stat -f %OLp "$expected_env" 2>/dev/null); then
  :
else
  fail "env file mode is unreadable"
fi
mode=${mode%$'\r'}
case "$mode" in
  600|0600) ;;
  *) fail "env file mode is not 0600" ;;
esac

if ! git -C "$script_dir" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  fail "lab directory is not in a git work tree"
fi
tracked=$(git -C "$script_dir" ls-files -- ".env")
if [ -n "$tracked" ]; then
  fail "lab .env is tracked by git"
fi

read_value() {
  key=$1
  raw=$(grep -E "^${key}=" "$expected_env" | tail -n 1 | cut -d= -f2- || true)
  raw=${raw%$'\r'}
  raw=${raw#\"}
  raw=${raw%\"}
  raw=${raw#\'}
  raw=${raw%\'}
  printf '%s' "$raw"
}

user_value=$(read_value ANCHOR_LAB_POSTGRES_USER)
db_value=$(read_value ANCHOR_LAB_POSTGRES_DB)
password_value=$(read_value ANCHOR_LAB_POSTGRES_PASSWORD)

[ -n "$user_value" ] || fail "lab database user is empty"
[ -n "$db_value" ] || fail "lab database name is empty"
if [ -z "$password_value" ]; then
  unset password_value
  fail "lab database password is empty"
fi
if [ "$password_value" = "change-me" ]; then
  unset password_value
  fail "placeholder password is forbidden"
fi
unset password_value user_value db_value

echo "ENV_FILE_GUARD=PASS"
