#!/bin/bash
# Isolated runner. Each @test/@admin block is its own psql process.
# An expected SQLSTATE does not abort the remaining blocks.
set -u
cd "$(dirname "$0")"
export PGUSER="${PGUSER:-proof_admin}"
export PGDATABASE="${PGDATABASE:-binding_privilege_proof_0_1i_r2}"
OUT="${OUT:-runs/attempt-3}"
mkdir -p "$OUT"
PASSES=0
FAILS=0
SUMMARY="$OUT/assertions.tsv"
printf 'test_id\texpected\tactual\tverdict\tstdout\tstderr\n' > "$SUMMARY"

record() {
  local id="$1" expect="$2" actual="$3" verdict="$4" out="$5" err="$6"
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$id" "$expect" "$actual" "$verdict" "$out" "$err" >> "$SUMMARY"
  printf '%s expected=%s actual=%s %s\n' "$id" "$expect" "$actual" "$verdict"
  if [ "$verdict" = "PASS" ]; then
    PASSES=$((PASSES + 1))
  else
    FAILS=$((FAILS + 1))
  fi
}

sqlstate_of() {
  local errf="$1"
  if grep -q '^ERROR:' "$errf"; then
    sed -n 's/^ERROR:  \([0-9A-Z]*\):.*/\1/p' "$errf" | head -n 1
  else
    printf 'OK'
  fi
}

run_sql() {
  local id="$1" role="$2" expect="$3" contains="$4" sql="$5"
  local outf="$OUT/${id}.stdout"
  local errf="$OUT/${id}.stderr"
  local sqlf="$OUT/${id}.sql"
  printf '%s\n' "$sql" > "$sqlf"
  psql -X -U "$role" -d "$PGDATABASE" -v VERBOSITY=verbose -v ON_ERROR_STOP=1 -c "$sql" >"$outf" 2>"$errf" || true
  local actual
  actual=$(sqlstate_of "$errf")
  [ -n "$actual" ] || actual=ERROR
  local verdict=PASS
  if [ "$actual" != "$expect" ]; then
    verdict=FAIL
  fi
  if [ -n "$contains" ] && ! grep -q -F -- "$contains" "$outf"; then
    verdict=FAIL
    actual="${actual};missing:${contains}"
  fi
  record "$id" "$expect" "$actual" "$verdict" "$outf" "$errf"
}

parse_file() {
  local file="$1"
  local mode="" id="" role="" expect="" contains="" sql="" line
  while IFS= read -r line || [ -n "$line" ]; do
    line=${line%$'\r'}
    case "$line" in
      "-- @test "*)
        mode=test
        id=$(printf '%s\n' "$line" | sed -n 's/.*id=\([^ ]*\).*/\1/p')
        role=$(printf '%s\n' "$line" | sed -n 's/.*role=\([^ ]*\).*/\1/p')
        expect=$(printf '%s\n' "$line" | sed -n 's/.*expect=\([^ ]*\).*/\1/p')
        contains=$(printf '%s\n' "$line" | sed -n 's/.*contains=\([^ ]*\).*/\1/p')
        sql=""
        ;;
      "-- @admin "*)
        mode=admin
        id=$(printf '%s\n' "$line" | sed -n 's/.*id=\([^ ]*\).*/\1/p')
        role="$PGUSER"
        expect=$(printf '%s\n' "$line" | sed -n 's/.*expect=\([^ ]*\).*/\1/p')
        [ -n "$expect" ] || expect=OK
        contains=$(printf '%s\n' "$line" | sed -n 's/.*contains=\([^ ]*\).*/\1/p')
        sql=""
        ;;
      "-- @end")
        if [ -n "$mode" ]; then
          run_sql "$id" "$role" "$expect" "$contains" "$sql"
        fi
        mode=""
        sql=""
        ;;
      *)
        if [ -n "$mode" ]; then
          sql="${sql}${line}"$'\n'
        fi
        ;;
    esac
  done < "$file"
}

{
  printf '%s\n' '-- SELECT version();'
  psql -X -U "$PGUSER" -d "$PGDATABASE" -c 'SELECT version();'
  printf '%s\n' '-- SHOW server_version_num;'
  psql -X -U "$PGUSER" -d "$PGDATABASE" -c 'SHOW server_version_num;'
  printf '%s\n' '-- psql --version'
  psql --version
  printf '%s\n' '-- current_database'
  psql -X -U "$PGUSER" -d "$PGDATABASE" -c 'SELECT current_database();'
  printf '%s\n' '-- pg_database'
  psql -X -U "$PGUSER" -d "$PGDATABASE" -c 'SELECT datname FROM pg_database ORDER BY 1;'
} > "$OUT/E01_version.stdout" 2> "$OUT/E01_version.stderr" || true

version_num=$(psql -X -q -tAc 'SHOW server_version_num;' | tr -d '[:space:]')
if [ "$version_num" -ge 160000 ] 2>/dev/null && [ "$version_num" -lt 170000 ]; then
  record E01_server_version_num 16 "$version_num" PASS "$OUT/E01_version.stdout" "$OUT/E01_version.stderr"
else
  record E01_server_version_num 16 "${version_num:-UNKNOWN}" FAIL "$OUT/E01_version.stdout" "$OUT/E01_version.stderr"
fi

