-- R6B-1-R1: tenant RLS, release safety, recovery fencing, composite reservation PK, definer owner.

DO $$
BEGIN
    CREATE ROLE autopilot_queue_definer_owner NOLOGIN NOSUPERUSER NOBYPASSRLS
        NOCREATEROLE NOCREATEDB NOREPLICATION;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

DO $$
BEGIN
    EXECUTE format('GRANT autopilot_queue_definer_owner TO %I', current_user);
EXCEPTION
    WHEN insufficient_privilege THEN NULL;
END
$$;

GRANT USAGE ON SCHEMA autopilot_queue TO autopilot_queue_definer_owner;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA autopilot_queue TO autopilot_queue_definer_owner;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA autopilot_queue TO autopilot_queue_definer_owner;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA autopilot_queue TO autopilot_queue_definer_owner;

ALTER TABLE autopilot_queue.budget_reservations DROP CONSTRAINT budget_reservations_pkey;
ALTER TABLE autopilot_queue.budget_reservations ADD PRIMARY KEY (tenant_id, launch_id);
DROP INDEX IF EXISTS autopilot_queue.budget_reservations_tenant_launch_uidx;

ALTER TABLE autopilot_queue.budget_verified_usage DROP CONSTRAINT budget_verified_usage_pkey;
ALTER TABLE autopilot_queue.budget_verified_usage ADD PRIMARY KEY (tenant_id, launch_id);

ALTER TABLE autopilot_queue.launch_intents
    ADD CONSTRAINT launch_intents_reservation_fk
    FOREIGN KEY (tenant_id, launch_id)
    REFERENCES autopilot_queue.budget_reservations (tenant_id, launch_id);

ALTER TABLE autopilot_queue.launch_intents ENABLE ROW LEVEL SECURITY;
ALTER TABLE autopilot_queue.launch_intents FORCE ROW LEVEL SECURITY;

REVOKE SELECT ON autopilot_queue.launch_intents FROM autopilot_queue_runtime;

CREATE OR REPLACE FUNCTION autopilot_queue._budget_release_unused_reserve(
    p_tenant_id text,
    p_launch_id text
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
SET row_security = off
AS $$
DECLARE
    v_row autopilot_queue.budget_reservations%ROWTYPE;
    v_daily_limit bigint;
    v_monthly_limit bigint;
BEGIN
    PERFORM autopilot_queue._launch_disable_rls();
    SELECT * INTO v_row
    FROM autopilot_queue.budget_reservations
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    IF v_row.state = 'RELEASED' THEN
        RETURN;
    END IF;
    IF v_row.state <> 'RESERVED' THEN
        RAISE EXCEPTION 'invalid_state';
    END IF;
    UPDATE autopilot_queue.budget_reservations
    SET state = 'RELEASED', updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    SELECT daily_limit_cents, monthly_limit_cents
    INTO v_daily_limit, v_monthly_limit
    FROM autopilot_queue.budget_global_ledger WHERE id = 1;
    PERFORM autopilot_queue._budget_window_apply(
        'global', '', v_row.ledger_utc_day, v_row.ledger_utc_month,
        -v_row.reserve_cents, -v_row.reserve_cents, 0, 0,
        false, v_daily_limit, v_monthly_limit
    );
    SELECT daily_limit_cents, monthly_limit_cents
    INTO v_daily_limit, v_monthly_limit
    FROM autopilot_queue.budget_tenant_ledger
    WHERE tenant_id = v_row.tenant_id;
    PERFORM autopilot_queue._budget_window_apply(
        'tenant', v_row.tenant_id, v_row.ledger_utc_day, v_row.ledger_utc_month,
        -v_row.reserve_cents, -v_row.reserve_cents, 0, 0,
        false, v_daily_limit, v_monthly_limit
    );
END;
$$;
REVOKE ALL ON FUNCTION autopilot_queue._budget_release_unused_reserve(text, text) FROM PUBLIC;

DROP FUNCTION IF EXISTS autopilot_queue._budget_release_unused_reserve(text);

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
    IF EXISTS (
        SELECT 1
        FROM autopilot_queue.launch_intents AS i
        WHERE i.tenant_id = p_row.tenant_id AND i.launch_id = p_row.launch_id
    ) THEN
        RAISE EXCEPTION 'launch_intent_recorded';
    END IF;
    IF p_row.agent_id IS NOT NULL OR p_row.run_id IS NOT NULL THEN
        RAISE EXCEPTION 'launch_identity_bound';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM autopilot_queue.budget_verified_usage AS u
        WHERE u.tenant_id = p_row.tenant_id AND u.launch_id = p_row.launch_id
    ) THEN
        RAISE EXCEPTION 'verified_usage_present';
    END IF;
