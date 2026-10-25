#!/usr/bin/env bash
# Shared path and interpreter helpers for one dedicated-host session.
# Path conversion must not resolve symbolic links. cygpath -w does.

to_native_path() {
  path=$1
  if ! command -v cygpath >/dev/null 2>&1; then
    printf '%s\n' "$path"
    return
  fi
  case "$path" in
    /cygdrive/[A-Za-z] | /cygdrive/[A-Za-z]/*)
      drive=${path#/cygdrive/}
      drive=${drive%%/*}
      rest=${path#/cygdrive/?}
      ;;
    /[A-Za-z] | /[A-Za-z]/*)
      drive=$(printf '%.1s' "${path#/}")
      rest=${path#/?}
      ;;
    /tmp | /tmp/*)
      base=$(cygpath -w /tmp)
      rest=${path#/tmp}
      rest=$(printf '%s' "$rest" | sed 's#/#\\#g')
      printf '%s%s\n' "$base" "$rest"
      return
      ;;
    /var/tmp | /var/tmp/*)
      base=$(cygpath -w /var/tmp)
      rest=${path#/var/tmp}
      rest=$(printf '%s' "$rest" | sed 's#/#\\#g')
      printf '%s%s\n' "$base" "$rest"
      return
      ;;
    *)
      printf '%s\n' "$path"
      return
      ;;
  esac
  drive=$(printf '%s' "$drive" | tr '[:lower:]' '[:upper:]')
  rest=$(printf '%s' "$rest" | sed 's#/#\\#g')
  printf '%s:%s\n' "$drive" "$rest"
}

stand_python() {
  bash "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/python-bin.sh"
}
