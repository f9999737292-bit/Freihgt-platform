INSERT INTO core.roles (tenant_id, code, name, description, scope, is_system)
SELECT NULL, 'PLATFORM_ADMIN', 'Platform Admin', 'Full platform administrator', 'GLOBAL', true
WHERE NOT EXISTS (SELECT 1 FROM core.roles WHERE tenant_id IS NULL AND code = 'PLATFORM_ADMIN');

INSERT INTO core.roles (tenant_id, code, name, description, scope, is_system)
SELECT NULL, 'SHIPPER_ADMIN', 'Shipper Admin', 'Shipper company administrator', 'TENANT', true
WHERE NOT EXISTS (SELECT 1 FROM core.roles WHERE tenant_id IS NULL AND code = 'SHIPPER_ADMIN');

INSERT INTO core.roles (tenant_id, code, name, description, scope, is_system)
SELECT NULL, 'SHIPPER_LOGIST', 'Shipper Logistician', 'Creates and manages transport orders', 'TENANT', true
WHERE NOT EXISTS (SELECT 1 FROM core.roles WHERE tenant_id IS NULL AND code = 'SHIPPER_LOGIST');

INSERT INTO core.roles (tenant_id, code, name, description, scope, is_system)
SELECT NULL, 'CARRIER_ADMIN', 'Carrier Admin', 'Carrier company administrator', 'TENANT', true
WHERE NOT EXISTS (SELECT 1 FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_ADMIN');

INSERT INTO core.roles (tenant_id, code, name, description, scope, is_system)
SELECT NULL, 'CARRIER_DISPATCHER', 'Carrier Dispatcher', 'Manages bids, vehicles and drivers', 'TENANT', true
WHERE NOT EXISTS (SELECT 1 FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_DISPATCHER');

INSERT INTO core.roles (tenant_id, code, name, description, scope, is_system)
SELECT NULL, 'DRIVER', 'Driver', 'Driver mobile app user', 'TENANT', true
WHERE NOT EXISTS (SELECT 1 FROM core.roles WHERE tenant_id IS NULL AND code = 'DRIVER');

INSERT INTO core.roles (tenant_id, code, name, description, scope, is_system)
SELECT NULL, 'CONSIGNEE_OPERATOR', 'Consignee Operator', 'Receives goods and confirms delivery', 'TENANT', true
WHERE NOT EXISTS (SELECT 1 FROM core.roles WHERE tenant_id IS NULL AND code = 'CONSIGNEE_OPERATOR');

INSERT INTO core.roles (tenant_id, code, name, description, scope, is_system)
SELECT NULL, 'PROCUREMENT_MANAGER', 'Procurement Manager', 'Manages RFx and tenders', 'TENANT', true
WHERE NOT EXISTS (SELECT 1 FROM core.roles WHERE tenant_id IS NULL AND code = 'PROCUREMENT_MANAGER');

INSERT INTO core.roles (tenant_id, code, name, description, scope, is_system)
SELECT NULL, 'FINANCE_MANAGER', 'Finance Manager', 'Manages billing registers and closing documents', 'TENANT', true
WHERE NOT EXISTS (SELECT 1 FROM core.roles WHERE tenant_id IS NULL AND code = 'FINANCE_MANAGER');

INSERT INTO core.roles (tenant_id, code, name, description, scope, is_system)
SELECT NULL, 'GOV_INSPECTOR', 'Government Inspector', 'Government inspection access', 'GLOBAL', true
WHERE NOT EXISTS (SELECT 1 FROM core.roles WHERE tenant_id IS NULL AND code = 'GOV_INSPECTOR');
