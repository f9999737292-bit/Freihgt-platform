BEGIN;
UPDATE transport.binding_authority_decisions
   SET consumed_at = clock_timestamp(),
       use_status = 'CONSUMED',
       binding_id = 'd1d1d1d1-d1d1-d1d1-d1d1-d1d1d1d1d1d1'
 WHERE decision_id = '66666666-6666-6666-6666-666666666675'
   AND use_status = 'ISSUED'
   AND consumed_at IS NULL;
DO $do$
DECLARE
    d record;
    a record;
BEGIN
    SELECT * INTO d
      FROM transport.binding_authority_decisions
     WHERE decision_id = '66666666-6666-6666-6666-666666666675';
    SELECT * INTO a
      FROM transport.read_binding_assignment(d.execution_id, d.shipment_id);
    IF d.owner_epoch IS DISTINCT FROM a.owner_epoch THEN
        RAISE EXCEPTION 'stale owner epoch' USING ERRCODE = '40001';
    END IF;
    RAISE EXCEPTION 'comparison did not fail' USING ERRCODE = 'P0001';
END
$do$;
COMMIT;
