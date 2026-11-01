-- R6B-1-R2: scope budget protocol and reconciler to (tenant_id, launch_id).

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
    WHERE tenant_id = v_tenant AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
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
    WHERE tenant_id = v_tenant AND launch_id = p_launch_id;
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
    WHERE tenant_id = v_tenant AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
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
    WHERE tenant_id = v_tenant AND launch_id = p_launch_id;
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
    WHERE tenant_id = v_tenant AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
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
    WHERE tenant_id = v_tenant AND launch_id = p_launch_id;
END;
$$;

DROP FUNCTION IF EXISTS autopilot_queue._budget_commit_from_verified(text);

CREATE OR REPLACE FUNCTION autopilot_queue._budget_commit_from_verified(
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
    v_usage autopilot_queue.budget_verified_usage%ROWTYPE;
    v_daily_limit bigint;
    v_monthly_limit bigint;
    v_charged_cents bigint;
    v_over_reserve boolean;
    v_new_state text;
BEGIN
    SELECT * INTO v_usage
    FROM autopilot_queue.budget_verified_usage
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'verified_usage_missing';
    END IF;
    IF NOT v_usage.charged_known THEN
        RAISE EXCEPTION 'unknown_usage';
    END IF;
    PERFORM 1 FROM autopilot_queue._budget_lock_global_gate_unbound();
    SELECT * INTO v_row
    FROM autopilot_queue.budget_reservations
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
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
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
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

REVOKE ALL ON FUNCTION autopilot_queue._budget_commit_from_verified(text, text) FROM PUBLIC;

DROP FUNCTION IF EXISTS autopilot_queue.budget_reconciler_apply_usage(
    text, text, text, text, text, boolean, text, text, text
);

CREATE OR REPLACE FUNCTION autopilot_queue.budget_reconciler_apply_usage(
    p_tenant_id text,
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
    IF p_tenant_id IS NULL OR char_length(trim(p_tenant_id)) = 0 THEN
        RAISE EXCEPTION 'invalid_input';
    END IF;
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
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found';
    END IF;
    IF v_row.agent_id IS NULL AND v_row.run_id IS NULL THEN
        IF EXISTS (
            SELECT 1
            FROM autopilot_queue.budget_reservations AS r
            WHERE r.run_id = p_run_id
              AND (r.tenant_id <> p_tenant_id OR r.launch_id <> p_launch_id)
        ) THEN
            RAISE EXCEPTION 'run_id_taken';
        END IF;
        UPDATE autopilot_queue.budget_reservations
        SET agent_id = p_agent_id,
            run_id = p_run_id,
            updated_at = clock_timestamp()
        WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
        SELECT * INTO v_row
        FROM autopilot_queue.budget_reservations
        WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
    ELSIF v_row.agent_id <> p_agent_id OR v_row.run_id <> p_run_id THEN
        RAISE EXCEPTION 'identity_mismatch';
    END IF;
    SELECT * INTO v_existing
    FROM autopilot_queue.budget_verified_usage
    WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
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
        WHERE tenant_id = p_tenant_id AND launch_id = p_launch_id;
        RETURN;
    END IF;
    IF p_charged_text IS NULL OR char_length(trim(p_charged_text)) = 0 THEN
        RAISE EXCEPTION 'unknown_usage';
    END IF;
    PERFORM autopilot_queue._budget_charged_cents(p_charged_text);
    PERFORM autopilot_queue._budget_commit_from_verified(p_tenant_id, p_launch_id);
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue.budget_reconciler_apply_usage(
    text, text, text, text, text, text, boolean, text, text, text
) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION autopilot_queue.budget_reconciler_apply_usage(
    text, text, text, text, text, text, boolean, text, text, text
) TO autopilot_queue_reconciler;
