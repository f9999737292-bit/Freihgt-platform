BEGIN;
SET lock_timeout = '2s';
UPDATE transport.shipments
   SET version = version + 1,
       updated_at = clock_timestamp()
 WHERE id = '22222222-2222-2222-2222-222222222222';
COMMIT;
