-- Project with TMS_RELATIONSHIP_ACTIVATION_ENABLED on, occupied active slot.
-- Entry: Project calls authorizeRelationshipActivation before insertExecution.
-- Transaction is still the Project transaction (BEGIN line 31, COMMIT line 126).
-- Lock order modeled: advisory, relationship head FOR UPDATE, shipment FOR UPDATE,
-- cargo FOR UPDATE, driver FOR UPDATE, vehicle FOR UPDATE, INSERT new execution,
-- UPDATE that new row, INSERT active link.
-- There is no FOR UPDATE of an existing execution.
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
   AND tenant_id = '00000000-0000-0000-0000-00000000000b'
   AND carrier_company_id = '99999999-9999-9999-9999-999999999999'
   AND status = 'ACTIVE'
   AND deleted_at IS NULL
   FOR UPDATE;
SELECT id
  FROM transport.vehicles
 WHERE id = '33333333-3333-3333-3333-333333333333'
   AND tenant_id = '00000000-0000-0000-0000-00000000000b'
   AND carrier_company_id = '99999999-9999-9999-9999-999999999999'
   AND status = 'ACTIVE'
   AND deleted_at IS NULL
   FOR UPDATE;
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
 WHERE id = '13131313-1313-1313-1313-131313131313'
   AND operating_tenant_id = '00000000-0000-0000-0000-00000000000b';
INSERT INTO transport.transport_execution_active_shipments (
    shipment_tenant_id, shipment_id, execution_id
) VALUES (
    '00000000-0000-0000-0000-00000000000a', '22222222-2222-2222-2222-222222222222',
    '13131313-1313-1313-1313-131313131313'
);
COMMIT;
