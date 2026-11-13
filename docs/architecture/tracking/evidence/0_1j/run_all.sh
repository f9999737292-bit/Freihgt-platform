#!/bin/bash
# Isolated full-function proof. Each invocation is its own psql process.
# An expected SQLSTATE does not abort the suite.
set -u
cd "$(dirname "$0")"
export PGUSER="${PGUSER:-proof_admin}"
export PGDATABASE="${PGDATABASE:-binding_full_function_proof_0_1j}"
OUT="${OUT:-runs/attempt-1}"
mkdir -p "$OUT"
SUMMARY="$OUT/assertions.tsv"
PASSES=0
FAILS=0
printf 'test_id\texpected\tactual\tverdict\tstdout\tstderr\n' > "$SUMMARY"

SHIP='22222222-2222-2222-2222-222222222222'
EXEC='11111111-1111-1111-1111-111111111111'
FULL_ROW='22222222-2222-2222-2222-222222222222|aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa|33333333-3333-3333-3333-333333333333|44444444-4444-4444-4444-444444444444|3|false|ACTIVE|11111111-1111-1111-1111-111111111111|cccccccc-cccc-cccc-cccc-cccccccccccc|33333333-3333-3333-3333-333333333333|44444444-4444-4444-4444-444444444444|55555555-5555-5555-5555-555555555555|7'
ASSIGN_ROW='aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa|33333333-3333-3333-3333-333333333333|44444444-4444-4444-4444-444444444444|11111111-1111-1111-1111-111111111111|55555555-5555-5555-5555-555555555555|7|cccccccc-cccc-cccc-cccc-cccccccccccc'

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
  local errf="$1"
  local state
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

data_lines() {
  tr -d '\r' < "$1" | sed -e '/^[[:space:]]*$/d' -e '/^BEGIN$/d' -e '/^COMMIT$/d' -e '/^ROLLBACK$/d' -e '/^SET$/d' -e '/^RESET$/d' -e '/^INSERT /d' -e '/^UPDATE /d' -e '/^DELETE /d' | paste -sd'/' -
}

