-- Launch lifecycle protocol (Autopilot 0.3C2-R6B). Platform head stays 000105.

DO $$
BEGIN
    CREATE ROLE autopilot_launch_adapter NOLOGIN NOSUPERUSER NOBYPASSRLS
        NOCREATEROLE NOCREATEDB NOREPLICATION;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
DO $$
BEGIN
    CREATE ROLE autopilot_launch_recovery NOLOGIN NOSUPERUSER NOBYPASSRLS
        NOCREATEROLE NOCREATEDB NOREPLICATION;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
DO $$
BEGIN
    CREATE ROLE autopilot_launch_admin NOLOGIN NOSUPERUSER NOBYPASSRLS
        NOCREATEROLE NOCREATEDB NOREPLICATION;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE autopilot_queue.autopilot_launch_control (
    id integer PRIMARY KEY CHECK (id = 1),
    autopilot_launch_mode text NOT NULL DEFAULT 'DISABLED'
        CHECK (autopilot_launch_mode IN ('DISABLED', 'MOCK_ONLY')),
    autopilot_paid_launch_enabled boolean NOT NULL DEFAULT false,
    autopilot_estop boolean NOT NULL DEFAULT false,
    max_recovery_polls integer NOT NULL DEFAULT 24 CHECK (max_recovery_polls >= 1),
    intent_stale_seconds integer NOT NULL DEFAULT 3600 CHECK (intent_stale_seconds >= 60),
    send_stale_seconds integer NOT NULL DEFAULT 3600 CHECK (send_stale_seconds >= 60),
    recovery_lease_seconds integer NOT NULL DEFAULT 300 CHECK (recovery_lease_seconds >= 30)
);

