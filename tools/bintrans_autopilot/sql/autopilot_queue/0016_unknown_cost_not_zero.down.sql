-- Isolated-test rollback only. Restores the previous helper bodies.
-- Does not restore migrations below 0015 and does not drop ledger rows.

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

CREATE OR REPLACE FUNCTION autopilot_queue._budget_release_unused_reserve(
    p_tenant_id text,
    p_launch_id text
)
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
