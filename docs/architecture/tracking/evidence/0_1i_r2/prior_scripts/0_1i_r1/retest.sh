#!/bin/bash
set -u
export PGUSER=proof_admin
export PGDATABASE=binding_privilege_proof_r1
LOG=/tmp/proof-r1-log.txt
: > "$LOG"

record() {
  local name="$1" state="$2" detail="$3"
  psql -X -q -c "INSERT INTO proof.results(test_name, sqlstate, detail) VALUES ('${name}', '${state}', \$d\$${detail}\$d\$) ON CONFLICT (test_name) DO UPDATE SET sqlstate = EXCLUDED.sqlstate, detail = EXCLUDED.detail"
  printf '%s %s %s\n' "$name" "$state" "$detail" | tee -a "$LOG"
}

run_as() {
  local name="$1" role="$2" sql="$3" expect="$4"
  local out state
  out=$(psql -X -U "$role" -d "$PGDATABASE" -v VERBOSITY=verbose -v ON_ERROR_STOP=1 -c "$sql" 2>&1) || true
  if printf '%s\n' "$out" | grep -q '^ERROR:'; then
    state=$(printf '%s\n' "$out" | sed -n 's/^ERROR:  \([0-9A-Z]*\):.*/\1/p' | head -n 1)
    [ -n "$state" ] || state=ERROR
  else
    state=OK
  fi
  printf '%s\n' "$out" >> "$LOG"
  local verdict=PASS
  if [ "$state" != "$expect" ]; then
    verdict=FAIL
  fi
  record "$name" "$state" "expect=${expect} verdict=${verdict} role=${role}"
}

psql -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE SCHEMA IF NOT EXISTS proof;
CREATE SCHEMA IF NOT EXISTS transport;
REVOKE ALL ON SCHEMA transport FROM PUBLIC;
REVOKE ALL ON SCHEMA proof FROM PUBLIC;

CREATE TABLE proof.results (
  test_name text PRIMARY KEY,
  sqlstate text NOT NULL,
  detail text NOT NULL
);

