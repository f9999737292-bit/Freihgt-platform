-- Test-only owner_epoch fence. Production has no owner_epoch column.
-- A writer may replace the epoch only with a greater bigint.
-- An operating-tenant change must carry a greater epoch in the same update.

CREATE FUNCTION transport.reject_owner_epoch_regression()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $guard$
BEGIN
    IF OLD.owner_epoch IS NOT NULL
       AND (NEW.owner_epoch IS NULL OR NEW.owner_epoch < OLD.owner_epoch) THEN
        RAISE EXCEPTION 'owner epoch cannot decrease' USING ERRCODE = '23514';
    END IF;
    IF NEW.operating_tenant_id IS DISTINCT FROM OLD.operating_tenant_id
       AND (NEW.owner_epoch IS NULL
            OR OLD.owner_epoch IS NULL
            OR NEW.owner_epoch <= OLD.owner_epoch) THEN
        RAISE EXCEPTION 'operating tenant change requires a newer owner epoch'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$guard$;

CREATE TRIGGER transport_executions_owner_epoch_guard
    BEFORE UPDATE OF owner_epoch, operating_tenant_id
    ON transport.transport_executions
    FOR EACH ROW
    EXECUTE FUNCTION transport.reject_owner_epoch_regression();
