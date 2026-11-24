-- Restores the committed binding the reader observes.
-- Two executions exist: the active one, and a second row used only as the
-- target of the direct active-link UPDATE. The second row is not locked by writers.

TRUNCATE
    transport.transport_execution_active_shipments,
    transport.transport_execution_stops,
    transport.transport_execution_revisions,
    transport.transport_executions,
    transport.shipments,
    transport.cargoes,
    transport.drivers,
    transport.vehicles,
    company.carrier_relationship_heads;

INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, status) VALUES
    ('77777777-7777-7777-7777-777777777777', '00000000-0000-0000-0000-00000000000b', '99999999-9999-9999-9999-999999999999', 'ACTIVE'),
    ('77777777-7777-7777-7777-777777777778', '00000000-0000-0000-0000-00000000000b', '99999999-9999-9999-9999-999999999999', 'ACTIVE');

INSERT INTO transport.vehicles (id, tenant_id, carrier_company_id, status) VALUES
    ('33333333-3333-3333-3333-333333333333', '00000000-0000-0000-0000-00000000000b', '99999999-9999-9999-9999-999999999999', 'ACTIVE'),
    ('33333333-3333-3333-3333-333333333334', '00000000-0000-0000-0000-00000000000b', '99999999-9999-9999-9999-999999999999', 'ACTIVE');

INSERT INTO transport.cargoes (id, tenant_id, version) VALUES
    ('44444444-4444-4444-4444-444444444444', '00000000-0000-0000-0000-00000000000a', 1),
    ('44444444-4444-4444-4444-444444444445', '00000000-0000-0000-0000-00000000000a', 1);

INSERT INTO transport.shipments (
    id, tenant_id, driver_id, vehicle_id, cargo_id, carrier_company_id, status, version
) VALUES
    ('22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-00000000000a',
     '77777777-7777-7777-7777-777777777777', '33333333-3333-3333-3333-333333333333',
     '44444444-4444-4444-4444-444444444444', '99999999-9999-9999-9999-999999999999', 'PLANNED', 1),
    ('22222222-2222-2222-2222-222222222223', '00000000-0000-0000-0000-00000000000a',
     '77777777-7777-7777-7777-777777777777', '33333333-3333-3333-3333-333333333333',
     '44444444-4444-4444-4444-444444444445', '99999999-9999-9999-9999-999999999999', 'PLANNED', 1);

INSERT INTO transport.transport_executions (
    id, operating_tenant_id, driver_id, vehicle_id, current_revision_id, event_seq
) VALUES
    ('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-00000000000b',
     '77777777-7777-7777-7777-777777777777', '33333333-3333-3333-3333-333333333333',
     '55555555-5555-5555-5555-555555555555', 1),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '00000000-0000-0000-0000-00000000000b',
     '77777777-7777-7777-7777-777777777777', '33333333-3333-3333-3333-333333333333',
     '55555555-5555-5555-5555-555555555557', 1);

INSERT INTO transport.transport_execution_revisions (id, execution_id, operating_tenant_id, status, version) VALUES
    ('55555555-5555-5555-5555-555555555555', '11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-00000000000b', 'ACTIVE', 1),
    ('55555555-5555-5555-5555-555555555557', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '00000000-0000-0000-0000-00000000000b', 'ACTIVE', 1);

INSERT INTO transport.transport_execution_stops (id, execution_id, status) VALUES
    ('66666666-6666-6666-6666-666666666666', '11111111-1111-1111-1111-111111111111', 'PENDING');

INSERT INTO transport.transport_execution_active_shipments (shipment_tenant_id, shipment_id, execution_id) VALUES
    ('00000000-0000-0000-0000-00000000000a', '22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111');

INSERT INTO company.carrier_relationship_heads (relationship_id, status, current_version_id) VALUES
    ('88888888-8888-8888-8888-888888888888', 'ACTIVE', '88888888-8888-8888-8888-888888888881');
