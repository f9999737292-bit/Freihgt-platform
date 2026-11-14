-- ShipmentRepository.updateStatus shipment_repository.go:511-552.
-- Transaction BEGIN :514. The shipment UPDATE is line 521.
-- Accept (:561) and Cancel (:604) use the same single shipment UPDATE shape
-- and are not executed as separate sessions.
-- No execution or active-link lock. History and cargo evidence are omitted.
BEGIN;
UPDATE transport.shipments
   SET status = 'IN_TRANSIT',
       version = version + 1,
       updated_at = now()
 WHERE id = '22222222-2222-2222-2222-222222222222'
   AND tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND deleted_at IS NULL
   AND version = 1;
COMMIT;
