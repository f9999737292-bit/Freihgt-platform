-- Reverts migration 0005 budget authority objects.

REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_mark_unknown(text, text, text, text) FROM autopilot_queue_runtime;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_release(text) FROM autopilot_queue_runtime;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_commit(text, text, text, text, text) FROM autopilot_queue_runtime;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_reserve(text, bigint) FROM autopilot_queue_runtime;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_insert_blocked(text, bigint, text) FROM autopilot_queue_runtime;

DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_mark_unknown(text, text, text, text);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_release(text);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_commit(text, text, text, text, text);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_reserve(text, bigint);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_insert_blocked(text, bigint, text);

DROP FUNCTION IF EXISTS autopilot_queue._budget_lock_global_gate();
DROP FUNCTION IF EXISTS autopilot_queue._budget_charged_cents(text);
DROP FUNCTION IF EXISTS autopilot_queue._budget_utc_windows();

DROP POLICY IF EXISTS budget_window_usage_select ON autopilot_queue.budget_window_usage;

ALTER TABLE autopilot_queue.budget_window_usage DISABLE ROW LEVEL SECURITY;
ALTER TABLE autopilot_queue.budget_window_usage NO FORCE ROW LEVEL SECURITY;

GRANT SELECT ON autopilot_queue.budget_window_usage TO autopilot_queue_runtime;

GRANT EXECUTE ON FUNCTION autopilot_queue.budget_global_lock_row() TO autopilot_queue_runtime;

ALTER TABLE autopilot_queue.budget_global_ledger
    DROP COLUMN IF EXISTS per_launch_cap_cents;

-- Restore 0004 protocol signatures via re-running 0004 down/up is handled by migration runner order.
