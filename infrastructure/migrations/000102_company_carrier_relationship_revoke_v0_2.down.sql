-- Refuses, before any drop, when a REVOKED version has a non-empty reason.
-- A refusal aborts this transaction and leaves the column, trigger, versions, and audit rows in place.
-- Empty reasons and ACTIVE versions do not block. Roots, versions, audit, and outbox are not deleted.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM company.carrier_relationship_versions
        WHERE status = 'REVOKED'
          AND revoke_reason IS NOT NULL
          AND btrim(revoke_reason) <> ''
    ) THEN
        RAISE EXCEPTION 'revoke reason blocks down'
            USING ERRCODE = '55000';
    END IF;
END $$;

DROP TRIGGER IF EXISTS cr_heads_transition_guard ON company.carrier_relationship_heads;
DROP FUNCTION IF EXISTS company.reject_relationship_head_illegal_transition();
DROP TRIGGER IF EXISTS cr_revoke_idempotency_tenant_guard ON company.carrier_relationship_revoke_idempotency;
DROP FUNCTION IF EXISTS company.enforce_revoke_idempotency_tenant();
ALTER TABLE company.carrier_relationship_versions DROP CONSTRAINT IF EXISTS chk_cr_version_revoke_reason;
ALTER TABLE company.carrier_relationship_versions DROP COLUMN IF EXISTS revoke_reason;
DROP TABLE IF EXISTS company.carrier_relationship_revoke_idempotency;
