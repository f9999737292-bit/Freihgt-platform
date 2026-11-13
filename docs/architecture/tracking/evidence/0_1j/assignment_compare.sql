BEGIN;
SELECT set_config('proof.decision_id', :'decision', true);
SELECT shipment_tenant_id::text
    || '|' || driver_id::text
    || '|' || coalesce(vehicle_id::text, 'null')
    || '|' || coalesce(execution_id::text, 'null')
    || '|' || coalesce(current_revision_id::text, 'null')
    || '|' || coalesce(revision_version::text, 'null')
    || '|' || coalesce(operating_tenant_id::text, 'null')
  FROM transport.read_binding_assignment(
      (SELECT execution_id FROM transport.binding_authority_decisions WHERE decision_id = current_setting('proof.decision_id')::uuid),
      (SELECT shipment_id FROM transport.binding_authority_decisions WHERE decision_id = current_setting('proof.decision_id')::uuid));
DO $do$
DECLARE
    d record;
    a record;
BEGIN
    SELECT * INTO d
      FROM transport.binding_authority_decisions
     WHERE decision_id = current_setting('proof.decision_id')::uuid;
    SELECT * INTO a
      FROM transport.read_binding_assignment(d.execution_id, d.shipment_id);
    IF d.shipment_tenant_id IS DISTINCT FROM a.shipment_tenant_id THEN
        RAISE EXCEPTION 'tenant mismatch' USING ERRCODE = '23514';
    END IF;
    IF d.driver_id IS DISTINCT FROM a.driver_id THEN
        RAISE EXCEPTION 'driver mismatch' USING ERRCODE = '23514';
    END IF;
    IF d.vehicle_id IS DISTINCT FROM a.vehicle_id THEN
        RAISE EXCEPTION 'vehicle mismatch' USING ERRCODE = '23514';
    END IF;
    IF d.execution_id IS NOT NULL AND d.owner_epoch = 'ABSENT' THEN
        RAISE EXCEPTION 'execution target blocked while owner epoch is absent' USING ERRCODE = '42501';
    END IF;
    IF d.execution_id IS NOT NULL AND d.owner_epoch IS DISTINCT FROM 'CURRENT' THEN
        RAISE EXCEPTION 'stale owner epoch' USING ERRCODE = '40001';
    END IF;
    IF d.execution_id IS NULL AND a.execution_id IS NOT NULL THEN
        RAISE EXCEPTION 'REAUTHORIZATION_REQUIRED' USING ERRCODE = '40001';
    END IF;
    IF d.execution_id IS NOT NULL AND (
           d.current_revision_id IS DISTINCT FROM a.current_revision_id
        OR d.revision_version IS DISTINCT FROM a.revision_version
    ) THEN
        RAISE EXCEPTION 'revision mismatch' USING ERRCODE = '40001';
    END IF;
    RAISE EXCEPTION 'comparison did not fail' USING ERRCODE = 'P0001';
END
$do$;
ROLLBACK;