one_line() {
  local file="$1"
  local cleaned n
  if [ ! -s "$file" ]; then
    printf '__EMPTY__'
    return
  fi
  cleaned=$(tr -d '\r' < "$file" | sed -e '/^[[:space:]]*$/d' -e '/^SET$/d' -e '/^RESET$/d' -e '/^BEGIN$/d' -e '/^COMMIT$/d' -e '/^ROLLBACK$/d')
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

psql_c() {
  local id="$1" role="$2" sql="$3"
  local rc
  set +e
  psql -X -U "$role" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -v VERBOSITY=verbose -t -A -c "$sql" \
    >"$OUT/${id}.stdout" 2>"$OUT/${id}.stderr"
  rc=$?
  printf '%s' "$rc" > "$OUT/${id}.rc"
  printf '%s' "$rc"
}

psql_f() {
  local id="$1" role="$2" file="$3"
  shift 3
  local rc
  set +e
  psql -X -U "$role" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -v VERBOSITY=verbose -t -A "$@" -f "$file" \
    >"$OUT/${id}.stdout" 2>"$OUT/${id}.stderr"
  rc=$?
  printf '%s' "$rc" > "$OUT/${id}.rc"
  printf '%s' "$rc"
}

expect_ok_line() {
  local id="$1" role="$2" expect="$3" sql="$4"
  local rc got
  rc=$(psql_c "$id" "$role" "$sql")
  got=$(one_line "$OUT/${id}.stdout")
  if [ "$rc" -eq 0 ] && [ "$(sqlstate_of "$OUT/${id}.stderr")" = "OK" ] && [ "$got" = "$expect" ]; then
    record "$id" "$expect" "$got" PASS
  else
    record "$id" "$expect" "rc:${rc};state:$(sqlstate_of "$OUT/${id}.stderr");value:${got}" FAIL
  fi
}

expect_state() {
  local id="$1" role="$2" expect="$3" sql="$4"
  local rc state
  rc=$(psql_c "$id" "$role" "$sql")
  state=$(sqlstate_of "$OUT/${id}.stderr")
  if [ "$rc" -ne 0 ] && [ "$state" = "$expect" ]; then
    record "$id" "$expect" "$state" PASS
  else
    record "$id" "$expect" "rc:${rc};state:${state}" FAIL
  fi
}

expect_file_state() {
  local id="$1" role="$2" expect="$3" file="$4"
  shift 4
  local rc state
  rc=$(psql_f "$id" "$role" "$file" "$@")
  state=$(sqlstate_of "$OUT/${id}.stderr")
  if [ "$rc" -ne 0 ] && [ "$state" = "$expect" ]; then
    record "$id" "$expect" "$state" PASS
  else
    record "$id" "$expect" "rc:${rc};state:${state}" FAIL
  fi
}

admin_c() {
  psql -X -q -tA -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -c "$1"
}

wait_true() {
  local sql="$1"
  local i val
  for i in $(seq 1 48); do
    val=$(admin_c "$sql" | tr -d '[:space:]')
    if [ "$val" = "1" ]; then
      printf '1'
      return
    fi
    sleep 0.25
  done
  printf '0'
}

{
  printf '%s\n' '-- SELECT version();'
  psql -X -U "$PGUSER" -d "$PGDATABASE" -c 'SELECT version();'
  printf '%s\n' '-- SHOW server_version_num;'
  psql -X -U "$PGUSER" -d "$PGDATABASE" -c 'SHOW server_version_num;'
  psql --version
  psql -X -U "$PGUSER" -d "$PGDATABASE" -c 'SELECT current_database();'
  psql -X -U "$PGUSER" -d "$PGDATABASE" -c 'SELECT datname FROM pg_database ORDER BY 1;'
} > "$OUT/E01_version.stdout" 2> "$OUT/E01_version.stderr" || true

db_name=$(admin_c 'SELECT current_database();' | tr -d '[:space:]')
if [ "$db_name" = "binding_full_function_proof_0_1j" ]; then
  record E01_database binding_full_function_proof_0_1j "$db_name" PASS
else
  record E01_database binding_full_function_proof_0_1j "${db_name:-EMPTY}" FAIL
  printf 'refusing to continue in database %s\n' "$db_name" >&2
  exit 2
fi
version_num=$(admin_c 'SHOW server_version_num;' | tr -d '[:space:]')
if [ "$version_num" -ge 160000 ] 2>/dev/null && [ "$version_num" -lt 170000 ]; then
  record E01_server_version_num 16 "$version_num" PASS
else
  record E01_server_version_num 16 "${version_num:-UNKNOWN}" FAIL
  exit 2
fi
datnames=$(admin_c "SELECT string_agg(datname, ',' ORDER BY datname) FROM pg_database;")
if [ "$datnames" = "binding_full_function_proof_0_1j,postgres,template0,template1" ]; then
  record E01_databases no_freight_platform "$datnames" PASS
else
  record E01_databases no_freight_platform "$datnames" FAIL
  exit 2
fi

psql -X -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -f setup.sql > "$OUT/E00_setup.stdout" 2> "$OUT/E00_setup.stderr"
setup_rc=$?
if [ "$setup_rc" -eq 0 ]; then
  record E00_setup OK OK PASS
else
  record E00_setup OK "exit:${setup_rc}" FAIL
  printf 'passes=%s fails=%s\n' "$PASSES" "$FAILS" | tee "$OUT/suite_counts.txt"
  exit 1
fi

psql -X -U "$PGUSER" -d "$PGDATABASE" -c "SELECT pg_get_functiondef('transport.read_binding_assignment(uuid,uuid)'::regprocedure);" \
  > "$OUT/E02_function_definition.stdout" 2> "$OUT/E02_function_definition.stderr"

expect_ok_line E02_owner_and_search_path proof_admin 'assignment_lock_owner|search_path=pg_catalog' \
  "SELECT r.rolname || '|' || coalesce(p.proconfig[1], 'none') FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace JOIN pg_roles r ON r.oid = p.proowner WHERE n.nspname = 'transport' AND p.proname = 'read_binding_assignment';"
expect_ok_line E02_no_dynamic_sql proof_admin NO_DYNAMIC_SQL \
  "SELECT CASE WHEN pg_get_functiondef('transport.read_binding_assignment(uuid,uuid)'::regprocedure) LIKE '%RETURN QUERY%' AND pg_get_functiondef('transport.read_binding_assignment(uuid,uuid)'::regprocedure) NOT LIKE '%EXECUTE %' AND pg_get_functiondef('transport.read_binding_assignment(uuid,uuid)'::regprocedure) NOT LIKE '%format(%' THEN 'NO_DYNAMIC_SQL' ELSE 'UNEXPECTED' END;"
expect_ok_line E02_execute_privileges proof_admin 'false|false|true' \
  "SELECT has_function_privilege('public', 'transport.read_binding_assignment(uuid,uuid)', 'EXECUTE')::text || '|' || has_function_privilege('tracking_runtime', 'transport.read_binding_assignment(uuid,uuid)', 'EXECUTE')::text || '|' || has_function_privilege('binding_authority_fence', 'transport.read_binding_assignment(uuid,uuid)', 'EXECUTE')::text;"

expect_state P01_column_select_for_share column_probe 42501 \
  "SELECT id::text FROM transport.shipments WHERE id = '${SHIP}' FOR SHARE;"
expect_state P02_table_select_for_share table_probe 42501 \
  "SELECT id::text FROM transport.shipments WHERE id = '${SHIP}' FOR SHARE;"
expect_state P07_runtime_execute runtime_probe 42501 \
  "SELECT shipment_id::text FROM transport.read_binding_assignment('${EXEC}', '${SHIP}');"
expect_state P08_runtime_set_role runtime_probe 42501 \
  "SET ROLE assignment_lock_owner;"
expect_state P09_runtime_update_updated_at runtime_probe 42501 \
  "UPDATE transport.shipments SET updated_at = clock_timestamp() WHERE id = '${SHIP}';"
expect_state P10_runtime_update_driver_id runtime_probe 42501 \
  "UPDATE transport.shipments SET driver_id = '77777777-7777-7777-7777-777777777777' WHERE id = '${SHIP}';"
expect_state P11_runtime_update_version runtime_probe 42501 \
  "UPDATE transport.shipments SET version = version + 1 WHERE id = '${SHIP}';"
expect_state P12_fence_update_updated_at fence_probe 42501 \
  "UPDATE transport.shipments SET updated_at = clock_timestamp() WHERE id = '${SHIP}';"
expect_state P13_fence_update_driver_id fence_probe 42501 \
  "UPDATE transport.shipments SET driver_id = '77777777-7777-7777-7777-777777777777' WHERE id = '${SHIP}';"
expect_state P14_fence_set_role fence_probe 42501 \
  "SET ROLE assignment_lock_owner;"
expect_state P15_fence_direct_select fence_probe 42501 \
  "SELECT id::text FROM transport.shipments WHERE id = '${SHIP}';"

expect_ok_line A06_return_query fence_probe "$FULL_ROW" \
  "SELECT shipment_id::text || '|' || shipment_tenant_id::text || '|' || driver_id::text || '|' || coalesce(vehicle_id::text, 'null') || '|' || shipment_version::text || '|' || shipment_deleted::text || '|' || coalesce(driver_status, 'null') || '|' || coalesce(execution_id::text, 'null') || '|' || coalesce(operating_tenant_id::text, 'null') || '|' || coalesce(execution_driver_id::text, 'null') || '|' || coalesce(execution_vehicle_id::text, 'null') || '|' || coalesce(current_revision_id::text, 'null') || '|' || coalesce(revision_version::text, 'null') FROM transport.read_binding_assignment('${EXEC}', '${SHIP}');"

expect_state A01_missing_shipment fence_probe P0002 \
  "SELECT shipment_id::text FROM transport.read_binding_assignment(NULL, '00000000-0000-0000-0000-000000000001');"
expect_state A02_deleted_shipment fence_probe P0002 \
  "SELECT shipment_id::text FROM transport.read_binding_assignment(NULL, '00000000-0000-0000-0000-000000000002');"
expect_state A03_missing_execution fence_probe P0002 \
  "SELECT shipment_id::text FROM transport.read_binding_assignment('00000000-0000-0000-0000-000000000003', '${SHIP}');"
expect_state A04_missing_driver fence_probe P0002 \
  "SELECT shipment_id::text FROM transport.read_binding_assignment(NULL, '00000000-0000-0000-0000-000000000004');"
expect_state A05_ambiguous_assignment fence_probe 21000 \
  "SELECT shipment_id::text FROM transport.read_binding_assignment('${EXEC}', '00000000-0000-0000-0000-000000000005');"

admin_c "REVOKE SELECT (driver_id) ON transport.shipments FROM assignment_lock_owner;" >/dev/null
expect_state A17_missing_column_select_fails_closed fence_probe 42501 \
  "SELECT shipment_id::text FROM transport.read_binding_assignment('${EXEC}', '${SHIP}');"
admin_c "GRANT SELECT (driver_id) ON transport.shipments TO assignment_lock_owner;" >/dev/null
expect_ok_line A17_column_select_restored fence_probe "$FULL_ROW" \
  "SELECT shipment_id::text || '|' || shipment_tenant_id::text || '|' || driver_id::text || '|' || coalesce(vehicle_id::text, 'null') || '|' || shipment_version::text || '|' || shipment_deleted::text || '|' || coalesce(driver_status, 'null') || '|' || coalesce(execution_id::text, 'null') || '|' || coalesce(operating_tenant_id::text, 'null') || '|' || coalesce(execution_driver_id::text, 'null') || '|' || coalesce(execution_vehicle_id::text, 'null') || '|' || coalesce(current_revision_id::text, 'null') || '|' || coalesce(revision_version::text, 'null') FROM transport.read_binding_assignment('${EXEC}', '${SHIP}');"

admin_c "GRANT assignment_lock_owner TO set_probe WITH INHERIT FALSE, SET TRUE;" >/dev/null
expect_state P21_set_without_role set_probe 42501 \
  "UPDATE transport.shipments SET updated_at = '2020-01-01+00' WHERE id = '${SHIP}';"
expect_state P18_owner_update_driver_id set_probe 42501 \
  "BEGIN; SET ROLE assignment_lock_owner; UPDATE transport.shipments SET driver_id = '77777777-7777-7777-7777-777777777777' WHERE id = '${SHIP}'; ROLLBACK;"
rc=$(psql_c P17_owner_update_updated_at set_probe "SET ROLE assignment_lock_owner; UPDATE transport.shipments SET updated_at = '2020-01-01+00' WHERE id = '${SHIP}'; RESET ROLE;")
if [ "$rc" -eq 0 ] && [ "$(sqlstate_of "$OUT/P17_owner_update_updated_at.stderr")" = "OK" ]; then
  record P17_owner_update_updated_at OK OK PASS
else
  record P17_owner_update_updated_at OK "rc:${rc};state:$(sqlstate_of "$OUT/P17_owner_update_updated_at.stderr")" FAIL
fi
expect_ok_line P17_updated_at_changed proof_admin CHANGED \
  "SELECT CASE WHEN updated_at = '2020-01-01+00' THEN 'CHANGED' ELSE 'UNCHANGED' END FROM transport.shipments WHERE id = '${SHIP}';"
admin_c "REVOKE assignment_lock_owner FROM set_probe;" >/dev/null
expect_state P21_set_role_after_revoke set_probe 42501 \
  "SET ROLE assignment_lock_owner;"
psql -X -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -f reset_fixture.sql > "$OUT/P17_reset.stdout" 2> "$OUT/P17_reset.stderr"

admin_c "GRANT assignment_lock_owner TO inherit_probe WITH INHERIT TRUE, SET FALSE;" >/dev/null
rc=$(psql_c P20_inherit_updates_updated_at inherit_probe "UPDATE transport.shipments SET updated_at = '2021-01-01+00' WHERE id = '${SHIP}';")
if [ "$rc" -eq 0 ] && [ "$(sqlstate_of "$OUT/P20_inherit_updates_updated_at.stderr")" = "OK" ]; then
  record P20_inherit_updates_updated_at OK OK PASS
else
  record P20_inherit_updates_updated_at OK "rc:${rc};state:$(sqlstate_of "$OUT/P20_inherit_updates_updated_at.stderr")" FAIL
fi
expect_ok_line P20_updated_at_changed proof_admin CHANGED \
  "SELECT CASE WHEN updated_at = '2021-01-01+00' THEN 'CHANGED' ELSE 'UNCHANGED' END FROM transport.shipments WHERE id = '${SHIP}';"
admin_c "REVOKE assignment_lock_owner FROM inherit_probe;" >/dev/null
expect_state P20_inherit_after_revoke inherit_probe 42501 \
  "UPDATE transport.shipments SET updated_at = '2022-01-01+00' WHERE id = '${SHIP}';"
expect_ok_line P19_no_membership proof_admin '0' \
  "SELECT count(*)::text FROM pg_auth_members m JOIN pg_roles r ON r.oid = m.roleid JOIN pg_roles u ON u.oid = m.member WHERE r.rolname = 'assignment_lock_owner' AND u.rolname IN ('binding_authority_fence', 'tracking_runtime', 'fence_probe', 'runtime_probe');"
psql -X -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -f reset_fixture.sql > "$OUT/P20_reset.stdout" 2> "$OUT/P20_reset.stderr"

expect_file_state A10_tenant_mismatch fence_probe 23514 assignment_compare.sql -v decision=66666666-6666-6666-6666-666666666671
expect_file_state A11_driver_mismatch fence_probe 23514 assignment_compare.sql -v decision=66666666-6666-6666-6666-666666666672
expect_file_state A12_vehicle_mismatch fence_probe 23514 assignment_compare.sql -v decision=66666666-6666-6666-6666-666666666673
expect_file_state A13_absent_owner_epoch fence_probe 42501 assignment_compare.sql -v decision=66666666-6666-6666-6666-666666666674
expect_file_state A14_stale_owner_epoch fence_probe 40001 assignment_compare.sql -v decision=66666666-6666-6666-6666-666666666675

psql_f A15_driver_only_consume fence_probe driver_only_consume.sql >/dev/null &
a15_pid=$!
a15_sleep=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;")
a15_probe=$(psql_f A15_execution_not_locked command_probe concurrency_execution_probe.sql)
set +e
wait "$a15_pid"
rc=$?
set +e
got=$(data_lines "$OUT/A15_driver_only_consume.stdout")
if [ "$a15_sleep" = "1" ] && [ "$a15_probe" -eq 0 ] && [ "$rc" -eq 0 ] && [ "$got" = "00000000-0000-0000-0000-000000000006|33333333-3333-3333-3333-333333333333|null/0" ]; then
  record A15_driver_only_consume 'row/0-and-execution-free' "$got" PASS
else
  record A15_driver_only_consume 'row/0-and-execution-free' "sleep:${a15_sleep};probe:${a15_probe};rc:${rc};value:${got}" FAIL
fi
expect_ok_line A15_driver_only_committed proof_admin 'CONSUMED|null|33333333-3333-3333-3333-333333333333' \
  "SELECT d.use_status || '|' || coalesce(b.execution_id::text, 'null') || '|' || b.driver_id::text FROM transport.binding_authority_decisions d JOIN tracking.shipment_tracking_binding b ON b.binding_id = d.binding_id WHERE d.decision_id = '66666666-6666-6666-6666-666666666676';"

rc=$(psql_f A16_existing_link_reauthorization fence_probe driver_only_existing_link.sql)
state=$(sqlstate_of "$OUT/A16_existing_link_reauthorization.stderr")
link_out=$(data_lines "$OUT/A16_existing_link_reauthorization.stdout")
if [ "$rc" -ne 0 ] && [ "$state" = "40001" ] && [ "$link_out" = "11111111-1111-1111-1111-111111111111/0" ]; then
  record A16_existing_link_reauthorization 'exec/0+40001' "${link_out};${state}" PASS
else
  record A16_existing_link_reauthorization 'exec/0+40001' "rc:${rc};state:${state};value:${link_out}" FAIL
fi
expect_ok_line A16_decision_unchanged proof_admin 'ISSUED|null' \
  "SELECT use_status || '|' || CASE WHEN consumed_at IS NULL THEN 'null' ELSE 'set' END FROM transport.binding_authority_decisions WHERE decision_id = '66666666-6666-6666-6666-666666666679';"

# C01. Command locks the execution first. Consume must wait there and must not lock the shipment yet.
psql_f C01_command command_probe concurrency_command_execution_first.sql >/dev/null &
c01_cmd=$!
c01_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'command_probe' AND a.query LIKE '%pg_sleep%' AND c.relname = 'transport_executions' AND l.mode = 'RowShareLock' AND l.granted AND NOT EXISTS (SELECT 1 FROM pg_locks l2 JOIN pg_class c2 ON c2.oid = l2.relation WHERE l2.pid = a.pid AND c2.relname = 'shipments' AND l2.mode = 'RowShareLock')) THEN 1 ELSE 0 END;")
psql_f C01_consume fence_probe concurrency_consume_function.sql >/dev/null &
c01_con=$!
c01_wait=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND pid <> pg_backend_pid() AND wait_event_type = 'Lock') THEN 1 ELSE 0 END;")
admin_c "SELECT a.usename || '|' || c.relname || '|' || l.mode || '|' || l.granted::text FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename IN ('command_probe', 'fence_probe') AND c.relname IN ('shipments', 'transport_executions', 'drivers', 'vehicles') ORDER BY 1;" \
  > "$OUT/C01_locks.stdout" 2> "$OUT/C01_locks.stderr" || true
