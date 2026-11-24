#!/bin/bash
# Isolated 0.1M writer/reader interleavings. Each psql invocation is one session.
set -u
cd "$(dirname "$0")"
export PGUSER="${PGUSER:-proof_admin}"
export PGDATABASE="${PGDATABASE:-bintrans_01m_writer_compat}"
OUT="${OUT:-runs/attempt-1}"
HOLD="${HOLD:-12}"
mkdir -p "$OUT"
SUMMARY="$OUT/assertions.tsv"
PASSES=0
FAILS=0
printf 'test_id\texpected\tactual\tverdict\tstdout\tstderr\n' > "$SUMMARY"
: > "$OUT/commands.log"

EXEC='11111111-1111-1111-1111-111111111111'
SHIP='22222222-2222-2222-2222-222222222222'
OTHER='aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
BASE_LINE="${EXEC}|${SHIP}|1|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333333|PLANNED|${EXEC}|55555555-5555-5555-5555-555555555555|1"
DRIVER_LINE="${EXEC}|${SHIP}|2|77777777-7777-7777-7777-777777777778|33333333-3333-3333-3333-333333333333|ASSIGNED|${EXEC}|55555555-5555-5555-5555-555555555555|1"
VEHICLE_LINE="${EXEC}|${SHIP}|2|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333334|ASSIGNED|${EXEC}|55555555-5555-5555-5555-555555555555|1"
STOP_LINE="${EXEC}|${SHIP}|2|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333333|LOADED|${EXEC}|55555555-5555-5555-5555-555555555555|2"
SUCCESSOR_LINE="${EXEC}|${SHIP}|1|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333333|PLANNED|${EXEC}|55555555-5555-5555-5555-555555555556|1"
STATUS_LINE="${EXEC}|${SHIP}|2|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333333|IN_TRANSIT|${EXEC}|55555555-5555-5555-5555-555555555555|1"
OTHER_LINE="${OTHER}|${SHIP}|1|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333333|PLANNED|${OTHER}|55555555-5555-5555-5555-555555555557|1"

record() {
  local id="$1" expect="$2" actual="$3" verdict="$4"
  printf '%s\t%s\t%s\t%s\t%s.stdout\t%s.stderr\n' "$id" "$expect" "$actual" "$verdict" "$id" "$id" >> "$SUMMARY"
  printf '%s expected=%s actual=%s %s\n' "$id" "$expect" "$actual" "$verdict"
  if [ "$verdict" = "PASS" ]; then
    PASSES=$((PASSES + 1))
  else
    FAILS=$((FAILS + 1))
  fi
}

sqlstate_of() {
  local errf="$1" state
  if [ ! -f "$errf" ]; then
    printf 'OK'
    return
  fi
  state=$(sed -n 's/.*ERROR:  \([0-9A-Z][0-9A-Z][0-9A-Z][0-9A-Z][0-9A-Z]\):.*/\1/p' "$errf" | head -n 1)
  if [ -n "$state" ]; then
    printf '%s' "$state"
  elif grep -q 'ERROR:' "$errf"; then
    printf 'UNPARSED'
  else
    printf 'OK'
  fi
}

one_line() {
  local file="$1" cleaned n
  if [ ! -s "$file" ]; then
    printf '__EMPTY__'
    return
  fi
  cleaned=$(tr -d '\r' < "$file" | sed -e '/^[[:space:]]*$/d' -e '/^SET$/d' -e '/^BEGIN$/d' -e '/^COMMIT$/d' -e '/^ROLLBACK$/d' -e '/^INSERT /d' -e '/^UPDATE /d' -e '/^TRUNCATE /d' -e '/^DO$/d' -e '/^GRANT$/d' -e '/^CREATE /d')
  if [ -z "$cleaned" ]; then
    printf '__EMPTY__'
    return
  fi
  n=$(printf '%s\n' "$cleaned" | wc -l | tr -d '[:space:]')
  if [ "$n" -ne 1 ]; then
    printf '__MULTI__'
    return
  fi
  printf '%s' "$cleaned"
}

psql_admin() {
  psql -X -U proof_admin -d "$PGDATABASE" -v ON_ERROR_STOP=1 -t -A "$@"
}

