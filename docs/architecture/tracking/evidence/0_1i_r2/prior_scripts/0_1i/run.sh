#!/bin/bash
set -u
export PGUSER=proof_admin
export PGDATABASE=binding_privilege_proof
PSQL=(psql -v ON_ERROR_STOP=1 -X -q)
LOG=/tmp/proof-log.txt
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
  record "$name" "$state" "expect=${expect} role=${role}"
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

INSERT INTO proof.results(test_name, sqlstate, detail)
VALUES ('POSTGRES_VERSION', 'OK', split_part(version(), ' ', 2)),
       ('ISOLATED_DATABASE', 'OK', current_database());

CREATE ROLE binding_authority_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE binding_authority_fence NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE shipment_assignment_reader NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE tracking_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE freight_migrations NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE runtime_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
CREATE ROLE migration_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
CREATE ROLE reader_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
CREATE ROLE fence_probe LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
CREATE ROLE nopriv_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;

GRANT tracking_runtime TO runtime_login WITH INHERIT TRUE, SET FALSE;
GRANT freight_migrations TO migration_login WITH INHERIT FALSE, SET TRUE;
GRANT shipment_assignment_reader TO reader_probe WITH INHERIT FALSE, SET TRUE;
GRANT binding_authority_fence TO fence_probe WITH INHERIT FALSE, SET TRUE;
GRANT binding_authority_owner TO freight_migrations WITH INHERIT FALSE, SET TRUE;

GRANT USAGE, CREATE ON SCHEMA transport TO freight_migrations;
GRANT USAGE, CREATE ON SCHEMA transport TO binding_authority_owner;
GRANT USAGE ON SCHEMA transport TO shipment_assignment_reader, binding_authority_fence, nopriv_login, runtime_login;
GRANT USAGE ON SCHEMA proof TO migration_login, reader_probe, fence_probe, runtime_login, nopriv_login;

CREATE TABLE transport.shipments (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL,
  driver_id uuid,
  vehicle_id uuid,
  version integer NOT NULL,
  deleted_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT pg_catalog.now()
);
CREATE TABLE transport.drivers (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL,
  status text NOT NULL,
  deleted_at timestamptz
);
CREATE TABLE transport.vehicles (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL,
  status text NOT NULL
);
CREATE TABLE transport.transport_executions (
  id uuid PRIMARY KEY,
  operating_tenant_id uuid NOT NULL,
  driver_id uuid,
  vehicle_id uuid,
  current_revision_id uuid,
  updated_at timestamptz NOT NULL DEFAULT pg_catalog.now()
);

INSERT INTO transport.drivers VALUES ('33333333-3333-3333-3333-333333333333', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'ACTIVE', NULL);
INSERT INTO transport.vehicles VALUES ('44444444-4444-4444-4444-444444444444', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'ACTIVE');
INSERT INTO transport.shipments VALUES (
  '22222222-2222-2222-2222-222222222222',
  'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
  '33333333-3333-3333-3333-333333333333',
  '44444444-4444-4444-4444-444444444444',
  1, NULL, pg_catalog.now()
);
INSERT INTO transport.transport_executions VALUES (
  '11111111-1111-1111-1111-111111111111',
  'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
  '33333333-3333-3333-3333-333333333333',
  '44444444-4444-4444-4444-444444444444',
  'cccccccc-cccc-cccc-cccc-cccccccccccc',
  pg_catalog.now()
);

CREATE TABLE public.shipments (
  id uuid PRIMARY KEY,
  driver_id uuid
);
INSERT INTO public.shipments VALUES (
  '22222222-2222-2222-2222-222222222222',
  '99999999-9999-9999-9999-999999999999'
);

GRANT SELECT (id) ON transport.shipments TO shipment_assignment_reader;
GRANT SELECT (id, operating_tenant_id, driver_id, vehicle_id, current_revision_id)
  ON transport.transport_executions TO shipment_assignment_reader;
SQL

run_as P1_for_share_granted_column reader_probe \
  "SET ROLE shipment_assignment_reader; SELECT id FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222' FOR SHARE;" \
  OK

run_as P1b_for_share_ungranted_column reader_probe \
  "SET ROLE shipment_assignment_reader; SELECT version FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222' FOR SHARE;" \
  42501

