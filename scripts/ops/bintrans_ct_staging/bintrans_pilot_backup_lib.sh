#!/usr/bin/env bash
# Shared validated PostgreSQL backup helpers for BINTRANS staging (no secret echo).
set -euo pipefail

_bintrans_pilot_backup_lib_loaded=1

bintrans_pilot_backup_dir() {
  echo "${BINTRANS_BACKUP_DIR:-/protected/bintrans/backups}"
}

bintrans_pilot_backup_metadata_file() {
  echo "$(bintrans_pilot_backup_dir)/last_validated_backup.json"
}

bintrans_pilot_backup_run_state_file() {
  echo "$(bintrans_pilot_backup_dir)/last_backup_run.json"
}

bintrans_pilot_backup_lock_file() {
  echo "${BINTRANS_BACKUP_LOCK:-/run/lock/bintrans-pilot-backup.lock}"
}

bintrans_pilot_backup_validate_dump() {
  local dump_file="$1"
  [[ -s "${dump_file}" ]] || bintrans_fail "backup file is empty"
  if ! head -c 5 "${dump_file}" | grep -q 'PGDMP'; then
    bintrans_fail "backup does not look like pg_dump custom format"
  fi
  docker run --rm -i postgres:16 pg_restore -l < "${dump_file}" >/dev/null \
    || bintrans_fail "pg_restore -l verification failed"
  local toc_lines
  toc_lines="$(docker run --rm -i postgres:16 pg_restore -l < "${dump_file}" 2>/dev/null | grep -cE '; [0-9]+ [0-9]+ TABLE DATA|; [0-9]+ [0-9]+ TABLE ' || true)"
  [[ "${toc_lines}" -gt 0 ]] || bintrans_fail "backup TOC has no application table content"
}