wait_true() {
  local sql="$1" i val
  i=0
  while [ "$i" -lt 30 ]; do
    val=$(psql_admin -c "$sql" | tr -d '[:space:]')
    if [ "$val" = "1" ]; then
      printf '1'
      return 0
    fi
    i=$((i + 1))
    sleep 0.2
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

snap_now() {
  psql_admin -f snapshot.sql | tr -d '\r' | sed '/^$/d'
}

activity() {
  local id="$1"
  psql_admin -c "SELECT usename || '|' || coalesce(wait_event_type,'none') || '|' || coalesce(wait_event,'none') || '|' || state || '|' || CASE WHEN query LIKE '%pg_sleep%' THEN 'pg_sleep' WHEN query LIKE '%transport_execution_active_shipments%' THEN 'active_shipments' WHEN query LIKE '%transport_executions%' THEN 'executions' WHEN query LIKE '%shipments%' THEN 'shipments' WHEN query LIKE '%drivers%' THEN 'drivers' ELSE 'other' END FROM pg_stat_activity WHERE usename IN ('reader_probe','writer_probe') AND pid <> pg_backend_pid() ORDER BY usename;" \
    >"$OUT/${id}.stdout" 2>"$OUT/${id}.stderr"
}

writer_execution_locks() {
  psql_admin -c "SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'writer_probe' AND a.query LIKE '%pg_sleep%' AND c.relname = 'transport_executions';" | tr -d '[:space:]'
}

link_lock_modes() {
  psql_admin -c "SELECT coalesce(string_agg(DISTINCT l.mode, ',' ORDER BY l.mode), 'NONE') FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'writer_probe' AND a.query LIKE '%pg_sleep%' AND c.relname = 'transport_execution_active_shipments' AND l.locktype = 'relation';" | tr -d '[:space:]'
}

reset_fixture() {
  local id="$1" rc
  rc=$(psql_f "${id}_reset" proof_admin fixture.sql)
  if [ "$rc" != "0" ]; then
    record "${id}_reset" 0 "$rc" FAIL
    return 1
  fi
  return 0
}

reader_sleeping() {
  wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'reader_probe' AND a.query LIKE '%pg_sleep%' AND c.relname = 'transport_execution_active_shipments' AND l.mode = 'RowShareLock' AND l.granted) THEN 1 ELSE 0 END;"
}

writer_sleeping() {
  wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'writer_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;"
}

role_waiting() {
  local role="$1"
  wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = '${role}' AND wait_event_type = 'Lock') THEN 1 ELSE 0 END;"
}

expect_state() {
  local id="$1" expect="$2" rc state
  rc=$(cat "$OUT/${id}.rc")
  state=$(sqlstate_of "$OUT/${id}.stderr")
  if [ "$expect" = "OK" ]; then
    if [ "$rc" = "0" ] && [ "$state" = "OK" ]; then
      record "$id" OK "$state" PASS
    else
      record "$id" OK "rc:${rc};state:${state}" FAIL
    fi
  else
    if [ "$rc" != "0" ] && [ "$state" = "$expect" ]; then
      record "$id" "$expect" "$state" PASS
    else
      record "$id" "$expect" "rc:${rc};state:${state}" FAIL
    fi
  fi
}

expect_line() {
  local id="$1" expect="$2" got
  got=$(one_line "$OUT/${id}.stdout")
  if [ "$got" = "$expect" ]; then
    record "${id}_line" "$expect" "$got" PASS
  else
    record "${id}_line" "$expect" "$got" FAIL
  fi
}

expect_snap() {
  local id="$1" expect="$2" got
  got=$(snap_now)
  printf '%s\n' "$got" > "$OUT/${id}_snapshot.stdout"
  if [ "$got" = "$expect" ]; then
    record "${id}_snapshot" "$expect" "$got" PASS
  else
    record "${id}_snapshot" "$expect" "$got" FAIL
  fi
}

