-- R6B-1: admin launch transitions, identity immutability, protocol hardening.

CREATE UNIQUE INDEX IF NOT EXISTS budget_reservations_tenant_launch_uidx
    ON autopilot_queue.budget_reservations (tenant_id, launch_id);

CREATE OR REPLACE FUNCTION autopilot_queue._launch_intents_immutable_ids_trigger()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP <> 'UPDATE' THEN
        RETURN NEW;
    END IF;
    IF OLD.planned_agent_id IS DISTINCT FROM NEW.planned_agent_id THEN
        RAISE EXCEPTION 'identity_immutable';
    END IF;
    IF OLD.agent_id IS NOT NULL AND OLD.agent_id IS DISTINCT FROM NEW.agent_id THEN
        RAISE EXCEPTION 'identity_immutable';
    END IF;
    IF OLD.run_id IS NOT NULL AND OLD.run_id IS DISTINCT FROM NEW.run_id THEN
        RAISE EXCEPTION 'identity_immutable';
    END IF;
    IF OLD.create_body_version IS DISTINCT FROM NEW.create_body_version
       OR OLD.create_body_hash IS DISTINCT FROM NEW.create_body_hash THEN
        RAISE EXCEPTION 'identity_immutable';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS launch_intents_immutable_ids ON autopilot_queue.launch_intents;
CREATE TRIGGER launch_intents_immutable_ids
    BEFORE UPDATE ON autopilot_queue.launch_intents
    FOR EACH ROW
    EXECUTE FUNCTION autopilot_queue._launch_intents_immutable_ids_trigger();

