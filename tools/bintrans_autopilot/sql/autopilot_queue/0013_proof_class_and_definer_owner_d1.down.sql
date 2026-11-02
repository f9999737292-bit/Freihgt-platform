-- Restore pre-D1 launch RLS helper; ownership reverted by MigrationRunner.

CREATE OR REPLACE FUNCTION autopilot_queue._launch_disable_rls()
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM set_config('row_security', 'off', true);
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
    END LOOP;
END
$$;
