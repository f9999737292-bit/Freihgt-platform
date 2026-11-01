-- Applied after migration 10 down (schema v9): launch functions call 1-arg release helper.

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_cancel_before_send(
    p_tenant_id text,
    p_launch_id text,
    p_lease_token uuid DEFAULT NULL
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
SET row_security = off
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
    v_ctrl autopilot_queue.autopilot_launch_control%ROWTYPE;
    v_recovery boolean;
    v_stale boolean := false;
BEGIN
    v_recovery := autopilot_queue._launch_has_role('autopilot_launch_recovery');
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_adapter') AND NOT v_recovery THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    PERFORM autopilot_queue._launch_disable_rls();
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT * INTO v_ctrl FROM autopilot_queue.autopilot_launch_control WHERE id = 1;
    SELECT * INTO v_row
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    IF v_row.lifecycle_state <> 'INTENT' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF v_recovery THEN
        IF NOT autopilot_queue._launch_fence_ok(v_row, p_lease_token) THEN
            RAISE EXCEPTION 'invalid_lease';
        END IF;
        v_stale := clock_timestamp() > v_row.intent_recorded_at + make_interval(secs => v_ctrl.intent_stale_seconds);
        IF NOT v_stale THEN
            RAISE EXCEPTION 'invalid_caller_or_state';
        END IF;
    END IF;
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'CANCELLED_BEFORE_SEND', updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    PERFORM autopilot_queue._budget_release_unused_reserve(p_launch_id);
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_reject_definite(
    p_tenant_id text,
    p_launch_id text,
    p_proof_class text
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
SET row_security = off
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
BEGIN
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_adapter') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    PERFORM autopilot_queue._launch_disable_rls();
    PERFORM autopilot_queue._launch_assert_mock_mode();
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT * INTO v_row
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND OR v_row.lifecycle_state <> 'SEND_STARTED' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF v_row.outbound_attempt_count <> 1 OR p_proof_class <> 'MOCK_TRUSTED' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF v_row.last_outbound_proof_class IS NOT NULL
       AND v_row.last_outbound_proof_class <> 'MOCK_TRUSTED' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'LAUNCH_REJECTED', updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    PERFORM autopilot_queue._budget_release_unused_reserve(p_launch_id);
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_manual_reject(
    p_tenant_id text,
    p_launch_id text,
    p_reason text
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
SET row_security = off
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
BEGIN
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_admin') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    PERFORM autopilot_queue._launch_disable_rls();
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT * INTO v_row
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND OR v_row.lifecycle_state <> 'MANUAL_RECONCILIATION_REQUIRED' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    INSERT INTO autopilot_queue.manual_reconciliation_audit (
        tenant_id, launch_id, actor_session_user, action, reason
    ) VALUES (
        p_tenant_id, p_launch_id, session_user, 'manual_reject', p_reason
    );
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'LAUNCH_REJECTED', updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    PERFORM autopilot_queue._budget_release_unused_reserve(p_launch_id);
END;
$$;
