#!/bin/bash
# Isolated assignment-integrity proof. Each psql invocation is its own session.
set -u
cd "$(dirname "$0")"
export PGUSER="${PGUSER:-proof_admin}"
export PGDATABASE="${PGDATABASE:-binding_assignment_integrity_0_1k}"
OUT="${OUT:-runs/attempt-1}"
mkdir -p "$OUT"
SUMMARY="$OUT/assertions.tsv"
PASSES=0
FAILS=0
printf 'test_id\texpected\tactual\tverdict\tstdout\tstderr\n' > "$SUMMARY"

SHIP='22222222-2222-2222-2222-222222222222'
EXEC='11111111-1111-1111-1111-111111111111'
HIST='99999999-9999-9999-9999-999999999999'
OTHER='abababab-abab-abab-abab-abababababab'
TENANT_EXEC='bcbcbcbc-bcbc-bcbc-bcbc-bcbcbcbcbcbc'
OTHER_SHIP='00000000-0000-0000-0000-000000000009'
TENANT_SHIP='00000000-0000-0000-0000-00000000000a'
ABSENT_EXEC='0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e01'
ABSENT_SHIP='0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e02'
NOREV_EXEC='0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e04'
NOREV_SHIP='0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e05'
SUPER_EXEC='0c0c0c0c-0c0c-0c0c-0c0c-0c0c0c0c0c01'
SUPER_SHIP='0c0c0c0c-0c0c-0c0c-0c0c-0c0c0c0c0c02'
ONLY_SHIP='00000000-0000-0000-0000-000000000006'
MISSING='00000000-0000-0000-0000-000000000003'
ROW="${EXEC}|4|55555555-5555-5555-5555-555555555555|7|cccccccc-cccc-cccc-cccc-cccccccccccc"
SUCCESSOR="${EXEC}|4|12121212-1212-1212-1212-121212121212|8|cccccccc-cccc-cccc-cccc-cccccccccccc"
EPOCH5="${EXEC}|5|55555555-5555-5555-5555-555555555555|7|cccccccc-cccc-cccc-cccc-cccccccccccc"
TENANT_ROW="${EXEC}|5|55555555-5555-5555-5555-555555555555|7|dddddddd-dddd-dddd-dddd-dddddddddddd"

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

data_lines() {
  tr -d '\r' < "$1" | sed -e '/^[[:space:]]*$/d' -e '/^BEGIN$/d' -e '/^COMMIT$/d' -e '/^ROLLBACK$/d' -e '/^SET$/d' -e '/^RESET$/d' -e '/^INSERT /d' -e '/^UPDATE /d' -e '/^DELETE /d' | paste -sd'/' -
}

one_line() {
  local file="$1" cleaned n
  if [ ! -s "$file" ]; then
    printf '__EMPTY__'
    return
  fi
  cleaned=$(tr -d '\r' < "$file" | sed -e '/^[[:space:]]*$/d' -e '/^SET$/d' -e '/^RESET$/d' -e '/^BEGIN$/d' -e '/^COMMIT$/d' -e '/^ROLLBACK$/d' -e '/^DO$/d' -e '/^INSERT /d' -e '/^UPDATE /d' -e '/^DELETE /d')
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
  local id="$1" role="$2" sql="$3" rc
  set +e
  psql -X -U "$role" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -v VERBOSITY=verbose -t -A -c "$sql" \
    >"$OUT/${id}.stdout" 2>"$OUT/${id}.stderr"
  rc=$?
  printf '%s' "$rc" > "$OUT/${id}.rc"
  printf '%s' "$rc"
}

psql_f() {
  local id="$1" role="$2" file="$3" rc
  shift 3
  set +e
  psql -X -U "$role" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -v VERBOSITY=verbose -t -A "$@" -f "$file" \
    >"$OUT/${id}.stdout" 2>"$OUT/${id}.stderr"
  rc=$?
  printf '%s' "$rc" > "$OUT/${id}.rc"
  printf '%s' "$rc"
}

expect_ok_line() {
  local id="$1" role="$2" expect="$3" sql="$4" rc got
  rc=$(psql_c "$id" "$role" "$sql")
  got=$(one_line "$OUT/${id}.stdout")
  if [ "$rc" -eq 0 ] && [ "$(sqlstate_of "$OUT/${id}.stderr")" = "OK" ] && [ "$got" = "$expect" ]; then
    record "$id" "$expect" "$got" PASS
  else
    record "$id" "$expect" "rc:${rc};state:$(sqlstate_of "$OUT/${id}.stderr");value:${got}" FAIL
  fi
}

