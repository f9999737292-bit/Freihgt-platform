-- Roll back launch lifecycle (R6B). Test DB only.

REVOKE ALL ON FUNCTION autopilot_queue.budget_orphan_spend_signal(text, text, bigint, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_escalate_manual(text, text, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_recovery_poll_attempt(text, text, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_recovery_lease_acquire(text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_reject_definite(text, text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_confirm(text, text, text, text, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_mark_unknown(text, text, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_report_outbound_result(text, text, text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_mark_send_started(text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_cancel_before_send(text, text, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_record_intent(text, text, text, integer, text) FROM PUBLIC;

DROP FUNCTION IF EXISTS autopilot_queue.budget_orphan_spend_signal(text, text, bigint, text);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_escalate_manual(text, text, uuid);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_recovery_poll_attempt(text, text, uuid);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_recovery_lease_acquire(text, text);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_reject_definite(text, text, text);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_confirm(text, text, text, text, uuid);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_mark_unknown(text, text, uuid);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_report_outbound_result(text, text, text, text);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_mark_send_started(text, text);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_cancel_before_send(text, text, uuid);
DROP FUNCTION IF EXISTS autopilot_queue.launch_protocol_record_intent(text, text, text, integer, text);

DROP FUNCTION IF EXISTS autopilot_queue._budget_release_unused_reserve(text);
DROP FUNCTION IF EXISTS autopilot_queue._launch_disable_rls();
DROP FUNCTION IF EXISTS autopilot_queue._launch_fence_ok(autopilot_queue.launch_intents, uuid);
DROP FUNCTION IF EXISTS autopilot_queue._launch_assert_mock_mode();
DROP FUNCTION IF EXISTS autopilot_queue._launch_control_row();
DROP FUNCTION IF EXISTS autopilot_queue._launch_has_role(name);

DROP TABLE IF EXISTS autopilot_queue.budget_orphan_spend_signal;
DROP TABLE IF EXISTS autopilot_queue.manual_reconciliation_audit;
DROP TABLE IF EXISTS autopilot_queue.recovery_poll_audit;
DROP TABLE IF EXISTS autopilot_queue.launch_intents;
DROP TABLE IF EXISTS autopilot_queue.autopilot_launch_control;

CREATE OR REPLACE FUNCTION autopilot_queue.budget_global_safety_block_reason()
RETURNS text
LANGUAGE sql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
STABLE
AS $$
    SELECT CASE
        WHEN EXISTS (
            SELECT 1 FROM autopilot_queue.budget_reservations WHERE state = 'OVERRUN' LIMIT 1
        ) THEN 'overrun_open'
        WHEN EXISTS (
            SELECT 1
            FROM autopilot_queue.budget_reservations
            WHERE state = 'RESERVED' AND usage_unknown = true
            LIMIT 1
        ) THEN 'unknown_usage_open'
        ELSE NULL
    END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue._budget_assert_release_allowed(
    p_row autopilot_queue.budget_reservations
)
RETURNS void
LANGUAGE plpgsql
STABLE
AS $$
BEGIN
    IF p_row.usage_unknown THEN
        RAISE EXCEPTION 'unknown_usage_open';
    END IF;
    IF p_row.agent_id IS NOT NULL OR p_row.run_id IS NOT NULL THEN
        RAISE EXCEPTION 'launch_identity_bound';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM autopilot_queue.budget_verified_usage AS u
        WHERE u.launch_id = p_row.launch_id
    ) THEN
        RAISE EXCEPTION 'verified_usage_present';
    END IF;
END;
$$;
