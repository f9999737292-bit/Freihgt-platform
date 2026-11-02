-- Restore the version-14 function bodies. Isolated test databases only.
-- Does not drop below migration 0014 and does not recreate pre-0012 commit helpers.

CREATE OR REPLACE FUNCTION autopilot_queue._launch_fence_ok(
    p_row autopilot_queue.launch_intents,
    p_token uuid
)
RETURNS boolean
LANGUAGE sql
STABLE
AS $$
    SELECT p_row.recovery_lease_token IS NOT NULL
       AND p_row.recovery_lease_token = p_token
       AND p_row.recovery_lease_holder = session_user
       AND p_row.recovery_lease_expires_at IS NOT NULL
       AND clock_timestamp() < p_row.recovery_lease_expires_at;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_recovery_lease_acquire(
    p_tenant_id text,
    p_launch_id text
)
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
    v_ctrl autopilot_queue.autopilot_launch_control%ROWTYPE;
    v_token uuid;
    v_stale boolean := false;
BEGIN
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_recovery') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    PERFORM autopilot_queue._launch_disable_rls();
    SELECT * INTO v_ctrl FROM autopilot_queue.autopilot_launch_control WHERE id = 1;
    SELECT * INTO v_row
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    IF v_row.recovery_lease_holder = session_user
       AND v_row.recovery_lease_expires_at IS NOT NULL
       AND v_row.recovery_lease_token IS NOT NULL
       AND clock_timestamp() < v_row.recovery_lease_expires_at THEN
        RETURN v_row.recovery_lease_token;
    END IF;
    IF v_row.lifecycle_state = 'INTENT' THEN
        v_stale := clock_timestamp() > v_row.intent_recorded_at + make_interval(secs => v_ctrl.intent_stale_seconds);
    ELSIF v_row.lifecycle_state = 'SEND_STARTED' THEN
        v_stale := v_row.send_started_at IS NOT NULL
            AND clock_timestamp() > v_row.send_started_at + make_interval(secs => v_ctrl.send_stale_seconds);
    ELSIF v_row.lifecycle_state = 'LAUNCH_UNKNOWN' THEN
        v_stale := true;
    ELSE
        RAISE EXCEPTION 'not_eligible';
    END IF;
    IF NOT v_stale THEN
        RAISE EXCEPTION 'not_eligible';
    END IF;
    IF v_row.recovery_lease_holder IS NOT NULL
       AND v_row.recovery_lease_expires_at IS NOT NULL
       AND clock_timestamp() < v_row.recovery_lease_expires_at
       AND v_row.recovery_lease_holder <> session_user THEN
        RAISE EXCEPTION 'lease_held_by_other';
    END IF;
    v_token := gen_random_uuid();
    UPDATE autopilot_queue.launch_intents
    SET recovery_lease_holder = session_user,
        recovery_lease_token = v_token,
        recovery_lease_expires_at = clock_timestamp() + make_interval(secs => v_ctrl.recovery_lease_seconds),
        updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    RETURN v_token;
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.budget_reconciler_apply_usage(
    p_tenant_id text,
    p_launch_id text,
    p_agent_id text,
    p_run_id text,
    p_raw_cost_text text,
    p_charged_text text,
    p_charged_known boolean,
    p_verification_id text,
    p_payload_digest text,
    p_source text DEFAULT 'cursor_usage_api'
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_row autopilot_queue.budget_reservations%ROWTYPE;
    v_launch autopilot_queue.launch_intents%ROWTYPE;
    v_existing autopilot_queue.budget_verified_usage%ROWTYPE;
BEGIN
    PERFORM autopilot_queue._budget_assert_reconciler();
    IF p_tenant_id IS NULL OR char_length(trim(p_tenant_id)) = 0 THEN
        RAISE EXCEPTION 'invalid_input';
    END IF;
    IF p_source IS NULL OR char_length(trim(p_source)) = 0 THEN
        RAISE EXCEPTION 'invalid_input';
    END IF;
    IF p_verification_id IS NULL OR char_length(trim(p_verification_id)) < 8 THEN
        RAISE EXCEPTION 'invalid_input';
    END IF;
    IF p_payload_digest IS NULL OR p_payload_digest !~ '^[0-9a-f]{64}$' THEN
        RAISE EXCEPTION 'invalid_input';
    END IF;
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT * INTO v_row
    FROM autopilot_queue.budget_reservations
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    SELECT * INTO v_launch
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    IF FOUND THEN
        IF v_launch.lifecycle_state NOT IN ('LAUNCH_CONFIRMED', 'EXTERNAL_AGENT_CANCELLED') THEN
            RAISE EXCEPTION 'invalid_caller_or_state';
        END IF;
        IF v_launch.agent_id IS NOT NULL AND v_launch.agent_id <> p_agent_id THEN
            RAISE EXCEPTION 'identity_mismatch';
        END IF;
        IF v_launch.run_id IS NOT NULL AND v_launch.run_id <> p_run_id THEN
            RAISE EXCEPTION 'identity_mismatch';
        END IF;
    END IF;
    IF v_row.agent_id IS NULL AND v_row.run_id IS NULL THEN
        IF EXISTS (
            SELECT 1
            FROM autopilot_queue.budget_reservations AS r
            WHERE r.run_id = p_run_id
              AND (r.tenant_id <> p_tenant_id OR r.launch_id <> p_launch_id)
        ) THEN
            RAISE EXCEPTION 'run_id_taken';
        END IF;
        UPDATE autopilot_queue.budget_reservations
        SET agent_id = p_agent_id,
            run_id = p_run_id,
            updated_at = clock_timestamp()
        WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
        SELECT * INTO v_row
        FROM autopilot_queue.budget_reservations
        WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    ELSIF v_row.agent_id <> p_agent_id OR v_row.run_id <> p_run_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    SELECT * INTO v_existing
    FROM autopilot_queue.budget_verified_usage
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    IF FOUND THEN
        IF v_existing.verification_id <> p_verification_id
           OR v_existing.payload_digest <> p_payload_digest
           OR v_existing.agent_id <> p_agent_id
           OR v_existing.run_id <> p_run_id
           OR v_existing.tenant_id <> v_row.tenant_id
           OR v_existing.charged_known <> p_charged_known
           OR v_existing.raw_cost_text IS DISTINCT FROM p_raw_cost_text
           OR v_existing.charged_text IS DISTINCT FROM p_charged_text
           OR v_existing.source <> p_source THEN
            RAISE EXCEPTION 'idempotency_conflict';
        END IF;
    ELSE
        INSERT INTO autopilot_queue.budget_verified_usage (
            launch_id, tenant_id, agent_id, run_id,
            raw_cost_text, charged_text, charged_known,
            source, verification_id, payload_digest
        ) VALUES (
            p_launch_id, v_row.tenant_id, p_agent_id, p_run_id,
            p_raw_cost_text, p_charged_text, p_charged_known,
            p_source, p_verification_id, p_payload_digest
        );
    END IF;
    IF NOT p_charged_known THEN
        IF v_row.state <> 'RESERVED' THEN
            IF v_row.usage_unknown THEN
                RETURN;
            END IF;
            RAISE EXCEPTION 'invalid_state';
        END IF;
        UPDATE autopilot_queue.budget_reservations
        SET agent_id = p_agent_id,
            run_id = p_run_id,
            raw_cost_text = p_raw_cost_text,
            usage_unknown = true,
            updated_at = clock_timestamp()
        WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
        RETURN;
    END IF;
    IF p_charged_text IS NULL OR char_length(trim(p_charged_text)) = 0 THEN
        RAISE EXCEPTION 'unknown_usage';
    END IF;
    PERFORM autopilot_queue._budget_charged_cents(p_charged_text);
    PERFORM autopilot_queue._budget_commit_from_verified(p_tenant_id, p_launch_id);
END;
$$;

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
