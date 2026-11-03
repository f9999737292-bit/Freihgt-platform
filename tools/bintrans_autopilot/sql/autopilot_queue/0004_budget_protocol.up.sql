-- Budget protocol hardening (Autopilot 0.3C2-R2).

CREATE TABLE autopilot_queue.budget_window_usage (
    scope text NOT NULL CHECK (scope IN ('global', 'tenant')),
    tenant_id text NOT NULL DEFAULT '' CHECK (char_length(tenant_id) <= 80),
    bucket text NOT NULL CHECK (bucket IN ('day', 'month')),
    bucket_start date NOT NULL,
    reserved_cents bigint NOT NULL DEFAULT 0 CHECK (reserved_cents >= 0),
    committed_cents bigint NOT NULL DEFAULT 0 CHECK (committed_cents >= 0),
    PRIMARY KEY (scope, tenant_id, bucket, bucket_start)
);

INSERT INTO autopilot_queue.budget_window_usage (scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents)
SELECT 'global', '', 'day', utc_day, daily_reserved_cents, daily_committed_cents
FROM autopilot_queue.budget_global_ledger
WHERE id = 1;

INSERT INTO autopilot_queue.budget_window_usage (scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents)
SELECT 'global', '', 'month', utc_month, monthly_reserved_cents, monthly_committed_cents
FROM autopilot_queue.budget_global_ledger
WHERE id = 1;

INSERT INTO autopilot_queue.budget_window_usage (scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents)
SELECT 'tenant', tenant_id, 'day', utc_day, daily_reserved_cents, daily_committed_cents
FROM autopilot_queue.budget_tenant_ledger;

INSERT INTO autopilot_queue.budget_window_usage (scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents)
SELECT 'tenant', tenant_id, 'month', utc_month, monthly_reserved_cents, monthly_committed_cents
FROM autopilot_queue.budget_tenant_ledger;

REVOKE INSERT, UPDATE ON autopilot_queue.budget_tenant_ledger FROM autopilot_queue_runtime;
REVOKE INSERT, UPDATE ON autopilot_queue.budget_reservations FROM autopilot_queue_runtime;
REVOKE EXECUTE ON FUNCTION autopilot_queue.budget_global_write_counters(
    date, date, bigint, bigint, bigint, bigint
) FROM autopilot_queue_runtime;

DROP POLICY IF EXISTS budget_ledger_insert ON autopilot_queue.budget_tenant_ledger;
DROP POLICY IF EXISTS budget_ledger_update ON autopilot_queue.budget_tenant_ledger;
DROP POLICY IF EXISTS budget_reservations_insert ON autopilot_queue.budget_reservations;
DROP POLICY IF EXISTS budget_reservations_update ON autopilot_queue.budget_reservations;

