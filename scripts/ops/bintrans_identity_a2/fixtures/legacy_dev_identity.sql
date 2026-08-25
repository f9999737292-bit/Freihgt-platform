-- Legacy dev identity fixture for disposable local migration testing only.
-- Requires schema migrations applied. Does NOT create passwords suitable for API login.

\set ON_ERROR_STOP on

INSERT INTO core.tenants (id, code, name, country_code, default_locale, default_currency, status)
VALUES (
  '74519f22-ff9b-4a8b-8fff-a958c689682f',
  'dev-7rights',
  '7Rights Dev Tenant',
  'RU',
  'ru-RU',
  'RUB',
  'ACTIVE'
)
ON CONFLICT (id) DO UPDATE SET
  code = EXCLUDED.code,
  name = EXCLUDED.name,
  updated_at = now(),
  version = core.tenants.version + 1;

INSERT INTO core.companies (
  id, tenant_id, legal_name, short_name, company_type, country_code, preferred_locale, status
) VALUES (
  'a1111111-1111-4111-8111-111111111101',
  '74519f22-ff9b-4a8b-8fff-a958c689682f',
  'ООО 7Rights Dev',
  '7Rights Dev',
  'PLATFORM_OPERATOR',
  'RU',
  'ru-RU',
  'ACTIVE'
)
ON CONFLICT (id) DO UPDATE SET
  legal_name = EXCLUDED.legal_name,
  short_name = EXCLUDED.short_name,
  updated_at = now(),
  version = core.companies.version + 1;

INSERT INTO core.users (
  id, tenant_id, email, password_hash, full_name, preferred_locale, status
) VALUES
  ('8541a3a3-bde7-4fed-9501-37b9953bf904', '74519f22-ff9b-4a8b-8fff-a958c689682f', 'admin@7rights.local', 'legacy_fixture_no_login', '7Rights Dev Admin', 'ru-RU', 'ACTIVE'),
  ('008e1462-6f67-4246-b7dc-4aae1669c0c5', '74519f22-ff9b-4a8b-8fff-a958c689682f', 'shipper@7rights.local', 'legacy_fixture_no_login', 'Демо Грузовладелец', 'ru-RU', 'ACTIVE'),
  ('11111111-1111-4111-8111-111111111102', '74519f22-ff9b-4a8b-8fff-a958c689682f', 'carrier@7rights.local', 'legacy_fixture_no_login', 'Демо Перевозчик', 'ru-RU', 'ACTIVE'),
  ('11111111-1111-4111-8111-111111111103', '74519f22-ff9b-4a8b-8fff-a958c689682f', 'forwarder@7rights.local', 'legacy_fixture_no_login', 'Демо Экспедитор', 'ru-RU', 'ACTIVE'),
  ('11111111-1111-4111-8111-111111111104', '74519f22-ff9b-4a8b-8fff-a958c689682f', 'consignee@7rights.local', 'legacy_fixture_no_login', 'Демо Грузополучатель', 'ru-RU', 'ACTIVE')
ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email,
  full_name = EXCLUDED.full_name,
  updated_at = now(),
  version = core.users.version + 1;

INSERT INTO core.company_memberships (tenant_id, company_id, user_id, position, status)
SELECT
  '74519f22-ff9b-4a8b-8fff-a958c689682f',
  'a1111111-1111-4111-8111-111111111101',
  u.id,
  'Legacy fixture member',
  'ACTIVE'
FROM core.users u
WHERE u.tenant_id = '74519f22-ff9b-4a8b-8fff-a958c689682f'::uuid
  AND u.id = '8541a3a3-bde7-4fed-9501-37b9953bf904'::uuid
ON CONFLICT (company_id, user_id) DO NOTHING;

INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id)
SELECT
  '74519f22-ff9b-4a8b-8fff-a958c689682f',
  '8541a3a3-bde7-4fed-9501-37b9953bf904',
  'a1111111-1111-4111-8111-111111111101',
  r.id
FROM core.roles r
WHERE r.code = 'PLATFORM_ADMIN'
  AND (r.tenant_id IS NULL OR r.tenant_id = '74519f22-ff9b-4a8b-8fff-a958c689682f'::uuid)
  AND NOT EXISTS (
    SELECT 1 FROM core.user_roles ur
    WHERE ur.user_id = '8541a3a3-bde7-4fed-9501-37b9953bf904'::uuid
      AND ur.company_id = 'a1111111-1111-4111-8111-111111111101'::uuid
      AND ur.role_id = r.id
  )
LIMIT 1;
