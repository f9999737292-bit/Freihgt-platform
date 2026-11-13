BEGIN;
SELECT shipment_id::text || '|' || driver_id::text || '|' || coalesce(execution_id::text, 'null')
  FROM transport.read_binding_assignment(
      NULL,
      '00000000-0000-0000-0000-000000000006');
SELECT count(*)::text
  FROM pg_locks AS l
  JOIN pg_class AS c ON c.oid = l.relation
 WHERE l.pid = pg_backend_pid()
   AND c.relname = 'transport_executions'
   AND l.mode = 'RowShareLock';
SELECT pg_sleep(4);
INSERT INTO tracking.shipment_tracking_binding (
    binding_id, shipment_tenant_id, shipment_id, driver_id, vehicle_id, execution_id, decision_id
) VALUES (
    'd0d0d0d0-d0d0-d0d0-d0d0-d0d0d0d0d0d0',
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    '00000000-0000-0000-0000-000000000006',
    '33333333-3333-3333-3333-333333333333',
    NULL,
    NULL,
    '66666666-6666-6666-6666-666666666676'
);
UPDATE transport.binding_authority_decisions
   SET consumed_at = clock_timestamp(),
       use_status = 'CONSUMED',
       binding_id = 'd0d0d0d0-d0d0-d0d0-d0d0-d0d0d0d0d0d0'
 WHERE decision_id = '66666666-6666-6666-6666-666666666676'
   AND use_status = 'ISSUED'
   AND consumed_at IS NULL
   AND execution_id IS NULL;
COMMIT;
