-- Roll back R6B-1-R1 security remediation (restore v9-compatible protocol surface).

ALTER TABLE autopilot_queue.launch_intents NO FORCE ROW LEVEL SECURITY;
ALTER TABLE autopilot_queue.launch_intents DISABLE ROW LEVEL SECURITY;
GRANT SELECT ON autopilot_queue.launch_intents TO autopilot_queue_runtime;

ALTER TABLE autopilot_queue.launch_intents DROP CONSTRAINT IF EXISTS launch_intents_reservation_fk;

ALTER TABLE autopilot_queue.budget_verified_usage DROP CONSTRAINT budget_verified_usage_pkey;
ALTER TABLE autopilot_queue.budget_verified_usage ADD PRIMARY KEY (launch_id);

ALTER TABLE autopilot_queue.budget_reservations DROP CONSTRAINT budget_reservations_pkey;
ALTER TABLE autopilot_queue.budget_reservations ADD PRIMARY KEY (launch_id);

CREATE UNIQUE INDEX IF NOT EXISTS budget_reservations_tenant_launch_uidx
    ON autopilot_queue.budget_reservations (tenant_id, launch_id);

DROP FUNCTION IF EXISTS autopilot_queue._budget_release_unused_reserve(text, text);
DROP FUNCTION IF EXISTS autopilot_queue._budget_release_unused_reserve(text);

CREATE OR REPLACE FUNCTION autopilot_queue._budget_release_unused_reserve(p_launch_id text)
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

-- Function bodies restored to v9 by re-applying 0009_launch_lifecycle_r6b1.up.sql on full rebuild;
-- isolated down keeps schema at v9 for migration tests.
