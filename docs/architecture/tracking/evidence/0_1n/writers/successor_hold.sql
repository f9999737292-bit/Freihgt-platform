BEGIN;
SELECT id FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111' FOR UPDATE;
SELECT pg_sleep(18);
INSERT INTO transport.transport_execution_commands (
    id, operating_tenant_id, execution_id, idempotency_key, command_name, request_sha256, result_json
) VALUES (
    'cb000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-00000000000b',
    '11111111-1111-1111-1111-111111111111', 'succ-01n', 'CREATE_SUCCESSOR_REVISION',
    'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', '{}'::jsonb
);
UPDATE transport.transport_execution_revisions
   SET status = 'SUPERSEDED' WHERE id = '55555555-5555-5555-5555-555555555555' AND status = 'ACTIVE';
INSERT INTO transport.transport_execution_revisions (id, execution_id, operating_tenant_id, status, supersedes_revision_id)
VALUES (
    '55555555-5555-5555-5555-555555555556', '11111111-1111-1111-1111-111111111111',
    '00000000-0000-0000-0000-00000000000b', 'ACTIVE', '55555555-5555-5555-5555-555555555555'
);
UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-555555555556'
 WHERE id = '11111111-1111-1111-1111-111111111111';
INSERT INTO transport.transport_execution_stops (
    id, execution_id, operating_tenant_id, ordinal, stop_role, point_kind, location_id, latitude, longitude, status
) VALUES (
    '66666666-6666-6666-6666-666666666680', '11111111-1111-1111-1111-111111111111',
    '00000000-0000-0000-0000-00000000000b', 1, 'CARGO', 'CANONICAL_LOCATION',
    'cccccccc-cccc-cccc-cccc-cccccccccccc', 55.75, 37.61, 'PLANNED'
);
INSERT INTO transport.transport_execution_revision_stops (revision_id, stop_id, source_ordinal, membership)
VALUES ('55555555-5555-5555-5555-555555555556', '66666666-6666-6666-6666-666666666680', 0, 'INTRODUCED');
COMMIT;
