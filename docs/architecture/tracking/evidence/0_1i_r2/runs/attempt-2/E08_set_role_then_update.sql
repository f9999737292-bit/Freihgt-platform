SET ROLE assignment_lock_owner;
UPDATE transport.shipments
   SET updated_at = clock_timestamp()
 WHERE id = '22222222-2222-2222-2222-222222222222';