expect_state() {
  local id="$1" role="$2" expect="$3" sql="$4" rc state
  rc=$(psql_c "$id" "$role" "$sql")
  state=$(sqlstate_of "$OUT/${id}.stderr")
  if [ "$rc" -ne 0 ] && [ "$state" = "$expect" ]; then
    record "$id" "$expect" "$state" PASS
  else
    record "$id" "$expect" "rc:${rc};state:${state}" FAIL
  fi
}

expect_file_line() {
  local id="$1" role="$2" expect="$3" file="$4" rc got
  shift 4
  rc=$(psql_f "$id" "$role" "$file" "$@")
  got=$(one_line "$OUT/${id}.stdout")
  if [ "$rc" -eq 0 ] && [ "$(sqlstate_of "$OUT/${id}.stderr")" = "OK" ] && [ "$got" = "$expect" ]; then
    record "$id" "$expect" "$got" PASS
  else
    record "$id" "$expect" "rc:${rc};state:$(sqlstate_of "$OUT/${id}.stderr");value:${got}" FAIL
  fi
}

expect_file_empty_state() {
  local id="$1" role="$2" expect="$3" file="$4" rc state got
  shift 4
  rc=$(psql_f "$id" "$role" "$file" "$@")
  state=$(sqlstate_of "$OUT/${id}.stderr")
  got=$(data_lines "$OUT/${id}.stdout")
  if [ "$rc" -ne 0 ] && [ "$state" = "$expect" ] && [ -z "$got" ]; then
    record "$id" "$expect" "$state" PASS
  else
    record "$id" "$expect" "rc:${rc};state:${state};value:${got}" FAIL
  fi
}

no_leak() {
  local id="$1" needle="$2"
  if grep -q "$needle" "$OUT/${id}.stdout"; then
    record "${id}_no_leak" absent present FAIL
  else
    record "${id}_no_leak" absent absent PASS
  fi
}

admin_c() {
  psql -X -q -tA -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -c "$1"
}

