-- R6B-1-R3: runtime EXECUTE deny, LT-06 proof class, trim definer_owner grants.

CREATE OR REPLACE FUNCTION autopilot_queue._launch_proof_class_lt06_eligible(
    p_proof_class text
)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT p_proof_class IN (
        'MOCK_TRUSTED',
        'HTTP_400_VALIDATION',
        'HTTP_400_REPOSITORY_ACCESS',
        'HTTP_401',
        'HTTP_403'
    );
$$;

REVOKE ALL ON FUNCTION autopilot_queue._launch_proof_class_lt06_eligible(text) FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue._launch_assert_lt06_proof_class(
    p_proof_class text
)
RETURNS void
LANGUAGE plpgsql
STABLE
AS $$
BEGIN
    IF NOT autopilot_queue._launch_proof_class_lt06_eligible(p_proof_class) THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF p_proof_class = 'MOCK_TRUSTED' THEN
        PERFORM autopilot_queue._launch_assert_mock_mode();
    END IF;
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue._launch_assert_lt06_proof_class(text) FROM PUBLIC;

DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_commit(text, text, text, text, text);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_mark_unknown(text, text, text, text);

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_report_outbound_result(
    p_tenant_id text,
    p_launch_id text,
    p_proof_class text,
    p_proof_digest text
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
    IF p_proof_digest IS NULL OR p_proof_digest !~ '^[0-9a-f]{64}$' THEN
        RAISE EXCEPTION 'invalid_input';
    END IF;
    PERFORM autopilot_queue._launch_assert_lt06_proof_class(p_proof_class);
    PERFORM autopilot_queue._launch_disable_rls();
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT * INTO v_row
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND OR v_row.lifecycle_state <> 'SEND_STARTED' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    UPDATE autopilot_queue.launch_intents
    SET outbound_attempt_count = outbound_attempt_count + 1,
        last_outbound_proof_class = p_proof_class,
        last_outbound_proof_digest = p_proof_digest,
        updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
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
    PERFORM autopilot_queue._launch_assert_lt06_proof_class(p_proof_class);
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT * INTO v_row
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND OR v_row.lifecycle_state <> 'SEND_STARTED' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF v_row.outbound_attempt_count <> 1 THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF v_row.last_outbound_proof_class IS NULL
       OR v_row.last_outbound_proof_class <> p_proof_class THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'LAUNCH_REJECTED', updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    PERFORM autopilot_queue._budget_release_unused_reserve(p_tenant_id, p_launch_id);
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

REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA autopilot_queue FROM autopilot_queue_definer_owner;
REVOKE ALL ON ALL TABLES IN SCHEMA autopilot_queue FROM autopilot_queue_definer_owner;
GRANT USAGE ON SCHEMA autopilot_queue TO autopilot_queue_definer_owner;
