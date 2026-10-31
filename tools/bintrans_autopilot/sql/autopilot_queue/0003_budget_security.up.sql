-- Budget controller security remediation (Autopilot 0.3C2-R1).

CREATE TABLE autopilot_queue.budget_global_ledger (
    id integer PRIMARY KEY CHECK (id = 1),
    utc_day date NOT NULL,
    utc_month date NOT NULL,
    daily_limit_cents bigint NOT NULL CHECK (daily_limit_cents >= 0),
    monthly_limit_cents bigint NOT NULL CHECK (monthly_limit_cents >= 0),
    daily_reserved_cents bigint NOT NULL DEFAULT 0 CHECK (daily_reserved_cents >= 0),
    monthly_reserved_cents bigint NOT NULL DEFAULT 0 CHECK (monthly_reserved_cents >= 0),
    daily_committed_cents bigint NOT NULL DEFAULT 0 CHECK (daily_committed_cents >= 0),
    monthly_committed_cents bigint NOT NULL DEFAULT 0 CHECK (monthly_committed_cents >= 0)
);

INSERT INTO autopilot_queue.budget_global_ledger (
    id, utc_day, utc_month, daily_limit_cents, monthly_limit_cents
) VALUES (
    1,
    (timezone('UTC', clock_timestamp()))::date,
    date_trunc('month', timezone('UTC', clock_timestamp()))::date,
    0,
    0
);

ALTER TABLE autopilot_queue.budget_reservations
    ADD COLUMN ledger_utc_day date,
    ADD COLUMN ledger_utc_month date,
    ADD COLUMN usage_unknown boolean NOT NULL DEFAULT false;

UPDATE autopilot_queue.budget_reservations
SET ledger_utc_day = (timezone('UTC', created_at))::date,
    ledger_utc_month = date_trunc('month', timezone('UTC', created_at))::date
WHERE ledger_utc_day IS NULL;

ALTER TABLE autopilot_queue.budget_reservations
    ALTER COLUMN ledger_utc_day SET NOT NULL,
    ALTER COLUMN ledger_utc_month SET NOT NULL;

ALTER TABLE autopilot_queue.budget_reservations
    DROP CONSTRAINT budget_reservations_state_check;

ALTER TABLE autopilot_queue.budget_reservations
    ADD CONSTRAINT budget_reservations_state_check CHECK (
        state IN ('REQUESTED', 'RESERVED', 'COMMITTED', 'RELEASED', 'BLOCKED', 'OVERRUN')
    );

CREATE POLICY budget_ledger_insert ON autopilot_queue.budget_tenant_ledger
    FOR INSERT
    WITH CHECK (tenant_id = autopilot_queue.bound_tenant());

CREATE POLICY budget_ledger_update ON autopilot_queue.budget_tenant_ledger
    FOR UPDATE
    USING (tenant_id = autopilot_queue.bound_tenant())
    WITH CHECK (tenant_id = autopilot_queue.bound_tenant());

CREATE POLICY budget_reservations_insert ON autopilot_queue.budget_reservations
    FOR INSERT
    WITH CHECK (tenant_id = autopilot_queue.bound_tenant());

CREATE POLICY budget_reservations_update ON autopilot_queue.budget_reservations
    FOR UPDATE
    USING (tenant_id = autopilot_queue.bound_tenant())
    WITH CHECK (tenant_id = autopilot_queue.bound_tenant());

GRANT INSERT, UPDATE ON autopilot_queue.budget_tenant_ledger TO autopilot_queue_runtime;
GRANT INSERT, UPDATE ON autopilot_queue.budget_reservations TO autopilot_queue_runtime;

ALTER TABLE autopilot_queue.budget_global_ledger ENABLE ROW LEVEL SECURITY;
ALTER TABLE autopilot_queue.budget_global_ledger FORCE ROW LEVEL SECURITY;

CREATE POLICY budget_global_select ON autopilot_queue.budget_global_ledger
    FOR SELECT
    USING (true);

GRANT SELECT ON autopilot_queue.budget_global_ledger TO autopilot_queue_runtime;

CREATE OR REPLACE FUNCTION autopilot_queue.budget_global_lock_row()
RETURNS TABLE (
    utc_day date,
    utc_month date,
    daily_limit_cents bigint,
    monthly_limit_cents bigint,
    daily_reserved_cents bigint,
    monthly_reserved_cents bigint,
    daily_committed_cents bigint,
    monthly_committed_cents bigint
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_bound text;
BEGIN
    v_bound := autopilot_queue.bound_tenant();
    IF v_bound IS NULL OR v_bound = '' THEN
        RAISE EXCEPTION 'tenant_unbound';
    END IF;
    RETURN QUERY
    SELECT g.utc_day, g.utc_month,
           g.daily_limit_cents, g.monthly_limit_cents,
           g.daily_reserved_cents, g.monthly_reserved_cents,
           g.daily_committed_cents, g.monthly_committed_cents
    FROM autopilot_queue.budget_global_ledger AS g
    WHERE g.id = 1
    FOR UPDATE;
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue.budget_global_lock_row() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_global_lock_row() TO autopilot_queue_runtime;

CREATE OR REPLACE FUNCTION autopilot_queue.budget_global_write_counters(
    p_utc_day date,
    p_utc_month date,
    p_daily_reserved_cents bigint,
    p_monthly_reserved_cents bigint,
    p_daily_committed_cents bigint,
    p_monthly_committed_cents bigint
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = autopilot_queue, pg_temp
AS $$
DECLARE
    v_bound text;
    v_daily_limit bigint;
    v_monthly_limit bigint;
BEGIN
    v_bound := autopilot_queue.bound_tenant();
    IF v_bound IS NULL OR v_bound = '' THEN
        RAISE EXCEPTION 'tenant_unbound';
    END IF;
    IF p_daily_reserved_cents < 0 OR p_monthly_reserved_cents < 0
       OR p_daily_committed_cents < 0 OR p_monthly_committed_cents < 0 THEN
        RAISE EXCEPTION 'invalid_amount';
    END IF;
    SELECT daily_limit_cents, monthly_limit_cents
    INTO v_daily_limit, v_monthly_limit
    FROM autopilot_queue.budget_global_ledger
    WHERE id = 1
    FOR UPDATE;
    IF p_daily_reserved_cents + p_daily_committed_cents > v_daily_limit
       OR p_monthly_reserved_cents + p_monthly_committed_cents > v_monthly_limit THEN
        RAISE EXCEPTION 'global_budget_exceeded';
    END IF;
    UPDATE autopilot_queue.budget_global_ledger
    SET utc_day = p_utc_day,
        utc_month = p_utc_month,
        daily_reserved_cents = p_daily_reserved_cents,
        monthly_reserved_cents = p_monthly_reserved_cents,
        daily_committed_cents = p_daily_committed_cents,
        monthly_committed_cents = p_monthly_committed_cents
    WHERE id = 1;
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue.budget_global_write_counters(
    date, date, bigint, bigint, bigint, bigint
) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_global_write_counters(
    date, date, bigint, bigint, bigint, bigint
) TO autopilot_queue_runtime;

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
        ELSE NULL
    END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue.budget_global_safety_block_reason() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_global_safety_block_reason() TO autopilot_queue_runtime;
