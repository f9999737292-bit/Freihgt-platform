#!/bin/bash
# Isolated 0.1N foreign-key and return/redirect interleavings.
set -u
cd "$(dirname "$0")"
export PGUSER="${PGUSER:-proof_admin}"
export PGDATABASE="${PGDATABASE:-bintrans_01n_fk_keyshare}"
OUT="${OUT:-runs/attempt-1}"
HOLD="${HOLD:-20}"
mkdir -p "$OUT"
SUMMARY="$OUT/assertions.tsv"
PASSES=0
FAILS=0
printf 'test_id\texpected\tactual\tverdict\tstdout\tstderr\n' > "$SUMMARY"
: > "$OUT/commands.log"

EXEC='11111111-1111-1111-1111-111111111111'
SHIP='22222222-2222-2222-2222-222222222222'
INITIAL='11111111-1111-1111-1111-111111111111|1|77777777-7777-7777-7777-777777777777|1|55555555-5555-5555-5555-555555555555|ACTIVE|DISPOSITION_PENDING|1|NONE|1'

record() {
  local id="$1" expect="$2" actual="$3" verdict="$4"
  printf '%s\t%s\t%s\t%s\t%s.stdout\t%s.stderr\n' "$id" "$expect" "$actual" "$verdict" "$id" "$id" >> "$SUMMARY"
  printf '%s expected=%s actual=%s %s\n' "$id" "$expect" "$actual" "$verdict"
  if [ "$verdict" = "PASS" ]; then PASSES=$((PASSES + 1)); else FAILS=$((FAILS + 1)); fi
}

sqlstate_of() {
  local errf="$1" state
  if [ ! -f "$errf" ]; then printf 'OK'; return; fi
  state=$(sed -n 's/.*ERROR:  \([0-9A-Z][0-9A-Z][0-9A-Z][0-9A-Z][0-9A-Z]\):.*/\1/p' "$errf" | head -n 1)
  if [ -n "$state" ]; then printf '%s' "$state"
  elif grep -q 'ERROR:' "$errf"; then printf 'UNPARSED'
  else printf 'OK'; fi
}

psql_admin() {
  psql -X -U proof_admin -d "$PGDATABASE" -v ON_ERROR_STOP=1 -t -A "$@"
}

wait_true() {
  local sql="$1" i=0 val
  while [ "$i" -lt 40 ]; do
    val=$(psql_admin -c "$sql" | tr -d '[:space:]')
    if [ "$val" = "1" ]; then printf '1'; return 0; fi
    i=$((i + 1)); sleep 0.2
  done
  printf '0'
}

psql_f() {
  local id="$1" role="$2" file="$3" rc
  shift 3
  printf '%s role=%s file=%s %s\n' "$id" "$role" "$file" "$*" >> "$OUT/commands.log"
  psql -X -U "$role" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -v VERBOSITY=verbose -t -A "$@" -f "$file" \
    >"$OUT/${id}.stdout" 2>"$OUT/${id}.stderr" || rc=$?
  rc=${rc:-0}
  printf '%s' "$rc" > "$OUT/${id}.rc"
  printf '%s' "$rc"
}

start_bg() {
  local id="$1" role="$2" file="$3"
  shift 3
  printf '%s role=%s file=%s %s\n' "$id" "$role" "$file" "$*" >> "$OUT/commands.log"
  psql -X -U "$role" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -v VERBOSITY=verbose -t -A "$@" -f "$file" \
    >"$OUT/${id}.stdout" 2>"$OUT/${id}.stderr" &
  printf '%s' "$!" > "$OUT/${id}.pid"
}

finish_bg() {
  local id="$1" pid rc
  pid=$(cat "$OUT/${id}.pid")
  wait "$pid" || rc=$?
  rc=${rc:-0}
  printf '%s' "$rc" > "$OUT/${id}.rc"
  printf '%s' "$rc"
}

snap_now() { psql_admin -f snapshot.sql | tr -d '\r' | sed '/^$/d'; }

