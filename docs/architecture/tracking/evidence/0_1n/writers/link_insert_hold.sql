BEGIN;
INSERT INTO transport.transport_execution_active_shipments
    (shipment_tenant_id, shipment_id, execution_id)
VALUES (
    '00000000-0000-0000-0000-00000000000a',
    '22222222-2222-2222-2222-222222222223',
    '11111111-1111-1111-1111-111111111111'
);
SELECT pg_sleep(18);
COMMIT;
