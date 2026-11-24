BEGIN;
SELECT shipment_id::text
  FROM transport.read_binding_assignment(
      '11111111-1111-1111-1111-111111111111',
      '22222222-2222-2222-2222-222222222222');
UPDATE transport.binding_authority_decisions
   SET consumed_at = clock_timestamp(),
       use_status = 'CONSUMED',
       binding_id = 'cdcdcdcd-cdcd-cdcd-cdcd-cdcdcdcdcdcd'
 WHERE decision_id = '66666666-6666-6666-6666-666666666666'
   AND use_status = 'ISSUED'
   AND consumed_at IS NULL;
SELECT count(*)::text
  FROM pg_locks AS l
  JOIN pg_class AS c ON c.oid = l.relation
 WHERE l.pid = pg_backend_pid()
   AND l.granted
   AND l.mode = 'RowShareLock'
   AND c.relname IN ('shipments', 'transport_executions', 'drivers', 'vehicles');
ROLLBACK;
