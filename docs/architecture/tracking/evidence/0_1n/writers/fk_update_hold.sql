BEGIN;
UPDATE transport.shipments
   SET driver_id = '77777777-7777-7777-7777-777777777778', version = version + 1
 WHERE id = '22222222-2222-2222-2222-222222222222' AND version = 1;
SELECT pg_sleep(18);
COMMIT;