wait_true() {
  local sql="$1" i val
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

reset_main() {
  local tag="$1"
  psql -X -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -f reset_fixture.sql \
    >"$OUT/reset_${tag}.stdout" 2>"$OUT/reset_${tag}.stderr"
}

{
  printf '%s\n' '-- SELECT version();'
  psql -X -U "$PGUSER" -d "$PGDATABASE" -c 'SELECT version();'
  printf '%s\n' '-- SHOW server_version_num;'
  psql -X -U "$PGUSER" -d "$PGDATABASE" -c 'SHOW server_version_num;'
  psql --version
  psql -X -U "$PGUSER" -d "$PGDATABASE" -c 'SELECT current_database();'
  psql -X -U "$PGUSER" -d "$PGDATABASE" -c 'SELECT datname FROM pg_database ORDER BY datname;'
} > "$OUT/E01_version.stdout" 2> "$OUT/E01_version.stderr" || true

db_name=$(admin_c 'SELECT current_database();' | tr -d '[:space:]')
if [ "$db_name" = "binding_assignment_integrity_0_1k" ]; then
  record E01_database binding_assignment_integrity_0_1k "$db_name" PASS
else
  record E01_database binding_assignment_integrity_0_1k "${db_name:-EMPTY}" FAIL
  printf 'refusing to continue in database %s\n' "$db_name" >&2
  exit 2
fi
version_num=$(admin_c 'SHOW server_version_num;' | tr -d '[:space:]')
if [ "$version_num" -ge 160000 ] 2>/dev/null && [ "$version_num" -lt 170000 ]; then
  record E01_server_version_num 16 "$version_num" PASS
else
  record E01_server_version_num 16 "${version_num:-EMPTY}" FAIL
  exit 2
fi
db_list=$(admin_c "SELECT string_agg(datname, ',' ORDER BY datname) FROM pg_database;" | tr -d '[:space:]')
if [ "$db_list" = "binding_assignment_integrity_0_1k,postgres,template0,template1" ]; then
  record E01_databases no_freight_platform "$db_list" PASS
else
  record E01_databases no_freight_platform "${db_list:-EMPTY}" FAIL
  exit 2
fi

psql -X -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 -f setup.sql >"$OUT/E00_setup.stdout" 2>"$OUT/E00_setup.stderr"
setup_rc=$?
if [ "$setup_rc" -eq 0 ]; then
  record E00_setup OK OK PASS
else
  record E00_setup OK "rc:${setup_rc}" FAIL
  printf 'passes=%s fails=%s\n' "$PASSES" "$FAILS" | tee "$OUT/suite_counts.txt"
  exit 1
fi

psql -X -U "$PGUSER" -d "$PGDATABASE" -c "SELECT pg_get_functiondef('transport.read_binding_assignment(uuid,uuid)'::regprocedure);" \
  >"$OUT/E02_function_definition.stdout" 2>"$OUT/E02_function_definition.stderr" || true
expect_ok_line E02_owner_and_search_path proof_admin 'assignment_lock_owner|search_path=pg_catalog' \
  "SELECT r.rolname || '|' || coalesce(p.proconfig[1], 'none') FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace JOIN pg_roles r ON r.oid = p.proowner WHERE n.nspname = 'transport' AND p.proname = 'read_binding_assignment';"
expect_ok_line E02_no_dynamic_sql proof_admin NO_DYNAMIC_SQL \
  "SELECT CASE WHEN pg_get_functiondef('transport.read_binding_assignment(uuid,uuid)'::regprocedure) LIKE '%RETURN QUERY%' AND pg_get_functiondef('transport.read_binding_assignment(uuid,uuid)'::regprocedure) NOT LIKE '%EXECUTE %' AND pg_get_functiondef('transport.read_binding_assignment(uuid,uuid)'::regprocedure) NOT LIKE '%format(%' THEN 'NO_DYNAMIC_SQL' ELSE 'UNEXPECTED' END;"
expect_ok_line E02_active_slot_only proof_admin ACTIVE_LINK_ONLY \
  "SELECT CASE WHEN pg_get_functiondef('transport.read_binding_assignment(uuid,uuid)'::regprocedure) LIKE '%transport_execution_active_shipments%' AND pg_get_functiondef('transport.read_binding_assignment(uuid,uuid)'::regprocedure) NOT LIKE '%participants%' THEN 'ACTIVE_LINK_ONLY' ELSE 'UNEXPECTED' END;"
expect_ok_line E02_execute_privileges proof_admin 'false|false|true' \
  "SELECT has_function_privilege('public', 'transport.read_binding_assignment(uuid,uuid)', 'EXECUTE')::text || '|' || has_function_privilege('tracking_runtime', 'transport.read_binding_assignment(uuid,uuid)', 'EXECUTE')::text || '|' || has_function_privilege('binding_authority_fence', 'transport.read_binding_assignment(uuid,uuid)', 'EXECUTE')::text;"

expect_state P01_column_select_for_share column_probe 42501 \
  "SELECT id::text FROM transport.shipments WHERE id = '${SHIP}' FOR SHARE;"
expect_state P02_runtime_execute runtime_probe 42501 \
  "SELECT shipment_id::text FROM transport.read_binding_assignment('${EXEC}', '${SHIP}');"
expect_state P03_runtime_set_role runtime_probe 42501 \
  "SET ROLE assignment_lock_owner;"
expect_state P04_runtime_update_updated_at runtime_probe 42501 \
  "UPDATE transport.shipments SET updated_at = clock_timestamp() WHERE id = '${SHIP}';"
expect_state P05_runtime_update_driver_id runtime_probe 42501 \
  "UPDATE transport.shipments SET driver_id = '77777777-7777-7777-7777-777777777777' WHERE id = '${SHIP}';"
expect_state P06_runtime_update_owner_epoch runtime_probe 42501 \
  "UPDATE transport.transport_executions SET owner_epoch = owner_epoch + 1 WHERE id = '${EXEC}';"
expect_state P07_fence_direct_select fence_probe 42501 \
  "SELECT id::text FROM transport.shipments WHERE id = '${SHIP}';"
expect_state P08_owner_update_driver_id proof_admin 42501 \
  "SET ROLE assignment_lock_owner; UPDATE transport.shipments SET driver_id = '77777777-7777-7777-7777-777777777777' WHERE id = '${SHIP}';"
expect_state P09_owner_update_owner_epoch proof_admin 42501 \
  "SET ROLE assignment_lock_owner; UPDATE transport.transport_executions SET owner_epoch = owner_epoch + 1 WHERE id = '${EXEC}';"
expect_ok_line P10_owner_update_updated_at proof_admin OK \
  "SET ROLE assignment_lock_owner; UPDATE transport.shipments SET updated_at = '2020-01-01+00' WHERE id = '${SHIP}'; SELECT 'OK';"
expect_ok_line P10_updated_at_changed proof_admin CHANGED \
  "SELECT CASE WHEN updated_at = '2020-01-01+00' THEN 'CHANGED' ELSE 'UNCHANGED' END FROM transport.shipments WHERE id = '${SHIP}';"

admin_c "GRANT assignment_lock_owner TO set_probe WITH INHERIT FALSE, SET TRUE;" >/dev/null
expect_state P11_set_without_role set_probe 42501 \
  "UPDATE transport.shipments SET updated_at = '2022-01-01+00' WHERE id = '${SHIP}';"
expect_ok_line P11_set_role_updates set_probe OK \
  "SET ROLE assignment_lock_owner; UPDATE transport.shipments SET updated_at = '2022-01-01+00' WHERE id = '${SHIP}'; SELECT 'OK';"
admin_c "REVOKE assignment_lock_owner FROM set_probe;" >/dev/null
expect_state P11_set_role_after_revoke set_probe 42501 \
  "SET ROLE assignment_lock_owner;"

admin_c "GRANT assignment_lock_owner TO inherit_probe WITH INHERIT TRUE, SET FALSE;" >/dev/null
expect_ok_line P12_inherit_updates inherit_probe OK \
  "UPDATE transport.shipments SET updated_at = '2021-01-01+00' WHERE id = '${SHIP}'; SELECT 'OK';"
admin_c "REVOKE assignment_lock_owner FROM inherit_probe;" >/dev/null
expect_state P12_inherit_after_revoke inherit_probe 42501 \
  "UPDATE transport.shipments SET updated_at = clock_timestamp() WHERE id = '${SHIP}';"
expect_ok_line P13_no_membership proof_admin 0 \
  "SELECT count(*)::text FROM pg_auth_members m JOIN pg_roles r ON r.oid = m.roleid JOIN pg_roles u ON u.oid = m.member WHERE r.rolname = 'assignment_lock_owner' AND u.rolname IN ('binding_authority_fence', 'tracking_runtime', 'fence_probe', 'runtime_probe');"
reset_main privileges

expect_file_line I01_matching_execution fence_probe "$ROW" identity_read.sql -v execution="$EXEC" -v shipment="$SHIP"
no_leak I01_matching_execution "$HIST"
expect_file_empty_state I02_other_shipment_execution fence_probe 23514 identity_read.sql -v execution="$OTHER" -v shipment="$SHIP"
no_leak I02_other_shipment_execution "$OTHER"
no_leak I02_other_shipment_execution "$EXEC"
expect_file_empty_state I03_other_tenant_execution fence_probe 23514 identity_read.sql -v execution="$TENANT_EXEC" -v shipment="$SHIP"
no_leak I03_other_tenant_execution "$TENANT_EXEC"
no_leak I03_other_tenant_execution "dddddddd-dddd-dddd-dddd-dddddddddddd"
expect_file_empty_state I04_historical_execution fence_probe 23514 identity_read.sql -v execution="$HIST" -v shipment="$SHIP"
no_leak I04_historical_execution "$HIST"
no_leak I04_historical_execution "$EXEC"
expect_file_empty_state I05_missing_execution fence_probe P0002 identity_read.sql -v execution="$MISSING" -v shipment="$SHIP"
expect_file_line I06_baseline_returns_linked_execution fence_probe "$EXEC" baseline_read.sql -v execution="$HIST" -v shipment="$SHIP"
no_leak I06_baseline_returns_linked_execution "$HIST"

expect_file_empty_state L01_superseded_pointer fence_probe 23514 identity_read.sql -v execution="$SUPER_EXEC" -v shipment="$SUPER_SHIP"
expect_file_empty_state L02_absent_current_revision fence_probe P0002 identity_read.sql -v execution="$NOREV_EXEC" -v shipment="$NOREV_SHIP"
expect_file_empty_state L03_second_active_link command_probe 23505 second_active_link.sql
expect_file_empty_state L04_second_active_revision command_probe 23505 second_active_revision.sql
expect_file_line L05_driver_only_without_link fence_probe null null_execution_read.sql

psql_f D01_driver_only_hold fence_probe driver_only_hold.sql >/dev/null &
d01=$!
d01_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;")
d01_locks=$(admin_c "SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'fence_probe' AND c.relname = 'transport_executions' AND l.mode = 'RowShareLock';" | tr -d '[:space:]')
d01_probe=$(psql_f D01_execution_not_locked command_probe concurrency_execution_probe.sql)
wait "$d01" || true
d01_out=$(data_lines "$OUT/D01_driver_only_hold.stdout")
if [ "$d01_held" = "1" ] && [ "${d01_locks:-0}" = "0" ] && [ "$d01_probe" -eq 0 ] && [ "$d01_out" = "${EXEC}|4" ]; then
  record D01_driver_only_does_not_lock_execution 'exec-visible-unlocked' "$d01_out" PASS
else
  record D01_driver_only_does_not_lock_execution 'exec-visible-unlocked' "held:${d01_held};locks:${d01_locks};probe:${d01_probe};value:${d01_out}" FAIL
fi

expect_file_empty_state E01_absent_epoch fence_probe 42501 identity_read.sql -v execution="$ABSENT_EXEC" -v shipment="$ABSENT_SHIP"
expect_file_line E02_matching_epoch fence_probe "$ROW" match_epoch.sql
expect_file_empty_state E03_stale_epoch_rolls_back fence_probe 40001 stale_epoch_consume.sql
expect_ok_line E03_decision_unchanged proof_admin 'ISSUED|null' \
  "SELECT use_status || '|' || CASE WHEN consumed_at IS NULL THEN 'null' ELSE 'set' END FROM transport.binding_authority_decisions WHERE decision_id = '66666666-6666-6666-6666-666666666675';"
expect_ok_line E03_epoch_unchanged proof_admin 4 \
  "SELECT owner_epoch::text FROM transport.transport_executions WHERE id = '${EXEC}';"
expect_file_empty_state E04_epoch_decrease command_probe 23514 decrease_epoch.sql
expect_ok_line E04_epoch_unchanged proof_admin 4 \
  "SELECT owner_epoch::text FROM transport.transport_executions WHERE id = '${EXEC}';"
expect_file_empty_state E05_tenant_change_without_epoch command_probe 23514 tenant_without_bump.sql
expect_ok_line E05_tenant_unchanged proof_admin 'cccccccc-cccc-cccc-cccc-cccccccccccc|4' \
  "SELECT operating_tenant_id::text || '|' || owner_epoch::text FROM transport.transport_executions WHERE id = '${EXEC}';"

psql_f C01_revision_writer command_probe concurrency_revision_writer.sql >/dev/null &
c01w=$!
c01_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'command_probe' AND a.query LIKE '%pg_sleep%' AND c.relname = 'transport_executions' AND l.mode = 'RowShareLock' AND l.granted) THEN 1 ELSE 0 END;")
psql_f C01_reader fence_probe identity_read.sql -v execution="$EXEC" -v shipment="$SHIP" >/dev/null &
c01r=$!
c01_wait=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND pid <> pg_backend_pid() AND wait_event_type = 'Lock') THEN 1 ELSE 0 END;")
admin_c "SELECT a.usename || '|' || coalesce(a.wait_event_type, 'none') || '|' || coalesce(a.wait_event, 'none') || '|' || a.state FROM pg_stat_activity a WHERE a.usename IN ('command_probe', 'fence_probe') AND a.pid <> pg_backend_pid();" \
  >"$OUT/C01_activity.stdout" 2>"$OUT/C01_activity.stderr" || true