CREATE TABLE transport.shipments (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL,
  driver_id uuid,
  vehicle_id uuid,
  version integer NOT NULL,
  deleted_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE transport.drivers (
  id uuid PRIMARY KEY,
  status text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE transport.vehicles (
  id uuid PRIMARY KEY,
  status text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE transport.transport_executions (
  id uuid PRIMARY KEY,
  driver_id uuid,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE transport.binding_authority_decisions (
  decision_id uuid PRIMARY KEY,
  use_status text NOT NULL,
  consumed_at timestamptz
);

INSERT INTO transport.shipments (id, tenant_id, driver_id, vehicle_id, version)
VALUES ('22222222-2222-2222-2222-222222222222',
        'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        '33333333-3333-3333-3333-333333333333',
        '44444444-4444-4444-4444-444444444444',
        1);
INSERT INTO transport.drivers (id, status)
VALUES ('33333333-3333-3333-3333-333333333333', 'ACTIVE');
INSERT INTO transport.vehicles (id, status)
VALUES ('44444444-4444-4444-4444-444444444444', 'ACTIVE');
INSERT INTO transport.transport_executions (id, driver_id)
VALUES ('11111111-1111-1111-1111-111111111111',
        '33333333-3333-3333-3333-333333333333');
INSERT INTO transport.binding_authority_decisions (decision_id, use_status, consumed_at)
VALUES ('66666666-6666-6666-6666-666666666666', 'ISSUED', NULL);

CREATE ROLE column_select_only NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE table_select_only NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE assignment_lock_owner NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE binding_authority_fence NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE tracking_runtime NOLOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;

CREATE ROLE column_probe LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE table_probe LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE lock_probe LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE fence_probe LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE runtime_login LOGIN NOSUPERUSER INHERIT NOBYPASSRLS;
CREATE ROLE nopriv_login LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;
CREATE ROLE inherit_probe LOGIN NOSUPERUSER INHERIT NOBYPASSRLS;
CREATE ROLE set_probe LOGIN NOSUPERUSER NOINHERIT NOBYPASSRLS;

GRANT column_select_only TO column_probe WITH INHERIT FALSE, SET TRUE;
GRANT table_select_only TO table_probe WITH INHERIT FALSE, SET TRUE;
GRANT assignment_lock_owner TO lock_probe WITH INHERIT FALSE, SET TRUE;
GRANT binding_authority_fence TO fence_probe WITH INHERIT FALSE, SET TRUE;
GRANT tracking_runtime TO runtime_login WITH INHERIT TRUE, SET FALSE;

GRANT USAGE ON SCHEMA transport TO column_select_only, table_select_only, assignment_lock_owner, binding_authority_fence;

GRANT SELECT (id, tenant_id, driver_id, vehicle_id, version, deleted_at)
  ON transport.shipments TO column_select_only;
GRANT SELECT ON transport.shipments TO table_select_only;

GRANT SELECT (id, tenant_id, driver_id, vehicle_id, version, deleted_at)
  ON transport.shipments TO assignment_lock_owner;
GRANT SELECT (id, status) ON transport.drivers TO assignment_lock_owner;
GRANT SELECT (id, status) ON transport.vehicles TO assignment_lock_owner;
GRANT SELECT (id, driver_id) ON transport.transport_executions TO assignment_lock_owner;
GRANT UPDATE (updated_at) ON transport.shipments TO assignment_lock_owner;
GRANT UPDATE (updated_at) ON transport.drivers TO assignment_lock_owner;
GRANT UPDATE (updated_at) ON transport.vehicles TO assignment_lock_owner;
GRANT UPDATE (updated_at) ON transport.transport_executions TO assignment_lock_owner;
SQL

psql -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE FUNCTION transport.read_binding_assignment(p_shipment_id uuid)
RETURNS TABLE (fn_user text, driver_id uuid)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
  PERFORM 1
    FROM transport.shipments
   WHERE id = p_shipment_id
     AND deleted_at IS NULL
     FOR SHARE;
  RETURN QUERY
  SELECT current_user::text, s.driver_id
    FROM transport.shipments AS s
   WHERE s.id = p_shipment_id
     AND s.deleted_at IS NULL;
END;
$fn$;
ALTER FUNCTION transport.read_binding_assignment(uuid) OWNER TO assignment_lock_owner;
REVOKE ALL ON FUNCTION transport.read_binding_assignment(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION transport.read_binding_assignment(uuid) TO binding_authority_fence;
SQL

SHIPMENT="'22222222-2222-2222-2222-222222222222'"
DRIVER="'33333333-3333-3333-3333-333333333333'"
VEHICLE="'44444444-4444-4444-4444-444444444444'"
EXEC="'11111111-1111-1111-1111-111111111111'"

run_as B1_column_select_for_share column_probe \
  "SET ROLE column_select_only; SELECT id FROM transport.shipments WHERE id = ${SHIPMENT} FOR SHARE;" \
  42501

run_as B2_table_select_for_share table_probe \
  "SET ROLE table_select_only; SELECT id FROM transport.shipments WHERE id = ${SHIPMENT} FOR SHARE;" \
  42501

run_as B3_lock_owner_for_share_shipment lock_probe \
  "SET ROLE assignment_lock_owner; SELECT id FROM transport.shipments WHERE id = ${SHIPMENT} FOR SHARE;" \
  OK

run_as B4_lock_owner_for_update_shipment lock_probe \
  "SET ROLE assignment_lock_owner; SELECT id FROM transport.shipments WHERE id = ${SHIPMENT} FOR UPDATE;" \
  OK

run_as B5_lock_owner_for_share_driver lock_probe \
  "SET ROLE assignment_lock_owner; SELECT id FROM transport.drivers WHERE id = ${DRIVER} FOR SHARE;" \
  OK

run_as B6_lock_owner_for_share_vehicle lock_probe \
  "SET ROLE assignment_lock_owner; SELECT id FROM transport.vehicles WHERE id = ${VEHICLE} FOR SHARE;" \
  OK

run_as B7_lock_owner_for_share_execution lock_probe \
  "SET ROLE assignment_lock_owner; SELECT id FROM transport.transport_executions WHERE id = ${EXEC} FOR SHARE;" \
  OK

run_as B8_lock_owner_update_driver_id lock_probe \
  "SET ROLE assignment_lock_owner; UPDATE transport.shipments SET driver_id = driver_id WHERE id = ${SHIPMENT};" \
  42501

updated_before=$(psql -X -q -tAc "SELECT updated_at FROM transport.shipments WHERE id = ${SHIPMENT};")
run_as B9_lock_owner_update_updated_at lock_probe \
  "SET ROLE assignment_lock_owner; UPDATE transport.shipments SET updated_at = clock_timestamp() WHERE id = ${SHIPMENT};" \
  OK
updated_after=$(psql -X -q -tAc "SELECT updated_at FROM transport.shipments WHERE id = ${SHIPMENT};")
if [ "$updated_before" != "$updated_after" ] && [ -n "$updated_after" ]; then
  record B9_updated_at_value_changed YES "before_differs=YES"
else
  record B9_updated_at_value_changed NO "before=${updated_before} after=${updated_after}"
fi

run_as B10_fence_direct_update_updated_at fence_probe \
  "SET ROLE binding_authority_fence; UPDATE transport.shipments SET updated_at = clock_timestamp() WHERE id = ${SHIPMENT};" \
  42501

run_as B11_runtime_set_role_lock_owner runtime_login \
  "SET ROLE assignment_lock_owner;" \
  42501

run_as B12_fence_set_role_lock_owner fence_probe \
  "SET ROLE binding_authority_fence; SET ROLE assignment_lock_owner;" \
  42501

run_as B13_public_execute nopriv_login \
  "SELECT driver_id FROM transport.read_binding_assignment(${SHIPMENT});" \
  42501

run_as B14_runtime_execute runtime_login \
  "SELECT driver_id FROM transport.read_binding_assignment(${SHIPMENT});" \
  42501

run_as B15_fence_execute_definer fence_probe \
  "SET ROLE binding_authority_fence; SELECT fn_user, driver_id FROM transport.read_binding_assignment(${SHIPMENT});" \
  OK

identity=$(psql -X -U fence_probe -q -tAc "SET ROLE binding_authority_fence; SELECT fn_user || '|' || driver_id::text FROM transport.read_binding_assignment(${SHIPMENT});")
record B15_identity "$identity" "expect=assignment_lock_owner|33333333-3333-3333-3333-333333333333"

psql -X -v ON_ERROR_STOP=1 -c "GRANT assignment_lock_owner TO inherit_probe WITH INHERIT TRUE, SET FALSE;"
run_as B16_inherit_true_updates_updated_at inherit_probe \
  "UPDATE transport.shipments SET updated_at = clock_timestamp() WHERE id = ${SHIPMENT};" \
  OK
psql -X -v ON_ERROR_STOP=1 -c "REVOKE assignment_lock_owner FROM inherit_probe;"
run_as B17_after_revoke_inherit_denied inherit_probe \
  "UPDATE transport.shipments SET updated_at = clock_timestamp() WHERE id = ${SHIPMENT};" \
  42501

psql -X -v ON_ERROR_STOP=1 -c "GRANT assignment_lock_owner TO set_probe WITH INHERIT FALSE, SET TRUE;"
run_as B18_set_true_without_set_role_denied set_probe \
  "UPDATE transport.shipments SET updated_at = clock_timestamp() WHERE id = ${SHIPMENT};" \
  42501
run_as B19_set_true_with_set_role_updates set_probe \
  "SET ROLE assignment_lock_owner; UPDATE transport.shipments SET updated_at = clock_timestamp() WHERE id = ${SHIPMENT};" \
  OK
psql -X -v ON_ERROR_STOP=1 -c "REVOKE assignment_lock_owner FROM set_probe;"
run_as B20_after_revoke_set_role_denied set_probe \
  "SET ROLE assignment_lock_owner;" \
  42501

member_count=$(psql -X -q -tAc "SELECT count(*) FROM pg_auth_members m JOIN pg_roles r ON r.oid = m.roleid WHERE r.rolname = 'assignment_lock_owner' AND (SELECT rolname FROM pg_roles WHERE oid = m.member) IN ('binding_authority_fence','tracking_runtime','inherit_probe','set_probe');")
record B21_runtime_fence_membership "$member_count" "expect=0"

# Old harness compared the whole psql stdout of a multi-statement script.
old_out=$(psql -X -U proof_admin -d "$PGDATABASE" -v ON_ERROR_STOP=1 -tAc "
BEGIN;
UPDATE transport.binding_authority_decisions
   SET consumed_at = clock_timestamp(), use_status = 'CONSUMED'
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
ROLLBACK;
SELECT use_status || '|' || CASE WHEN consumed_at IS NULL THEN 'null' ELSE 'set' END
  FROM transport.binding_authority_decisions
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
" 2>>"$LOG" || true)
printf 'OLD_STDOUT_BEGIN\n%s\nOLD_STDOUT_END\n' "$old_out" >> "$LOG"
if [ "$old_out" = "ISSUED|null" ]; then
  record H1_old_whole_string_compare MATCH "unexpected exact match"
else
  record H1_old_whole_string_compare MISMATCH "whole stdout is not ISSUED|null"
fi

psql -X -q -v ON_ERROR_STOP=1 -c "
BEGIN;
UPDATE transport.binding_authority_decisions
   SET consumed_at = clock_timestamp(), use_status = 'CONSUMED'
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
ROLLBACK;
" >>"$LOG" 2>&1
use_status=$(psql -X -q -tAc "SELECT use_status FROM transport.binding_authority_decisions WHERE decision_id = '66666666-6666-6666-6666-666666666666';")
consumed_null=$(psql -X -q -tAc "SELECT consumed_at IS NULL FROM transport.binding_authority_decisions WHERE decision_id = '66666666-6666-6666-6666-666666666666';")
field_state=OK
if [ "$use_status" != "ISSUED" ] || [ "$consumed_null" != "t" ]; then
  field_state=FAIL
fi
record H2_use_status "$use_status" "expect=ISSUED verdict_input=${field_state}"
record H3_consumed_at_is_null "$consumed_null" "expect=t verdict_input=${field_state}"
if [ "$field_state" = "OK" ]; then
  record H4_field_compare PASS "use_status=ISSUED consumed_at_is_null=t sqlstate=NONE_SUCCESS"
else
  record H4_field_compare FAIL "use_status=${use_status} consumed_at_is_null=${consumed_null}"
fi

# Lock owner FOR SHARE still conflicts with UPDATE.
psql -X -U lock_probe -d "$PGDATABASE" -v ON_ERROR_STOP=1 -c "
SET ROLE assignment_lock_owner;
BEGIN;
SELECT id FROM transport.shipments WHERE id = ${SHIPMENT} FOR SHARE;
SELECT pg_sleep(8);
COMMIT;
" >/tmp/holder.out 2>/tmp/holder.err &
holder_pid=$!
sleep 1
waiter=$(psql -X -U proof_admin -d "$PGDATABASE" -v VERBOSITY=verbose -v ON_ERROR_STOP=1 -c "SET lock_timeout = '2s'; UPDATE transport.shipments SET version = version WHERE id = ${SHIPMENT};" 2>&1) || true
printf '%s\n' "$waiter" >> "$LOG"
if printf '%s\n' "$waiter" | grep -q '55P03'; then
  record L1_share_blocks_update 55P03 "expect=55P03"
else
  record L1_share_blocks_update OTHER "see log"
fi
wait "$holder_pid" || true
version_after=$(psql -X -q -tAc "SELECT version FROM transport.shipments WHERE id = ${SHIPMENT};")
record L2_version_unchanged_by_timed_out_update "$version_after" "expect=1"

psql -X -q -c "SELECT test_name, sqlstate, detail FROM proof.results ORDER BY test_name;"
