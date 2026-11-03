-- Release safety and launch identity binding (R5).

CREATE UNIQUE INDEX IF NOT EXISTS budget_reservations_run_id_uidx
    ON autopilot_queue.budget_reservations (run_id)
    WHERE run_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS budget_verified_usage_run_id_uidx
    ON autopilot_queue.budget_verified_usage (run_id);

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

REVOKE ALL ON FUNCTION autopilot_queue._budget_assert_release_allowed(autopilot_queue.budget_reservations) FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue.budget_protocol_release(
    p_launch_id text
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_tenant text;
    v_row autopilot_queue.budget_reservations%ROWTYPE;
    v_daily_limit bigint;
    v_monthly_limit bigint;
BEGIN
    v_tenant := autopilot_queue._budget_bound_tenant();
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate();
    SELECT * INTO v_row
    FROM autopilot_queue.budget_reservations
    WHERE launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    IF v_row.tenant_id <> v_tenant THEN
        RAISE EXCEPTION 'tenant_mismatch';
    END IF;
    IF v_row.state = 'RELEASED' THEN
        RETURN;
    END IF;
    IF v_row.state = 'OVERRUN' THEN
        RAISE EXCEPTION 'overrun_open';
    END IF;
    IF v_row.state = 'COMMITTED' THEN
        RAISE EXCEPTION 'invalid_state';
    END IF;
    IF v_row.state <> 'RESERVED' THEN
        RAISE EXCEPTION 'invalid_state';
    END IF;
    PERFORM autopilot_queue._budget_assert_release_allowed(v_row);
    UPDATE autopilot_queue.budget_reservations
    SET state = 'RELEASED', updated_at = clock_timestamp()
    WHERE launch_id = p_launch_id;
    SELECT daily_limit_cents, monthly_limit_cents
    INTO v_daily_limit, v_monthly_limit
    FROM autopilot_queue.budget_global_ledger
    WHERE id = 1;
    PERFORM autopilot_queue._budget_window_apply(
        'global', '', v_row.ledger_utc_day, v_row.ledger_utc_month,
        -v_row.reserve_cents, -v_row.reserve_cents, 0, 0,
        false, v_daily_limit, v_monthly_limit
    );
    SELECT daily_limit_cents, monthly_limit_cents
    INTO v_daily_limit, v_monthly_limit
    FROM autopilot_queue.budget_tenant_ledger
    WHERE tenant_id = v_tenant;
    PERFORM autopilot_queue._budget_window_apply(
        'tenant', v_tenant, v_row.ledger_utc_day, v_row.ledger_utc_month,
        -v_row.reserve_cents, -v_row.reserve_cents, 0, 0,
        false, v_daily_limit, v_monthly_limit
    );
END;
$$;

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
    IF (p_agent_id IS NULL) <> (p_run_id IS NULL) THEN
        RAISE EXCEPTION 'identity_incomplete';
    END IF;
    IF p_agent_id IS NOT NULL
       AND p_agent_id !~ '^bc-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN
        RAISE EXCEPTION 'invalid_identity';
    END IF;
    IF p_run_id IS NOT NULL
       AND p_run_id !~ '^run-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN
        RAISE EXCEPTION 'invalid_identity';
    END IF;
    IF p_run_id IS NOT NULL AND EXISTS (
        SELECT 1
        FROM autopilot_queue.budget_reservations AS r
        WHERE r.run_id = p_run_id
    ) THEN
        RAISE EXCEPTION 'run_id_taken';
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
    IF v_row.agent_id IS NULL AND v_row.run_id IS NULL THEN
        IF EXISTS (
            SELECT 1
            FROM autopilot_queue.budget_reservations AS r
            WHERE r.run_id = p_run_id AND r.launch_id <> p_launch_id
        ) THEN
            RAISE EXCEPTION 'run_id_taken';
        END IF;
        UPDATE autopilot_queue.budget_reservations
        SET agent_id = p_agent_id,
            run_id = p_run_id,
            updated_at = clock_timestamp()
        WHERE launch_id = p_launch_id;
        SELECT * INTO v_row
        FROM autopilot_queue.budget_reservations
        WHERE launch_id = p_launch_id;
    ELSIF v_row.agent_id <> p_agent_id OR v_row.run_id <> p_run_id THEN
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