bintrans_pilot_backup_write_metadata_json() {
  local backup_path="$1"
  local checksum="$2"
  local size_bytes="$3"
  local created_utc="$4"
  local validated_utc="$5"
  local meta_file
  meta_file="$(bintrans_pilot_backup_metadata_file)"
  local tmp="${meta_file}.tmp.$$"
  python3 - <<'PY' "${tmp}" "${backup_path}" "${checksum}" "${size_bytes}" "${created_utc}" "${validated_utc}"
import json, os, sys
path, backup_path, checksum, size_bytes, created_utc, validated_utc = sys.argv[1:7]
doc = {
    "BACKUP_PATH": backup_path,
    "BACKUP_SHA256": checksum,
    "BACKUP_SIZE_BYTES": int(size_bytes),
    "BACKUP_CREATED_UTC": created_utc,
    "BACKUP_VALIDATED_UTC": validated_utc,
    "BACKUP_VERIFIED": "YES",
}
with open(path, "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
os.chmod(path, 0o600)
PY
  mv "${tmp}" "${meta_file}"
  chmod 600 "${meta_file}"
}

bintrans_pilot_backup_write_run_state() {
  local success="$1"
  local message="$2"
  local run_utc="$3"
  local state_file
  state_file="$(bintrans_pilot_backup_run_state_file)"
  local tmp="${state_file}.tmp.$$"
  python3 - <<'PY' "${tmp}" "${success}" "${message}" "${run_utc}"
import json, os, sys
path, success, message, run_utc = sys.argv[1:5]
doc = {
    "BACKUP_RUN_UTC": run_utc,
    "BACKUP_RUN_SUCCESS": int(success),
    "BACKUP_RUN_MESSAGE": message[:200],
}
with open(path, "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
os.chmod(path, 0o600)
PY
  mv "${tmp}" "${state_file}"
  chmod 600 "${state_file}"
}

bintrans_pilot_backup_update_staging_env() {
  local backup_path="$1"
  local checksum="$2"
  bintrans_require_env_file
  python3 - <<'PY' "${BINTRANS_STAGING_ENV}" "${backup_path}" "${checksum}"
import os, re, sys
path, backup_path, sha = sys.argv[1:4]
with open(path, encoding="utf-8") as f:
    lines = f.readlines()
updates = {
    "BACKUP_PATH": backup_path,
    "BACKUP_SHA256": sha,
    "BACKUP_VERIFIED": "YES",
}
out = []
seen = set()
for line in lines:
    m = re.match(r"^([A-Z0-9_]+)=", line)
    if m and m.group(1) in updates:
        key = m.group(1)
        if key not in seen:
            out.append(f"{key}={updates[key]}\n")
            seen.add(key)
        continue
    out.append(line)
for key, val in updates.items():
    if key not in seen:
        out.append(f"{key}={val}\n")
tmp = path + ".tmp"
with open(tmp, "w", encoding="utf-8") as f:
    f.writelines(out)
os.replace(tmp, path)
os.chmod(path, 0o600)
PY
}

bintrans_pilot_backup_publish_metrics() {
  local metrics_script="${ROOT}/scripts/ops/bintrans_ct_staging/bintrans_pilot_backup_metrics.sh"
  [[ -x "${metrics_script}" ]] || chmod +x "${metrics_script}"
  bash "${metrics_script}"
}

# Sets BINTRANS_LAST_BACKUP_PATH, BINTRANS_LAST_BACKUP_SHA256, BINTRANS_LAST_BACKUP_SIZE_BYTES.
bintrans_pilot_create_validated_backup() {
  local publish="${1:-no}"
  bintrans_require_env_file

  local postgres_user postgres_db pg_cid backup_dir timestamp partial_file backup_file
  BINTRANS_LAST_BACKUP_PATH=""
  BINTRANS_LAST_BACKUP_SHA256=""
  BINTRANS_LAST_BACKUP_SIZE_BYTES=""
  postgres_user="$(grep -E '^POSTGRES_USER=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2-)"
  postgres_db="$(grep -E '^POSTGRES_DB=' "${BINTRANS_STAGING_ENV}" | tail -n1 | cut -d= -f2-)"
  [[ -n "${postgres_user}" && -n "${postgres_db}" ]] || bintrans_fail "POSTGRES_USER and POSTGRES_DB required"

  backup_dir="$(bintrans_pilot_backup_dir)"
  mkdir -p "${backup_dir}"
  chmod 700 "${backup_dir}"

  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  partial_file="${backup_dir}/freight_platform_${timestamp}.partial"
  backup_file="${backup_dir}/freight_platform_${timestamp}.dump"

  pg_cid="$(bintrans_postgres_container)"
  [[ -n "${pg_cid}" ]] || bintrans_fail "postgres container not running — start foundation first"

  rm -f "${partial_file}"
  docker exec "${pg_cid}" pg_dump \
    -U "${postgres_user}" \
    -d "${postgres_db}" \
    -Fc \
    --no-owner \
    --no-privileges \
    > "${partial_file}"

  bintrans_pilot_backup_validate_dump "${partial_file}"

  chmod 600 "${partial_file}"
  mv "${partial_file}" "${backup_file}"

  backup_sha256="$(sha256sum "${backup_file}" | awk '{print $1}')"
  backup_size_bytes="$(stat -c '%s' "${backup_file}" 2>/dev/null || stat -f '%z' "${backup_file}")"
  BINTRANS_LAST_BACKUP_PATH="${backup_file}"
  BINTRANS_LAST_BACKUP_SHA256="${backup_sha256}"
  BINTRANS_LAST_BACKUP_SIZE_BYTES="${backup_size_bytes}"
  local created_utc validated_utc
  created_utc="${timestamp}"
  validated_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

  if [[ "${publish}" == "yes" ]]; then
    bintrans_pilot_backup_write_metadata_json \
      "${backup_file}" "${backup_sha256}" "${backup_size_bytes}" \
      "${created_utc}" "${validated_utc}"
    bintrans_pilot_backup_update_staging_env "${backup_file}" "${backup_sha256}"
    bintrans_pilot_backup_publish_metrics
  fi
}
