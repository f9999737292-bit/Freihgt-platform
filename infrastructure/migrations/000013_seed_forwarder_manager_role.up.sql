INSERT INTO core.roles (tenant_id, code, name, description, scope, is_system)
SELECT NULL, 'FORWARDER_MANAGER', 'Forwarder Manager', 'Manages delegated freight and shipment flows on behalf of shippers', 'TENANT', true
WHERE NOT EXISTS (SELECT 1 FROM core.roles WHERE tenant_id IS NULL AND code = 'FORWARDER_MANAGER');