CREATE OR REPLACE FUNCTION autopilot_queue._budget_bound_tenant()
RETURNS text
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    v_bound text;
BEGIN
    v_bound := autopilot_queue.bound_tenant();
    IF v_bound IS NULL OR v_bound = '' THEN
        RAISE EXCEPTION 'tenant_unbound';
    END IF;
    RETURN v_bound;
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue._budget_assert_safety_after_lock()
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    v_reason text;
BEGIN
    v_reason := autopilot_queue.budget_global_safety_block_reason();
    IF v_reason IS NOT NULL THEN
        RAISE EXCEPTION '%', v_reason;
    END IF;
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue._budget_window_apply(
    p_scope text,
    p_tenant_id text,
    p_day date,
    p_month date,
    p_daily_reserved_delta bigint,
    p_monthly_reserved_delta bigint,
    p_daily_committed_delta bigint,
    p_monthly_committed_delta bigint,
    p_enforce_limits boolean,
    p_daily_limit bigint,
    p_monthly_limit bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    v_dr bigint := 0;
    v_dc bigint := 0;
    v_mr bigint := 0;
    v_mc bigint := 0;
BEGIN
    INSERT INTO autopilot_queue.budget_window_usage (
        scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents
    ) VALUES (
        p_scope, p_tenant_id, 'day', p_day, 0, 0
    )
    ON CONFLICT (scope, tenant_id, bucket, bucket_start) DO NOTHING;

    INSERT INTO autopilot_queue.budget_window_usage (
        scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents
    ) VALUES (
        p_scope, p_tenant_id, 'month', p_month, 0, 0
    )
    ON CONFLICT (scope, tenant_id, bucket, bucket_start) DO NOTHING;

    SELECT reserved_cents, committed_cents
    INTO v_dr, v_dc
    FROM autopilot_queue.budget_window_usage
    WHERE scope = p_scope
      AND tenant_id = p_tenant_id
      AND bucket = 'day'
      AND bucket_start = p_day
    FOR UPDATE;

    SELECT reserved_cents, committed_cents
    INTO v_mr, v_mc
    FROM autopilot_queue.budget_window_usage
    WHERE scope = p_scope
      AND tenant_id = p_tenant_id
      AND bucket = 'month'
      AND bucket_start = p_month
    FOR UPDATE;

    v_dr := v_dr + p_daily_reserved_delta;
    v_dc := v_dc + p_daily_committed_delta;
    v_mr := v_mr + p_monthly_reserved_delta;
    v_mc := v_mc + p_monthly_committed_delta;

    IF v_dr < 0 OR v_dc < 0 OR v_mr < 0 OR v_mc < 0 THEN
        RAISE EXCEPTION 'invalid_amount';
    END IF;

    IF p_enforce_limits THEN
        IF v_dr + v_dc > p_daily_limit OR v_mr + v_mc > p_monthly_limit THEN
            RAISE EXCEPTION 'global_budget_exceeded';
        END IF;
    END IF;

    UPDATE autopilot_queue.budget_window_usage
    SET reserved_cents = v_dr, committed_cents = v_dc
    WHERE scope = p_scope
      AND tenant_id = p_tenant_id
      AND bucket = 'day'
      AND bucket_start = p_day;

    UPDATE autopilot_queue.budget_window_usage
    SET reserved_cents = v_mr, committed_cents = v_mc
    WHERE scope = p_scope
      AND tenant_id = p_tenant_id
      AND bucket = 'month'
      AND bucket_start = p_month;
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.budget_protocol_insert_blocked(
    p_launch_id text,
    p_reserve_cents bigint,
    p_block_reason text,
    p_utc_day date,
    p_utc_month date
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_tenant text;
BEGIN
    v_tenant := autopilot_queue._budget_bound_tenant();
    INSERT INTO autopilot_queue.budget_reservations (
        launch_id, tenant_id, state, reserve_cents, block_reason,
        ledger_utc_day, ledger_utc_month, usage_unknown, charged_known
    ) VALUES (
        p_launch_id, v_tenant, 'BLOCKED', p_reserve_cents, p_block_reason,
        p_utc_day, p_utc_month, false, false
    );
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.budget_protocol_reserve(
    p_launch_id text,
    p_reserve_cents bigint,
    p_per_launch_cap bigint,
    p_utc_day date,
    p_utc_month date
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_tenant text;
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
    IF p_per_launch_cap <= 0 OR p_reserve_cents > p_per_launch_cap THEN
        RAISE EXCEPTION 'per_launch_cap_exceeded';
    END IF;
    v_tenant := autopilot_queue._budget_bound_tenant();
    PERFORM autopilot_queue._budget_assert_safety_after_lock();
    SELECT daily_limit_cents, monthly_limit_cents
    INTO v_g_daily, v_g_monthly
    FROM autopilot_queue.budget_global_ledger
    WHERE id = 1
    FOR UPDATE;
    PERFORM autopilot_queue._budget_assert_safety_after_lock();
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
    ) VALUES ('global', '', 'day', p_utc_day, 0, 0)
    ON CONFLICT (scope, tenant_id, bucket, bucket_start) DO NOTHING;
    INSERT INTO autopilot_queue.budget_window_usage (
        scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents
    ) VALUES ('global', '', 'month', p_utc_month, 0, 0)
    ON CONFLICT (scope, tenant_id, bucket, bucket_start) DO NOTHING;
    INSERT INTO autopilot_queue.budget_window_usage (
        scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents
    ) VALUES ('tenant', v_tenant, 'day', p_utc_day, 0, 0)
    ON CONFLICT (scope, tenant_id, bucket, bucket_start) DO NOTHING;
    INSERT INTO autopilot_queue.budget_window_usage (
        scope, tenant_id, bucket, bucket_start, reserved_cents, committed_cents
    ) VALUES ('tenant', v_tenant, 'month', p_utc_month, 0, 0)
    ON CONFLICT (scope, tenant_id, bucket, bucket_start) DO NOTHING;
    SELECT reserved_cents, committed_cents INTO v_dr, v_dc
    FROM autopilot_queue.budget_window_usage
    WHERE scope = 'global' AND tenant_id = '' AND bucket = 'day' AND bucket_start = p_utc_day
    FOR UPDATE;
    SELECT reserved_cents, committed_cents INTO v_mr, v_mc
    FROM autopilot_queue.budget_window_usage
    WHERE scope = 'global' AND tenant_id = '' AND bucket = 'month' AND bucket_start = p_utc_month
    FOR UPDATE;
    IF v_dr + v_dc + p_reserve_cents > v_g_daily
       OR v_mr + v_mc + p_reserve_cents > v_g_monthly THEN
        RAISE EXCEPTION 'global_budget_exceeded';
    END IF;
    SELECT reserved_cents, committed_cents INTO v_dr, v_dc
    FROM autopilot_queue.budget_window_usage
    WHERE scope = 'tenant' AND tenant_id = v_tenant AND bucket = 'day' AND bucket_start = p_utc_day
    FOR UPDATE;
    SELECT reserved_cents, committed_cents INTO v_mr, v_mc
    FROM autopilot_queue.budget_window_usage
    WHERE scope = 'tenant' AND tenant_id = v_tenant AND bucket = 'month' AND bucket_start = p_utc_month
    FOR UPDATE;
    IF v_dr + v_dc + p_reserve_cents > v_t_daily
       OR v_mr + v_mc + p_reserve_cents > v_t_monthly THEN
        RAISE EXCEPTION 'tenant_budget_exceeded';
    END IF;
    INSERT INTO autopilot_queue.budget_reservations (
        launch_id, tenant_id, state, reserve_cents,
        block_reason, ledger_utc_day, ledger_utc_month, usage_unknown, charged_known
    ) VALUES (
        p_launch_id, v_tenant, 'RESERVED', p_reserve_cents,
        NULL, p_utc_day, p_utc_month, false, false
    );
    PERFORM autopilot_queue._budget_window_apply(
        'global', '', p_utc_day, p_utc_month,
        p_reserve_cents, p_reserve_cents, 0, 0,
        true, v_g_daily, v_g_monthly
    );
    PERFORM autopilot_queue._budget_window_apply(
        'tenant', v_tenant, p_utc_day, p_utc_month,
        p_reserve_cents, p_reserve_cents, 0, 0,
        true, v_t_daily, v_t_monthly
    );
END;
$$;

CREATE OR REPLACE FUNCTION autopilot_queue.budget_protocol_commit(
    p_launch_id text,
    p_raw_cost_text text,
    p_charged_text text,
    p_charged_cents bigint,
    p_agent_id text,
    p_run_id text,
    p_over_reserve boolean
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
    v_new_state text;
BEGIN
    v_tenant := autopilot_queue._budget_bound_tenant();
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
    IF v_row.state NOT IN ('RESERVED', 'COMMITTED', 'OVERRUN') THEN
        RAISE EXCEPTION 'invalid_state';
    END IF;
    IF v_row.state IN ('COMMITTED', 'OVERRUN') THEN
        RETURN;
    END IF;
    IF v_row.agent_id IS NOT NULL AND p_agent_id IS NOT NULL AND v_row.agent_id <> p_agent_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    IF v_row.run_id IS NOT NULL AND p_run_id IS NOT NULL AND v_row.run_id <> p_run_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    v_new_state := CASE WHEN p_over_reserve THEN 'OVERRUN' ELSE 'COMMITTED' END;
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
        p_charged_cents, p_charged_cents,
        false, v_daily_limit, v_monthly_limit
    );
    SELECT daily_limit_cents, monthly_limit_cents
    INTO v_daily_limit, v_monthly_limit
    FROM autopilot_queue.budget_tenant_ledger
    WHERE tenant_id = v_tenant;
    PERFORM autopilot_queue._budget_window_apply(
        'tenant', v_tenant, v_row.ledger_utc_day, v_row.ledger_utc_month,
        -v_row.reserve_cents, -v_row.reserve_cents,
        p_charged_cents, p_charged_cents,
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

REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_insert_blocked(text, bigint, text, date, date) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_reserve(text, bigint, bigint, date, date) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_commit(text, text, text, bigint, text, text, boolean) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_release(text) FROM PUBLIC;
REVOKE ALL ON FUNCTION autopilot_queue.budget_protocol_mark_unknown(text, text, text, text) FROM PUBLIC;

GRANT EXECUTE ON FUNCTION autopilot_queue.budget_protocol_insert_blocked(text, bigint, text, date, date) TO autopilot_queue_runtime;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_protocol_reserve(text, bigint, bigint, date, date) TO autopilot_queue_runtime;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_protocol_commit(text, text, text, bigint, text, text, boolean) TO autopilot_queue_runtime;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_protocol_release(text) TO autopilot_queue_runtime;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_protocol_mark_unknown(text, text, text, text) TO autopilot_queue_runtime;

GRANT SELECT ON autopilot_queue.budget_window_usage TO autopilot_queue_runtime;
