-- Stand-in for the 000088 current-revision trigger. Not migration 000105.
-- A null pointer is allowed here, as it is in that trigger.
-- The read function rejects a null pointer and a non-ACTIVE revision.

CREATE FUNCTION transport.reject_current_revision_mismatch()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $guard$
BEGIN
    IF NEW.current_revision_id IS NULL THEN
        RETURN NEW;
    END IF;
    IF NOT EXISTS (
        SELECT 1
          FROM transport.transport_execution_revisions AS revision
         WHERE revision.id = NEW.current_revision_id
           AND revision.execution_id = NEW.id
           AND revision.operating_tenant_id = NEW.operating_tenant_id
    ) THEN
        RAISE EXCEPTION 'current revision does not belong to this execution'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$guard$;

CREATE TRIGGER transport_executions_current_revision_guard
    BEFORE INSERT OR UPDATE OF current_revision_id, operating_tenant_id
    ON transport.transport_executions
    FOR EACH ROW
    EXECUTE FUNCTION transport.reject_current_revision_mismatch();