expect_state() {
  local id="$1" expect="$2" rc state
  rc=$(cat "$OUT/${id}.rc")
  state=$(sqlstate_of "$OUT/${id}.stderr")
  if [ "$expect" = "OK" ]; then
    if [ "$rc" = "0" ] && [ "$state" = "OK" ]; then record "$id" OK "$state" PASS
    else record "$id" OK "rc:${rc};state:${state}" FAIL; fi
  else
    if [ "$rc" != "0" ] && [ "$state" = "$expect" ]; then record "$id" "$expect" "$state" PASS
    else record "$id" "$expect" "rc:${rc};state:${state}" FAIL; fi
  fi
}

expect_snap() {
  local id="$1" expect="$2" got
  got=$(snap_now)
  printf '%s\n' "$got" > "$OUT/${id}_snapshot.stdout"
  if [ "$got" = "$expect" ]; then record "${id}_snapshot" "$expect" "$got" PASS
  else record "${id}_snapshot" "$expect" "$got" FAIL; fi
}

reset_fixture() {
  local id="$1" rc
  rc=$(psql_f "${id}_reset" proof_admin fixture.sql)
  if [ "$rc" != "0" ]; then record "${id}_reset" 0 "$rc" FAIL; return 1; fi
  return 0
}

reader_sleeping() {
  wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid=a.pid JOIN pg_class c ON c.oid=l.relation WHERE a.usename='reader_probe' AND a.query LIKE '%pg_sleep%' AND c.relname='transport_execution_active_shipments' AND l.mode='RowShareLock' AND l.granted) THEN 1 ELSE 0 END;"
}
writer_sleeping() {
  wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename='writer_probe' AND state='active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;"
}
role_waiting() {
  local role="$1"
  wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename='${role}' AND wait_event_type='Lock') THEN 1 ELSE 0 END;"
}

dump_locks() {
  local id="$1"
  psql_admin -c "SELECT a.usename || '|' || l.locktype || '|' || coalesce(c.relname,'') || '|' || l.mode || '|' || l.granted FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid LEFT JOIN pg_class c ON c.oid=l.relation WHERE a.usename IN ('reader_probe','writer_probe') AND l.locktype IN ('tuple','relation','transactionid') ORDER BY 1;" \
    >"$OUT/${id}_locks.stdout" 2>"$OUT/${id}_locks.stderr"
}

row_modes() {
  local id="$1" table="$2"
  psql_admin -c "SELECT coalesce(string_agg(m, ',' ORDER BY m), 'NONE') FROM (SELECT unnest(modes)::text AS m FROM pgrowlocks('transport.${table}')) s;" \
    >"$OUT/${id}_${table}_rows.stdout" 2>"$OUT/${id}_${table}_rows.stderr"
}

mode_has() {
  local file="$1" token="$2" modes
  modes=$(tr -d '[:space:]' < "$file")
  case ",${modes}," in
    *",${token},"*) return 0 ;;
  esac
  return 1
}

assert_tuple_wait() {
  local id="$1" rel="$2" hw="$3"
  if grep -q "writer_probe|tuple|${rel}|${hw}|true" "$OUT/${id}_locks.stdout"; then
    record "${id}_tuple_hw" "$hw" present PASS
  else
    record "${id}_tuple_hw" "$hw" absent FAIL
  fi
}

# Reader holds. Writer is not expected to wait, and must commit before the reader finishes.
nowait_reader_first() {
  local id="$1" writer="$2" final="$3" observed=UNKNOWN i=0 waited
  reset_fixture "$id" || return
  start_bg "${id}_reader" reader_probe reader_hold.sql -v execution="$EXEC" -v shipment="$SHIP" -v hold="$HOLD"
  local held; held=$(reader_sleeping)
  if [ "$held" = "1" ]; then record "${id}_reader_held" 1 1 PASS; else record "${id}_reader_held" 1 "$held" FAIL; fi
  start_bg "${id}_writer" writer_probe "$writer"
  while [ "$i" -lt 25 ]; do
    if ! kill -0 "$(cat "$OUT/${id}_writer.pid")" 2>/dev/null; then observed=NO_WAIT; break; fi
    waited=$(psql_admin -c "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename='writer_probe' AND wait_event_type='Lock') THEN 1 ELSE 0 END;" | tr -d '[:space:]')
    if [ "$waited" = "1" ]; then observed=WAIT; break; fi
    i=$((i + 1)); sleep 0.2
  done
  if [ "$observed" = "NO_WAIT" ]; then
    finish_bg "${id}_writer" >/dev/null
    local still; still=$(psql_admin -c "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename='reader_probe' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;" | tr -d '[:space:]')
    if [ "$still" = "1" ]; then record "${id}_committed_while_reader_held" 1 1 PASS; else record "${id}_committed_while_reader_held" 1 "$still" FAIL; fi
    finish_bg "${id}_reader" >/dev/null
  else
    record "${id}_progress" NO_WAIT "$observed" FAIL
    finish_bg "${id}_reader" >/dev/null
    finish_bg "${id}_writer" >/dev/null
  fi
  record "${id}_writer_lock_wait" NO_WAIT "$observed" "$([ "$observed" = "NO_WAIT" ] && printf PASS || printf FAIL)"
  expect_state "${id}_reader" OK
  expect_state "${id}_writer" OK
  expect_snap "$id" "$final"
}

