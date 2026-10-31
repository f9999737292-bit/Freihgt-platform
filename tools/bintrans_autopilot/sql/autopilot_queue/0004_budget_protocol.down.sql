-- Reverts migration 0004 budget protocol objects.

REVOKE SELECT ON autopilot_queue.budget_window_usage FROM autopilot_queue_runtime;

REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_mark_unknown(text, text, text, text) FROM autopilot_queue_runtime;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_release(text) FROM autopilot_queue_runtime;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_commit(text, text, text, bigint, text, text, boolean) FROM autopilot_queue_runtime;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_reserve(text, bigint, bigint, date, date) FROM autopilot_queue_runtime;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_insert_blocked(text, bigint, text, date, date) FROM autopilot_queue_runtime;

DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_mark_unknown(text, text, text, text);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_release(text);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_commit(text, text, text, bigint, text, text, boolean);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_reserve(text, bigint, bigint, date, date);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_insert_blocked(text, bigint, text, date, date);

DROP FUNCTION IF EXISTS autopilot_queue._budget_window_apply(
    text, text, date, date, bigint, bigint, bigint, bigint, boolean, bigint, bigint
);
DROP FUNCTION IF EXISTS autopilot_queue._budget_assert_safety_after_lock();
DROP FUNCTION IF EXISTS autopilot_queue._budget_bound_tenant();

DROP TABLE IF EXISTS autopilot_queue.budget_window_usage;

CREATE POLICY budget_ledger_insert ON autopilot_queue.budget_tenant_ledger
    FOR INSERT
    WITH CHECK (tenant_id = autopilot_queue.bound_tenant());

CREATE POLICY budget_ledger_update ON autopilot_queue.budget_tenant_ledger
    FOR UPDATE
    USING (tenant_id = autopilot_queue.bound_tenant())
    WITH CHECK (tenant_id = autopilot_queue.bound_tenant());

CREATE POLICY budget_reservations_insert ON autopilot_queue.budget_reservations
    FOR INSERT
    WITH CHECK (tenant_id = autopilot_queue.bound_tenant());

CREATE POLICY budget_reservations_update ON autopilot_queue.budget_reservations
    FOR UPDATE
    USING (tenant_id = autopilot_queue.bound_tenant())
    WITH CHECK (tenant_id = autopilot_queue.bound_tenant());

GRANT INSERT, UPDATE ON autopilot_queue.budget_tenant_ledger TO autopilot_queue_runtime;
GRANT INSERT, UPDATE ON autopilot_queue.budget_reservations TO autopilot_queue_runtime;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_global_write_counters(
    date, date, bigint, bigint, bigint, bigint
) TO autopilot_queue_runtime;
