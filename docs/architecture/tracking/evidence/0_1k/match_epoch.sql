BEGIN;
SELECT execution_id::text
    || '|' || coalesce(owner_epoch::text, 'null')
    || '|' || coalesce(current_revision_id::text, 'null')
    || '|' || coalesce(revision_version::text, 'null')
    || '|' || coalesce(operating_tenant_id::text, 'null')
  FROM transport.read_binding_assignment(
      '11111111-1111-1111-1111-111111111111',
      '22222222-2222-2222-2222-222222222222');
DO $do$
DECLARE
    d record;
    a record;
BEGIN
    SELECT * INTO d
      FROM transport.binding_authority_decisions
     WHERE decision_id = '66666666-6666-6666-6666-666666666666';
    SELECT * INTO a
      FROM transport.read_binding_assignment(d.execution_id, d.shipment_id);
    IF a.execution_id IS DISTINCT FROM d.execution_id THEN
        RAISE EXCEPTION 'returned execution does not match the decision' USING ERRCODE = '23514';
    END IF;
    IF d.owner_epoch IS DISTINCT FROM a.owner_epoch THEN
        RAISE EXCEPTION 'stale owner epoch' USING ERRCODE = '40001';
    END IF;
    IF d.current_revision_id IS DISTINCT FROM a.current_revision_id
       OR d.revision_version IS DISTINCT FROM a.revision_version THEN
        RAISE EXCEPTION 'revision mismatch' USING ERRCODE = '40001';
    END IF;
END
$do$;
COMMIT;