# Writer sleeps after its FK key-share. Reader must finish without waiting.
nowait_writer_first() {
  local id="$1" writer="$2" final="$3" kind="${4:-visible}"
  reset_fixture "$id" || return
  start_bg "${id}_writer" writer_probe "$writer"
  local held; held=$(writer_sleeping)
  if [ "$held" = "1" ]; then record "${id}_writer_held" 1 1 PASS; else record "${id}_writer_held" 1 "$held" FAIL; fi
  dump_locks "$id"
  row_modes "$id" transport_executions
  local modes; modes=$(tr -d '[:space:]' < "$OUT/${id}_transport_executions_rows.stdout")
  if [ "$kind" = "uncommitted" ]; then
    psql_admin -c "SELECT count(*) FROM pg_stat_activity w, generate_series(0, (SELECT greatest(relpages,1)-1 FROM pg_class WHERE oid='transport.transport_executions'::regclass)) AS p, LATERAL heap_page_items(get_raw_page('transport.transport_executions', p)) h WHERE w.usename='writer_probe' AND w.query LIKE '%pg_sleep%' AND h.t_xmin::text=w.backend_xid::text AND (h.t_infomask & 16)<>0 AND (h.t_infomask & 128)<>0 AND (h.t_infomask & 64)=0 AND encode(substring(h.t_data from 1 for 16),'hex') LIKE '12121212%';" \
      >"$OUT/${id}_newrow_keyshare.stdout" 2>"$OUT/${id}_newrow_keyshare.stderr"
    local bits; bits=$(tr -d '[:space:]' < "$OUT/${id}_newrow_keyshare.stdout")
    if [ "$modes" = "NONE" ] && [ "$bits" = "1" ] \
       && grep -q 'writer_probe|relation|transport_executions|RowShareLock|true' "$OUT/${id}_locks.stdout" \
       && grep -q 'writer_probe|relation|transport_executions|RowExclusiveLock|true' "$OUT/${id}_locks.stdout" \
       && ! grep -q '|tuple|' "$OUT/${id}_locks.stdout"; then
      record "${id}_keyshare" "KEYSHR_LOCK on uncommitted 1212" "rows:${modes};bits:${bits}" PASS
    else
      record "${id}_keyshare" "KEYSHR_LOCK on uncommitted 1212" "rows:${modes};bits:${bits}" FAIL
    fi
  elif mode_has "$OUT/${id}_transport_executions_rows.stdout" ForKeyShare \
     && grep -q 'writer_probe|relation|transport_executions|RowShareLock|true' "$OUT/${id}_locks.stdout" \
     && ! grep -q '|tuple|' "$OUT/${id}_locks.stdout"; then
    record "${id}_keyshare" "ForKeyShare" "$modes" PASS
  else
    record "${id}_keyshare" "ForKeyShare" "$modes" FAIL
  fi
  start_bg "${id}_reader" reader_probe reader_read.sql -v execution="$EXEC" -v shipment="$SHIP"
  local finished; finished=$(wait_true "SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename='reader_probe' AND pid<>pg_backend_pid()) THEN 1 ELSE 0 END;")
  local still; still=$(psql_admin -c "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename='writer_probe' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;" | tr -d '[:space:]')
  if [ "$finished" = "1" ] && [ "$still" = "1" ]; then record "${id}_reader_did_not_wait" 1 "finished:${finished};writer:${still}" PASS
  else record "${id}_reader_did_not_wait" 1 "finished:${finished};writer:${still}" FAIL; fi
  finish_bg "${id}_reader" >/dev/null
  finish_bg "${id}_writer" >/dev/null
  expect_state "${id}_reader" OK
  expect_state "${id}_writer" OK
  expect_snap "$id" "$final"
}