c01_ship=$(admin_c "SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'fence_probe' AND c.relname = 'shipments' AND l.mode = 'RowShareLock' AND l.granted;" | tr -d '[:space:]')
wait "$c01_cmd" || true
set +e
wait "$c01_con"
c01_con_rc=$?
set +e
c01_row=$(one_line "$OUT/C01_consume.stdout")
c01_dead=0
if grep -q '40P01' "$OUT/C01_command.stderr" "$OUT/C01_consume.stderr"; then
  c01_dead=1
fi
if [ "$c01_held" = "1" ] && [ "$c01_wait" = "1" ] && [ "${c01_ship:-0}" = "0" ] && [ "$c01_con_rc" -eq 0 ] && [ "$c01_row" = "${SHIP}|33333333-3333-3333-3333-333333333333|44444444-4444-4444-4444-444444444444|${EXEC}|55555555-5555-5555-5555-555555555555|7" ] && [ "${c01_dead:-0}" = "0" ]; then
  record C01_execution_before_shipment 'wait-then-row' "$c01_row" PASS
else
  record C01_execution_before_shipment 'wait-then-row' "held:${c01_held};wait:${c01_wait};ship:${c01_ship};rc:${c01_con_rc};dead:${c01_dead};row:${c01_row}" FAIL
fi

