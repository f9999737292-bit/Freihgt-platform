-- AssignDriver. Sleeps after the shipment UPDATE and before COMMIT.
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
SELECT pg_sleep(15);
COMMIT;