wait_reader_first() {
  local id="$1" writer="$2" wstate="$3" final="$4"
  reset_fixture "$id" || return
  start_bg "${id}_reader" reader_probe reader_hold.sql -v execution="$EXEC" -v shipment="$SHIP" -v hold="$HOLD"
  local held; held=$(reader_sleeping)
  if [ "$held" = "1" ]; then record "${id}_reader_held" 1 1 PASS; else record "${id}_reader_held" 1 "$held" FAIL; fi
  start_bg "${id}_writer" writer_probe "$writer"
  local waited; waited=$(role_waiting writer_probe)
  dump_locks "$id"
  if grep -q 'writer_probe|transactionid||ShareLock|false' "$OUT/${id}_locks.stdout"; then
    record "${id}_xid_wait" "transactionid ShareLock false" present PASS
  else
    record "${id}_xid_wait" "transactionid ShareLock false" absent FAIL
  fi
  if [ "$waited" = "1" ]; then record "${id}_writer_wait" 1 1 PASS; else record "${id}_writer_wait" 1 "$waited" FAIL; fi
  local mid; mid=$(snap_now)
  if [ "$mid" = "$INITIAL" ]; then record "${id}_no_commit_while_reader_holds" "$INITIAL" "$mid" PASS
  else record "${id}_no_commit_while_reader_holds" "$INITIAL" "$mid" FAIL; fi
  finish_bg "${id}_reader" >/dev/null
  finish_bg "${id}_writer" >/dev/null
  expect_state "${id}_reader" OK
  expect_state "${id}_writer" "$wstate"
  expect_snap "$id" "$final"
}

wait_writer_first() {
  local id="$1" writer="$2" wstate="$3" final="$4"
  reset_fixture "$id" || return
  start_bg "${id}_writer" writer_probe "$writer"
  local held; held=$(writer_sleeping)
  if [ "$held" = "1" ]; then record "${id}_writer_held" 1 1 PASS; else record "${id}_writer_held" 1 "$held" FAIL; fi
  dump_locks "$id"
  row_modes "$id" transport_executions
  row_modes "$id" delivery_disposition_cases
  row_modes "$id" shipments
  start_bg "${id}_reader" reader_probe reader_read.sql -v execution="$EXEC" -v shipment="$SHIP"
  local waited; waited=$(role_waiting reader_probe)
  if [ "$waited" = "1" ]; then record "${id}_reader_wait" 1 1 PASS; else record "${id}_reader_wait" 1 "$waited" FAIL; fi
  finish_bg "${id}_writer" >/dev/null
  finish_bg "${id}_reader" >/dev/null
  expect_state "${id}_writer" "$wstate"
  expect_state "${id}_reader" OK
  expect_snap "$id" "$final"
}

db=$(psql_admin -c 'SELECT current_database();' | tr -d '[:space:]')
ver=$(psql_admin -c 'SHOW server_version_num;' | tr -d '[:space:]')
printf 'database=%s\nserver_version_num=%s\n' "$db" "$ver" | tee "$OUT/server.txt"
if [ "$db" != "bintrans_01n_fk_keyshare" ]; then printf 'refusing database %s\n' "$db" >&2; exit 2; fi
if [ "$ver" -lt 160000 ] || [ "$ver" -ge 170000 ]; then printf 'refusing server_version_num %s\n' "$ver" >&2; exit 2; fi

psql_f setup proof_admin setup.sql >/dev/null
if [ "$(cat "$OUT/setup.rc")" != "0" ]; then record setup 0 "$(cat "$OUT/setup.rc")" FAIL; exit 2; fi
psql_admin -c "SELECT conname || '|' || condeferrable::text || '|' || condeferred::text FROM pg_constraint WHERE contype='f' AND connamespace='transport'::regnamespace ORDER BY 1;" > "$OUT/constraints.stdout"
deferred=$(psql_admin -c "SELECT count(*) FROM pg_constraint WHERE contype='f' AND connamespace='transport'::regnamespace AND condeferred;" | tr -d '[:space:]')
if [ "$deferred" = "1" ] && grep -q 'transport_executions_current_revision_fk|true|true' "$OUT/constraints.stdout"; then
  record deferred_fk_catalog "1:current_revision" "${deferred}:current_revision" PASS