# C02. AssignDriver holds the shipment. The function waits, then sees the new driver and does not consume.
psql_f C02_assign_driver writer_probe concurrency_assign_driver.sql >/dev/null &
c02_w=$!
c02_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'writer_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;")
psql_f C02_consume fence_probe concurrency_consume_function.sql >/dev/null &
c02_c=$!
c02_wait=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND pid <> pg_backend_pid() AND wait_event_type = 'Lock') THEN 1 ELSE 0 END;")
admin_c "SELECT a.usename || '|' || coalesce(a.wait_event_type, 'none') || '|' || coalesce(a.wait_event, 'none') || '|' || a.state FROM pg_stat_activity a WHERE a.usename IN ('writer_probe', 'fence_probe') AND a.pid <> pg_backend_pid();" \
  > "$OUT/C02_activity.stdout" 2> "$OUT/C02_activity.stderr" || true
wait "$c02_w" || true
set +e
wait "$c02_c"
c02_rc=$?
set +e
c02_row=$(one_line "$OUT/C02_consume.stdout")
if [ "$c02_held" = "1" ] && [ "$c02_wait" = "1" ] && [ "$c02_rc" -eq 0 ] && [ "$c02_row" = "${SHIP}|77777777-7777-7777-7777-777777777777|44444444-4444-4444-4444-444444444444|${EXEC}|55555555-5555-5555-5555-555555555555|7" ]; then
  record C02_assign_driver_waits 'new-driver' "$c02_row" PASS
