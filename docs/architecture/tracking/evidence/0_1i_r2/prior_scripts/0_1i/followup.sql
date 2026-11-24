\set VERBOSITY verbose
SET ROLE shipment_assignment_reader;
SELECT current_user;
SELECT shipment_id, driver_id, execution_locked
  FROM transport.read_binding_assignment(NULL, '22222222-2222-2222-2222-222222222222');
RESET ROLE;
SET ROLE shipment_assignment_reader;
UPDATE transport.shipments
   SET updated_at = clock_timestamp()
 WHERE id = '22222222-2222-2222-2222-222222222222';
