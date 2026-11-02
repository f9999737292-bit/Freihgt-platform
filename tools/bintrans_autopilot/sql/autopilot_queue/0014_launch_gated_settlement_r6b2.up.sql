-- R6B-2: launch-gated settlement over the existing reconciler ledger.
-- Does not replace budget_reconciler_apply_usage (accepted R6B-1 path).

CREATE OR REPLACE FUNCTION autopilot_queue.launch_budget_settle(
    p_tenant_id text,
    p_launch_id text,
    p_agent_id text,
    p_run_id text,
    p_raw_cost_text text,
    p_charged_text text,
    p_charged_known boolean,
    p_verification_id text,
    p_payload_digest text,
    p_proof_class text,
    p_lease_token uuid DEFAULT NULL,
    p_source text DEFAULT 'cursor_usage_api'
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_launch autopilot_queue.launch_intents%ROWTYPE;
    v_ctrl autopilot_queue.autopilot_launch_control%ROWTYPE;
BEGIN
    PERFORM autopilot_queue._budget_assert_reconciler();
    PERFORM autopilot_queue._launch_assert_mock_mode();
    SELECT * INTO v_ctrl FROM autopilot_queue.autopilot_launch_control WHERE id = 1;
    IF v_ctrl.autopilot_estop THEN
        RAISE EXCEPTION 'estop_active';
    END IF;
    IF p_proof_class IS DISTINCT FROM 'MOCK_TRUSTED' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF p_tenant_id IS NULL OR p_launch_id IS NULL OR p_agent_id IS NULL OR p_run_id IS NULL THEN
        RAISE EXCEPTION 'invalid_input';
    END IF;
    SELECT * INTO v_launch
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    IF v_launch.lifecycle_state NOT IN ('LAUNCH_CONFIRMED', 'EXTERNAL_AGENT_CANCELLED') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF v_launch.agent_id IS NOT NULL AND v_launch.agent_id <> p_agent_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    IF v_launch.run_id IS NOT NULL AND v_launch.run_id <> p_run_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    IF v_launch.last_outbound_proof_class IS NOT NULL
       AND v_launch.last_outbound_proof_class <> 'MOCK_TRUSTED' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF v_launch.recovery_lease_token IS NOT NULL THEN
        IF v_launch.recovery_lease_expires_at IS NULL
           OR clock_timestamp() >= v_launch.recovery_lease_expires_at
           OR p_lease_token IS DISTINCT FROM v_launch.recovery_lease_token THEN
            RAISE EXCEPTION 'invalid_lease';
        END IF;
    END IF;
    IF NOT EXISTS (
        SELECT 1
        FROM autopilot_queue.budget_reservations
        WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    ) THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    PERFORM autopilot_queue.budget_reconciler_apply_usage(
        p_tenant_id,
        p_launch_id,
        p_agent_id,
        p_run_id,
        p_raw_cost_text,
        p_charged_text,
        p_charged_known,
        p_verification_id,
        p_payload_digest,
        p_source
    );
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue.launch_budget_settle(
    text, text, text, text, text, text, boolean, text, text, text, uuid, text
) FROM PUBLIC;

GRANT EXECUTE ON FUNCTION autopilot_queue.launch_budget_settle(
    text, text, text, text, text, text, boolean, text, text, text, uuid, text
) TO autopilot_queue_reconciler;

ALTER FUNCTION autopilot_queue.launch_budget_settle(
    text, text, text, text, text, text, boolean, text, text, text, uuid, text
) OWNER TO autopilot_queue_definer_owner;

GRANT EXECUTE ON FUNCTION autopilot_queue.launch_budget_settle(
    text, text, text, text, text, text, boolean, text, text, text, uuid, text
) TO autopilot_queue_definer_owner;