INSERT INTO autopilot_queue.autopilot_launch_control (id) VALUES (1)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE autopilot_queue.launch_intents (
    tenant_id text NOT NULL,
    launch_id text NOT NULL,
    lifecycle_state text NOT NULL,
    planned_agent_id text NOT NULL,
    agent_id text,
    run_id text,
    create_body_version integer NOT NULL,
    create_body_hash text NOT NULL,
    intent_recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    send_started_at timestamptz,
    outbound_attempt_count integer NOT NULL DEFAULT 0 CHECK (outbound_attempt_count >= 0),
    last_outbound_proof_class text,
    last_outbound_proof_digest text,
    recovery_lease_holder text,
    recovery_lease_expires_at timestamptz,
    recovery_lease_token uuid,
    recovery_poll_count integer NOT NULL DEFAULT 0 CHECK (recovery_poll_count >= 0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, launch_id),
    CONSTRAINT launch_intents_tenant_check CHECK (tenant_id ~ '^[A-Za-z0-9._:-]{1,80}$'),
    CONSTRAINT launch_intents_launch_check CHECK (launch_id ~ '^[A-Za-z0-9._:-]{1,120}$'),
    CONSTRAINT launch_intents_state_check CHECK (
        lifecycle_state IN (
            'INTENT', 'SEND_STARTED', 'LAUNCH_UNKNOWN', 'MANUAL_RECONCILIATION_REQUIRED',
            'LAUNCH_CONFIRMED', 'LAUNCH_REJECTED', 'CANCELLED_BEFORE_SEND', 'EXTERNAL_AGENT_CANCELLED'
        )
    ),
    CONSTRAINT launch_intents_planned_agent_check CHECK (
        planned_agent_id ~ '^bc-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    ),
    CONSTRAINT launch_intents_agent_check CHECK (
        agent_id IS NULL OR agent_id ~ '^bc-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    ),
    CONSTRAINT launch_intents_run_check CHECK (
        run_id IS NULL OR run_id ~ '^run-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    ),
    CONSTRAINT launch_intents_body_check CHECK (
        create_body_version >= 1 AND create_body_hash ~ '^[0-9a-f]{64}$'
    )
);

CREATE UNIQUE INDEX launch_intents_agent_id_uidx ON autopilot_queue.launch_intents (agent_id)
    WHERE agent_id IS NOT NULL;
CREATE UNIQUE INDEX launch_intents_run_id_uidx ON autopilot_queue.launch_intents (run_id)
    WHERE run_id IS NOT NULL;

CREATE TABLE autopilot_queue.recovery_poll_audit (
    id bigserial PRIMARY KEY,
    tenant_id text NOT NULL,
    launch_id text NOT NULL,
    poll_count integer NOT NULL,
    actor_session_user text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE autopilot_queue.manual_reconciliation_audit (
    id bigserial PRIMARY KEY,
    tenant_id text NOT NULL,
    launch_id text NOT NULL,
    actor_session_user text NOT NULL,
    action text NOT NULL,
    reason text,
    proof_digest text,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE autopilot_queue.budget_orphan_spend_signal (
    id bigserial PRIMARY KEY,
    tenant_id text NOT NULL,
    launch_id text NOT NULL,
    detected_cents bigint NOT NULL CHECK (detected_cents >= 0),
    source text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    acknowledged_at timestamptz
);

REVOKE ALL ON autopilot_queue.launch_intents FROM PUBLIC;
REVOKE ALL ON autopilot_queue.autopilot_launch_control FROM PUBLIC;
REVOKE ALL ON autopilot_queue.recovery_poll_audit FROM PUBLIC;
REVOKE ALL ON autopilot_queue.manual_reconciliation_audit FROM PUBLIC;
REVOKE ALL ON autopilot_queue.budget_orphan_spend_signal FROM PUBLIC;

GRANT SELECT ON autopilot_queue.launch_intents TO autopilot_queue_runtime;
GRANT SELECT ON autopilot_queue.autopilot_launch_control TO autopilot_queue_runtime;

CREATE OR REPLACE FUNCTION autopilot_queue._launch_has_role(p_role name)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $$
    SELECT EXISTS (
        SELECT 1
        FROM pg_auth_members m
        JOIN pg_roles member ON member.oid = m.member
        JOIN pg_roles granted ON granted.oid = m.roleid
        WHERE member.rolname = session_user AND granted.rolname = p_role
    );
$$;
REVOKE ALL ON FUNCTION autopilot_queue._launch_has_role(name) FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue._launch_control_row()
RETURNS autopilot_queue.autopilot_launch_control
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
    SELECT * FROM autopilot_queue.autopilot_launch_control WHERE id = 1;
$$;
REVOKE ALL ON FUNCTION autopilot_queue._launch_control_row() FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue._launch_assert_mock_mode()
RETURNS void
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    v_mode text;
BEGIN
    SELECT autopilot_launch_mode INTO v_mode FROM autopilot_queue._launch_control_row();
    IF v_mode <> 'MOCK_ONLY' THEN
        RAISE EXCEPTION 'launch_mode_blocked';
    END IF;
END;
$$;
REVOKE ALL ON FUNCTION autopilot_queue._launch_assert_mock_mode() FROM PUBLIC;

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
REVOKE ALL ON FUNCTION autopilot_queue._launch_fence_ok(autopilot_queue.launch_intents, uuid) FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue._budget_release_unused_reserve(p_launch_id text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_row autopilot_queue.budget_reservations%ROWTYPE;
    v_daily_limit bigint;
    v_monthly_limit bigint;
BEGIN
    SELECT * INTO v_row
    FROM autopilot_queue.budget_reservations
    WHERE launch_id = p_launch_id
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
    WHERE launch_id = p_launch_id;
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
REVOKE ALL ON FUNCTION autopilot_queue._budget_release_unused_reserve(text) FROM PUBLIC;

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
        WHEN EXISTS (
            SELECT 1
            FROM autopilot_queue.launch_intents
            WHERE lifecycle_state IN ('LAUNCH_UNKNOWN', 'MANUAL_RECONCILIATION_REQUIRED')
            LIMIT 1
        ) THEN 'launch_reconciliation_open'
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
        WHERE u.launch_id = p_row.launch_id
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
AS $$
DECLARE
    v_res autopilot_queue.budget_reservations%ROWTYPE;
BEGIN
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_adapter') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    PERFORM autopilot_queue._launch_assert_mock_mode();
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT * INTO v_res
    FROM autopilot_queue.budget_reservations
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND OR v_res.state <> 'RESERVED' THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    INSERT INTO autopilot_queue.launch_intents (
        tenant_id, launch_id, lifecycle_state, planned_agent_id,
        create_body_version, create_body_hash
    ) VALUES (
        p_tenant_id, p_launch_id, 'INTENT', p_planned_agent_id,
        p_create_body_version, p_create_body_hash
    )
    ON CONFLICT (tenant_id, launch_id) DO NOTHING;
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
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
    v_recovery boolean;
BEGIN
    v_recovery := autopilot_queue._launch_has_role('autopilot_launch_recovery');
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_adapter') AND NOT v_recovery THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
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
    IF v_row.lifecycle_state NOT IN ('INTENT', 'SEND_STARTED') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'CANCELLED_BEFORE_SEND', updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    PERFORM autopilot_queue._budget_release_unused_reserve(p_launch_id);
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_mark_send_started(
    p_tenant_id text,
    p_launch_id text
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
    v_ctrl autopilot_queue.autopilot_launch_control%ROWTYPE;
BEGIN
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_adapter') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    SELECT * INTO v_ctrl FROM autopilot_queue.autopilot_launch_control WHERE id = 1 FOR UPDATE;
    IF v_ctrl.autopilot_estop THEN
        RAISE EXCEPTION 'estop_active';
    END IF;
    PERFORM autopilot_queue._launch_assert_mock_mode();
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT * INTO v_row
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND OR v_row.lifecycle_state <> 'INTENT' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'SEND_STARTED',
        send_started_at = clock_timestamp(),
        updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
END;
$$;

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
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
BEGIN
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_adapter') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
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

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_mark_unknown(
    p_tenant_id text,
    p_launch_id text,
    p_lease_token uuid DEFAULT NULL
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
    v_recovery boolean;
BEGIN
    v_recovery := autopilot_queue._launch_has_role('autopilot_launch_recovery');
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_adapter') AND NOT v_recovery THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
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
    IF v_row.lifecycle_state NOT IN ('SEND_STARTED', 'LAUNCH_UNKNOWN') THEN
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
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
    v_recovery boolean;
BEGIN
    v_recovery := autopilot_queue._launch_has_role('autopilot_launch_recovery');
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_adapter') AND NOT v_recovery THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
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
    IF v_row.lifecycle_state NOT IN ('SEND_STARTED', 'LAUNCH_UNKNOWN') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF p_agent_id <> v_row.planned_agent_id THEN
        RAISE EXCEPTION 'identity_conflict';
    END IF;
    IF v_row.agent_id IS NOT NULL AND (v_row.agent_id <> p_agent_id OR v_row.run_id <> p_run_id) THEN
        RAISE EXCEPTION 'identity_conflict';
    END IF;
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'LAUNCH_CONFIRMED',
        agent_id = COALESCE(agent_id, p_agent_id),
        run_id = COALESCE(run_id, p_run_id),
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
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
BEGIN
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_adapter') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
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
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'LAUNCH_REJECTED', updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    PERFORM autopilot_queue._budget_release_unused_reserve(p_launch_id);
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
    SELECT * INTO v_ctrl FROM autopilot_queue.autopilot_launch_control WHERE id = 1;
    SELECT * INTO v_row
    FROM autopilot_queue.launch_intents
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
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

CREATE OR REPLACE FUNCTION autopilot_queue.launch_protocol_recovery_poll_attempt(
    p_tenant_id text,
    p_launch_id text,
    p_lease_token uuid
)
RETURNS TABLE (poll_count integer, decision text)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
    v_max integer;
    v_new integer;
BEGIN
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_recovery') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    PERFORM 1
    FROM autopilot_queue.budget_reservations
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
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
    SELECT max_recovery_polls INTO v_max FROM autopilot_queue.autopilot_launch_control WHERE id = 1;
    v_new := v_row.recovery_poll_count + 1;
    UPDATE autopilot_queue.launch_intents
    SET recovery_poll_count = v_new, updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    INSERT INTO autopilot_queue.recovery_poll_audit (
        tenant_id, launch_id, poll_count, actor_session_user
    ) VALUES (p_tenant_id, p_launch_id, v_new, session_user);
    poll_count := v_new;
    decision := CASE WHEN v_new >= v_max THEN 'MANUAL' ELSE 'CONTINUE' END;
    RETURN NEXT;
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
AS $$
DECLARE
    v_row autopilot_queue.launch_intents%ROWTYPE;
BEGIN
    IF NOT autopilot_queue._launch_has_role('autopilot_launch_recovery') THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
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
    UPDATE autopilot_queue.launch_intents
    SET lifecycle_state = 'MANUAL_RECONCILIATION_REQUIRED', updated_at = clock_timestamp()
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.budget_orphan_spend_signal(
    p_tenant_id text,
    p_launch_id text,
    p_detected_cents bigint,
    p_source text
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
BEGIN
    IF NOT (
        autopilot_queue._launch_has_role('autopilot_launch_admin')
        OR EXISTS (SELECT 1 FROM pg_roles WHERE rolname = session_user AND rolsuper)
    ) THEN
        PERFORM autopilot_queue._budget_assert_reconciler();
    END IF;
    INSERT INTO autopilot_queue.budget_orphan_spend_signal (
        tenant_id, launch_id, detected_cents, source
    ) VALUES (p_tenant_id, p_launch_id, p_detected_cents, p_source);
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_record_intent(text, text, text, integer, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_record_intent(text, text, text, integer, text)
    TO autopilot_launch_adapter;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_cancel_before_send(text, text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_cancel_before_send(text, text, uuid)
    TO autopilot_launch_adapter, autopilot_launch_recovery;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_mark_send_started(text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_mark_send_started(text, text) TO autopilot_launch_adapter;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_report_outbound_result(text, text, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_report_outbound_result(text, text, text, text)
    TO autopilot_launch_adapter;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_mark_unknown(text, text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_mark_unknown(text, text, uuid)
    TO autopilot_launch_adapter, autopilot_launch_recovery;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_confirm(text, text, text, text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_confirm(text, text, text, text, uuid)
    TO autopilot_launch_adapter, autopilot_launch_recovery;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_reject_definite(text, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_reject_definite(text, text, text) TO autopilot_launch_adapter;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_recovery_lease_acquire(text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_recovery_lease_acquire(text, text) TO autopilot_launch_recovery;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_recovery_poll_attempt(text, text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_recovery_poll_attempt(text, text, uuid) TO autopilot_launch_recovery;
REVOKE ALL ON FUNCTION autopilot_queue.launch_protocol_escalate_manual(text, text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.launch_protocol_escalate_manual(text, text, uuid) TO autopilot_launch_recovery;
REVOKE ALL ON FUNCTION autopilot_queue.budget_orphan_spend_signal(text, text, bigint, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_orphan_spend_signal(text, text, bigint, text)
    TO autopilot_launch_admin, autopilot_queue_reconciler;

GRANT USAGE ON SCHEMA autopilot_queue TO autopilot_launch_adapter, autopilot_launch_recovery, autopilot_launch_admin;