REVOKE UPDATE ON autopilot_queue.launch_intents
    FROM autopilot_launch_adapter, autopilot_launch_recovery, autopilot_launch_admin;

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_record_intent(
    p_tenant_id text,
    p_launch_id text,
    p_planned_agent_id text,
    p_create_body_version integer,
    p_create_body_hash text
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
SET row_security = off
AS $$
DECLARE
    v_res autopilot_queue.budget_reservations%ROWTYPE;
    v_existing autopilot_queue.launch_intents%ROWTYPE;
BEGIN
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_adapter') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    PERFORM autopilot_queue._launch_disable_rls();
    PERFORM autopilot_queue._launch_assert_mock_mode();
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT * INTO v_res
    FROM autopilot_queue.budget_reservations
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    IF v_res.state <> 'RESERVED' THEN
        RAISE EXCEPTION 'invalid_state';
    END IF;
    SELECT * INTO v_existing
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    IF FOUND THEN
        IF v_existing.planned_agent_id <> p_planned_agent_id
           OR v_existing.create_body_version <> p_create_body_version
           OR v_existing.create_body_hash <> p_create_body_hash THEN
            RAISE EXCEPTION 'idempotency_conflict';
        END IF;
        RETURN;
    END IF;
    INSERT INTO autopilot_queue.launch_intents (
        tenant_id, launch_id, lifecycle_state, planned_agent_id,
        create_body_version, create_body_hash
    ) VALUES (
        p_tenant_id, p_launch_id, 'INTENT', p_planned_agent_id,
        p_create_body_version, p_create_body_hash
    );
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_mark_unknown(
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
    IF v_recovery AND NOT autopilot_queue._launch_fence_ok(v_row, p_lease_token) THEN
        RAISE EXCEPTION 'invalid_lease';
    END IF;
    IF v_row.lifecycle_state = 'SEND_STARTED' AND v_recovery THEN
        v_stale := v_row.send_started_at IS NOT NULL
            AND clock_timestamp() > v_row.send_started_at + make_interval(secs => v_ctrl.send_stale_seconds);
        IF NOT v_stale THEN
            RAISE EXCEPTION 'invalid_caller_or_state';
        END IF;
    ELSIF v_row.lifecycle_state NOT IN ('SEND_STARTED', 'LAUNCH_UNKNOWN') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'LAUNCH_UNKNOWN', updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_confirm(
    p_tenant_id text,
    p_launch_id text,
    p_agent_id text,
    p_run_id text,
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
    v_recovery boolean;
BEGIN
    v_recovery := autopilot_queue._launch_has_role('autopilot_launch_recovery');
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_adapter') AND NOT v_recovery THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    PERFORM autopilot_queue._launch_disable_rls();
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT * INTO v_row
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    IF v_recovery AND NOT autopilot_queue._launch_fence_ok(v_row, p_lease_token) THEN
        RAISE EXCEPTION 'invalid_lease';
    END IF;
    IF v_row.lifecycle_state = 'LAUNCH_CONFIRMED' THEN
        IF v_row.agent_id = p_agent_id AND v_row.run_id = p_run_id THEN
            RETURN;
        END IF;
        RAISE EXCEPTION 'identity_conflict';
    END IF;
    IF v_row.lifecycle_state NOT IN ('SEND_STARTED', 'LAUNCH_UNKNOWN') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF p_agent_id <> v_row.planned_agent_id THEN
        RAISE EXCEPTION 'identity_conflict';
    END IF;
    IF v_row.agent_id IS NOT NULL AND (v_row.agent_id <> p_agent_id OR v_row.run_id <> p_run_id) THEN
        RAISE EXCEPTION 'identity_conflict';
    END IF;
    IF EXISTS (
        SELECT 1 FROM autopilot_queue.launch_intents AS i
        WHERE i.run_id = p_run_id
          AND (i.tenant_id <> p_tenant_id OR i.launch_id <> p_launch_id)
    ) THEN
        RAISE EXCEPTION 'run_id_taken';
    END IF;
    IF EXISTS (
        SELECT 1 FROM autopilot_queue.launch_intents AS i
        WHERE i.agent_id = p_agent_id
          AND (i.tenant_id <> p_tenant_id OR i.launch_id <> p_launch_id)
    ) THEN
        RAISE EXCEPTION 'agent_id_taken';
    END IF;
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'LAUNCH_CONFIRMED',
        agent_id = COALESCE(agent_id, p_agent_id),
        run_id = COALESCE(run_id, p_run_id),
        updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_manual_confirm(
    p_tenant_id text,
    p_launch_id text,
    p_agent_id text,
    p_run_id text,
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
    IF p_agent_id <> v_row.planned_agent_id THEN
        RAISE EXCEPTION 'identity_conflict';
    END IF;
    IF EXISTS (
        SELECT 1 FROM autopilot_queue.launch_intents AS i
        WHERE i.run_id = p_run_id
          AND (i.tenant_id <> p_tenant_id OR i.launch_id <> p_launch_id)
    ) THEN
        RAISE EXCEPTION 'run_id_taken';
    END IF;
    IF EXISTS (
        SELECT 1 FROM autopilot_queue.launch_intents AS i
        WHERE i.agent_id = p_agent_id
          AND (i.tenant_id <> p_tenant_id OR i.launch_id <> p_launch_id)
    ) THEN
        RAISE EXCEPTION 'agent_id_taken';
    END IF;
    INSERT INTO autopilot_queue.manual_reconciliation_audit (
        tenant_id, launch_id, actor_session_user, action, reason
    ) VALUES (
        p_tenant_id, p_launch_id, session_user, 'manual_confirm', p_reason
    );
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'LAUNCH_CONFIRMED',
        agent_id = COALESCE(agent_id, p_agent_id),
        run_id = COALESCE(run_id, p_run_id),
        updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
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

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_reopen_unknown(
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
        p_tenant_id, p_launch_id, session_user, 'reopen_unknown', p_reason
    );
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'LAUNCH_UNKNOWN',
        recovery_poll_count = 0,
        recovery_lease_holder = NULL,
        recovery_lease_token = NULL,
        recovery_lease_expires_at = NULL,
        updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_mark_external_cancelled(
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
    IF NOT FOUND OR v_row.lifecycle_state <> 'LAUNCH_CONFIRMED' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF p_proof_class <> 'MOCK_TRUSTED' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'EXTERNAL_AGENT_CANCELLED', updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_manual_confirm(
    text, text, text, text, text
) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_manual_confirm(
    text, text, text, text, text
) TO autopilot_launch_admin;

REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_manual_reject(text, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_manual_reject(text, text, text)
    TO autopilot_launch_admin;

REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_reopen_unknown(text, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_reopen_unknown(text, text, text)
    TO autopilot_launch_admin;

REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_mark_external_cancelled(text, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_mark_external_cancelled(text, text, text)
    TO autopilot_launch_adapter;

GRANT INSERT ON autopilot_queue.manual_reconciliation_audit TO autopilot_launch_admin;