db=$(psql_admin -c 'SELECT current_database();' | tr -d '[:space:]')
ver=$(psql_admin -c 'SHOW server_version_num;' | tr -d '[:space:]')
printf 'database=%s\nserver_version_num=%s\n' "$db" "$ver" | tee "$OUT/server.txt"
if [ "$db" != "bintrans_01m_writer_compat" ]; then
  printf 'refusing database %s\n' "$db" >&2
  exit 2
fi
if [ "$ver" -lt 160000 ] || [ "$ver" -ge 170000 ]; then
  printf 'refusing server_version_num %s\n' "$ver" >&2
  exit 2
fi

if grep -E 'FOR UPDATE' writers/project_on_occupied.sql writers/project_on_occupied_hold.sql writers/project_on_free.sql writers/project_on_free_hold.sql writers/project_off_occupied.sql writers/project_off_free.sql | grep -q 'transport_executions'; then
  record static_project_has_no_existing_execution_lock absent present FAIL
else
  record static_project_has_no_existing_execution_lock absent absent PASS
fi

psql_f setup proof_admin setup.sql >/dev/null
reset_fixture baseline
INITIAL=$(snap_now)
printf '%s\n' "$INITIAL" > "$OUT/initial_snapshot.txt"
if [ "$INITIAL" = "${EXEC}|NONE|2|1|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333333|PLANNED|1|55555555-5555-5555-5555-555555555555|ACTIVE" ]; then
  record baseline_snapshot "$INITIAL" "$INITIAL" PASS
else
  record baseline_snapshot "${EXEC}|NONE|2|..." "$INITIAL" FAIL
fi

FREE_OFF="${EXEC}|12121212-1212-1212-1212-121212121212|3|1|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333333|PLANNED|1|55555555-5555-5555-5555-555555555555|ACTIVE"
FREE_ON="${EXEC}|14141414-1414-1414-1414-141414141414|3|1|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333333|PLANNED|1|55555555-5555-5555-5555-555555555555|ACTIVE"
SNAP_DRIVER="${EXEC}|NONE|2|2|77777777-7777-7777-7777-777777777778|33333333-3333-3333-3333-333333333333|ASSIGNED|1|55555555-5555-5555-5555-555555555555|ACTIVE"
SNAP_VEHICLE="${EXEC}|NONE|2|2|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333334|ASSIGNED|1|55555555-5555-5555-5555-555555555555|ACTIVE"
SNAP_STOP="${EXEC}|NONE|2|2|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333333|LOADED|2|55555555-5555-5555-5555-555555555555|ACTIVE"
SNAP_SUCCESSOR="${EXEC}|NONE|2|1|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333333|PLANNED|1|55555555-5555-5555-5555-555555555556|SUPERSEDED"
SNAP_STATUS="${EXEC}|NONE|2|2|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333333|IN_TRANSIT|1|55555555-5555-5555-5555-555555555555|ACTIVE"
SNAP_LINK="${OTHER}|NONE|2|1|77777777-7777-7777-7777-777777777777|33333333-3333-3333-3333-333333333333|PLANNED|1|55555555-5555-5555-5555-555555555555|ACTIVE"

reader_first() {
  local id="$1" writer="$2" wstate="$3" final="$4" held waited mid rc
  reset_fixture "$id" || return
  start_bg "${id}_reader" reader_probe reader_hold.sql -v execution="$EXEC" -v shipment="$SHIP" -v hold="$HOLD"
  held=$(reader_sleeping)
  if [ "$held" = "1" ]; then record "${id}_reader_held" 1 1 PASS; else record "${id}_reader_held" 1 "$held" FAIL; fi
  start_bg "${id}_writer" writer_probe "$writer"
  waited=$(role_waiting writer_probe)
  activity "${id}_activity"
  if [ "$waited" = "1" ]; then record "${id}_writer_wait" 1 1 PASS; else record "${id}_writer_wait" 1 "$waited" FAIL; fi
  mid=$(snap_now)
  if [ "$mid" = "$INITIAL" ]; then record "${id}_no_commit_while_reader_holds" "$INITIAL" "$mid" PASS; else record "${id}_no_commit_while_reader_holds" "$INITIAL" "$mid" FAIL; fi
  finish_bg "${id}_reader" >/dev/null
  finish_bg "${id}_writer" >/dev/null
  expect_state "${id}_reader" OK
  expect_state "${id}_writer" "$wstate"
  expect_snap "$id" "$final"
}

