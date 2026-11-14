BEGIN;
SELECT transport.read_binding_line(:'execution'::uuid, :'shipment'::uuid);
COMMIT;