else
  record C02_assign_driver_waits 'new-driver' "held:${c02_held};wait:${c02_wait};rc:${c02_rc};row:${c02_row}" FAIL
fi
expect_file_state C02_old_driver_not_consumed fence_probe 23514 assignment_compare.sql -v decision=66666666-6666-6666-6666-666666666666
psql -X -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -f reset_fixture.sql > "$OUT/C02_reset.stdout" 2> "$OUT/C02_reset.stderr"

# C03. AssignVehicle.
psql_f C03_assign_vehicle writer_probe concurrency_assign_vehicle.sql >/dev/null &
c03_w=$!
c03_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'writer_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;")
psql_f C03_consume fence_probe concurrency_consume_function.sql >/dev/null &
c03_c=$!
c03_wait=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND pid <> pg_backend_pid() AND wait_event_type = 'Lock') THEN 1 ELSE 0 END;")
wait "$c03_w" || true
set +e
wait "$c03_c"
c03_rc=$?
set +e
c03_row=$(one_line "$OUT/C03_consume.stdout")
if [ "$c03_held" = "1" ] && [ "$c03_wait" = "1" ] && [ "$c03_rc" -eq 0 ] && [ "$c03_row" = "${SHIP}|33333333-3333-3333-3333-333333333333|88888888-8888-8888-8888-888888888888|${EXEC}|55555555-5555-5555-5555-555555555555|7" ]; then
  record C03_assign_vehicle_waits 'new-vehicle' "$c03_row" PASS