writer_first() {
  local id="$1" writer="$2" rstate="$3" rline="$4" final="$5" held waited
  reset_fixture "$id" || return
  start_bg "${id}_writer" writer_probe "$writer"
  held=$(writer_sleeping)
  if [ "$held" = "1" ]; then record "${id}_writer_held" 1 1 PASS; else record "${id}_writer_held" 1 "$held" FAIL; fi
  start_bg "${id}_reader" reader_probe reader_read.sql -v execution="$EXEC" -v shipment="$SHIP"
  waited=$(role_waiting reader_probe)
  activity "${id}_activity"
  if [ "$waited" = "1" ]; then record "${id}_reader_wait" 1 1 PASS; else record "${id}_reader_wait" 1 "$waited" FAIL; fi
  finish_bg "${id}_writer" >/dev/null
  finish_bg "${id}_reader" >/dev/null
  expect_state "${id}_writer" OK
  expect_state "${id}_reader" "$rstate"
  if [ "$rstate" = "OK" ]; then
    expect_line "${id}_reader" "$rline"
  fi
  expect_snap "$id" "$final"
}

# Flag off, reader first, occupied slot.
# Plain SELECTs do not conflict with FOR SHARE. A duplicate link INSERT against a
# committed row is expected to return 23505. If PostgreSQL waits instead, release
# the reader first so the writer can finish, and record the wait.
reset_fixture P0R
start_bg P0R_reader reader_probe reader_hold.sql -v execution="$EXEC" -v shipment="$SHIP" -v hold="$HOLD"
held=$(reader_sleeping)
if [ "$held" = "1" ]; then record P0R_reader_held 1 1 PASS; else record P0R_reader_held 1 "$held" FAIL; fi
start_bg P0R_writer writer_probe writers/project_off_occupied.sql
observed=UNKNOWN
i=0
while [ "$i" -lt 20 ]; do
  if ! kill -0 "$(cat "$OUT/P0R_writer.pid")" 2>/dev/null; then
    observed=NO_WAIT
    break
  fi
  waited=$(psql_admin -c "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'writer_probe' AND wait_event_type = 'Lock') THEN 1 ELSE 0 END;" | tr -d '[:space:]')
  if [ "$waited" = "1" ]; then
    observed=WAIT
    break
  fi
  i=$((i + 1))
  sleep 0.2
done
activity P0R_activity
if [ "$observed" = "WAIT" ]; then
  finish_bg P0R_reader >/dev/null
  finish_bg P0R_writer >/dev/null
elif [ "$observed" = "NO_WAIT" ]; then
  finish_bg P0R_writer >/dev/null
  still=$(psql_admin -c "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'reader_probe' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;" | tr -d '[:space:]')
  if [ "$still" = "1" ]; then record P0R_reader_open_after_writer 1 1 PASS; else record P0R_reader_open_after_writer 1 "$still" FAIL; fi
  finish_bg P0R_reader >/dev/null
else
  record P0R_writer_progress exited_or_waiting "$observed" FAIL
  finish_bg P0R_reader >/dev/null
  finish_bg P0R_writer >/dev/null
fi
record P0R_writer_lock_wait "NO_WAIT_OR_WAIT" "$observed" "$([ "$observed" = "UNKNOWN" ] && printf FAIL || printf PASS)"
expect_state P0R_reader OK
expect_state P0R_writer 23505
expect_snap P0R "$INITIAL"
absent=$(psql_admin -c "SELECT count(*) FROM transport.transport_executions WHERE id = '15151515-1515-1515-1515-151515151515';" | tr -d '[:space:]')
if [ "$absent" = "0" ]; then record P0R_rolled_back_execution 0 0 PASS; else record P0R_rolled_back_execution 0 "$absent" FAIL; fi
rc=$(psql_f P0R_retry writer_probe writers/project_off_free.sql)
expect_state P0R_retry OK
expect_snap P0R_retry "$FREE_OFF"