wait "$c01w" || true
wait "$c01r" || true
c01_row=$(one_line "$OUT/C01_reader.stdout")
c01_rc=$(tr -d '[:space:]' < "$OUT/C01_reader.rc")
if [ "$c01_held" = "1" ] && [ "$c01_wait" = "1" ] && [ "$c01_rc" = "0" ] && [ "$c01_row" = "$SUCCESSOR" ]; then
  record C01_revision_waits_then_successor 'successor-after-commit' "$c01_row" PASS
else
  record C01_revision_waits_then_successor 'successor-after-commit' "held:${c01_held};wait:${c01_wait};rc:${c01_rc};row:${c01_row}" FAIL
fi
reset_main c01

psql_f C02_link_writer command_probe concurrency_link_writer.sql >/dev/null &
c02w=$!
c02_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'command_probe' AND a.query LIKE '%pg_sleep%' AND c.relname = 'transport_executions' AND l.mode = 'RowShareLock' AND l.granted) THEN 1 ELSE 0 END;")
psql_f C02_reader fence_probe identity_read.sql -v execution="$EXEC" -v shipment="$SHIP" >/dev/null &
c02r=$!
c02_wait=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND pid <> pg_backend_pid() AND wait_event_type = 'Lock') THEN 1 ELSE 0 END;")
admin_c "SELECT a.usename || '|' || coalesce(a.wait_event_type, 'none') || '|' || coalesce(a.wait_event, 'none') || '|' || a.state FROM pg_stat_activity a WHERE a.usename IN ('command_probe', 'fence_probe') AND a.pid <> pg_backend_pid();" \
  >"$OUT/C02_activity.stdout" 2>"$OUT/C02_activity.stderr" || true
