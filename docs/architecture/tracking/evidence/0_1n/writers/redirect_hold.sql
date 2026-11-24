BEGIN;
SELECT operating_tenant_id, current_revision_id
  FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
INSERT INTO transport.delivery_disposition_commands (
    id, operating_tenant_id, execution_id, idempotency_key, command_name, request_sha256, result_json
) VALUES (
    'ca000000-0000-0000-0000-00000000000b', '00000000-0000-0000-0000-00000000000b',
    '11111111-1111-1111-1111-111111111111', 'authorize-redirect-01n', 'AUTHORIZE_REDIRECT',
    'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', '{}'::jsonb
);
SELECT id FROM transport.delivery_disposition_cases
 WHERE id = '99999999-9999-9999-9999-999999999999'
   AND execution_id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
SELECT tenant_id, location_type, lat, lon FROM transport.locations
 WHERE id = 'dddddddd-dddd-dddd-dddd-dddddddddddd';
SELECT origin_location_id, tenant_id FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222';
SELECT s.id FROM transport.transport_execution_revision_stops rs
  JOIN transport.transport_execution_stops s ON s.id = rs.stop_id
 WHERE rs.revision_id = '55555555-5555-5555-5555-555555555555'
   AND rs.membership <> 'SUPERSEDED' AND s.status IN ('PLANNED', 'ARRIVED');
SELECT pg_sleep(18);
SELECT id FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111' FOR UPDATE;
INSERT INTO transport.transport_execution_commands (
    id, operating_tenant_id, execution_id, idempotency_key, command_name, request_sha256, result_json
) VALUES (
    'cb000000-0000-0000-0000-00000000000b', '00000000-0000-0000-0000-00000000000b',
    '11111111-1111-1111-1111-111111111111', 'succ-redirect-01n', 'CREATE_SUCCESSOR_REVISION',
    'dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd', '{}'::jsonb
);
UPDATE transport.transport_execution_revisions
   SET status = 'SUPERSEDED' WHERE id = '55555555-5555-5555-5555-555555555555' AND status = 'ACTIVE';
INSERT INTO transport.transport_execution_revisions (
    id, execution_id, operating_tenant_id, status, supersedes_revision_id
) VALUES (
    '55555555-5555-5555-5555-55555555555b', '11111111-1111-1111-1111-111111111111',
    '00000000-0000-0000-0000-00000000000b', 'ACTIVE', '55555555-5555-5555-5555-555555555555'
);
UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-55555555555b'
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
    ('66666666-6666-6666-6666-666666666675', '11111111-1111-1111-1111-111111111111',
     '00000000-0000-0000-0000-00000000000b', 1, 'CARGO', 'CANONICAL_LOCATION',
     'cccccccc-cccc-cccc-cccc-cccccccccccc', 55.75, 37.61, 'PLANNED'),
    ('66666666-6666-6666-6666-666666666677', '11111111-1111-1111-1111-111111111111',
     '00000000-0000-0000-0000-00000000000b', 2, 'CARGO', 'CANONICAL_LOCATION',
     'dddddddd-dddd-dddd-dddd-dddddddddddd', 59.93, 30.31, 'PLANNED');
INSERT INTO transport.transport_execution_actions (
    id, execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, action_type, ordinal, status
) VALUES
    ('66666666-6666-6666-6666-666666666676', '66666666-6666-6666-6666-666666666675',
     '22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-00000000000a',
     '44444444-4444-4444-4444-444444444444', 'DELIVERY', 0, 'PENDING'),
    ('66666666-6666-6666-6666-666666666678', '66666666-6666-6666-6666-666666666677',
     '22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-00000000000a',
     '44444444-4444-4444-4444-444444444444', 'DELIVERY', 0, 'PENDING');
INSERT INTO transport.transport_execution_revision_stops (revision_id, stop_id, source_ordinal, membership) VALUES
    ('55555555-5555-5555-5555-55555555555b', '66666666-6666-6666-6666-666666666675', 0, 'INTRODUCED'),
    ('55555555-5555-5555-5555-55555555555b', '66666666-6666-6666-6666-666666666677', 1, 'INTRODUCED');
INSERT INTO transport.transport_execution_revision_actions (revision_id, action_id, source_action_ordinal, membership) VALUES
    ('55555555-5555-5555-5555-55555555555b', '66666666-6666-6666-6666-666666666676', 0, 'INTRODUCED'),
    ('55555555-5555-5555-5555-55555555555b', '66666666-6666-6666-6666-666666666678', 0, 'INTRODUCED');
UPDATE transport.delivery_disposition_cases
   SET disposition_type = 'REDIRECT', status = 'IN_REDIRECT_TRANSIT',
       pending_quantity = 0, return_reserved_quantity = 0, redirect_reserved_quantity = 1,
       hold_reserved_quantity = 0, redirect_target_location_id = 'dddddddd-dddd-dddd-dddd-dddddddddddd',
       successor_revision_id = '55555555-5555-5555-5555-55555555555b', version = 2
 WHERE id = '99999999-9999-9999-9999-999999999999' AND status = 'DISPOSITION_PENDING';
INSERT INTO transport.delivery_disposition_audit (
    id, case_id, command_id, operating_tenant_id, execution_id, previous_status, new_status
) VALUES (
    'aa000000-0000-0000-0000-00000000000b', '99999999-9999-9999-9999-999999999999',
    'ca000000-0000-0000-0000-00000000000b', '00000000-0000-0000-0000-00000000000b',
    '11111111-1111-1111-1111-111111111111', 'DISPOSITION_PENDING', 'IN_REDIRECT_TRANSIT'
);
COMMIT;
