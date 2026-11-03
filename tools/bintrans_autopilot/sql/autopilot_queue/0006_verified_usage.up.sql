-- Verified usage authority: commits read server-stored reconciliation only.

DO $$
BEGIN
    CREATE ROLE autopilot_queue_reconciler NOLOGIN NOSUPERUSER NOBYPASSRLS
        NOCREATEROLE NOCREATEDB NOREPLICATION;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

CREATE TABLE autopilot_queue.budget_verified_usage (
    launch_id text PRIMARY KEY,
    tenant_id text NOT NULL,
    agent_id text NOT NULL,
    run_id text NOT NULL,
    raw_cost_text text,
    charged_text text,
    charged_known boolean NOT NULL,
    source text NOT NULL DEFAULT 'cursor_usage_api',
    verification_id text NOT NULL,
    payload_digest text NOT NULL,
    verified_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT budget_verified_usage_launch_check CHECK (launch_id ~ '^[A-Za-z0-9._:-]{1,120}$'),
    CONSTRAINT budget_verified_usage_tenant_check CHECK (tenant_id ~ '^[A-Za-z0-9._:-]{1,80}$'),
    CONSTRAINT budget_verified_usage_agent_check CHECK (
        agent_id ~ '^bc-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    ),
    CONSTRAINT budget_verified_usage_run_check CHECK (
        run_id ~ '^run-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    ),
    CONSTRAINT budget_verified_usage_source_check CHECK (char_length(source) BETWEEN 1 AND 80),
    CONSTRAINT budget_verified_usage_verification_check CHECK (char_length(verification_id) BETWEEN 8 AND 200),
    CONSTRAINT budget_verified_usage_digest_check CHECK (payload_digest ~ '^[0-9a-f]{64}$')
);

ALTER TABLE autopilot_queue.budget_verified_usage ENABLE ROW LEVEL SECURITY;
ALTER TABLE autopilot_queue.budget_verified_usage FORCE ROW LEVEL SECURITY;

CREATE POLICY budget_verified_usage_select ON autopilot_queue.budget_verified_usage
    FOR SELECT
    USING (
        tenant_id = autopilot_queue.bound_tenant()
        OR current_user = pg_get_userbyid(
            (SELECT c.relowner FROM pg_class c
             JOIN pg_namespace n ON n.oid = c.relnamespace
             WHERE n.nspname = 'autopilot_queue' AND c.relname = 'budget_verified_usage')
        )
    );

REVOKE ALL ON autopilot_queue.budget_verified_usage FROM PUBLIC;
GRANT SELECT ON autopilot_queue.budget_verified_usage TO autopilot_queue_runtime;

CREATE OR REPLACE FUNCTION autopilot_queue._budget_assert_reconciler()
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_owner name;
BEGIN
    SELECT pg_get_userbyid(c.relowner)
    INTO v_owner
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'autopilot_queue' AND c.relname = 'budget_reservations';
    IF session_user = v_owner THEN
        RETURN;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM pg_auth_members m
        JOIN pg_roles member ON member.oid = m.member
        JOIN pg_roles granted ON granted.oid = m.roleid
        WHERE member.rolname = session_user
          AND granted.rolname = 'autopilot_queue_reconciler'
    ) THEN
        RETURN;
    END IF;
    RAISE EXCEPTION 'reconciler_required';
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue._budget_assert_reconciler() FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue._budget_lock_global_gate_unbound()
RETURNS TABLE (
    per_launch_cap_cents bigint,
    daily_limit_cents bigint,
    monthly_limit_cents bigint
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
BEGIN
    RETURN QUERY
    SELECT g.per_launch_cap_cents, g.daily_limit_cents, g.monthly_limit_cents
    FROM autopilot_queue.budget_global_ledger AS g
    WHERE g.id = 1
    FOR UPDATE;
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue._budget_lock_global_gate_unbound() FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue._budget_commit_from_verified(p_launch_id text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_row autopilot_queue.budget_reservations%ROWTYPE;
    v_usage autopilot_queue.budget_verified_usage%ROWTYPE;
    v_daily_limit bigint;
    v_monthly_limit bigint;
    v_charged_cents bigint;
    v_over_reserve boolean;
    v_new_state text;
BEGIN
    SELECT * INTO v_usage
    FROM autopilot_queue.budget_verified_usage
    WHERE launch_id = p_launch_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'verified_usage_missing';
    END IF;
    IF NOT v_usage.charged_known THEN
        RAISE EXCEPTION 'unknown_usage';
    END IF;
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT * INTO v_row
    FROM autopilot_queue.budget_reservations
    WHERE launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    IF v_row.tenant_id <> v_usage.tenant_id THEN
        RAISE EXCEPTION 'tenant_mismatch';
    END IF;
    IF v_row.agent_id IS NOT NULL AND v_row.agent_id <> v_usage.agent_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    IF v_row.run_id IS NOT NULL AND v_row.run_id <> v_usage.run_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    IF v_row.state IN ('COMMITTED', 'OVERRUN') THEN
        IF v_row.raw_cost_text IS DISTINCT FROM v_usage.raw_cost_text
           OR v_row.charged_text IS DISTINCT FROM v_usage.charged_text
           OR v_row.agent_id IS DISTINCT FROM v_usage.agent_id
           OR v_row.run_id IS DISTINCT FROM v_usage.run_id THEN
            RAISE EXCEPTION 'idempotency_conflict';
        END IF;
        RETURN;
    END IF;
    IF v_row.state <> 'RESERVED' THEN
        RAISE EXCEPTION 'invalid_state';
    END IF;
    v_charged_cents := autopilot_queue._budget_charged_cents(v_usage.charged_text);
    v_over_reserve := v_charged_cents > v_row.reserve_cents;
    v_new_state := CASE WHEN v_over_reserve THEN 'OVERRUN' ELSE 'COMMITTED' END;
    UPDATE autopilot_queue.budget_reservations
    SET state = v_new_state,
        raw_cost_text = v_usage.raw_cost_text,
        charged_text = v_usage.charged_text,
        charged_known = true,
        agent_id = v_usage.agent_id,
        run_id = v_usage.run_id,
        usage_unknown = false,
        updated_at = clock_timestamp()
    WHERE launch_id = p_launch_id;
    SELECT daily_limit_cents, monthly_limit_cents
    INTO v_daily_limit, v_monthly_limit
    FROM autopilot_queue.budget_global_ledger
    WHERE id = 1;
    PERFORM autopilot_queue._budget_window_apply(
        'global', '', v_row.ledger_utc_day, v_row.ledger_utc_month,
        -v_row.reserve_cents, -v_row.reserve_cents,
        v_charged_cents, v_charged_cents,
        false, v_daily_limit, v_monthly_limit
    );
    SELECT daily_limit_cents, monthly_limit_cents
    INTO v_daily_limit, v_monthly_limit
    FROM autopilot_queue.budget_tenant_ledger
    WHERE tenant_id = v_usage.tenant_id;
    PERFORM autopilot_queue._budget_window_apply(
        'tenant', v_usage.tenant_id, v_row.ledger_utc_day, v_row.ledger_utc_month,
        -v_row.reserve_cents, -v_row.reserve_cents,
        v_charged_cents, v_charged_cents,
        false, v_daily_limit, v_monthly_limit
    );
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue._budget_commit_from_verified(text) FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue.budget_reconciler_apply_usage(
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
    v_existing autopilot_queue.budget_verified_usage%ROWTYPE;
BEGIN
    PERFORM autopilot_queue._budget_assert_reconciler();
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
    WHERE launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    IF v_row.agent_id IS NOT NULL AND v_row.agent_id <> p_agent_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    IF v_row.run_id IS NOT NULL AND v_row.run_id <> p_run_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    SELECT * INTO v_existing
    FROM autopilot_queue.budget_verified_usage
    WHERE launch_id = p_launch_id;
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
        WHERE launch_id = p_launch_id;
        RETURN;
    END IF;
    IF p_charged_text IS NULL OR char_length(trim(p_charged_text)) = 0 THEN
        RAISE EXCEPTION 'unknown_usage';
    END IF;
    PERFORM autopilot_queue._budget_charged_cents(p_charged_text);
    PERFORM autopilot_queue._budget_commit_from_verified(p_launch_id);
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue.budget_reconciler_apply_usage(
    text, text, text, text, text, boolean, text, text, text
) FROM PUBLIC;
GRANT USAGE ON SCHEMA autopilot_queue TO autopilot_queue_reconciler;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_reconciler_apply_usage(
    text, text, text, text, text, boolean, text, text, text
) TO autopilot_queue_reconciler;

REVOKE EXECUTE ON FUNCTION autopilot_queue.budget_protocol_commit(text, text, text, text, text)
    FROM autopilot_queue_runtime;
REVOKE EXECUTE ON FUNCTION autopilot_queue.budget_protocol_mark_unknown(text, text, text, text)
    FROM autopilot_queue_runtime;

DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_commit(text, text, text, text, text);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_mark_unknown(text, text, text, text);

DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_reserve(text, bigint);

CREATE OR REPLACE FUNCTION autopilot_queue.budget_protocol_reserve(
    p_launch_id text,
    p_reserve_cents bigint,
    p_agent_id text DEFAULT NULL,
    p_run_id text DEFAULT NULL
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_tenant text;
    v_day date;
    v_month date;
    v_per_launch bigint;
    v_g_daily bigint;
    v_g_monthly bigint;
    v_t_daily bigint;
    v_t_monthly bigint;
    v_dr bigint;
    v_dc bigint;
    v_mr bigint;
    v_mc bigint;
BEGIN
    IF p_reserve_cents <= 0 THEN
        RAISE EXCEPTION 'invalid_amount';
    END IF;
    IF p_agent_id IS NOT NULL
       AND p_agent_id !~ '^bc-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN
        RAISE EXCEPTION 'invalid_identity';
    END IF;
    IF p_run_id IS NOT NULL
       AND p_run_id !~ '^run-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN
        RAISE EXCEPTION 'invalid_identity';
    END IF;
    v_tenant := autopilot_queue._budget_bound_tenant();
    SELECT per_launch_cap_cents, daily_limit_cents, monthly_limit_cents
    INTO v_per_launch, v_g_daily, v_g_monthly
    FROM autopilot_queue._budget_lock_global_gate();
    PERFORM autopilot_queue._budget_assert_safety_after_lock();
    IF v_per_launch <= 0 OR p_reserve_cents > v_per_launch THEN
        RAISE EXCEPTION 'per_launch_cap_exceeded';
    END IF;
    SELECT utc_day, utc_month INTO v_day, v_month FROM autopilot_queue._budget_utc_windows();
    SELECT daily_limit_cents, monthly_limit_cents
    INTO v_t_daily, v_t_monthly
    FROM autopilot_queue.budget_tenant_ledger
    WHERE tenant_id = v_tenant
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'store_unavailable';
    END IF;
    INSERT INTO autopilot_queue.budget_window_usage (
        scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents
    ) VALUES ('global', '', 'day', v_day, 0, 0)
    ON CONFLICT (scope, tenant_id, bucket, bucket_start) DO NOTHING;
    INSERT INTO autopilot_queue.budget_window_usage (
        scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents
    ) VALUES ('global', '', 'month', v_month, 0, 0)
    ON CONFLICT (scope, tenant_id, bucket, bucket_start) DO NOTHING;
    INSERT INTO autopilot_queue.budget_window_usage (
        scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents
    ) VALUES ('tenant', v_tenant, 'day', v_day, 0, 0)
    ON CONFLICT (scope, tenant_id, bucket, bucket_start) DO NOTHING;
    INSERT INTO autopilot_queue.budget_window_usage (
        scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents
    ) VALUES ('tenant', v_tenant, 'month', v_month, 0, 0)
    ON CONFLICT (scope, tenant_id, bucket, bucket_start) DO NOTHING;
    SELECT reserved_cents, committed_cents INTO v_dr, v_dc
    FROM autopilot_queue.budget_window_usage
    WHERE scope = 'global' AND tenant_id = '' AND bucket = 'day' AND bucket_start = v_day
    FOR UPDATE;
    SELECT reserved_cents, committed_cents INTO v_mr, v_mc
    FROM autopilot_queue.budget_window_usage
    WHERE scope = 'global' AND tenant_id = '' AND bucket = 'month' AND bucket_start = v_month
    FOR UPDATE;
    IF v_dr + v_dc + p_reserve_cents > v_g_daily
       OR v_mr + v_mc + p_reserve_cents > v_g_monthly THEN
        RAISE EXCEPTION 'global_budget_exceeded';
    END IF;
    SELECT reserved_cents, committed_cents INTO v_dr, v_dc
    FROM autopilot_queue.budget_window_usage
    WHERE scope = 'tenant' AND tenant_id = v_tenant AND bucket = 'day' AND bucket_start = v_day
    FOR UPDATE;
    SELECT reserved_cents, committed_cents INTO v_mr, v_mc
    FROM autopilot_queue.budget_window_usage
    WHERE scope = 'tenant' AND tenant_id = v_tenant AND bucket = 'month' AND bucket_start = v_month
    FOR UPDATE;
    IF v_dr + v_dc + p_reserve_cents > v_t_daily
       OR v_mr + v_mc + p_reserve_cents > v_t_monthly THEN
        RAISE EXCEPTION 'tenant_budget_exceeded';
    END IF;
    PERFORM autopilot_queue._budget_assert_safety_after_lock();
    INSERT INTO autopilot_queue.budget_reservations (
        launch_id, tenant_id, state, reserve_cents, agent_id, run_id,
        block_reason, ledger_utc_day, ledger_utc_month, usage_unknown, charged_known
    ) VALUES (
        p_launch_id, v_tenant, 'RESERVED', p_reserve_cents, p_agent_id, p_run_id,
        NULL, v_day, v_month, false, false
    );
    PERFORM autopilot_queue._budget_window_apply(
        'global', '', v_day, v_month,
        p_reserve_cents, p_reserve_cents, 0, 0,
        true, v_g_daily, v_g_monthly
    );
    PERFORM autopilot_queue._budget_window_apply(
        'tenant', v_tenant, v_day, v_month,
        p_reserve_cents, p_reserve_cents, 0, 0,
        true, v_t_daily, v_t_monthly
    );
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_reserve(text, bigint, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_protocol_reserve(text, bigint, text, text)
    TO autopilot_queue_runtime;