else record deferred_fk_catalog "1:current_revision" "$deferred" FAIL; fi

reset_fixture baseline
INITIAL_GOT=$(snap_now)
printf '%s\n' "$INITIAL_GOT" > "$OUT/initial_snapshot.txt"
if [ "$INITIAL_GOT" = "$INITIAL" ]; then record baseline_snapshot "$INITIAL" "$INITIAL_GOT" PASS
else record baseline_snapshot "$INITIAL" "$INITIAL_GOT" FAIL; fi

RI='11111111-1111-1111-1111-111111111111|1|77777777-7777-7777-7777-777777777777|1|55555555-5555-5555-5555-555555555555|ACTIVE|DISPOSITION_PENDING|2|NONE|1'
AL='11111111-1111-1111-1111-111111111111|1|77777777-7777-7777-7777-777777777777|1|55555555-5555-5555-5555-555555555555|ACTIVE|DISPOSITION_PENDING|1|11111111-1111-1111-1111-111111111111|1'
EX='11111111-1111-1111-1111-111111111111|1|77777777-7777-7777-7777-777777777777|2|55555555-5555-5555-5555-555555555555|ACTIVE|DISPOSITION_PENDING|1|NONE|1'
FKU='11111111-1111-1111-1111-111111111111|2|77777777-7777-7777-7777-777777777778|1|55555555-5555-5555-5555-555555555555|ACTIVE|DISPOSITION_PENDING|1|NONE|1'
SU='11111111-1111-1111-1111-111111111111|1|77777777-7777-7777-7777-777777777777|1|55555555-5555-5555-5555-555555555556|SUPERSEDED|DISPOSITION_PENDING|1|NONE|2'
DF='11111111-1111-1111-1111-111111111111|1|77777777-7777-7777-7777-777777777777|1|55555555-5555-5555-5555-555555555559|SUPERSEDED|DISPOSITION_PENDING|1|NONE|1'
RT='11111111-1111-1111-1111-111111111111|1|77777777-7777-7777-7777-777777777777|1|55555555-5555-5555-5555-55555555555a|SUPERSEDED|IN_RETURN_TRANSIT|1|NONE|3'
RD='11111111-1111-1111-1111-111111111111|1|77777777-7777-7777-7777-777777777777|1|55555555-5555-5555-5555-55555555555b|SUPERSEDED|IN_REDIRECT_TRANSIT|1|NONE|3'

nowait_reader_first RI_R writers/ri_insert.sql "$RI"
nowait_writer_first RI_W writers/ri_insert_hold.sql "$RI"
wait_reader_first FKU_R writers/fk_update.sql OK "$FKU"
assert_tuple_wait FKU_R shipments ExclusiveLock
wait_writer_first FKU_W writers/fk_update_hold.sql OK "$FKU"
wait_reader_first PKU_R writers/pk_update.sql 23503 "$INITIAL"
assert_tuple_wait PKU_R transport_executions AccessExclusiveLock
wait_writer_first PKU_W writers/pk_update_hold.sql 23503 "$INITIAL"
wait_reader_first DEL_R writers/delete_exec.sql 23503 "$INITIAL"
assert_tuple_wait DEL_R transport_executions AccessExclusiveLock
wait_writer_first DEL_W writers/delete_exec_hold.sql 23503 "$INITIAL"
nowait_reader_first EX_R writers/new_execution.sql "$EX"
nowait_writer_first EX_W writers/new_execution_hold.sql "$EX" uncommitted
nowait_reader_first AL_R writers/link_insert.sql "$AL"
nowait_writer_first AL_W writers/link_insert_hold.sql "$AL"
wait_reader_first SU_R writers/successor.sql OK "$SU"
assert_tuple_wait SU_R transport_executions AccessExclusiveLock
wait_writer_first SU_W writers/successor_hold.sql OK "$SU"