else
  record C03_assign_vehicle_waits 'new-vehicle' "held:${c03_held};wait:${c03_wait};rc:${c03_rc};row:${c03_row}" FAIL
fi
expect_file_state C03_old_vehicle_not_consumed fence_probe 23514 assignment_compare.sql -v decision=66666666-6666-6666-6666-666666666666
psql -X -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -f reset_fixture.sql > "$OUT/C03_reset.stdout" 2> "$OUT/C03_reset.stderr"

# C04. Successor revision locks the execution, then changes current_revision_id.
psql_f C04_successor command_probe concurrency_successor.sql >/dev/null &
c04_w=$!
c04_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'command_probe' AND a.query LIKE '%pg_sleep%' AND c.relname = 'transport_executions' AND l.mode = 'RowShareLock' AND l.granted) THEN 1 ELSE 0 END;")
psql_f C04_consume fence_probe concurrency_consume_function.sql >/dev/null &
c04_c=$!
c04_wait=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND pid <> pg_backend_pid() AND wait_event_type = 'Lock') THEN 1 ELSE 0 END;")
wait "$c04_w" || true
set +e
wait "$c04_c"
c04_rc=$?
set +e
c04_row=$(one_line "$OUT/C04_consume.stdout")
if [ "$c04_held" = "1" ] && [ "$c04_wait" = "1" ] && [ "$c04_rc" -eq 0 ] && [ "$c04_row" = "${SHIP}|33333333-3333-3333-3333-333333333333|44444444-4444-4444-4444-444444444444|${EXEC}|12121212-1212-1212-1212-121212121212|8" ]; then
  record C04_successor_revision 'new-revision' "$c04_row" PASS
else
  record C04_successor_revision 'new-revision' "held:${c04_held};wait:${c04_wait};rc:${c04_rc};row:${c04_row}" FAIL
fi
expect_file_state C04_old_revision_not_consumed fence_probe 40001 assignment_compare.sql -v decision=66666666-6666-6666-6666-666666666666
psql -X -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -f reset_fixture.sql > "$OUT/C04_reset.stdout" 2> "$OUT/C04_reset.stderr"

