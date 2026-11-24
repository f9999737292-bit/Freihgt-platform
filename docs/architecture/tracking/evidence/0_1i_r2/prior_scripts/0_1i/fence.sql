\set VERBOSITY verbose
SET ROLE binding_authority_fence;
SELECT current_user, session_user;
SELECT shipment_id, driver_id, execution_locked
  FROM transport.read_binding_assignment(NULL, '22222222-2222-2222-2222-222222222222');
