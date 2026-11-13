BEGIN;
UPDATE transport.shipments
   SET vehicle_id = '88888888-8888-8888-8888-888888888888',
       updated_at = clock_timestamp()
 WHERE id = '22222222-2222-2222-2222-222222222222';
SELECT pg_sleep(6);
COMMIT;