wait "$c02w" || true
wait "$c02r" || true
c02_state=$(sqlstate_of "$OUT/C02_reader.stderr")
c02_got=$(data_lines "$OUT/C02_reader.stdout")
if [ "$c02_held" = "1" ] && [ "$c02_wait" = "1" ] && [ "$c02_state" = "23514" ] && [ -z "$c02_got" ]; then
  record C02_compatible_link_reassignment 23514 "$c02_state" PASS
else
  record C02_compatible_link_reassignment 23514 "held:${c02_held};wait:${c02_wait};state:${c02_state};value:${c02_got}" FAIL
fi
no_leak C02_reader "$OTHER"
reset_main c02

psql_f C03_hold fence_probe concurrency_hold.sql >/dev/null &
c03h=$!
c03_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;")
c03_rc=$(psql_f C03_plain_link command_probe concurrency_link_plain.sql)
c03_still=$(admin_c "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;" | tr -d '[:space:]')
wait "$c03h" || true
c03_hold_row=$(one_line "$OUT/C03_hold.stdout")
if [ "$c03_held" = "1" ] && [ "$c03_rc" = "0" ] && [ "$c03_still" = "1" ] && [ "$c03_hold_row" = "$ROW" ]; then
  record C03_incompatible_link_writer_not_fenced 'no-wait' "$c03_hold_row" PASS
