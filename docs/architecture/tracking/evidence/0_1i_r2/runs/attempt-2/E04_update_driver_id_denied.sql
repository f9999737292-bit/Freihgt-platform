SET ROLE assignment_lock_owner;
UPDATE transport.shipments
   SET driver_id = driver_id
 WHERE id = '22222222-2222-2222-2222-222222222222';