END;
$$;

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
    IF v_res.agent_id IS NOT NULL AND v_res.agent_id <> p_planned_agent_id THEN
        RAISE EXCEPTION 'identity_conflict';
    END IF;
    IF v_res.run_id IS NOT NULL THEN
        RAISE EXCEPTION 'identity_conflict';
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
    PERFORM autopilot_queue._budget_release_unused_reserve(p_tenant_id, p_launch_id);
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
    v_res autopilot_queue.budget_reservations%ROWTYPE;
    v_recovery boolean;
BEGIN
    v_recovery := autopilot_queue._launch_has_role('autopilot_launch_recovery');
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_adapter') AND NOT v_recovery THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF p_run_id IS NULL OR char_length(trim(p_run_id)) = 0 THEN
        RAISE EXCEPTION 'invalid_input';
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
    IF v_row.lifecycle_state = 'LAUNCH_CONFIRMED' THEN
        IF v_row.agent_id = p_agent_id AND v_row.run_id = p_run_id THEN
            RETURN;
        END IF;
        RAISE EXCEPTION 'identity_conflict';
    END IF;
    IF v_recovery THEN
        IF v_row.lifecycle_state <> 'LAUNCH_UNKNOWN' THEN
            RAISE EXCEPTION 'invalid_caller_or_state';
        END IF;
        IF NOT autopilot_queue._launch_fence_ok(v_row, p_lease_token) THEN
            RAISE EXCEPTION 'invalid_lease';
        END IF;
    ELSE
        IF v_row.lifecycle_state <> 'SEND_STARTED' THEN
            RAISE EXCEPTION 'invalid_caller_or_state';
        END IF;
    END IF;
    IF p_agent_id <> v_row.planned_agent_id THEN
        RAISE EXCEPTION 'identity_conflict';
    END IF;
    IF v_row.agent_id IS NOT NULL AND (v_row.agent_id <> p_agent_id OR v_row.run_id <> p_run_id) THEN
        RAISE EXCEPTION 'identity_conflict';
    END IF;
    SELECT * INTO v_res
    FROM autopilot_queue.budget_reservations
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND OR v_res.state <> 'RESERVED' THEN
        RAISE EXCEPTION 'invalid_state';
    END IF;
    IF v_res.agent_id IS NOT NULL AND v_res.agent_id <> p_agent_id THEN
        RAISE EXCEPTION 'identity_conflict';
    END IF;
    IF v_res.run_id IS NOT NULL AND v_res.run_id <> p_run_id THEN
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
    UPDATE autopilot_queue.budget_reservations
    SET agent_id = p_agent_id, run_id = p_run_id, updated_at = clock_timestamp()
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
    PERFORM autopilot_queue._launch_assert_mock_mode();
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
    IF p_proof_class <> 'MOCK_TRUSTED' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF v_row.last_outbound_proof_class IS NOT NULL
       AND v_row.last_outbound_proof_class <> 'MOCK_TRUSTED' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'LAUNCH_REJECTED', updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    PERFORM autopilot_queue._budget_release_unused_reserve(p_tenant_id, p_launch_id);
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_recovery_lease_acquire(
    p_tenant_id text,
    p_launch_id text
)
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
SET row_security = off
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

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_escalate_manual(
    p_tenant_id text,
    p_launch_id text,
    p_lease_token uuid
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
SET row_security = off
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
    v_max integer;
BEGIN
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_recovery') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    PERFORM autopilot_queue._launch_disable_rls();
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT max_recovery_polls INTO v_max FROM autopilot_queue.autopilot_launch_control WHERE id = 1;
    SELECT * INTO v_row
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND OR v_row.lifecycle_state <> 'LAUNCH_UNKNOWN' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF NOT autopilot_queue._launch_fence_ok(v_row, p_lease_token) THEN
        RAISE EXCEPTION 'invalid_lease';
    END IF;
    IF v_row.recovery_poll_count < v_max THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'MANUAL_RECONCILIATION_REQUIRED', updated_at = clock_timestamp()
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
    PERFORM autopilot_queue._budget_release_unused_reserve(p_tenant_id, p_launch_id);
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
    IF p_run_id IS NULL OR char_length(trim(p_run_id)) = 0 THEN
        RAISE EXCEPTION 'invalid_input';
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
    UPDATE autopilot_queue.budget_reservations
    SET agent_id = p_agent_id, run_id = p_run_id, updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
END;
$$;

-- autopilot_queue_definer_owner is provisioned for a future table-owner alignment
-- migration; SECURITY DEFINER launch helpers keep the migration owner until then
-- so SET row_security = off remains effective under FORCE RLS.
