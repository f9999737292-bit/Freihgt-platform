-- Architecture permission: USE_MANUAL_SPOT_PRICE
INSERT INTO core.permissions (code, resource, action, description)
VALUES (
    'rates.manual_spot.use',
    'rates',
    'manual_spot_use',
    'Authorize manual spot price override when no contract rate matches'
)
ON CONFLICT (code) DO NOTHING;

INSERT INTO core.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM core.roles r
JOIN core.permissions p ON p.code = 'rates.manual_spot.use'
WHERE r.tenant_id IS NULL
  AND r.code IN ('PROCUREMENT_MANAGER', 'SHIPPER_ADMIN', 'FORWARDER_MANAGER')
  AND NOT EXISTS (
      SELECT 1
      FROM core.role_permissions rp
      WHERE rp.role_id = r.id AND rp.permission_id = p.id
  );
