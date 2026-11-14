-- AuthorizeReturn. Same transaction as authorizeRoute, not a bare successor.
-- Lock order: execution FOR UPDATE, disposition command insert (no FK),
-- case FOR UPDATE, plain location and shipment reads, then successor writes.
BEGIN;
SELECT operating_tenant_id, current_revision_id
  FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
INSERT INTO transport.delivery_disposition_commands (
    id, operating_tenant_id, execution_id, idempotency_key, command_name, request_sha256, result_json
) VALUES (
    'ca000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-00000000000b',
    '11111111-1111-1111-1111-111111111111', 'authorize-return-01n', 'AUTHORIZE_RETURN',
    'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', '{}'::jsonb
);
SELECT id FROM transport.delivery_disposition_cases
 WHERE id = '99999999-9999-9999-9999-999999999999'
   AND execution_id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
SELECT tenant_id, location_type, lat, lon FROM transport.locations
 WHERE id = 'cccccccc-cccc-cccc-cccc-cccccccccccc';
SELECT origin_location_id, tenant_id FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222';
SELECT s.id FROM transport.transport_execution_revision_stops rs
  JOIN transport.transport_execution_stops s ON s.id = rs.stop_id
 WHERE rs.revision_id = '55555555-5555-5555-5555-555555555555'
   AND rs.membership <> 'SUPERSEDED' AND s.status IN ('PLANNED', 'ARRIVED');
SELECT id FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111' FOR UPDATE;
INSERT INTO transport.transport_execution_commands (
    id, operating_tenant_id, execution_id, idempotency_key, command_name, request_sha256, result_json
) VALUES (
    'cb000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-00000000000b',
    '11111111-1111-1111-1111-111111111111', 'succ-return-01n', 'CREATE_SUCCESSOR_REVISION',
    'cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc', '{}'::jsonb
);
UPDATE transport.transport_execution_revisions
   SET status = 'SUPERSEDED' WHERE id = '55555555-5555-5555-5555-555555555555' AND status = 'ACTIVE';
INSERT INTO transport.transport_execution_revisions (
    id, execution_id, operating_tenant_id, status, supersedes_revision_id
) VALUES (
    '55555555-5555-5555-5555-55555555555a', '11111111-1111-1111-1111-111111111111',
    '00000000-0000-0000-0000-00000000000b', 'ACTIVE', '55555555-5555-5555-5555-555555555555'
);
UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-55555555555a'
 WHERE id = '11111111-1111-1111-1111-111111111111';
UPDATE transport.transport_execution_revision_stops
   SET membership = 'SUPERSEDED'
 WHERE revision_id = '55555555-5555-5555-5555-555555555555'
   AND stop_id = '66666666-6666-6666-6666-666666666666';
UPDATE transport.transport_execution_revision_actions
   SET membership = 'SUPERSEDED'
 WHERE revision_id = '55555555-5555-5555-5555-555555555555'
   AND action_id = '66666666-6666-6666-6666-666666666667';
INSERT INTO transport.transport_execution_stops (
    id, execution_id, operating_tenant_id, ordinal, stop_role, point_kind, location_id, latitude, longitude, status
) VALUES
    ('66666666-6666-6666-6666-666666666671', '11111111-1111-1111-1111-111111111111',
     '00000000-0000-0000-0000-00000000000b', 1, 'CARGO', 'CANONICAL_LOCATION',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', 55.75, 37.61, 'PLANNED'),
    ('66666666-6666-6666-6666-666666666673', '11111111-1111-1111-1111-111111111111',
     '00000000-0000-0000-0000-00000000000b', 2, 'CARGO', 'CANONICAL_LOCATION',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', 55.75, 37.61, 'PLANNED');
INSERT INTO transport.transport_execution_actions (
    id, execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, action_type, ordinal, status
) VALUES
    ('66666666-6666-6666-6666-666666666672', '66666666-6666-6666-6666-666666666671',
     '22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-00000000000a',
     '44444444-4444-4444-4444-444444444444', 'DELIVERY', 0, 'PENDING'),
    ('66666666-6666-6666-6666-666666666674', '66666666-6666-6666-6666-666666666673',
     '22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-00000000000a',
     '44444444-4444-4444-4444-444444444444', 'DELIVERY', 0, 'PENDING');
INSERT INTO transport.transport_execution_revision_stops (revision_id, stop_id, source_ordinal, membership) VALUES
    ('55555555-5555-5555-5555-55555555555a', '66666666-6666-6666-6666-666666666671', 0, 'INTRODUCED'),
    ('55555555-5555-5555-5555-55555555555a', '66666666-6666-6666-6666-666666666673', 1, 'INTRODUCED');
INSERT INTO transport.transport_execution_revision_actions (revision_id, action_id, source_action_ordinal, membership) VALUES
    ('55555555-5555-5555-5555-55555555555a', '66666666-6666-6666-6666-666666666672', 0, 'INTRODUCED'),
    ('55555555-5555-5555-5555-55555555555a', '66666666-6666-6666-6666-666666666674', 0, 'INTRODUCED');
UPDATE transport.delivery_disposition_cases
   SET disposition_type = 'RETURN_TO_ORIGIN', status = 'IN_RETURN_TRANSIT',
       pending_quantity = 0, return_reserved_quantity = 1, redirect_reserved_quantity = 0,
       hold_reserved_quantity = 0, return_target_location_id = 'cccccccc-cccc-cccc-cccc-cccccccccccc',
       successor_revision_id = '55555555-5555-5555-5555-55555555555a', version = 2
 WHERE id = '99999999-9999-9999-9999-999999999999' AND status = 'DISPOSITION_PENDING';
INSERT INTO transport.delivery_disposition_audit (
    id, case_id, command_id, operating_tenant_id, execution_id, previous_status, new_status
) VALUES (
    'aa000000-0000-0000-0000-00000000000a', '99999999-9999-9999-9999-999999999999',
    'ca000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-00000000000b',
    '11111111-1111-1111-1111-111111111111', 'DISPOSITION_PENDING', 'IN_RETURN_TRANSIT'
);
COMMIT;
