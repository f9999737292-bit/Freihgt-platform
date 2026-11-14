BEGIN;
INSERT INTO transport.transport_execution_participants
    (execution_id, shipment_id, shipment_tenant_id, cargo_id)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    '22222222-2222-2222-2222-222222222224',
    '00000000-0000-0000-0000-00000000000a',
    '44444444-4444-4444-4444-444444444444'
);
SELECT pg_sleep(18);
COMMIT;
