DROP FUNCTION IF EXISTS autopilot_queue.budget_reconciler_apply_usage(
    text, text, text, text, text, boolean, text, text, text
);
DROP FUNCTION IF EXISTS autopilot_queue._budget_commit_from_verified(text);
DROP FUNCTION IF EXISTS autopilot_queue._budget_lock_global_gate_unbound();
DROP FUNCTION IF EXISTS autopilot_queue._budget_assert_reconciler();

DROP POLICY IF EXISTS budget_verified_usage_select ON autopilot_queue.budget_verified_usage;
DROP TABLE IF EXISTS autopilot_queue.budget_verified_usage;

DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_reserve(text, bigint, text, text);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_reserve(text, bigint);
