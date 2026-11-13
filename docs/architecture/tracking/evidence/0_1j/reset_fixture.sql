UPDATE transport.shipments
   SET driver_id = '33333333-3333-3333-3333-333333333333',
       vehicle_id = '44444444-4444-4444-4444-444444444444',
       version = 3,
       updated_at = '2000-01-01+00'
 WHERE id = '22222222-2222-2222-2222-222222222222';

UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-555555555555',
       driver_id = '33333333-3333-3333-3333-333333333333',
       vehicle_id = '44444444-4444-4444-4444-444444444444',
       updated_at = '2000-01-01+00'
 WHERE id = '11111111-1111-1111-1111-111111111111';

DELETE FROM transport.transport_execution_active_shipments
 WHERE shipment_id = '00000000-0000-0000-0000-000000000007';

UPDATE transport.binding_authority_decisions
   SET use_status = 'ISSUED', consumed_at = NULL, binding_id = NULL
 WHERE decision_id IN (
    '66666666-6666-6666-6666-666666666666',
    '66666666-6666-6666-6666-666666666670',
    '66666666-6666-6666-6666-666666666678'
 );

DELETE FROM tracking.shipment_tracking_binding
 WHERE decision_id IN (
    '66666666-6666-6666-6666-666666666666',
    '66666666-6666-6666-6666-666666666670',
    '66666666-6666-6666-6666-666666666678'
 );