reset_fixture DF_TRIGGER
rc=$(psql_f DF_TRIGGER_writer writer_probe writers/df_trigger.sql)
expect_state DF_TRIGGER_writer 23514
expect_snap DF_TRIGGER "$INITIAL"
reset_fixture DF_COMMIT
rc=$(psql_f DF_COMMIT_writer writer_probe writers/df_commit.sql)
expect_state DF_COMMIT_writer 23503
expect_snap DF_COMMIT "$INITIAL"
wait_reader_first DF_R writers/df_ok.sql OK "$DF"
wait_writer_first DF_W writers/df_ok_hold.sql OK "$DF"

wait_reader_first RT_R writers/return.sql OK "$RT"
assert_tuple_wait RT_R transport_executions AccessExclusiveLock
wait_writer_first RT_W writers/return_hold.sql OK "$RT"
if mode_has "$OUT/RT_W_transport_executions_rows.stdout" ForUpdate \
   && mode_has "$OUT/RT_W_delivery_disposition_cases_rows.stdout" ForUpdate \
   && [ "$(tr -d '[:space:]' < "$OUT/RT_W_shipments_rows.stdout")" = "NONE" ] \
   && grep -q 'writer_probe|relation|transport_executions|RowShareLock|true' "$OUT/RT_W_locks.stdout" \
   && grep -q 'writer_probe|relation|shipments|AccessShareLock|true' "$OUT/RT_W_locks.stdout" \
   && ! grep -q '|tuple|' "$OUT/RT_W_locks.stdout"; then
  record RT_W_lock_order "ForUpdate exec+case, shipment unlocked" present PASS
else
  record RT_W_lock_order "ForUpdate exec+case, shipment unlocked" "exec=$(tr -d '[:space:]' < "$OUT/RT_W_transport_executions_rows.stdout") case=$(tr -d '[:space:]' < "$OUT/RT_W_delivery_disposition_cases_rows.stdout") ship=$(tr -d '[:space:]' < "$OUT/RT_W_shipments_rows.stdout")" FAIL
fi

wait_reader_first RD_R writers/redirect.sql OK "$RD"
assert_tuple_wait RD_R transport_executions AccessExclusiveLock
wait_writer_first RD_W writers/redirect_hold.sql OK "$RD"
if mode_has "$OUT/RD_W_transport_executions_rows.stdout" ForUpdate \
   && mode_has "$OUT/RD_W_delivery_disposition_cases_rows.stdout" ForUpdate \
   && [ "$(tr -d '[:space:]' < "$OUT/RD_W_shipments_rows.stdout")" = "NONE" ] \
   && grep -q 'writer_probe|relation|transport_executions|RowShareLock|true' "$OUT/RD_W_locks.stdout" \
   && grep -q 'writer_probe|relation|shipments|AccessShareLock|true' "$OUT/RD_W_locks.stdout" \
   && ! grep -q '|tuple|' "$OUT/RD_W_locks.stdout"; then
  record RD_W_same_prefix_as_return present present PASS
else
  record RD_W_same_prefix_as_return present "exec=$(tr -d '[:space:]' < "$OUT/RD_W_transport_executions_rows.stdout") case=$(tr -d '[:space:]' < "$OUT/RD_W_delivery_disposition_cases_rows.stdout") ship=$(tr -d '[:space:]' < "$OUT/RD_W_shipments_rows.stdout")" FAIL
fi

if grep -R -n '40P01' "$OUT" --include='*.stderr' >/dev/null 2>&1; then
  grep -R -n '40P01' "$OUT" --include='*.stderr' > "$OUT/deadlock.txt" || true
  record deadlock_scan NO_DEADLOCK_IN_TESTED_FK_INTERLEAVINGS FOUND FAIL
else
  printf 'NO_DEADLOCK_IN_TESTED_FK_INTERLEAVINGS\n' > "$OUT/deadlock.txt"
  record deadlock_scan NO_DEADLOCK_IN_TESTED_FK_INTERLEAVINGS NO_DEADLOCK_IN_TESTED_FK_INTERLEAVINGS PASS
fi

printf 'passes=%s\nfails=%s\n' "$PASSES" "$FAILS" | tee "$OUT/suite_counts.txt"
if [ "$FAILS" -ne 0 ]; then exit 1; fi
exit 0
