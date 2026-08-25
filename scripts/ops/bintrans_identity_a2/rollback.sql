-- BINTRANS identity A2 — controlled rollback (UUID-preserving)
-- FAIL CLOSED on reverse email collision
-- Scope: tenant 74519f22-ff9b-4a8b-8fff-a958c689682f only

BEGIN;

\set ON_ERROR_STOP on
\set tenant_id '74519f22-ff9b-4a8b-8fff-a958c689682f'

DO $$
DECLARE
  v_tenant_id uuid := '74519f22-ff9b-4a8b-8fff-a958c689682f';
  v_collision_count integer;
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM core.tenants WHERE id = v_tenant_id AND deleted_at IS NULL
  ) THEN
    RAISE EXCEPTION 'ABORT: expected tenant % not found', v_tenant_id;
  END IF;

  IF EXISTS (
    SELECT 1 FROM core.tenants
    WHERE code = 'dev-7rights'
      AND id <> v_tenant_id
      AND deleted_at IS NULL
  ) THEN
    RAISE EXCEPTION 'ABORT: dev-7rights tenant code already used by another tenant';
  END IF;

  WITH mapping(old_email, new_email) AS (
    VALUES
      ('admin@7rights.local', 'admin@bintrans.local'),
      ('shipper@7rights.local', 'shipper@bintrans.local'),
      ('carrier@7rights.local', 'carrier@bintrans.local'),
      ('forwarder@7rights.local', 'forwarder@bintrans.local'),
      ('consignee@7rights.local', 'consignee@bintrans.local')
  ),
  sources AS (
    SELECT m.old_email, m.new_email, u.id AS source_user_id
    FROM mapping m
    JOIN core.users u
      ON u.tenant_id = v_tenant_id
     AND u.deleted_at IS NULL
     AND lower(u.email) = m.new_email
  ),
  targets AS (
    SELECT m.old_email, u.id AS target_user_id
    FROM mapping m
    JOIN core.users u
      ON u.tenant_id = v_tenant_id
     AND u.deleted_at IS NULL
     AND lower(u.email) = m.old_email
  )
  SELECT count(*) INTO v_collision_count
  FROM sources s
  JOIN targets t ON t.old_email = s.old_email
  WHERE t.target_user_id <> s.source_user_id;

  IF v_collision_count > 0 THEN
    RAISE EXCEPTION 'ABORT: rollback email collision (% conflicts)', v_collision_count;
  END IF;
END $$;

UPDATE core.tenants
SET
  code = 'dev-7rights',
  name = '7Rights Dev Tenant',
  updated_at = now(),
  version = version + 1
WHERE id = :'tenant_id'::uuid
  AND deleted_at IS NULL;

WITH mapping(old_email, new_email) AS (
  VALUES
    ('admin@7rights.local', 'admin@bintrans.local'),
    ('shipper@7rights.local', 'shipper@bintrans.local'),
    ('carrier@7rights.local', 'carrier@bintrans.local'),
    ('forwarder@7rights.local', 'forwarder@bintrans.local'),
    ('consignee@7rights.local', 'consignee@bintrans.local')
)
UPDATE core.users u
SET
  email = m.old_email,
  updated_at = now(),
  version = u.version + 1
FROM mapping m
WHERE u.tenant_id = :'tenant_id'::uuid
  AND u.deleted_at IS NULL
  AND lower(u.email) = m.new_email;

UPDATE core.companies
SET
  legal_name = 'ООО 7Rights Dev',
  short_name = '7Rights Dev',
  updated_at = now(),
  version = version + 1
WHERE tenant_id = :'tenant_id'::uuid
  AND deleted_at IS NULL
  AND company_type = 'PLATFORM_OPERATOR'
  AND legal_name = 'ООО Bintrans Dev';

UPDATE core.users
SET
  full_name = '7Rights Dev Admin',
  updated_at = now(),
  version = version + 1
WHERE tenant_id = :'tenant_id'::uuid
  AND deleted_at IS NULL
  AND lower(email) = 'admin@7rights.local'
  AND full_name = 'Bintrans Dev Admin';

COMMIT;
