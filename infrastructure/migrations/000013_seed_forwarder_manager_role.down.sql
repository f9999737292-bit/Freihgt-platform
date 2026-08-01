DELETE FROM core.roles
WHERE tenant_id IS NULL
  AND code = 'FORWARDER_MANAGER';
