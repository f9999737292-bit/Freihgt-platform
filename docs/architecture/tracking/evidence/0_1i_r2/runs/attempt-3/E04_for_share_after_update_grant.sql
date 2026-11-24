SET ROLE assignment_lock_owner;
SELECT id
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222'
   FOR SHARE;