# C05. A driver-only read holds no execution lock. A link committed during the hold is visible to the re-read.
psql_f C05_hold fence_probe concurrency_driver_only_hold.sql >/dev/null &
c05_h=$!
c05_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;")
c05_exec_locks=$(admin_c "SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'fence_probe' AND c.relname = 'transport_executions' AND l.mode = 'RowShareLock';" | tr -d '[:space:]')
c05_probe=$(psql_f C05_execution_not_locked command_probe concurrency_execution_probe.sql)
c05_link_rc=$(psql_f C05_link proof_admin concurrency_link_insert.sql)
set +e
wait "$c05_h"
c05_rc=$?
set +e
c05_out=$(data_lines "$OUT/C05_hold.stdout")
if [ "$c05_held" = "1" ] && [ "${c05_exec_locks:-0}" = "0" ] && [ "$c05_probe" -eq 0 ] && [ "$c05_link_rc" -eq 0 ] && [ "$c05_rc" -eq 0 ] && [ "$c05_out" = "null/${EXEC}/0" ]; then
  record C05_link_appears_during_driver_only 'null/exec/0' "$c05_out" PASS
else
  record C05_link_appears_during_driver_only 'null/exec/0' "held:${c05_held};locks:${c05_exec_locks};probe:${c05_probe};link:${c05_link_rc};rc:${c05_rc};value:${c05_out}" FAIL
fi
psql -X -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -f reset_fixture.sql > "$OUT/C05_reset.stdout" 2> "$OUT/C05_reset.stderr"

# C06. Share held by the function blocks an assignment UPDATE until lock_timeout.
psql_f C06_hold fence_probe concurrency_share_hold.sql >/dev/null &
c06_h=$!
c06_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'fence_probe' AND a.query LIKE '%pg_sleep%' AND c.relname = 'shipments' AND l.mode = 'RowShareLock' AND l.granted) THEN 1 ELSE 0 END;")
rc=$(psql_f C06_lock_timeout writer_probe concurrency_lock_timeout.sql)
state=$(sqlstate_of "$OUT/C06_lock_timeout.stderr")
set +e
wait "$c06_h"
set +e
version_now=$(admin_c "SELECT version::text FROM transport.shipments WHERE id = '${SHIP}';" | tr -d '[:space:]')
if [ "$c06_held" = "1" ] && [ "$rc" -ne 0 ] && [ "$state" = "55P03" ] && [ "$version_now" = "3" ]; then
  record C06_lock_timeout 55P03 "$state" PASS
else
  record C06_lock_timeout 55P03 "held:${c06_held};rc:${rc};state:${state};version:${version_now}" FAIL
fi

# C09. After the function transaction commits, another connection does not keep the share.
rc=$(psql_f C09_other_connection writer_probe concurrency_other_connection.sql)
if [ "$rc" -eq 0 ] && [ "$(sqlstate_of "$OUT/C09_other_connection.stderr")" = "OK" ]; then
  record C09_lock_not_held_across_connections OK OK PASS
else
  record C09_lock_not_held_across_connections OK "rc:${rc};state:$(sqlstate_of "$OUT/C09_other_connection.stderr")" FAIL
fi
version_now=$(admin_c "SELECT version::text FROM transport.shipments WHERE id = '${SHIP}';" | tr -d '[:space:]')
if [ "$version_now" = "3" ]; then
  record C09_version_unchanged 3 "$version_now" PASS
else
  record C09_version_unchanged 3 "$version_now" FAIL
fi

# C08. Function call and decision update share one transaction. Rollback removes both effects.
rc=$(psql_f C08_same_transaction fence_probe concurrency_same_transaction.sql)
same_out=$(data_lines "$OUT/C08_same_transaction.stdout")
if [ "$rc" -eq 0 ] && [ "$same_out" = "${SHIP}/4" ]; then
  record C08_same_transaction_locks "${SHIP}/4" "$same_out" PASS
else
  record C08_same_transaction_locks "${SHIP}/4" "rc:${rc};value:${same_out}" FAIL
fi
expect_ok_line C08_rollback_restores_issued proof_admin 'ISSUED|null' \
  "SELECT use_status || '|' || CASE WHEN consumed_at IS NULL THEN 'null' ELSE 'set' END FROM transport.binding_authority_decisions WHERE decision_id = '66666666-6666-6666-6666-666666666666';"
expect_ok_line C08_no_binding proof_admin '0' \
  "SELECT count(*)::text FROM tracking.shipment_tracking_binding WHERE decision_id = '66666666-6666-6666-6666-666666666666';"

