-- Budget authority remediation (Autopilot 0.3C2-R3).

ALTER TABLE autopilot_queue.budget_global_ledger
    ADD COLUMN IF NOT EXISTS per_launch_cap_cents bigint NOT NULL DEFAULT 0
    CHECK (per_launch_cap_cents >= 0);

UPDATE autopilot_queue.budget_global_ledger
SET per_launch_cap_cents = daily_limit_cents
WHERE per_launch_cap_cents = 0 AND daily_limit_cents > 0;

ALTER TABLE autopilot_queue.budget_window_usage ENABLE ROW LEVEL SECURITY;
ALTER TABLE autopilot_queue.budget_window_usage FORCE ROW LEVEL SECURITY;

CREATE POLICY budget_window_usage_select ON autopilot_queue.budget_window_usage
    FOR SELECT
    USING (
        scope = 'global'
        OR tenant_id = autopilot_queue.bound_tenant()
    );

REVOKE INSERT, UPDATE, DELETE ON autopilot_queue.budget_window_usage FROM autopilot_queue_runtime;

REVOKE EXECUTE ON FUNCTION autopilot_queue.budget_global_lock_row() FROM autopilot_queue_runtime;
REVOKE EXECUTE ON FUNCTION autopilot_queue.budget_global_write_counters(
    date, date, bigint, bigint, bigint, bigint
) FROM PUBLIC;

REVOKE ALL ON FUNCTION autopilot_queue._budget_bound_tenant() FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue._budget_assert_safety_after_lock() FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue._budget_window_apply(
    text, text, date, date, bigint, bigint, bigint, bigint, boolean, bigint, bigint
) FROM PUBLIC;

DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_insert_blocked(text, bigint, text, date, date);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_reserve(text, bigint, bigint, date, date);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_commit(text, text, text, bigint, text, text, boolean);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_release(text);
DROP FUNCTION IF EXISTS autopilot_queue.budget_protocol_mark_unknown(text, text, text, text);

CREATE OR REPLACE FUNCTION autopilot_queue._budget_utc_windows(
    OUT utc_day date,
    OUT utc_month date
)
LANGUAGE sql
STABLE
AS $$
    SELECT
        (timezone('UTC', clock_timestamp()))::date,
        date_trunc('month', timezone('UTC', clock_timestamp()))::date;
$$;

REVOKE ALL ON FUNCTION autopilot_queue._budget_utc_windows() FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue._budget_charged_cents(p_charged_text text)
RETURNS bigint
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
    v_num numeric;
