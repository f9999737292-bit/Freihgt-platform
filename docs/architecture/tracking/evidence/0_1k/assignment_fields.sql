SELECT driver_id::text
    || '|' || coalesce(vehicle_id::text, 'null')
    || '|' || execution_id::text
  FROM transport.read_binding_assignment(:'execution'::uuid, :'shipment'::uuid);
