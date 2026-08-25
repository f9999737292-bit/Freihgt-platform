-- Collision fixture: different UUID already owns admin@bintrans.local
-- Use only on disposable DB after legacy_dev_identity.sql

\set ON_ERROR_STOP on

INSERT INTO core.users (
  id, tenant_id, email, password_hash, full_name, preferred_locale, status
) VALUES (
  '22222222-2222-4222-8222-222222222222',
  '74519f22-ff9b-4a8b-8fff-a958c689682f',
  'admin@bintrans.local',
  'collision_fixture_no_login',
  'Collision Bintrans Admin',
  'ru-RU',
  'ACTIVE'
)
ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email,
  updated_at = now(),
  version = core.users.version + 1;
