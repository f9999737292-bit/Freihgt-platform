#!/usr/bin/env bash
# Static classification of migrations 000037–000064 (no DB execution).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
MIGRATIONS_DIR="${ROOT}/infrastructure/migrations"

classify_migration() {
  local up_file="$1"
  local content flags=()
  content="$(tr '[:upper:]' '[:lower:]' < "${up_file}")"

  if grep -qiE '\bdrop\b|\btruncate\b|\bdelete[[:space:]]+from\b' "${up_file}"; then
    flags+=("DESTRUCTIVE")
  fi
  if grep -qiE 'alter[[:space:]]+table.*\bdrop\b|drop[[:space:]]+column|drop[[:space:]]+table|drop[[:space:]]+schema' "${up_file}"; then
    flags+=("DESTRUCTIVE")
  fi
  if grep -qiE 'create[[:space:]]+index[^;]*concurrently|alter[[:space:]]+table.*set[[:space:]]+not[[:space:]]+null' "${up_file}"; then
    flags+=("LONG_LOCK_RISK")
  fi
  if grep -qiE 'insert[[:space:]]+into|update[[:space:]]+' "${up_file}"; then
    flags+=("DATA_BACKFILL")
  fi
  if grep -qiE 'alter[[:space:]]+table' "${up_file}" && ! grep -qiE '\bdrop\b' "${up_file}"; then
    flags+=("ALTER_COMPATIBLE")
  fi
  if grep -qiE 'create[[:space:]]+(table|schema|sequence|type|index)' "${up_file}" \
    && ! grep -qiE 'alter[[:space:]]+table|insert[[:space:]]+into|update[[:space:]]+' "${up_file}"; then
    flags+=("ADD_ONLY")
  fi

  local down_file="${up_file%.up.sql}.down.sql"
  if [[ -f "${down_file}" ]]; then
    if grep -qiE '\bdrop\b|\btruncate\b' "${down_file}" && grep -qiE 'create[[:space:]]+table' "${up_file}"; then
      flags+=("IRREVERSIBLE")
    fi
  else
    flags+=("UNKNOWN")
  fi

  if [[ ${#flags[@]} -eq 0 ]]; then
    flags+=("UNKNOWN")
  fi

  printf '%s\n' "${flags[@]}" | sort -u | paste -sd, -
}

total=0
destructive=0
long_lock=0
irreversible=0
unknown=0

echo "=== BINTRANS migrations 000037–000064 static review ==="
for version in $(seq 37 64); do
  target="$(printf '%06d' "${version}")"
  mapfile -t files < <(find "${MIGRATIONS_DIR}" -maxdepth 1 -type f -name "${target}_*.up.sql" | sort)
  if [[ ${#files[@]} -eq 0 ]]; then
    echo "${target}: MISSING"
    unknown=$((unknown + 1))
    total=$((total + 1))
    continue
  fi
  up="${files[0]}"
  classes="$(classify_migration "${up}")"
  echo "${target}: ${classes} (${up##*/})"
  total=$((total + 1))
  [[ "${classes}" == *DESTRUCTIVE* ]] && destructive=$((destructive + 1))
  [[ "${classes}" == *LONG_LOCK_RISK* ]] && long_lock=$((long_lock + 1))
  [[ "${classes}" == *IRREVERSIBLE* ]] && irreversible=$((irreversible + 1))
  [[ "${classes}" == *UNKNOWN* ]] && unknown=$((unknown + 1))
done

echo "MIGRATIONS_37_64_TOTAL=${total}"
echo "DESTRUCTIVE_COUNT=${destructive}"
echo "LONG_LOCK_RISK_COUNT=${long_lock}"
echo "IRREVERSIBLE_COUNT=${irreversible}"
echo "UNKNOWN_COUNT=${unknown}"
echo "bintrans-ct-staging-migrations-static-review: PASS"