# Flag off, writer first on the occupied slot: the committed row rejects the insert without a lock wait.
reset_fixture P0W
start=$(date +%s)
rc=$(psql_f P0W_writer writer_probe writers/project_off_occupied.sql)
end=$(date +%s)
expect_state P0W_writer 23505
expect_snap P0W "$INITIAL"
if [ $((end - start)) -le 5 ]; then record P0W_no_lock_wait "<=5s" "$((end - start))s" PASS; else record P0W_no_lock_wait "<=5s" "$((end - start))s" FAIL; fi

# Flag off, writer first on a free slot: reader of the old binding does not wait.
reset_fixture P0W_FREE
start_bg P0W_FREE_writer writer_probe writers/project_off_free_hold.sql
held=$(writer_sleeping)
if [ "$held" = "1" ]; then record P0W_FREE_writer_held 1 1 PASS; else record P0W_FREE_writer_held 1 "$held" FAIL; fi
start_bg P0W_FREE_reader reader_probe reader_read.sql -v execution="$EXEC" -v shipment="$SHIP"
finished=$(wait_true "SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'reader_probe' AND pid <> pg_backend_pid()) THEN 1 ELSE 0 END;")
still=$(psql_admin -c "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'writer_probe' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;" | tr -d '[:space:]')
activity P0W_FREE_activity
if [ "$finished" = "1" ] && [ "$still" = "1" ]; then record P0W_FREE_reader_did_not_wait 1 "finished:${finished};writer_sleep:${still}" PASS; else record P0W_FREE_reader_did_not_wait 1 "finished:${finished};writer_sleep:${still}" FAIL; fi
finish_bg P0W_FREE_reader >/dev/null
expect_state P0W_FREE_reader OK
expect_line P0W_FREE_reader "$BASE_LINE"
finish_bg P0W_FREE_writer >/dev/null
expect_state P0W_FREE_writer OK
expect_snap P0W_FREE "$FREE_OFF"

# Flag on, reader first, occupied slot. Writer waits and must not lock the existing execution before the insert.
reader_first P1R writers/project_on_occupied.sql 23505 "$INITIAL"
absent=$(psql_admin -c "SELECT count(*) FROM transport.transport_executions WHERE id = '13131313-1313-1313-1313-131313131313';" | tr -d '[:space:]')
if [ "$absent" = "0" ]; then record P1R_rolled_back_execution 0 0 PASS; else record P1R_rolled_back_execution 0 "$absent" FAIL; fi

# Flag on, writer first, occupied slot. Sleep is before the new execution INSERT.
reset_fixture P1W
start_bg P1W_writer writer_probe writers/project_on_occupied_hold.sql
held=$(writer_sleeping)
if [ "$held" = "1" ]; then record P1W_writer_held 1 1 PASS; else record P1W_writer_held 1 "$held" FAIL; fi
exec_locks=$(writer_execution_locks)
printf '%s\n' "$exec_locks" > "$OUT/P1W_execution_locks.stdout"
if [ "$exec_locks" = "0" ]; then record P1W_no_existing_execution_lock 0 0 PASS; else record P1W_no_existing_execution_lock 0 "$exec_locks" FAIL; fi
start_bg P1W_reader reader_probe reader_read.sql -v execution="$EXEC" -v shipment="$SHIP"
waited=$(role_waiting reader_probe)
activity P1W_activity
if [ "$waited" = "1" ]; then record P1W_reader_wait 1 1 PASS; else record P1W_reader_wait 1 "$waited" FAIL; fi
finish_bg P1W_writer >/dev/null
finish_bg P1W_reader >/dev/null
expect_state P1W_writer 23505
expect_state P1W_reader OK
expect_line P1W_reader "$BASE_LINE"
expect_snap P1W "$INITIAL"

