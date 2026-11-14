TRUNCATE
    transport.delivery_disposition_audit,
    transport.delivery_disposition_commands,
    transport.delivery_disposition_cases,
    transport.transport_execution_commands,
    transport.transport_execution_revision_actions,
    transport.transport_execution_revision_stops,
    transport.transport_execution_actions,
    transport.transport_execution_stops,
    transport.transport_execution_active_shipments,
    transport.transport_execution_participants,
    transport.transport_execution_revisions,
    transport.transport_executions,
    transport.shipments,
    transport.cargoes,
    transport.locations,
    transport.drivers,
    transport.vehicles;

INSERT INTO transport.drivers (id) VALUES
    ('77777777-7777-7777-7777-777777777777'),
    ('77777777-7777-7777-7777-777777777778');
INSERT INTO transport.vehicles (id) VALUES ('33333333-3333-3333-3333-333333333333');
INSERT INTO transport.cargoes (id) VALUES ('44444444-4444-4444-4444-444444444444');
INSERT INTO transport.locations (id, tenant_id, location_type, lat, lon) VALUES
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', '00000000-0000-0000-0000-00000000000a', 'WAREHOUSE', 55.75, 37.61),
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', '00000000-0000-0000-0000-00000000000b', 'WAREHOUSE', 59.93, 30.31);

INSERT INTO transport.shipments (id, tenant_id, driver_id, vehicle_id, origin_location_id, cargo_id, version) VALUES
    ('22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-00000000000a',
     '77777777-7777-7777-7777-777777777777', '33333333-3333-3333-3333-333333333333',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', '44444444-4444-4444-4444-444444444444', 1),
    ('22222222-2222-2222-2222-222222222223', '00000000-0000-0000-0000-00000000000a',
     '77777777-7777-7777-7777-777777777777', '33333333-3333-3333-3333-333333333333',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', '44444444-4444-4444-4444-444444444444', 1);

INSERT INTO transport.transport_executions (id, operating_tenant_id, driver_id, vehicle_id, current_revision_id, event_seq) VALUES
    ('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-00000000000b',
     '77777777-7777-7777-7777-777777777777', '33333333-3333-3333-3333-333333333333', NULL, 1);
INSERT INTO transport.transport_execution_revisions (id, execution_id, operating_tenant_id, status) VALUES
    ('55555555-5555-5555-5555-555555555555', '11111111-1111-1111-1111-111111111111',
     '00000000-0000-0000-0000-00000000000b', 'ACTIVE');
UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-555555555555'
 WHERE id = '11111111-1111-1111-1111-111111111111';

INSERT INTO transport.transport_execution_participants (execution_id, shipment_id, shipment_tenant_id, cargo_id) VALUES
    ('11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222',
     '00000000-0000-0000-0000-00000000000a', '44444444-4444-4444-4444-444444444444');
INSERT INTO transport.transport_execution_active_shipments (shipment_tenant_id, shipment_id, execution_id) VALUES
    ('00000000-0000-0000-0000-00000000000a', '22222222-2222-2222-2222-222222222222',
     '11111111-1111-1111-1111-111111111111');

INSERT INTO transport.transport_execution_stops (
    id, execution_id, operating_tenant_id, ordinal, stop_role, point_kind, location_id, latitude, longitude, status
) VALUES (
    '66666666-6666-6666-6666-666666666666', '11111111-1111-1111-1111-111111111111',
    '00000000-0000-0000-0000-00000000000b', 0, 'CARGO', 'CANONICAL_LOCATION',
    'cccccccc-cccc-cccc-cccc-cccccccccccc', 55.75, 37.61, 'PLANNED'
);
INSERT INTO transport.transport_execution_actions (
    id, execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, action_type, ordinal, status
) VALUES (
    '66666666-6666-6666-6666-666666666667', '66666666-6666-6666-6666-666666666666',
    '22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-00000000000a',
    '44444444-4444-4444-4444-444444444444', 'DELIVERY', 0, 'PENDING'
);
INSERT INTO transport.transport_execution_revision_stops (revision_id, stop_id, source_ordinal, membership) VALUES
    ('55555555-5555-5555-5555-555555555555', '66666666-6666-6666-6666-666666666666', 0, 'INTRODUCED');
INSERT INTO transport.transport_execution_revision_actions (revision_id, action_id, source_action_ordinal, membership) VALUES
    ('55555555-5555-5555-5555-555555555555', '66666666-6666-6666-6666-666666666667', 0, 'INTRODUCED');

INSERT INTO transport.delivery_disposition_cases (
    id, operating_tenant_id, execution_id, source_revision_id, source_execution_stop_id, source_action_id,
    shipment_id, shipment_tenant_id, cargo_id,
    attempted_quantity, accepted_quantity, rejected_quantity, pending_quantity,
    return_reserved_quantity, redirect_reserved_quantity, hold_reserved_quantity, resolved_quantity,
    status, version
) VALUES (
    '99999999-9999-9999-9999-999999999999', '00000000-0000-0000-0000-00000000000b',
    '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555',
    '66666666-6666-6666-6666-666666666666', '66666666-6666-6666-6666-666666666667',
    '22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-00000000000a',
    '44444444-4444-4444-4444-444444444444',
    2, 1, 1, 1, 0, 0, 0, 0, 'DISPOSITION_PENDING', 1
);
