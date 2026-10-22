-- Enforcement activation gate. Not a numbered migration.
-- Apply only in the same release as the shipment-service Project that writes
-- company.carrier_relationship_activation_authorizations in the same transaction.
-- Installing this trigger in front of today's Project makes every headerless
-- activation fail at COMMIT, whether or not TMS_RELATIONSHIP_ACTIVATION_ENABLED
-- is set. The assert fails closed while the shared freight login is superuser,
-- bypasses row security, owns the protected tables, or can write pins or evidence.

SELECT transport.assert_activation_enforcement_role_split();

CREATE CONSTRAINT TRIGGER trg_new_activation_authorization
    AFTER INSERT OR DELETE
        OR UPDATE OF id, source_activation_id, supersedes_revision_id, operating_tenant_id
        ON transport.transport_execution_revisions
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION transport.enforce_new_activation_authorization();
