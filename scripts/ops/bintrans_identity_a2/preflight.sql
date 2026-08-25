-- BINTRANS identity A2 — read-only preflight (no mutations)
-- Scope: synthetic dev tenant 74519f22-ff9b-4a8b-8fff-a958c689682f only

\set ON_ERROR_STOP on
\set tenant_id '74519f22-ff9b-4a8b-8fff-a958c689682f'

\echo '=== TENANT PRESENCE ==='
SELECT id, code, name, status
FROM core.tenants
WHERE id = :'tenant_id'::uuid;

\echo '=== TENANT CODE COLLISION (dev-bintrans on different tenant) ==='
SELECT id, code, name
FROM core.tenants
WHERE code = 'dev-bintrans'
  AND id <> :'tenant_id'::uuid
  AND deleted_at IS NULL;

\echo '=== OLD_IDENTITY_COUNT (@7rights.local users in tenant) ==='
SELECT count(*) AS old_identity_count
FROM core.users
WHERE tenant_id = :'tenant_id'::uuid
  AND deleted_at IS NULL
  AND lower(email) LIKE '%@7rights.local';

\echo '=== TARGET_IDENTITY_COUNT (@bintrans.local users in tenant) ==='
SELECT count(*) AS target_identity_count
FROM core.users
WHERE tenant_id = :'tenant_id'::uuid
  AND deleted_at IS NULL
  AND lower(email) LIKE '%@bintrans.local';

\echo '=== EMAIL_COLLISION_COUNT (target exists with different UUID than source) ==='
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
  LEFT JOIN core.users u
    ON u.tenant_id = :'tenant_id'::uuid
   AND u.deleted_at IS NULL
   AND lower(u.email) = m.old_email
),
targets AS (
  SELECT m.old_email, m.new_email, u.id AS target_user_id
  FROM mapping m
  JOIN core.users u
    ON u.tenant_id = :'tenant_id'::uuid
   AND u.deleted_at IS NULL
   AND lower(u.email) = m.new_email
)
SELECT count(*) AS email_collision_count
FROM sources s
JOIN targets t ON t.new_email = s.new_email
WHERE s.source_user_id IS NOT NULL
  AND t.target_user_id IS DISTINCT FROM s.source_user_id;

\echo '=== PER-EMAIL COLLISION DETAIL ==='
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
  LEFT JOIN core.users u
    ON u.tenant_id = :'tenant_id'::uuid
   AND u.deleted_at IS NULL
   AND lower(u.email) = m.old_email
),
targets AS (
  SELECT m.old_email, m.new_email, u.id AS target_user_id
  FROM mapping m
  LEFT JOIN core.users u
    ON u.tenant_id = :'tenant_id'::uuid
   AND u.deleted_at IS NULL
   AND lower(u.email) = m.new_email
)
SELECT
  s.old_email,
  s.new_email,
  s.source_user_id,
  t.target_user_id,
  CASE
    WHEN s.source_user_id IS NULL THEN 'NO_SOURCE'
    WHEN t.target_user_id IS NULL THEN 'NO_COLLISION'
    WHEN t.target_user_id = s.source_user_id THEN 'ALREADY_MIGRATED'
    ELSE 'COLLISION'
  END AS collision_status
FROM sources s
LEFT JOIN targets t ON t.new_email = s.new_email
ORDER BY s.old_email;

\echo '=== COMPANY_MATCH_COUNT (platform operator company) ==='
SELECT id, legal_name, short_name, company_type, status
FROM core.companies
WHERE tenant_id = :'tenant_id'::uuid
  AND deleted_at IS NULL
  AND company_type = 'PLATFORM_OPERATOR'
ORDER BY created_at;

\echo '=== TENANT_CODE_COLLISION FLAG ==='
SELECT EXISTS (
  SELECT 1
  FROM core.tenants
  WHERE code = 'dev-bintrans'
    AND id <> :'tenant_id'::uuid
    AND deleted_at IS NULL
) AS tenant_code_collision;
