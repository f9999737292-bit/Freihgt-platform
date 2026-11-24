BEGIN;
SELECT shipment_id::text || '|' || driver_id::text || '|' || vehicle_id::text || '|' || execution_id::text || '|' || current_revision_id::text || '|' || revision_version::text
  FROM transport.read_binding_assignment(
      '11111111-1111-1111-1111-111111111111',
      '22222222-2222-2222-2222-222222222222');
COMMIT;