psql -X -v ON_ERROR_STOP=1 -c "GRANT SELECT (tenant_id, driver_id, vehicle_id, version, deleted_at) ON transport.shipments TO shipment_assignment_reader;"

run_as P1c_for_share_after_column_grant reader_probe \
  "SET ROLE shipment_assignment_reader; SELECT id, version FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222' FOR SHARE;" \
  OK

run_as P9_for_update_without_update_grant reader_probe \
  "SET ROLE shipment_assignment_reader; SELECT id FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222' FOR UPDATE;" \
  42501

psql -X -v ON_ERROR_STOP=1 -c "REVOKE SELECT ON transport.shipments FROM shipment_assignment_reader;"
run_as P3_no_select reader_probe \
  "SET ROLE shipment_assignment_reader; SELECT id FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222';" \
  42501
psql -X -v ON_ERROR_STOP=1 -c "GRANT SELECT (id, tenant_id, driver_id, vehicle_id, version, deleted_at) ON transport.shipments TO shipment_assignment_reader;"

psql -X -v ON_ERROR_STOP=1 -c "REVOKE USAGE ON SCHEMA transport FROM shipment_assignment_reader;"
run_as P2_no_schema_usage reader_probe \
  "SET ROLE shipment_assignment_reader; SELECT id FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222';" \
  42501
psql -X -v ON_ERROR_STOP=1 -c "GRANT USAGE ON SCHEMA transport TO shipment_assignment_reader;"

psql -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE FUNCTION transport.privilege_identity()
RETURNS TABLE (fn_user text, fn_session text, fn_path text, driver_id uuid)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
  RETURN QUERY
  SELECT pg_catalog.current_user::text,
         pg_catalog.session_user::text,
         pg_catalog.current_setting('search_path'),
         s.driver_id
    FROM transport.shipments AS s
   WHERE s.id = '22222222-2222-2222-2222-222222222222'::uuid;
END;
$fn$;
REVOKE ALL ON FUNCTION transport.privilege_identity() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION transport.privilege_identity() TO binding_authority_fence;

CREATE FUNCTION transport.unqualified_shipments()
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
  v uuid;
BEGIN
  SELECT id INTO v FROM shipments LIMIT 1;
  RETURN v;
END;
$fn$;
REVOKE ALL ON FUNCTION transport.unqualified_shipments() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION transport.unqualified_shipments() TO binding_authority_fence;

CREATE FUNCTION transport.read_binding_assignment(p_execution_id uuid, p_shipment_id uuid)
RETURNS TABLE (shipment_id uuid, driver_id uuid, execution_locked boolean)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
  IF p_execution_id IS NOT NULL THEN
    PERFORM 1 FROM transport.transport_executions WHERE id = p_execution_id FOR SHARE;
  END IF;
  PERFORM 1 FROM transport.shipments WHERE id = p_shipment_id AND deleted_at IS NULL FOR SHARE;
  RETURN QUERY
  SELECT s.id, s.driver_id, p_execution_id IS NOT NULL
    FROM transport.shipments AS s
   WHERE s.id = p_shipment_id;