BEGIN
    IF p_charged_text IS NULL OR btrim(p_charged_text) = '' THEN
        RETURN 0;
    END IF;
    BEGIN
        v_num := p_charged_text::numeric;
    EXCEPTION
        WHEN others THEN
            RAISE EXCEPTION 'invalid_amount';
    END;
    IF v_num IS NULL OR v_num = 'NaN'::numeric OR v_num = 'Infinity'::numeric OR v_num = '-Infinity'::numeric THEN
        RAISE EXCEPTION 'invalid_amount';
    END IF;
    IF v_num < 0 THEN
        RAISE EXCEPTION 'invalid_amount';
    END IF;
    RETURN CEIL(v_num)::bigint;
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue._budget_charged_cents(text) FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue._budget_lock_global_gate()
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
    PERFORM autopilot_queue._budget_bound_tenant();
    RETURN QUERY
    SELECT g.per_launch_cap_cents, g.daily_limit_cents, g.monthly_limit_cents
    FROM autopilot_queue.budget_global_ledger AS g
    WHERE g.id = 1
    FOR UPDATE;
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue._budget_lock_global_gate() FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue.budget_protocol_insert_blocked(
    p_launch_id text,
    p_reserve_cents bigint,
    p_block_reason text
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
BEGIN
    v_tenant := autopilot_queue._budget_bound_tenant();
    SELECT utc_day, utc_month INTO v_day, v_month FROM autopilot_queue._budget_utc_windows();
    INSERT INTO autopilot_queue.budget_reservations (
        launch_id, tenant_id, state, reserve_cents, block_reason,
        ledger_utc_day, ledger_utc_month, usage_unknown, charged_known
    ) VALUES (
        p_launch_id, v_tenant, 'BLOCKED', p_reserve_cents, p_block_reason,
        v_day, v_month, false, false
    );
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.budget_protocol_reserve(
    p_launch_id text,
    p_reserve_cents bigint
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
        launch_id, tenant_id, state, reserve_cents,
        block_reason, ledger_utc_day, ledger_utc_month, usage_unknown, charged_known
    ) VALUES (
        p_launch_id, v_tenant, 'RESERVED', p_reserve_cents,
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

CREATE OR REPLACE FUNCTION autopilot_queue.budget_protocol_commit(
    p_launch_id text,
    p_raw_cost_text text,
    p_charged_text text,
    p_agent_id text,
    p_run_id text
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
    v_charged_cents bigint;
    v_over_reserve boolean;
    v_new_state text;
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
    IF v_row.state IN ('COMMITTED', 'OVERRUN') THEN
        IF v_row.raw_cost_text IS DISTINCT FROM p_raw_cost_text
           OR v_row.charged_text IS DISTINCT FROM p_charged_text
           OR (p_agent_id IS NOT NULL AND v_row.agent_id IS DISTINCT FROM p_agent_id)
           OR (p_run_id IS NOT NULL AND v_row.run_id IS DISTINCT FROM p_run_id) THEN
            RAISE EXCEPTION 'idempotency_conflict';
        END IF;
        RETURN;
    END IF;
    IF v_row.state <> 'RESERVED' THEN
        RAISE EXCEPTION 'invalid_state';
    END IF;
    IF v_row.agent_id IS NOT NULL AND p_agent_id IS NOT NULL AND v_row.agent_id <> p_agent_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    IF v_row.run_id IS NOT NULL AND p_run_id IS NOT NULL AND v_row.run_id <> p_run_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    v_charged_cents := autopilot_queue._budget_charged_cents(p_charged_text);
    v_over_reserve := v_charged_cents > v_row.reserve_cents;
    v_new_state := CASE WHEN v_over_reserve THEN 'OVERRUN' ELSE 'COMMITTED' END;
    UPDATE autopilot_queue.budget_reservations
    SET state = v_new_state,
        raw_cost_text = p_raw_cost_text,
        charged_text = p_charged_text,
        charged_known = true,
        agent_id = COALESCE(p_agent_id, agent_id),
        run_id = COALESCE(p_run_id, run_id),
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
    WHERE tenant_id = v_tenant;
    PERFORM autopilot_queue._budget_window_apply(
        'tenant', v_tenant, v_row.ledger_utc_day, v_row.ledger_utc_month,
        -v_row.reserve_cents, -v_row.reserve_cents,
        v_charged_cents, v_charged_cents,
        false, v_daily_limit, v_monthly_limit
    );
END;
$$;

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
    IF v_row.state <> 'RESERVED' THEN
        RAISE EXCEPTION 'invalid_state';
    END IF;
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

CREATE OR REPLACE FUNCTION autopilot_queue.budget_protocol_mark_unknown(
    p_launch_id text,
    p_agent_id text,
    p_run_id text,
    p_raw_cost_text text
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_tenant text;
    v_row autopilot_queue.budget_reservations%ROWTYPE;
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
    IF v_row.state <> 'RESERVED' THEN
        RAISE EXCEPTION 'invalid_state';
    END IF;
    IF v_row.agent_id IS NOT NULL AND v_row.agent_id <> p_agent_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    IF v_row.run_id IS NOT NULL AND v_row.run_id <> p_run_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    UPDATE autopilot_queue.budget_reservations
    SET agent_id = p_agent_id,
        run_id = p_run_id,
        raw_cost_text = p_raw_cost_text,
        usage_unknown = true,
        updated_at = clock_timestamp()
    WHERE launch_id = p_launch_id;
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_insert_blocked(text, bigint, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_reserve(text, bigint) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_commit(text, text, text, text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_release(text) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_mark_unknown(text, text, text, text) FROM PUBLIC;

GRANT EXECUTE ON FUNCTION autopilot_queue.budget_protocol_insert_blocked(text, bigint, text) TO autopilot_queue_runtime;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_protocol_reserve(text, bigint) TO autopilot_queue_runtime;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_protocol_commit(text, text, text, text, text) TO autopilot_queue_runtime;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_protocol_release(text) TO autopilot_queue_runtime;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_protocol_mark_unknown(text, text, text, text) TO autopilot_queue_runtime;
