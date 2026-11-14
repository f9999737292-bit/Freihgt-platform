-- ShipmentRepository.AssignDriver shipment_repository.go:423-457.
-- Transaction BEGIN :426, COMMIT :454.
-- The only row lock is the UPDATE of transport.shipments at :433.
-- No execution, active-link, driver, or vehicle FOR UPDATE.
-- Status-history and outbox inserts are omitted.
BEGIN;
UPDATE transport.shipments
   SET driver_id = '77777777-7777-7777-7777-777777777778',
       status = 'ASSIGNED',
       version = version + 1,
       updated_at = now()
 WHERE id = '22222222-2222-2222-2222-222222222222'
   AND tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND deleted_at IS NULL
   AND version = 1;
COMMIT;