else
  record C03_incompatible_link_writer_not_fenced 'no-wait' "held:${c03_held};rc:${c03_rc};still:${c03_still};row:${c03_hold_row}" FAIL
fi
expect_file_empty_state C03_reread_after_plain_link fence_probe 23514 identity_read.sql -v execution="$EXEC" -v shipment="$SHIP"
reset_main c03

psql_f C04_epoch_writer command_probe concurrency_epoch_writer.sql >/dev/null &
c04w=$!
c04_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'command_probe' AND a.query LIKE '%pg_sleep%' AND c.relname = 'transport_executions' AND l.mode = 'RowShareLock' AND l.granted) THEN 1 ELSE 0 END;")
psql_f C04_reader fence_probe identity_read.sql -v execution="$EXEC" -v shipment="$SHIP" >/dev/null &
c04r=$!
c04_wait=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND pid <> pg_backend_pid() AND wait_event_type = 'Lock') THEN 1 ELSE 0 END;")
admin_c "SELECT a.usename || '|' || coalesce(a.wait_event_type, 'none') || '|' || coalesce(a.wait_event, 'none') || '|' || a.state FROM pg_stat_activity a WHERE a.usename IN ('command_probe', 'fence_probe') AND a.pid <> pg_backend_pid();" \
  >"$OUT/C04_activity.stdout" 2>"$OUT/C04_activity.stderr" || true
wait "$c04w" || true
wait "$c04r" || true
c04_row=$(one_line "$OUT/C04_reader.stdout")
c04_rc=$(tr -d '[:space:]' < "$OUT/C04_reader.rc")
if [ "$c04_held" = "1" ] && [ "$c04_wait" = "1" ] && [ "$c04_rc" = "0" ] && [ "$c04_row" = "$EPOCH5" ]; then
  record C04_epoch_increment_visible_after_commit 'epoch-5' "$c04_row" PASS
else
  record C04_epoch_increment_visible_after_commit 'epoch-5' "held:${c04_held};wait:${c04_wait};rc:${c04_rc};row:${c04_row}" FAIL
