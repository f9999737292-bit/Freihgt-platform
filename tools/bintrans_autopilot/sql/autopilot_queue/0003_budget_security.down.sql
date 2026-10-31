-- Reverts migration 0003 budget security objects.

REVOKE ALL ON FUNCTION autopilot_queue.budget_global_safety_block_reason() FROM autopilot_queue_runtime;
DROP FUNCTION IF EXISTS autopilot_queue.budget_global_safety_block_reason();

REVOKE ALL ON FUNCTION autopilot_queue.budget_global_write_counters(
    date, date, bigint, bigint, bigint, bigint
) FROM autopilot_queue_runtime;
DROP FUNCTION IF EXISTS autopilot_queue.budget_global_write_counters(
    date, date, bigint, bigint, bigint, bigint
);

REVOKE ALL ON FUNCTION autopilot_queue.budget_global_lock_row() FROM autopilot_queue_runtime;
DROP FUNCTION IF EXISTS autopilot_queue.budget_global_lock_row();

REVOKE SELECT ON autopilot_queue.budget_global_ledger FROM autopilot_queue_runtime;
DROP POLICY IF EXISTS budget_global_select ON autopilot_queue.budget_global_ledger;

REVOKE INSERT, UPDATE ON autopilot_queue.budget_reservations FROM autopilot_queue_runtime;
REVOKE INSERT, UPDATE ON autopilot_queue.budget_tenant_ledger FROM autopilot_queue_runtime;

DROP POLICY IF EXISTS budget_reservations_update ON autopilot_queue.budget_reservations;
DROP POLICY IF EXISTS budget_reservations_insert ON autopilot_queue.budget_reservations;
DROP POLICY IF EXISTS budget_ledger_update ON autopilot_queue.budget_tenant_ledger;
DROP POLICY IF EXISTS budget_ledger_insert ON autopilot_queue.budget_tenant_ledger;

ALTER TABLE autopilot_queue.budget_reservations
    DROP CONSTRAINT budget_reservations_state_check;

ALTER TABLE autopilot_queue.budget_reservations
    ADD CONSTRAINT budget_reservations_state_check CHECK (
        state IN ('REQUESTED', 'RESERVED', 'COMMITTED', 'RELEASED', 'BLOCKED')
    );

ALTER TABLE autopilot_queue.budget_reservations
    DROP COLUMN IF EXISTS usage_unknown,
    DROP COLUMN IF EXISTS ledger_utc_month,
    DROP COLUMN IF EXISTS ledger_utc_day;

DROP TABLE IF EXISTS autopilot_queue.budget_global_ledger;