# Flag on, reader first, free slot. Shared driver waits. New execution is a different id.
reset_fixture P1R_FREE
start_bg P1R_FREE_reader reader_probe reader_hold.sql -v execution="$EXEC" -v shipment="$SHIP" -v hold="$HOLD"
held=$(reader_sleeping)
if [ "$held" = "1" ]; then record P1R_FREE_reader_held 1 1 PASS; else record P1R_FREE_reader_held 1 "$held" FAIL; fi
start_bg P1R_FREE_writer writer_probe writers/project_on_free.sql
waited=$(role_waiting writer_probe)
activity P1R_FREE_activity
exec_locks=$(psql_admin -c "SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'writer_probe' AND a.wait_event_type = 'Lock' AND c.relname = 'transport_executions';" | tr -d '[:space:]')
printf '%s\n' "$exec_locks" > "$OUT/P1R_FREE_execution_locks.stdout"
if [ "$waited" = "1" ]; then record P1R_FREE_writer_wait 1 1 PASS; else record P1R_FREE_writer_wait 1 "$waited" FAIL; fi
if [ "$exec_locks" = "0" ]; then record P1R_FREE_no_existing_execution_lock 0 0 PASS; else record P1R_FREE_no_existing_execution_lock 0 "$exec_locks" FAIL; fi
finish_bg P1R_FREE_reader >/dev/null
finish_bg P1R_FREE_writer >/dev/null
expect_state P1R_FREE_writer OK
expect_snap P1R_FREE "$FREE_ON"

# Flag on, writer first, free slot. Sleep before INSERT. Reader waits on the shared driver.
reset_fixture P1W_FREE
start_bg P1W_FREE_writer writer_probe writers/project_on_free_hold.sql
held=$(writer_sleeping)
exec_locks=$(writer_execution_locks)
printf '%s\n' "$exec_locks" > "$OUT/P1W_FREE_execution_locks.stdout"
if [ "$held" = "1" ]; then record P1W_FREE_writer_held 1 1 PASS; else record P1W_FREE_writer_held 1 "$held" FAIL; fi
if [ "$exec_locks" = "0" ]; then record P1W_FREE_no_existing_execution_lock 0 0 PASS; else record P1W_FREE_no_existing_execution_lock 0 "$exec_locks" FAIL; fi
start_bg P1W_FREE_reader reader_probe reader_read.sql -v execution="$EXEC" -v shipment="$SHIP"
waited=$(role_waiting reader_probe)
activity P1W_FREE_activity
if [ "$waited" = "1" ]; then record P1W_FREE_reader_wait 1 1 PASS; else record P1W_FREE_reader_wait 1 "$waited" FAIL; fi
finish_bg P1W_FREE_writer >/dev/null
finish_bg P1W_FREE_reader >/dev/null
expect_state P1W_FREE_writer OK
expect_state P1W_FREE_reader OK
expect_line P1W_FREE_reader "$BASE_LINE"
expect_snap P1W_FREE "$FREE_ON"

reader_first AD_R writers/assign_driver.sql OK "$SNAP_DRIVER"
writer_first AD_W writers/assign_driver_hold.sql OK "$DRIVER_LINE" "$SNAP_DRIVER"

reader_first AV_R writers/assign_vehicle.sql OK "$SNAP_VEHICLE"
writer_first AV_W writers/assign_vehicle_hold.sql OK "$VEHICLE_LINE" "$SNAP_VEHICLE"

reader_first DS_R writers/driver_stop.sql OK "$SNAP_STOP"
writer_first DS_W writers/driver_stop_hold.sql OK "$STOP_LINE" "$SNAP_STOP"
# Re-check the plain link read from the saved writer-first session: modes were captured while it slept.
# The hold session has finished, so rerun the lock observation inside DS_W before finish by using the activity already stored.
# A dedicated probe records the modes while the writer is still in pg_sleep.
reset_fixture DS_LINK
start_bg DS_LINK_writer writer_probe writers/driver_stop_hold.sql
held=$(writer_sleeping)
modes=$(link_lock_modes)
printf '%s\n' "$modes" > "$OUT/DS_LINK_modes.stdout"
tuples=$(psql_admin -c "SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'writer_probe' AND a.query LIKE '%pg_sleep%' AND c.relname = 'transport_execution_active_shipments' AND l.locktype = 'tuple';" | tr -d '[:space:]')
printf '%s\n' "$tuples" > "$OUT/DS_LINK_tuples.stdout"
if [ "$held" = "1" ] && [ "$modes" = "AccessShareLock" ] && [ "$tuples" = "0" ]; then
  record DS_LINK_plain_select_is_not_a_row_lock "AccessShareLock/0" "${modes}/${tuples}" PASS