END;
$fn$;
REVOKE ALL ON FUNCTION transport.read_binding_assignment(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION transport.read_binding_assignment(uuid, uuid) TO binding_authority_fence;
SQL

ident=$(psql -X -U fence_probe -d "$PGDATABASE" -tAc "SET ROLE binding_authority_fence; SELECT fn_user || '|' || fn_session || '|' || fn_path || '|' || driver_id FROM transport.privilege_identity();" 2>>"$LOG" || true)
printf 'P4 raw %s\n' "$ident" >> "$LOG"
if printf '%s' "$ident" | grep -q '^shipment_assignment_reader|fence_probe|pg_catalog|33333333-3333-3333-3333-333333333333$'; then
  record P4_security_definer_search_path OK "$ident"
else
  record P4_security_definer_search_path FAIL "$ident"
fi

run_as P4b_unqualified_name_rejected fence_probe \
  "SET ROLE binding_authority_fence; SELECT transport.unqualified_shipments();" \
  42P01

run_as P5_runtime_direct_update runtime_login \
  "UPDATE transport.shipments SET version = version + 1 WHERE id = '22222222-2222-2222-2222-222222222222';" \
  42501

run_as P6_public_execute_denied nopriv_login \
  "SELECT transport.privilege_identity();" \
  42501

run_as P6b_runtime_execute_denied runtime_login \
  "SELECT transport.privilege_identity();" \
  42501

run_as P6c_fence_execute_allowed fence_probe \
  "SET ROLE binding_authority_fence; SELECT driver_id FROM transport.read_binding_assignment(NULL, '22222222-2222-2222-2222-222222222222');" \
  OK

run_as P7_runtime_cannot_set_role_owner runtime_login \
  "SET ROLE shipment_assignment_reader;" \
  42501

run_as P7b_runtime_cannot_set_role_table_owner runtime_login \
  "SET ROLE binding_authority_owner;" \
  42501

# Ownership and trigger. migration_login is not a superuser.
psql -X -U migration_login -d "$PGDATABASE" -v ON_ERROR_STOP=1 <<'SQL'
SET ROLE freight_migrations;
CREATE TABLE transport.binding_authority_decisions (
  decision_id uuid PRIMARY KEY,
  caller_user_id uuid NOT NULL,
  shipment_tenant_id uuid NOT NULL,
  shipment_id uuid NOT NULL,
  driver_id uuid,
  vehicle_id uuid,
  execution_id uuid,
  consumed_at timestamptz,
  use_status text NOT NULL,
  binding_id uuid
);
RESET ROLE;
SQL
owner_before=$(psql -X -tAc "SELECT r.rolname FROM pg_class c JOIN pg_roles r ON r.oid = c.relowner JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'transport' AND c.relname = 'binding_authority_decisions';")
record O1_create_table_owner "$owner_before" "expect=freight_migrations"

run_as O2_alter_owner migration_login \
  "SET ROLE freight_migrations; ALTER TABLE transport.binding_authority_decisions OWNER TO binding_authority_owner;" \
  OK
owner_after=$(psql -X -tAc "SELECT r.rolname FROM pg_class c JOIN pg_roles r ON r.oid = c.relowner JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'transport' AND c.relname = 'binding_authority_decisions';")
record O2_owner_after_transfer "$owner_after" "expect=binding_authority_owner"

psql -X -U migration_login -d "$PGDATABASE" -v ON_ERROR_STOP=1 <<'SQL'
SET ROLE binding_authority_owner;
CREATE FUNCTION transport.reject_binding_decision_reversal()
RETURNS trigger
LANGUAGE plpgsql
SECURITY INVOKER
SET search_path = pg_catalog
AS $fn$
BEGIN
  IF NEW.decision_id IS DISTINCT FROM OLD.decision_id
     OR NEW.caller_user_id IS DISTINCT FROM OLD.caller_user_id
     OR NEW.shipment_tenant_id IS DISTINCT FROM OLD.shipment_tenant_id
     OR NEW.shipment_id IS DISTINCT FROM OLD.shipment_id
     OR NEW.driver_id IS DISTINCT FROM OLD.driver_id
     OR NEW.vehicle_id IS DISTINCT FROM OLD.vehicle_id
     OR NEW.execution_id IS DISTINCT FROM OLD.execution_id
  THEN
    RAISE EXCEPTION 'binding decision identity is immutable' USING ERRCODE = '23514';
  END IF;
  IF OLD.use_status = 'CONSUMED' OR OLD.consumed_at IS NOT NULL THEN
    IF NEW.use_status IS DISTINCT FROM 'CONSUMED'
       OR NEW.consumed_at IS DISTINCT FROM OLD.consumed_at
       OR NEW.binding_id IS DISTINCT FROM OLD.binding_id
    THEN
      RAISE EXCEPTION 'binding decision consumption is one way' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
  END IF;
  IF OLD.use_status = 'ISSUED' AND OLD.consumed_at IS NULL AND OLD.binding_id IS NULL
     AND NEW.use_status = 'CONSUMED' AND NEW.consumed_at IS NOT NULL AND NEW.binding_id IS NOT NULL
  THEN
    RETURN NEW;
  END IF;
  IF NEW.use_status IS NOT DISTINCT FROM OLD.use_status
     AND NEW.consumed_at IS NOT DISTINCT FROM OLD.consumed_at
     AND NEW.binding_id IS NOT DISTINCT FROM OLD.binding_id
  THEN
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'binding decision consumption is one way' USING ERRCODE = '23514';
END;
$fn$;
REVOKE ALL ON FUNCTION transport.reject_binding_decision_reversal() FROM PUBLIC;
CREATE TRIGGER trg_binding_decision_one_way
  BEFORE UPDATE ON transport.binding_authority_decisions
  FOR EACH ROW
  EXECUTE FUNCTION transport.reject_binding_decision_reversal();
GRANT EXECUTE ON FUNCTION transport.reject_binding_decision_reversal() TO binding_authority_fence;
GRANT SELECT (decision_id, shipment_id, consumed_at, use_status, binding_id)
  ON transport.binding_authority_decisions TO binding_authority_fence;
GRANT UPDATE (consumed_at, use_status, binding_id)
  ON transport.binding_authority_decisions TO binding_authority_fence;
INSERT INTO transport.binding_authority_decisions VALUES
  ('55555555-5555-5555-5555-555555555555', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '22222222-2222-2222-2222-222222222222', '33333333-3333-3333-3333-333333333333', NULL, '11111111-1111-1111-1111-111111111111', NULL, 'ISSUED', NULL),
  ('66666666-6666-6666-6666-666666666666', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '22222222-2222-2222-2222-222222222222', '33333333-3333-3333-3333-333333333333', NULL, NULL, NULL, 'ISSUED', NULL);
RESET ROLE;
SQL
fn_owner=$(psql -X -tAc "SELECT r.rolname FROM pg_proc p JOIN pg_roles r ON r.oid = p.proowner JOIN pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname = 'transport' AND p.proname = 'reject_binding_decision_reversal';")
trg=$(psql -X -tAc "SELECT count(*) FROM pg_trigger WHERE tgname = 'trg_binding_decision_one_way' AND NOT tgisinternal;")
record O4_function_owner "$fn_owner" "expect=binding_authority_owner"
record O4_trigger_installed "$trg" "expect=1"

run_as O3_create_trigger_without_set_role_fails migration_login \
  "SET ROLE freight_migrations; CREATE TRIGGER trg_without_owner BEFORE UPDATE ON transport.binding_authority_decisions FOR EACH ROW EXECUTE FUNCTION transport.reject_binding_decision_reversal();" \
  42501

run_as O5_inherit_false_cannot_update_as_migration_role migration_login \
  "SET ROLE freight_migrations; UPDATE transport.binding_authority_decisions SET consumed_at = pg_catalog.now() WHERE decision_id = '66666666-6666-6666-6666-666666666666';" \
  42501

psql -X -v ON_ERROR_STOP=1 -c "REVOKE binding_authority_owner FROM freight_migrations;"

run_as O6_set_role_after_revoke migration_login \
  "SET ROLE binding_authority_owner;" \
  42501

run_as O6b_create_trigger_after_revoke migration_login \
  "SET ROLE freight_migrations; CREATE TRIGGER trg_after_revoke BEFORE UPDATE ON transport.binding_authority_decisions FOR EACH ROW EXECUTE FUNCTION transport.reject_binding_decision_reversal();" \
  42501

member=$(psql -X -tAc "SELECT count(*) FROM pg_auth_members m JOIN pg_roles r ON r.oid = m.roleid JOIN pg_roles u ON u.oid = m.member WHERE r.rolname = 'binding_authority_owner' AND u.rolname = 'freight_migrations';")
record O6_membership_after_revoke "$member" "expect=0"

run_as C1_consume_issued_to_consumed fence_probe \
  "SET ROLE binding_authority_fence; UPDATE transport.binding_authority_decisions SET consumed_at = pg_catalog.clock_timestamp(), use_status = 'CONSUMED', binding_id = '77777777-7777-7777-7777-777777777777' WHERE decision_id = '55555555-5555-5555-5555-555555555555' AND use_status = 'ISSUED' AND consumed_at IS NULL;" \
  OK

run_as C2_reverse_to_issued fence_probe \
  "SET ROLE binding_authority_fence; UPDATE transport.binding_authority_decisions SET use_status = 'ISSUED' WHERE decision_id = '55555555-5555-5555-5555-555555555555';" \
  23514

run_as C3_reset_consumed_at fence_probe \
  "SET ROLE binding_authority_fence; UPDATE transport.binding_authority_decisions SET consumed_at = NULL WHERE decision_id = '55555555-5555-5555-5555-555555555555';" \
  23514

run_as C4_fence_cannot_change_shipment_id fence_probe \
  "SET ROLE binding_authority_fence; UPDATE transport.binding_authority_decisions SET shipment_id = '88888888-8888-8888-8888-888888888888' WHERE decision_id = '55555555-5555-5555-5555-555555555555';" \
  42501

run_as C5_owner_identifier_change_blocked_by_trigger proof_admin \
  "SET ROLE binding_authority_owner; UPDATE transport.binding_authority_decisions SET shipment_id = '88888888-8888-8888-8888-888888888888' WHERE decision_id = '55555555-5555-5555-5555-555555555555';" \
  23514

run_as C6_runtime_decision_update runtime_login \
  "UPDATE transport.binding_authority_decisions SET consumed_at = NULL WHERE decision_id = '55555555-5555-5555-5555-555555555555';" \
  42501

rollback_out=$(psql -X -U proof_admin -d "$PGDATABASE" -v ON_ERROR_STOP=1 -tAc "
BEGIN;
SET ROLE binding_authority_owner;
UPDATE transport.binding_authority_decisions
   SET consumed_at = pg_catalog.clock_timestamp(), use_status = 'CONSUMED', binding_id = '77777777-7777-7777-7777-777777777777'
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
ROLLBACK;
SELECT use_status || '|' || CASE WHEN consumed_at IS NULL THEN 'null' ELSE 'set' END
  FROM transport.binding_authority_decisions
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
" 2>>"$LOG" || true)
printf 'rollback raw %s\n' "$rollback_out" >> "$LOG"
if [ "$rollback_out" = "ISSUED|null" ]; then
  record R1_rollback_restores_issued OK "$rollback_out"
else
  record R1_rollback_restores_issued FAIL "$rollback_out"
fi

# Lock proofs.
wait_for_lock() {
  local app="$1" rel="$2" tries="${3:-50}"
  local i count
  for i in $(seq 1 "$tries"); do
    count=$(psql -X -tAc "SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.application_name = '${app}' AND c.relname = '${rel}' AND l.granted;")
    if [ "$count" != "0" ]; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

psql -X -U proof_admin -d "$PGDATABASE" -v ON_ERROR_STOP=1 <<'SQL' >/tmp/holder.out 2>&1 &
\set ON_ERROR_STOP on
SET application_name = 'holder';
BEGIN;
SELECT id FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222' FOR SHARE;
SELECT pg_sleep(30);
ROLLBACK;
SQL
holder_pid=$!
if wait_for_lock holder shipments; then
  record L0_holder_share_granted OK "shipments"
else
  record L0_holder_share_granted FAIL "timeout"
fi

psql -X -U proof_admin -d "$PGDATABASE" -v VERBOSITY=verbose <<'SQL' >/tmp/waiter-update.out 2>&1 &
SET application_name = 'waiter_update';
SET lock_timeout = '8s';
UPDATE transport.shipments SET version = version + 1 WHERE id = '22222222-2222-2222-2222-222222222222';
SQL
waiter_pid=$!
sleep 0.8
waiting=$(psql -X -tAc "SELECT count(*) FROM pg_stat_activity WHERE application_name = 'waiter_update' AND wait_event_type = 'Lock';")
record L1_update_waits_on_share "$waiting" "expect_at_least_1"
if [ "$waiting" != "0" ]; then
  psql -X -c "SELECT pg_cancel_backend(pid) FROM pg_stat_activity WHERE application_name = 'holder' AND pid <> pg_backend_pid();" >/dev/null
fi
wait "$waiter_pid" || true
if grep -q '^ERROR:' /tmp/waiter-update.out; then
  state=$(sed -n 's/^ERROR:  \([0-9A-Z]*\):.*/\1/p' /tmp/waiter-update.out | head -n 1)
else
  state=OK
fi
record L1_update_after_share_released "$state" "expect=OK"
wait "$holder_pid" || true

psql -X -c "UPDATE transport.shipments SET version = 1 WHERE id = '22222222-2222-2222-2222-222222222222';" >/dev/null

psql -X <<'SQL' >/tmp/holder2.out 2>&1 &
SET application_name = 'holder2';
BEGIN;
SELECT id FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222' FOR SHARE;
SELECT pg_sleep(30);
ROLLBACK;
SQL
holder2_pid=$!
wait_for_lock holder2 shipments || record L2_holder_missing FAIL "timeout"
psql -X -v VERBOSITY=verbose <<'SQL' >/tmp/waiter-forupdate.out 2>&1 &
SET application_name = 'waiter_for_update';
SET lock_timeout = '8s';
SELECT id FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222' FOR UPDATE;
SQL
waiter2_pid=$!
sleep 0.8
waiting=$(psql -X -tAc "SELECT count(*) FROM pg_stat_activity WHERE application_name = 'waiter_for_update' AND wait_event_type = 'Lock';")
record L2_for_update_waits_on_share "$waiting" "expect_at_least_1"
psql -X -c "SELECT pg_cancel_backend(pid) FROM pg_stat_activity WHERE application_name = 'holder2' AND pid <> pg_backend_pid();" >/dev/null
wait "$waiter2_pid" || true
if grep -q '^ERROR:' /tmp/waiter-forupdate.out; then
  state=$(sed -n 's/^ERROR:  \([0-9A-Z]*\):.*/\1/p' /tmp/waiter-forupdate.out | head -n 1)
else
  state=OK
fi
record L2_for_update_after_share_released "$state" "expect=OK"
wait "$holder2_pid" || true

# Execution-first. Command holds the execution. Consume requests the execution before any shipment lock.
psql -X <<'SQL' >/tmp/cmd-hold.out 2>&1 &
SET application_name = 'cmd_hold';
BEGIN;
SELECT id FROM transport.transport_executions WHERE id = '11111111-1111-1111-1111-111111111111' FOR UPDATE;
SELECT pg_sleep(6);
SELECT id FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222' FOR UPDATE;
COMMIT;
SQL
cmd_pid=$!
wait_for_lock cmd_hold transport_executions || record L3_cmd_execution_missing FAIL "timeout"
psql -X -v VERBOSITY=verbose <<'SQL' >/tmp/consume-good.out 2>&1 &
SET application_name = 'consume_good';
SET lock_timeout = '1500ms';
SELECT id FROM transport.transport_executions WHERE id = '11111111-1111-1111-1111-111111111111' FOR SHARE;
SQL
consume_pid=$!
sleep 0.4
ship_locks=$(psql -X -tAc "SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.application_name = 'consume_good' AND c.relname = 'shipments' AND l.granted;")
exec_wait=$(psql -X -tAc "SELECT count(*) FROM pg_stat_activity WHERE application_name = 'consume_good' AND wait_event_type = 'Lock';")
record L3_consume_waiting_on_execution "$exec_wait" "expect_at_least_1"
record L3_consume_does_not_hold_shipment "$ship_locks" "expect=0"
wait "$consume_pid" || true
if grep -q '^ERROR:' /tmp/consume-good.out; then
  state=$(sed -n 's/^ERROR:  \([0-9A-Z]*\):.*/\1/p' /tmp/consume-good.out | head -n 1)
else
  state=OK
fi
record L3_execution_first_timeout "$state" "expect=55P03"
wait "$cmd_pid" || true
if grep -q '^ERROR:' /tmp/cmd-hold.out; then
  state=$(sed -n 's/^ERROR:  \([0-9A-Z]*\):.*/\1/p' /tmp/cmd-hold.out | head -n 1)
else
  state=OK
fi
record L3_command_locks_shipment_after_execution "$state" "expect=OK"

# Driver-only holds the shipment and never requests the execution.
psql -X <<'SQL' >/tmp/driver-only.out 2>&1 &
SET application_name = 'driver_only';
BEGIN;
SELECT id FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222' FOR SHARE;
SELECT pg_sleep(4);
COMMIT;
SQL
driver_pid=$!
wait_for_lock driver_only shipments || record L4_driver_only_missing FAIL "timeout"
exec_locks=$(psql -X -tAc "SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid JOIN pg_class c ON c.oid = l.relation WHERE a.application_name = 'driver_only' AND c.relname = 'transport_executions';")
record L4_driver_only_no_execution_lock "$exec_locks" "expect=0"
psql -X -v VERBOSITY=verbose <<'SQL' >/tmp/cmd-vs-driver.out 2>&1 &
SET application_name = 'cmd_vs_driver';
BEGIN;
SET lock_timeout = '1500ms';
SELECT id FROM transport.transport_executions WHERE id = '11111111-1111-1111-1111-111111111111' FOR UPDATE;
SELECT id FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222' FOR UPDATE;
ROLLBACK;
SQL
cmd2_pid=$!
sleep 0.6
dead=$(grep -c '40P01' /tmp/cmd-vs-driver.out || true)
record L4_no_deadlock_while_driver_only_holds_shipment "$dead" "expect=0"
wait "$cmd2_pid" || true
if grep -q '40P01' /tmp/cmd-vs-driver.out; then
  record L4_command_result 40P01 "unexpected deadlock"
elif grep -q '55P03' /tmp/cmd-vs-driver.out; then
  record L4_command_result 55P03 "waited on shipment then timed out"
else
  record L4_command_result UNKNOWN "see log"
fi
wait "$driver_pid" || true
cat /tmp/cmd-vs-driver.out >> "$LOG"

# Forbidden late execution lock: shipment first, then execution, against a command that holds execution and then wants the shipment.
psql -X -v VERBOSITY=verbose <<'SQL' >/tmp/dead-a.out 2>&1 &
SET application_name = 'dead_cmd';
BEGIN;
SET deadlock_timeout = '200ms';
SET lock_timeout = '8s';
SELECT id FROM transport.transport_executions WHERE id = '11111111-1111-1111-1111-111111111111' FOR UPDATE;
DO $body$
BEGIN
  FOR i IN 1..80 LOOP
    EXIT WHEN EXISTS (
      SELECT 1
        FROM pg_locks l
        JOIN pg_stat_activity a ON a.pid = l.pid
        JOIN pg_class c ON c.oid = l.relation
       WHERE a.application_name = 'dead_consume'
         AND c.relname = 'shipments'
         AND l.granted
    );
    PERFORM pg_catalog.pg_sleep(0.05);
  END LOOP;
END;
$body$;
SELECT id FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222' FOR UPDATE;
ROLLBACK;
SQL
psql -X -v VERBOSITY=verbose <<'SQL' >/tmp/dead-b.out 2>&1 &
SET application_name = 'dead_consume';
BEGIN;
SET deadlock_timeout = '200ms';
SET lock_timeout = '8s';
DO $body$
BEGIN
  FOR i IN 1..80 LOOP
    EXIT WHEN EXISTS (
      SELECT 1
        FROM pg_locks l
        JOIN pg_stat_activity a ON a.pid = l.pid
        JOIN pg_class c ON c.oid = l.relation
       WHERE a.application_name = 'dead_cmd'
         AND c.relname = 'transport_executions'
         AND l.granted
    );
    PERFORM pg_catalog.pg_sleep(0.05);
  END LOOP;
END;
$body$;
SELECT id FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222' FOR SHARE;
SELECT id FROM transport.transport_executions WHERE id = '11111111-1111-1111-1111-111111111111' FOR SHARE;
ROLLBACK;
SQL
wait || true
cat /tmp/dead-a.out /tmp/dead-b.out >> "$LOG"
if grep -q '40P01' /tmp/dead-a.out /tmp/dead-b.out; then
  record L5_forbidden_order_deadlock 40P01 "observed"
else
  record L5_forbidden_order_deadlock FAIL "40P01 not observed"
fi

# READ COMMITTED: a plain read does not keep the first version.
psql -X <<'SQL' >/tmp/rc-reader.out 2>&1 &
SET application_name = 'rc_reader';
BEGIN;
SELECT version FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222';
SELECT pg_sleep(2);
SELECT version FROM transport.shipments WHERE id = '22222222-2222-2222-2222-222222222222';
COMMIT;
SQL
rc_pid=$!
sleep 0.4
psql -X -c "UPDATE transport.shipments SET version = 7 WHERE id = '22222222-2222-2222-2222-222222222222';" >/dev/null
wait "$rc_pid" || true
cat /tmp/rc-reader.out >> "$LOG"
if grep -q '^ *1$' /tmp/rc-reader.out && grep -q '^ *7$' /tmp/rc-reader.out; then
  record L6_read_committed_sees_other_commit OK "1 then 7"
else
  record L6_read_committed_sees_other_commit FAIL "$(tr '\n' ' ' < /tmp/rc-reader.out)"
fi

psql -X -c "SELECT test_name, sqlstate, detail FROM proof.results ORDER BY test_name;"
