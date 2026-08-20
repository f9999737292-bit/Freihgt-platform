DELETE FROM core.role_permissions rp
USING core.permissions p, core.roles r
WHERE rp.permission_id = p.id
  AND rp.role_id = r.id
  AND p.code = 'rates.manual_spot.use'
  AND r.tenant_id IS NULL
  AND r.code IN ('PROCUREMENT_MANAGER', 'SHIPPER_ADMIN', 'FORWARDER_MANAGER');

DELETE FROM core.permissions WHERE code = 'rates.manual_spot.use';