psql -X -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -f setup.sql > "$OUT/E00_setup.stdout" 2> "$OUT/E00_setup.stderr"
setup_rc=$?
if [ "$setup_rc" -eq 0 ]; then
  record E00_setup OK OK PASS "$OUT/E00_setup.stdout" "$OUT/E00_setup.stderr"
else
  record E00_setup OK "exit:${setup_rc}" FAIL "$OUT/E00_setup.stdout" "$OUT/E00_setup.stderr"
fi

parse_file privilege_tests.sql
parse_file security_definer_tests.sql
parse_file rollback_tests.sql

# Harness demonstration. This file is not required to equal ISSUED|null.
psql -X -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -tAc "
BEGIN;
UPDATE transport.binding_authority_decisions
   SET consumed_at = clock_timestamp(), use_status = 'CONSUMED'
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
ROLLBACK;
SELECT use_status || '|' || CASE WHEN consumed_at IS NULL THEN 'null' ELSE 'set' END
  FROM transport.binding_authority_decisions
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
" > "$OUT/HARNESS_multistatement_tags.stdout" 2> "$OUT/HARNESS_multistatement_tags.stderr" || true
if grep -q 'ISSUED|null' "$OUT/HARNESS_multistatement_tags.stdout" \
   && grep -q '^BEGIN$' "$OUT/HARNESS_multistatement_tags.stdout" \
   && grep -q '^ROLLBACK$' "$OUT/HARNESS_multistatement_tags.stdout"; then
  record HARNESS_multistatement_tags "tags-plus-ISSUED|null" "tags-plus-ISSUED|null" PASS \
    "$OUT/HARNESS_multistatement_tags.stdout" "$OUT/HARNESS_multistatement_tags.stderr"
else
  record HARNESS_multistatement_tags "tags-plus-ISSUED|null" "see-stdout" FAIL \
    "$OUT/HARNESS_multistatement_tags.stdout" "$OUT/HARNESS_multistatement_tags.stderr"
fi

# E05. Two sessions. Session A holds FOR SHARE. Session B updates, then A commits.
psql -X -U lock_probe -d "$PGDATABASE" -v ON_ERROR_STOP=1 -f concurrency_session_a.sql \
  > "$OUT/E05_session_a.stdout" 2> "$OUT/E05_session_a.stderr" &
holder_pid=$!
holder_seen=0
for _ in $(seq 1 40); do
  if ! kill -0 "$holder_pid" 2>/dev/null; then
    break
  fi
  held=$(psql -X -q -tAc "SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'lock_probe' AND c.relname = 'shipments' AND l.granted AND l.mode = 'RowShareLock';" | tr -d '[:space:]')
  if [ "${held:-0}" != "0" ]; then
    holder_seen=1
    break
  fi
  sleep 0.25
done
printf 'holder_seen=%s held=%s\n' "$holder_seen" "${held:-none}" > "$OUT/E05_holder.stdout"
if [ "$holder_seen" = "1" ]; then
  record E05_holder_share_granted 1 1 PASS "$OUT/E05_holder.stdout" "$OUT/E05_session_a.stderr"
else
  record E05_holder_share_granted 1 "${held:-0}" FAIL "$OUT/E05_holder.stdout" "$OUT/E05_session_a.stderr"
fi

psql -X -U writer_probe -d "$PGDATABASE" -v ON_ERROR_STOP=1 -f concurrency_session_b.sql \
  > "$OUT/E05_session_b.stdout" 2> "$OUT/E05_session_b.stderr" &
waiter_pid=$!
waiting=0
for _ in $(seq 1 40); do
  if ! kill -0 "$waiter_pid" 2>/dev/null; then
    break
  fi
  mark=$(psql -X -q -tAc "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'writer_probe' AND pid <> pg_backend_pid() AND wait_event_type = 'Lock') THEN 'WAITING' ELSE 'NOT_WAITING' END;" | tr -d '[:space:]')
  printf '%s\n' "$mark" >> "$OUT/E05_waiter_poll.stdout"
  if [ "$mark" = "WAITING" ]; then
    waiting=1
    break
  fi
  sleep 0.25
done
psql -X -U "$PGUSER" -d "$PGDATABASE" -c "SELECT usename, wait_event_type, wait_event, state FROM pg_stat_activity WHERE usename = 'writer_probe' AND pid <> pg_backend_pid();" > "$OUT/E05_waiter_detail.stdout" 2> "$OUT/E05_waiter_detail.stderr" || true
if [ "$waiting" = "1" ]; then
  record E05_waiter_blocked WAITING WAITING PASS "$OUT/E05_waiter_poll.stdout" "$OUT/E05_waiter_detail.stdout"
else
  record E05_waiter_blocked WAITING "${mark:-NOT_SAMPLED}" FAIL "$OUT/E05_waiter_poll.stdout" "$OUT/E05_session_b.stderr"
fi

wait "$holder_pid" || true
wait "$waiter_pid" || true
if grep -q 'UPDATE 1' "$OUT/E05_session_b.stdout"; then
  record E05_update_after_release "UPDATE 1" "UPDATE 1" PASS "$OUT/E05_session_b.stdout" "$OUT/E05_session_b.stderr"
else
  record E05_update_after_release "UPDATE 1" "see-stdout" FAIL "$OUT/E05_session_b.stdout" "$OUT/E05_session_b.stderr"
fi

printf 'passes=%s fails=%s\n' "$PASSES" "$FAILS" | tee "$OUT/suite_counts.txt"
if [ "$FAILS" -ne 0 ]; then
  exit 1
fi
exit 0