else
  record DS_LINK_plain_select_is_not_a_row_lock "AccessShareLock/0" "held:${held};modes:${modes};tuples:${tuples}" FAIL
fi
finish_bg DS_LINK_writer >/dev/null

reader_first SU_R writers/successor.sql OK "$SNAP_SUCCESSOR"
writer_first SU_W writers/successor_hold.sql OK "$SUCCESSOR_LINE" "$SNAP_SUCCESSOR"

reader_first SM_R writers/shipment_status.sql OK "$SNAP_STATUS"
writer_first SM_W writers/shipment_status_hold.sql OK "$STATUS_LINE" "$SNAP_STATUS"

# Direct link, reader first: writer waits, cannot commit, then commits after the reader.
reader_first DL_R writers/direct_link.sql OK "$SNAP_LINK"
if grep -q 'reader_probe|Timeout|PgSleep|active|pg_sleep' "$OUT/DL_R_activity.stdout" && grep -q 'writer_probe|Lock|' "$OUT/DL_R_activity.stdout"; then
  record DL_R_reader_still_open_while_writer_waits present present PASS
else
  record DL_R_reader_still_open_while_writer_waits present "$(tr '\n' ' ' < "$OUT/DL_R_activity.stdout")" FAIL
fi
rc=$(psql_f DL_R_old_read reader_probe reader_read.sql -v execution="$EXEC" -v shipment="$SHIP")
expect_state DL_R_old_read 23514
rc=$(psql_f DL_R_new_read reader_probe reader_read.sql -v execution="$OTHER" -v shipment="$SHIP")
expect_state DL_R_new_read OK
expect_line DL_R_new_read "$OTHER_LINE"

# Direct link, writer first.
writer_first DL_W writers/direct_link_hold.sql 23514 "" "$SNAP_LINK"
rc=$(psql_f DL_W_new_read reader_probe reader_read.sql -v execution="$OTHER" -v shipment="$SHIP")
expect_state DL_W_new_read OK
expect_line DL_W_new_read "$OTHER_LINE"

# Lock timeout, rollback, unchanged binding, then retry after the reader releases.
reset_fixture DL_T
start_bg DL_T_reader reader_probe reader_hold.sql -v execution="$EXEC" -v shipment="$SHIP" -v hold="$HOLD"
held=$(reader_sleeping)
if [ "$held" = "1" ]; then record DL_T_reader_held 1 1 PASS; else record DL_T_reader_held 1 "$held" FAIL; fi
rc=$(psql_f DL_T_writer writer_probe writers/direct_link_timeout.sql)
expect_state DL_T_writer 55P03
still=$(psql_admin -c "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'reader_probe' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;" | tr -d '[:space:]')
if [ "$still" = "1" ]; then record DL_T_reader_open_after_timeout 1 1 PASS; else record DL_T_reader_open_after_timeout 1 "$still" FAIL; fi
expect_snap DL_T "$INITIAL"
finish_bg DL_T_reader >/dev/null
rc=$(psql_f DL_T_retry writer_probe writers/direct_link.sql)
expect_state DL_T_retry OK
expect_snap DL_T_retry "$SNAP_LINK"

if grep -R -n '40P01' "$OUT" --include='*.stderr' >/dev/null 2>&1; then
  grep -R -n '40P01' "$OUT" --include='*.stderr' > "$OUT/deadlock.txt" || true
  record deadlock_scan NO_DEADLOCK_IN_TESTED_INTERLEAVINGS FOUND FAIL
else
  printf 'NO_DEADLOCK_IN_TESTED_INTERLEAVINGS\n' > "$OUT/deadlock.txt"
  record deadlock_scan NO_DEADLOCK_IN_TESTED_INTERLEAVINGS NO_DEADLOCK_IN_TESTED_INTERLEAVINGS PASS
fi

printf 'passes=%s\nfails=%s\n' "$PASSES" "$FAILS" | tee "$OUT/suite_counts.txt"
if [ "$FAILS" -ne 0 ]; then
  exit 1
fi
exit 0
