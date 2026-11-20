#!/usr/bin/env bash
# Check the future lab .env without printing its values.
# The file must be the regular .env beside this script, not a symlink,
# mode 0600, owned by the account running the wrapper, and untracked when
# the directory is a git work tree.
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
if [ -L "$env_file" ]; then
  fail "env file is a symlink"
fi
if [ ! -f "$env_file" ] || [ ! -r "$env_file" ]; then
  fail "env file is missing or unreadable"
fi
requested="$(cd "$(dirname "$env_file")" && pwd)/$(basename "$env_file")"
if [ "$requested" != "$expected_env" ]; then
  fail "env file must be the lab .env"
fi
if [ -L "$expected_env" ] || [ ! -f "$expected_env" ]; then
  fail "env file is not a regular file"
fi

# The trusted owner is the account running the wrapper. A git work tree is
# not evidence that the directory is trusted.
mode_owner=""
if mode_owner=$(stat -c '%a %u' "$expected_env" 2>/dev/null); then
  :
elif mode_owner=$(stat -f '%OLp %u' "$expected_env" 2>/dev/null); then
  :
else
  fail "env file mode is unreadable"
fi
mode_owner=${mode_owner%$'\r'}
mode=${mode_owner%% *}
owner=${mode_owner#* }
case "$mode" in
  600|0600) ;;
  *) fail "env file mode is not 0600" ;;
esac
trusted_uid=$(id -u) || fail "current user id is unreadable"
if [ -z "$owner" ] || [ "$owner" = "$mode_owner" ] || [ "$owner" != "$trusted_uid" ]; then
  fail "env file owner is not the trusted lab user"
fi

# Standalone placement, such as /opt/bintrans/anchor-lab without .git, is
# allowed. Inside a git work tree the .env must stay untracked. Any other
# git failure, or a .git without a git binary, stops the guard.
has_dot_git=0
probe_dir=$script_dir
while :; do
  if [ -e "$probe_dir/.git" ]; then
    has_dot_git=1
    break
  fi
  parent=$(dirname "$probe_dir")
  [ "$parent" != "$probe_dir" ] || break
  probe_dir=$parent
done

placement=""
if command -v git >/dev/null 2>&1; then
  git_err=$(mktemp)
  if inside=$(git -C "$script_dir" rev-parse --is-inside-work-tree 2>"$git_err"); then
    rm -f "$git_err"
    [ "$inside" = "true" ] || fail "lab directory git state is unrecognized"
    placement="GIT_WORK_TREE"
  else
    err=$(cat "$git_err" 2>/dev/null || true)
    rm -f "$git_err"
    case "$err" in
      *"not a git repository"*)
        [ "$has_dot_git" -eq 0 ] || fail "lab directory git state is unrecognized"
        placement="STANDALONE"
        ;;
      *)
        fail "lab directory git state is unrecognized"
        ;;
    esac
  fi
else
  [ "$has_dot_git" -eq 0 ] || fail "lab directory has .git but git is unavailable"
  placement="STANDALONE"
fi

if [ "$placement" = "GIT_WORK_TREE" ]; then
  tracked=$(git -C "$script_dir" ls-files -- ".env") || fail "git tracked status is unreadable"
  if [ -n "$tracked" ]; then
    fail "lab .env is tracked by git"
  fi
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

echo "ENV_FILE_GUARD_PLACEMENT=$placement"
echo "ENV_FILE_GUARD=PASS"
