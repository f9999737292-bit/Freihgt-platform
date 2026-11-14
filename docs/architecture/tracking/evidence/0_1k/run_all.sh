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
OTHER_ROW="${OTHER}|2|0a0a0a0a-0a0a-0a0a-0a0a-0a0a0a0a0a0a|1|cccccccc-cccc-cccc-cccc-cccccccccccc"
DRIVER_ROW='77777777-7777-7777-7777-777777777777|44444444-4444-4444-4444-444444444444|'"${EXEC}"
VEHICLE_ROW='33333333-3333-3333-3333-333333333333|88888888-8888-8888-8888-888888888888|'"${EXEC}"

classify_wait() {
  case "$1" in
    Lock) printf 'WAITING' ;;
    *) printf 'NOT_WAITING' ;;
  esac
}
if [ "$(classify_wait Lock)" != "WAITING" ] || [ "$(classify_wait Timeout)" != "NOT_WAITING" ] || [ "$(classify_wait PgSleep)" != "NOT_WAITING" ] || [ "$(classify_wait '')" != "NOT_WAITING" ]; then
  printf 'harness wait classifier failed\n' >&2
  exit 2
fi
unchanged_token=UNCHANGED
if [ "$unchanged_token" = "CHANGED" ]; then
  printf 'harness changed-token check failed\n' >&2
  exit 2
fi

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
expect_state P14_link_column_select_for_share column_probe 42501 \
  "SELECT execution_id::text FROM transport.transport_execution_active_shipments WHERE shipment_id = '${SHIP}' FOR SHARE;"
admin_c "REVOKE UPDATE (updated_at) ON transport.transport_execution_active_shipments FROM assignment_lock_owner;" >/dev/null
expect_file_empty_state P15_link_share_without_update fence_probe 42501 identity_read.sql -v execution="$EXEC" -v shipment="$SHIP"
admin_c "GRANT UPDATE (updated_at) ON transport.transport_execution_active_shipments TO assignment_lock_owner;" >/dev/null
expect_file_line P15_link_share_restored fence_probe "$ROW" identity_read.sql -v execution="$EXEC" -v shipment="$SHIP"
expect_state P16_owner_update_link_execution proof_admin 42501 \
  "SET ROLE assignment_lock_owner; UPDATE transport.transport_execution_active_shipments SET execution_id = '${OTHER}' WHERE shipment_id = '${SHIP}';"
expect_ok_line P16_owner_update_link_updated_at proof_admin OK \
  "SET ROLE assignment_lock_owner; UPDATE transport.transport_execution_active_shipments SET updated_at = '2024-01-01+00' WHERE shipment_id = '${SHIP}'; SELECT 'OK';"
expect_ok_line P16_link_updated_at_changed proof_admin CHANGED \
  "SELECT CASE WHEN updated_at = '2024-01-01+00' THEN 'CHANGED' ELSE 'UNCHANGED' END FROM transport.transport_execution_active_shipments WHERE shipment_id = '${SHIP}';"
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
expect_file_empty_state I07_missing_active_link fence_probe P0002 identity_read.sql -v execution="$EXEC" -v shipment="$ONLY_SHIP"
no_leak I07_missing_active_link "$EXEC"
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

link_id() {
  admin_c "SELECT execution_id::text FROM transport.transport_execution_active_shipments WHERE shipment_tenant_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa' AND shipment_id = '${SHIP}';" | tr -d '[:space:]'
}
psql_f C03_hold fence_probe concurrency_hold.sql >/dev/null &
c03h=$!
c03_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'fence_probe' AND a.state = 'active' AND a.query LIKE '%pg_sleep%' AND c.relname = 'transport_execution_active_shipments' AND l.mode = 'RowShareLock' AND l.granted) THEN 1 ELSE 0 END;")
c03_rc=$(psql_f C03_plain_link command_probe concurrency_link_plain.sql)
c03_state=$(sqlstate_of "$OUT/C03_plain_link.stderr")
c03_still=$(admin_c "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;" | tr -d '[:space:]')
c03_link=$(link_id)
wait "$c03h" || true
if [ "$c03_held" = "1" ] && [ "$c03_rc" != "0" ] && [ "$c03_state" = "55P03" ] && [ "$c03_still" = "1" ] && [ "$c03_link" = "$EXEC" ]; then
  record C03_plain_update_lock_timeout 55P03 "$c03_state" PASS
else
  record C03_plain_update_lock_timeout 55P03 "held:${c03_held};rc:${c03_rc};state:${c03_state};still:${c03_still};link:${c03_link}" FAIL
