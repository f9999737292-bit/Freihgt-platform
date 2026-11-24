BEGIN;
UPDATE transport.shipments
   SET driver_id = '77777777-7777-7777-7777-777777777777',
       updated_at = clock_timestamp()
 WHERE id = '22222222-2222-2222-2222-222222222222';
SELECT pg_sleep(6);
COMMIT;
