-- UPDATE of shipments.driver_id. The foreign key checks the new driver.
-- driver_id is not the shipment primary key, so the row lock is FOR NO KEY UPDATE.
BEGIN;
UPDATE transport.shipments
   SET driver_id = '77777777-7777-7777-7777-777777777778', version = version + 1
 WHERE id = '22222222-2222-2222-2222-222222222222' AND version = 1;
COMMIT;