fi
expect_ok_line C03_link_unchanged_while_reader_open proof_admin "$EXEC" \
  "SELECT execution_id::text FROM transport.transport_execution_active_shipments WHERE shipment_id = '${SHIP}';"

psql_f C03b_hold fence_probe concurrency_hold.sql >/dev/null &
c03bh=$!
c03b_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename = 'fence_probe' AND a.state = 'active' AND a.query LIKE '%pg_sleep%' AND c.relname = 'transport_execution_active_shipments' AND l.mode = 'RowShareLock' AND l.granted) THEN 1 ELSE 0 END;")
psql_f C03b_writer command_probe concurrency_link_plain_wait.sql >/dev/null &
c03bw=$!
c03b_wait=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'command_probe' AND state = 'active' AND wait_event_type = 'Lock' AND query LIKE '%UPDATE transport.transport_execution_active_shipments%') THEN 1 ELSE 0 END;")
admin_c "SELECT usename || '|' || coalesce(wait_event_type, 'none') || '|' || coalesce(wait_event, 'none') || '|' || state FROM pg_stat_activity WHERE usename IN ('fence_probe', 'command_probe') AND pid <> pg_backend_pid() ORDER BY usename;" \
  >"$OUT/C03b_activity.stdout" 2>"$OUT/C03b_activity.stderr" || true
admin_c "SELECT a.usename || '|' || c.relname || '|' || l.mode || '|' || l.granted::text FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.usename IN ('fence_probe', 'command_probe') AND c.relname = 'transport_execution_active_shipments' ORDER BY 1;" \
  >"$OUT/C03b_locks.stdout" 2>"$OUT/C03b_locks.stderr" || true
c03b_event=$(admin_c "SELECT coalesce(wait_event_type, 'none') FROM pg_stat_activity WHERE usename = 'command_probe' AND pid <> pg_backend_pid() AND query LIKE '%transport_execution_active_shipments%' ORDER BY pid LIMIT 1;" | tr -d '[:space:]')
c03b_class=$(classify_wait "$c03b_event")
c03b_link_during=$(link_id)
wait "$c03bh" || true
wait "$c03bw" || true
c03b_writer_rc=$(tr -d '[:space:]' < "$OUT/C03b_writer.rc")
c03b_link_after=$(link_id)
if [ "$c03b_held" = "1" ] && [ "$c03b_wait" = "1" ] && [ "$c03b_class" = "WAITING" ] && [ "$c03b_link_during" = "$EXEC" ] && [ "$c03b_writer_rc" = "0" ] && [ "$c03b_link_after" = "$OTHER" ]; then
  record C03_plain_update_waits_then_commits WAITING "$c03b_event" PASS
else
  record C03_plain_update_waits_then_commits WAITING "held:${c03b_held};wait:${c03b_wait};class:${c03b_class};event:${c03b_event};during:${c03b_link_during};rc:${c03b_writer_rc};after:${c03b_link_after}" FAIL
fi
expect_file_empty_state C03_reread_old_execution fence_probe 23514 identity_read.sql -v execution="$EXEC" -v shipment="$SHIP"
no_leak C03_reread_old_execution "$OTHER"
expect_file_line C03_reread_new_execution fence_probe "$OTHER_ROW" identity_read.sql -v execution="$OTHER" -v shipment="$SHIP"
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

psql_f C07_assign_driver writer_probe concurrency_assign_driver.sql >/dev/null &
c07w=$!
c07_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'writer_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;")
psql_f C07_reader fence_probe assignment_fields.sql -v execution="$EXEC" -v shipment="$SHIP" >/dev/null &
c07r=$!
c07_wait=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND pid <> pg_backend_pid() AND wait_event_type = 'Lock') THEN 1 ELSE 0 END;")
admin_c "SELECT usename || '|' || coalesce(wait_event_type, 'none') || '|' || coalesce(wait_event, 'none') || '|' || state FROM pg_stat_activity WHERE usename IN ('writer_probe', 'fence_probe') AND pid <> pg_backend_pid();" \
  >"$OUT/C07_activity.stdout" 2>"$OUT/C07_activity.stderr" || true
wait "$c07w" || true
wait "$c07r" || true
c07_row=$(one_line "$OUT/C07_reader.stdout")
c07_rc=$(tr -d '[:space:]' < "$OUT/C07_reader.rc")
if [ "$c07_held" = "1" ] && [ "$c07_wait" = "1" ] && [ "$c07_rc" = "0" ] && [ "$c07_row" = "$DRIVER_ROW" ]; then
  record C07_assign_driver_waits 'new-driver' "$c07_row" PASS
else
  record C07_assign_driver_waits 'new-driver' "held:${c07_held};wait:${c07_wait};rc:${c07_rc};row:${c07_row}" FAIL
