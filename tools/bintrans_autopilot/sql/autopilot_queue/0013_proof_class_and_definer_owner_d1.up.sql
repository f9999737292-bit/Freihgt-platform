-- R6B-1-R4: LT-06 MOCK-only proof class; SECURITY DEFINER ownership model D1.

CREATE OR REPLACE FUNCTION autopilot_queue._launch_proof_class_lt06_eligible(
    p_proof_class text
)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT p_proof_class = 'MOCK_TRUSTED';
$$;

REVOKE ALL ON FUNCTION autopilot_queue._launch_proof_class_lt06_eligible(text) FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue._launch_assert_lt06_proof_class(
    p_proof_class text
)
RETURNS void
LANGUAGE plpgsql
STABLE
AS $$
BEGIN
    IF p_proof_class IS NULL OR char_length(trim(p_proof_class)) = 0 THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF p_proof_class LIKE 'HTTP\_%' ESCAPE '\' THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF upper(p_proof_class) IN (
        'TIMEOUT', 'TRANSPORT_TIMEOUT', 'CONNECTION_RESET',
        'HTTP_404', 'HTTP_408', 'HTTP_409', 'HTTP_429',
        'HTTP_500', 'HTTP_502', 'HTTP_503', 'HTTP_504'
    ) THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    IF NOT autopilot_queue._launch_proof_class_lt06_eligible(p_proof_class) THEN
        RAISE EXCEPTION 'invalid_caller_or_state';
    END IF;
    PERFORM autopilot_queue._launch_assert_mock_mode();
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue._launch_assert_lt06_proof_class(text) FROM PUBLIC;

CREATE OR REPLACE FUNCTION autopilot_queue._launch_disable_rls()
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN;
END;
$$;

REVOKE ALL ON FUNCTION autopilot_queue._launch_disable_rls() FROM PUBLIC;

DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'jobs',
        'tenant_principals',
        'schema_migrations',
        'budget_reservations',
        'budget_tenant_ledger',
        'budget_global_ledger',
        'budget_window_usage',
        'budget_verified_usage',
        'launch_intents',
        'autopilot_launch_control',
        'manual_reconciliation_audit',
        'recovery_poll_audit',
        'budget_orphan_spend_signal'
    ]
    LOOP
        EXECUTE format(
            'DROP POLICY IF EXISTS definer_owner_d1 ON autopilot_queue.%I',
            t
        );
        EXECUTE format(
            'CREATE POLICY definer_owner_d1 ON autopilot_queue.%I '
            'FOR ALL TO autopilot_queue_definer_owner USING (true) WITH CHECK (true)',
            t
        );
    END LOOP;
END
$$;

REVOKE ALL ON ALL TABLES IN SCHEMA autopilot_queue FROM autopilot_queue_definer_owner;
GRANT USAGE ON SCHEMA autopilot_queue TO autopilot_queue_definer_owner;
GRANT SELECT, INSERT, UPDATE ON autopilot_queue.budget_reservations TO autopilot_queue_definer_owner;
GRANT SELECT, UPDATE ON autopilot_queue.budget_tenant_ledger TO autopilot_queue_definer_owner;
GRANT SELECT, UPDATE ON autopilot_queue.budget_global_ledger TO autopilot_queue_definer_owner;
GRANT SELECT, INSERT, UPDATE ON autopilot_queue.budget_window_usage TO autopilot_queue_definer_owner;
GRANT SELECT, INSERT, UPDATE ON autopilot_queue.budget_verified_usage TO autopilot_queue_definer_owner;
GRANT SELECT, INSERT, UPDATE ON autopilot_queue.launch_intents TO autopilot_queue_definer_owner;
GRANT SELECT, UPDATE ON autopilot_queue.autopilot_launch_control TO autopilot_queue_definer_owner;
GRANT SELECT, INSERT ON autopilot_queue.manual_reconciliation_audit TO autopilot_queue_definer_owner;
GRANT SELECT, INSERT ON autopilot_queue.recovery_poll_audit TO autopilot_queue_definer_owner;
GRANT SELECT, INSERT, UPDATE ON autopilot_queue.jobs TO autopilot_queue_definer_owner;
GRANT SELECT ON autopilot_queue.tenant_principals TO autopilot_queue_definer_owner;
GRANT SELECT, INSERT, DELETE ON autopilot_queue.schema_migrations TO autopilot_queue_definer_owner;
GRANT SELECT, INSERT, UPDATE ON autopilot_queue.budget_orphan_spend_signal TO autopilot_queue_definer_owner;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA autopilot_queue TO autopilot_queue_definer_owner;

DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT p.oid::regprocedure AS sig
        FROM pg_proc p
        JOIN pg_namespace n ON n.oid = p.pronamespace
        WHERE n.nspname = 'autopilot_queue'
          AND p.prokind = 'f'
          AND p.prosecdef
    LOOP
        EXECUTE format('ALTER FUNCTION %s OWNER TO autopilot_queue_definer_owner', r.sig);
    END LOOP;
END
$$;

DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT p.oid::regprocedure AS sig
        FROM pg_proc p
        JOIN pg_namespace n ON n.oid = p.pronamespace
        WHERE n.nspname = 'autopilot_queue'
          AND p.prokind = 'f'
          AND p.proconfig IS NOT NULL
          AND EXISTS (
              SELECT 1
              FROM unnest(p.proconfig) AS cfg(item)
              WHERE item LIKE 'row_security=%'
          )
    LOOP
        EXECUTE format('ALTER FUNCTION %s RESET row_security', r.sig);
    END LOOP;
END
$$;

REVOKE CREATE ON SCHEMA autopilot_queue FROM autopilot_queue_definer_owner;
