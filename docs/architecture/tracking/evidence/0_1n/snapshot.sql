SELECT
    (SELECT execution_id::text FROM transport.transport_execution_active_shipments
      WHERE shipment_id = '22222222-2222-2222-2222-222222222222')
    || '|' ||
    (SELECT version::text || '|' || driver_id::text FROM transport.shipments
      WHERE id = '22222222-2222-2222-2222-222222222222')
    || '|' ||
    (SELECT count(*)::text FROM transport.transport_executions)
    || '|' ||
    (SELECT current_revision_id::text FROM transport.transport_executions
      WHERE id = '11111111-1111-1111-1111-111111111111')
    || '|' ||
    (SELECT status FROM transport.transport_execution_revisions
      WHERE id = '55555555-5555-5555-5555-555555555555')
    || '|' ||
    (SELECT status FROM transport.delivery_disposition_cases
      WHERE id = '99999999-9999-9999-9999-999999999999')
    || '|' ||
    (SELECT count(*)::text FROM transport.transport_execution_participants)
    || '|' ||
    coalesce((SELECT execution_id::text FROM transport.transport_execution_active_shipments
      WHERE shipment_id = '22222222-2222-2222-2222-222222222223'), 'NONE')
    || '|' ||
    (SELECT count(*)::text FROM transport.transport_execution_stops);