fi
reset_main c07

psql_f C08_assign_vehicle writer_probe concurrency_assign_vehicle.sql >/dev/null &
c08w=$!
c08_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'writer_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;")
psql_f C08_reader fence_probe assignment_fields.sql -v execution="$EXEC" -v shipment="$SHIP" >/dev/null &
c08r=$!
c08_wait=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'fence_probe' AND pid <> pg_backend_pid() AND wait_event_type = 'Lock') THEN 1 ELSE 0 END;")
admin_c "SELECT usename || '|' || coalesce(wait_event_type, 'none') || '|' || coalesce(wait_event, 'none') || '|' || state FROM pg_stat_activity WHERE usename IN ('writer_probe', 'fence_probe') AND pid <> pg_backend_pid();" \
  >"$OUT/C08_activity.stdout" 2>"$OUT/C08_activity.stderr" || true
wait "$c08w" || true
wait "$c08r" || true
c08_row=$(one_line "$OUT/C08_reader.stdout")
c08_rc=$(tr -d '[:space:]' < "$OUT/C08_reader.rc")
if [ "$c08_held" = "1" ] && [ "$c08_wait" = "1" ] && [ "$c08_rc" = "0" ] && [ "$c08_row" = "$VEHICLE_ROW" ]; then
  record C08_assign_vehicle_waits 'new-vehicle' "$c08_row" PASS
else
  record C08_assign_vehicle_waits 'new-vehicle' "held:${c08_held};wait:${c08_wait};rc:${c08_rc};row:${c08_row}" FAIL
fi
reset_main c08

admin_c "ALTER ROLE command_probe SET deadlock_timeout = '8s';" >"$OUT/C09_alter_command.stdout" 2>"$OUT/C09_alter_command.stderr"
admin_c "ALTER ROLE fence_probe SET deadlock_timeout = '6s';" >"$OUT/C09_alter_fence.stdout" 2>"$OUT/C09_alter_fence.stderr"
psql_f C09_shipment_first command_probe concurrency_shipment_first.sql >/dev/null &
c09w=$!
c09_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'command_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;")
psql_f C09_reader fence_probe identity_read.sql -v execution="$EXEC" -v shipment="$SHIP" >/dev/null &
c09r=$!
wait "$c09w" || true
wait "$c09r" || true
c09_writer_state=$(sqlstate_of "$OUT/C09_shipment_first.stderr")
c09_reader_state=$(sqlstate_of "$OUT/C09_reader.stderr")
c09_link=$(link_id)
if [ "$c09_held" = "1" ] && { [ "$c09_writer_state" = "40P01" ] || [ "$c09_reader_state" = "40P01" ]; } && [ "$c09_link" = "$EXEC" ]; then
  record C09_shipment_first_deadlock 40P01 "writer:${c09_writer_state};reader:${c09_reader_state}" PASS
else
  record C09_shipment_first_deadlock 40P01 "held:${c09_held};writer:${c09_writer_state};reader:${c09_reader_state};link:${c09_link}" FAIL
fi

psql_f C10_link_first command_probe concurrency_link_first.sql >/dev/null &
c10w=$!
c10_held=$(wait_true "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_stat_activity WHERE usename = 'command_probe' AND state = 'active' AND query LIKE '%pg_sleep%') THEN 1 ELSE 0 END;")
psql_f C10_reader fence_probe identity_read.sql -v execution="$EXEC" -v shipment="$SHIP" >/dev/null &
c10r=$!
wait "$c10w" || true
wait "$c10r" || true
c10_writer_state=$(sqlstate_of "$OUT/C10_link_first.stderr")
c10_reader_state=$(sqlstate_of "$OUT/C10_reader.stderr")
c10_link=$(link_id)
if [ "$c10_held" = "1" ] && { [ "$c10_writer_state" = "40P01" ] || [ "$c10_reader_state" = "40P01" ]; } && [ "$c10_link" = "$EXEC" ]; then
  record C10_link_first_deadlock 40P01 "writer:${c10_writer_state};reader:${c10_reader_state}" PASS
else
  record C10_link_first_deadlock 40P01 "held:${c10_held};writer:${c10_writer_state};reader:${c10_reader_state};link:${c10_link}" FAIL
fi
admin_c "ALTER ROLE command_probe RESET deadlock_timeout;" >"$OUT/C10_reset_command.stdout" 2>"$OUT/C10_reset_command.stderr"
admin_c "ALTER ROLE fence_probe RESET deadlock_timeout;" >"$OUT/C10_reset_fence.stdout" 2>"$OUT/C10_reset_fence.stderr"
reset_main c10

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
