BEGIN;
SELECT transport.read_binding_line(:'execution'::uuid, :'shipment'::uuid);
SELECT pg_sleep(:hold);
COMMIT;
