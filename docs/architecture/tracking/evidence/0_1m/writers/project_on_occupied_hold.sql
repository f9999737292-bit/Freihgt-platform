-- Flag-on Project. Sleeps after the vehicle lock and before the new execution INSERT.
-- The existing execution row is not locked during the sleep.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(
    pg_catalog.hashtextextended(
        'relationship:' || '00000000-0000-0000-0000-00000000000a'
            || ':' || '99999999-9999-9999-9999-999999999999'
            || ':' || '00000000-0000-0000-0000-00000000000b',
        0
    )
);
SELECT status
  FROM company.carrier_relationship_heads
 WHERE relationship_id = '88888888-8888-8888-8888-888888888888'
   FOR UPDATE;
SELECT version
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222'
   AND tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND deleted_at IS NULL
   FOR UPDATE;
SELECT version
  FROM transport.cargoes
 WHERE id = '44444444-4444-4444-4444-444444444444'
   AND tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND deleted_at IS NULL
   FOR UPDATE;
SELECT id
  FROM transport.drivers
 WHERE id = '77777777-7777-7777-7777-777777777777'
   FOR UPDATE;
SELECT id
  FROM transport.vehicles
 WHERE id = '33333333-3333-3333-3333-333333333333'
   FOR UPDATE;
SELECT pg_sleep(15);
INSERT INTO transport.transport_executions (
    id, operating_tenant_id, driver_id, vehicle_id, current_revision_id, event_seq
) VALUES (
    '13131313-1313-1313-1313-131313131313', '00000000-0000-0000-0000-00000000000b',
    '77777777-7777-7777-7777-777777777777', '33333333-3333-3333-3333-333333333333', NULL, 0
);
INSERT INTO transport.transport_execution_revisions (
    id, execution_id, operating_tenant_id, status, version
) VALUES (
    '55555555-5555-5555-5555-555555555553', '13131313-1313-1313-1313-131313131313',
    '00000000-0000-0000-0000-00000000000b', 'ACTIVE', 1
);
UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-555555555553', updated_at = now()
 WHERE id = '13131313-1313-1313-1313-131313131313';
INSERT INTO transport.transport_execution_active_shipments (
    shipment_tenant_id, shipment_id, execution_id
) VALUES (
    '00000000-0000-0000-0000-00000000000a', '22222222-2222-2222-2222-222222222222',
    '13131313-1313-1313-1313-131313131313'
);
COMMIT;