fi
expect_file_empty_state C04_old_epoch_rejected fence_probe 40001 stale_epoch_consume.sql
reset_main c04

psql_f C05_command command_probe concurrency_command.sql >/dev/null &
c05w=$!
c05_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'command_probe' AND a.query LIKE '%pg_sleep%' AND c.relname = 'transport_executions' AND l.mode = 'RowShareLock' AND l.granted AND NOT EXISTS (SELECT 1 FROM pg_locks l2 JOIN pg_class c2 ON c2.oid = l2.relation WHERE l2.pid = a.pid AND c2.relname = 'shipments' AND l2.mode = 'RowShareLock')) THEN 1 ELSE 0 END;")
psql_f C05_reader fence_probe identity_read.sql -v execution="$EXEC" -v shipment="$SHIP" >/dev/null &
c05r=$!
c05_wait=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND pid <> pg_backend_pid() AND wait_event_type = 'Lock') THEN 1 ELSE 0 END;")
c05_ship=$(admin_c "SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'fence_probe' AND c.relname = 'shipments' AND l.mode = 'RowShareLock' AND l.granted;" | tr -d '[:space:]')
admin_c "SELECT a.usename || '|' || c.relname || '|' || l.mode || '|' || l.granted::text FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename IN ('command_probe', 'fence_probe') AND c.relname IN ('shipments', 'transport_executions') ORDER BY 1;" \
  >"$OUT/C05_locks.stdout" 2>"$OUT/C05_locks.stderr" || true
wait "$c05w" || true
wait "$c05r" || true
c05_row=$(one_line "$OUT/C05_reader.stdout")
c05_rc=$(tr -d '[:space:]' < "$OUT/C05_reader.rc")
c05_dead=0
if grep -q '40P01' "$OUT/C05_command.stderr" "$OUT/C05_reader.stderr"; then
  c05_dead=1
fi
if [ "$c05_held" = "1" ] && [ "$c05_wait" = "1" ] && [ "${c05_ship:-0}" = "0" ] && [ "$c05_rc" = "0" ] && [ "$c05_row" = "$ROW" ] && [ "$c05_dead" = "0" ]; then
  record C05_execution_before_shipment 'wait-then-row' "$c05_row" PASS
else
  record C05_execution_before_shipment 'wait-then-row' "held:${c05_held};wait:${c05_wait};ship:${c05_ship};rc:${c05_rc};dead:${c05_dead};row:${c05_row}" FAIL
fi

psql_f C06_hold fence_probe concurrency_hold.sql >/dev/null &
c06h=$!
c06_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;")
c06_rc=$(psql_f C06_lock_timeout writer_probe concurrency_lock_timeout.sql)
c06_state=$(sqlstate_of "$OUT/C06_lock_timeout.stderr")
wait "$c06h" || true
c06_version=$(admin_c "SELECT version::text FROM transport.shipments WHERE id = '${SHIP}';" | tr -d '[:space:]')
if [ "$c06_held" = "1" ] && [ "$c06_rc" != "0" ] && [ "$c06_state" = "55P03" ] && [ "$c06_version" = "3" ]; then
  record C06_lock_timeout_rollback 55P03 "$c06_state" PASS
else
  record C06_lock_timeout_rollback 55P03 "held:${c06_held};rc:${c06_rc};state:${c06_state};version:${c06_version}" FAIL
fi

e06_rc=$(psql_f E06_tenant_change_with_epoch command_probe tenant_with_bump.sql)
e06_state=$(sqlstate_of "$OUT/E06_tenant_change_with_epoch.stderr")
if [ "$e06_rc" = "0" ] && [ "$e06_state" = "OK" ]; then
  record E06_tenant_change_with_epoch OK OK PASS
else
  record E06_tenant_change_with_epoch OK "rc:${e06_rc};state:${e06_state}" FAIL
fi
expect_file_line E06_reread_after_tenant_commit fence_probe "$TENANT_ROW" identity_read.sql -v execution="$EXEC" -v shipment="$SHIP"
reset_main e06
expect_file_line E06_restored_after_reset fence_probe "$ROW" identity_read.sql -v execution="$EXEC" -v shipment="$SHIP"

printf 'passes=%s fails=%s\n' "$PASSES" "$FAILS" | tee "$OUT/suite_counts.txt"
if [ "$FAILS" -ne 0 ]; then
  exit 1
fi