# C07. Shipment-first writer versus execution-first function. The cycle is a deadlock, and the consume transaction rolls back.
# deadlock_timeout is superuser-only on this server (42501). Apply it to new sessions, then reset it.
# The check runs once per wait. The consume timeout is longer than the shipment-first sleep,
# so the cycle exists when that one check runs. The shipment-first timeout is longer still.
admin_c "ALTER ROLE command_probe SET deadlock_timeout = '8s';" > "$OUT/C07_alter_command.stdout" 2> "$OUT/C07_alter_command.stderr"
admin_c "ALTER ROLE fence_probe SET deadlock_timeout = '5s';" > "$OUT/C07_alter_fence.stdout" 2> "$OUT/C07_alter_fence.stderr"
psql_f C07_shipment_first command_probe concurrency_deadlock_shipment_first.sql >/dev/null &
c07_a=$!
c07_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'command_probe' AND a.query LIKE '%pg_sleep%' AND c.relname = 'shipments' AND l.mode = 'RowShareLock' AND l.granted AND NOT EXISTS (SELECT 1 FROM pg_locks l2 JOIN pg_class c2 ON c2.oid = l2.relation WHERE l2.pid = a.pid AND c2.relname = 'transport_executions' AND l2.mode = 'RowShareLock')) THEN 1 ELSE 0 END;")
psql_f C07_consume fence_probe concurrency_deadlock_consume.sql >/dev/null &
c07_b=$!
set +e
wait "$c07_a" || true
wait "$c07_b" || true
c07_a_rc=$(tr -d '[:space:]' < "$OUT/C07_shipment_first.rc")
c07_b_rc=$(tr -d '[:space:]' < "$OUT/C07_consume.rc")
set +e
c07_state_a=$(sqlstate_of "$OUT/C07_shipment_first.stderr")
c07_state_b=$(sqlstate_of "$OUT/C07_consume.stderr")
c07_decision=$(admin_c "SELECT use_status || '|' || CASE WHEN consumed_at IS NULL THEN 'null' ELSE 'set' END FROM transport.binding_authority_decisions WHERE decision_id = '66666666-6666-6666-6666-666666666670';" | tr -d '[:space:]')
c07_bindings=$(admin_c "SELECT count(*)::text FROM tracking.shipment_tracking_binding WHERE decision_id = '66666666-6666-6666-6666-666666666670';" | tr -d '[:space:]')
if [ "$c07_held" = "1" ] && [ "$c07_state_b" = "40P01" ] && [ "$c07_b_rc" -ne 0 ] && [ "$c07_decision" = "ISSUED|null" ] && [ "$c07_bindings" = "0" ]; then
  record C07_deadlock_rollback 40P01 "$c07_state_b" PASS
else
  record C07_deadlock_rollback 40P01 "held:${c07_held};a:${c07_a_rc}:${c07_state_a};b:${c07_b_rc}:${c07_state_b};decision:${c07_decision};bindings:${c07_bindings}" FAIL
fi
admin_c "ALTER ROLE command_probe RESET deadlock_timeout;" > "$OUT/C07_reset_command.stdout" 2> "$OUT/C07_reset_command.stderr"
admin_c "ALTER ROLE fence_probe RESET deadlock_timeout;" > "$OUT/C07_reset_fence.stdout" 2> "$OUT/C07_reset_fence.stderr"

# C10. Terminating the session is the crash-before-commit stand-in.
psql_f C10_open fence_probe concurrency_crash_open.sql >/dev/null &
c10=$!
c10_open=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;")
c10_pid=$(admin_c "SELECT pid::text FROM pg_stat_activity WHERE usename = 'fence_probe' AND pid <> pg_backend_pid() AND state = 'active' ORDER BY pid LIMIT 1;" | tr -d '[:space:]')
if [ -n "$c10_pid" ]; then
  admin_c "SELECT pg_terminate_backend(${c10_pid});" > "$OUT/C10_terminate.stdout" 2> "$OUT/C10_terminate.stderr" || true
fi
set +e
wait "$c10"
set +e
c10_decision=$(admin_c "SELECT use_status || '|' || CASE WHEN consumed_at IS NULL THEN 'null' ELSE 'set' END FROM transport.binding_authority_decisions WHERE decision_id = '66666666-6666-6666-6666-666666666678';" | tr -d '[:space:]')
c10_bindings=$(admin_c "SELECT count(*)::text FROM tracking.shipment_tracking_binding WHERE binding_id = 'abababab-abab-abab-abab-abababababab';" | tr -d '[:space:]')
if [ "$c10_open" = "1" ] && [ "$c10_decision" = "ISSUED|null" ] && [ "$c10_bindings" = "0" ]; then
  record C10_session_terminate_rollback 'ISSUED|null' "$c10_decision" PASS
else
  record C10_session_terminate_rollback 'ISSUED|null' "open:${c10_open};pid:${c10_pid:-none};decision:${c10_decision};bindings:${c10_bindings}" FAIL
fi

printf 'passes=%s fails=%s\n' "$PASSES" "$FAILS" | tee "$OUT/suite_counts.txt"
if [ "$FAILS" -ne 0 ]; then
  exit 1
fi
exit 0
