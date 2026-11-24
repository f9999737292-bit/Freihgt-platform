-- BINTRANS 0.1Q isolated proof: privilege catalog snapshot.
-- Runs as proof_admin. Each line of output is "CATALOG_JSON <name> <json>".
-- Role and privilege checks here are not tenant authorization.

\set ON_ERROR_STOP on
\pset format unaligned
\pset tuples_only on

SELECT 'CATALOG_JSON roles ' || json_agg(json_build_object(
  'rolname', rolname, 'rolcanlogin', rolcanlogin, 'rolsuper', rolsuper,
  'rolinherit', rolinherit, 'rolcreaterole', rolcreaterole,
  'rolcreatedb', rolcreatedb, 'rolreplication', rolreplication,
  'rolbypassrls', rolbypassrls) ORDER BY rolname)
FROM pg_roles WHERE rolname LIKE 'proof\_%' ESCAPE '\';

SELECT 'CATALOG_JSON memberships ' || coalesce(json_agg(json_build_object(
  'role', r.rolname, 'member', m.rolname, 'grantor', g.rolname,
  'admin_option', am.admin_option, 'inherit_option', am.inherit_option,
  'set_option', am.set_option) ORDER BY r.rolname, m.rolname), '[]')
FROM pg_auth_members am
JOIN pg_roles r ON r.oid = am.roleid
JOIN pg_roles m ON m.oid = am.member
JOIN pg_roles g ON g.oid = am.grantor
WHERE r.rolname LIKE 'proof\_%' ESCAPE '\' OR m.rolname LIKE 'proof\_%' ESCAPE '\';

WITH runtime(r) AS (VALUES ('proof_issuer_login'), ('proof_issuer_b_login'), ('proof_writer_login'),
                           ('proof_other_service_login'), ('proof_fencing_login')),
     target(t) AS (VALUES ('proof_authority_owner'), ('proof_fence_owner'), ('proof_issuer_login'),
                          ('proof_issuer_b_login'), ('proof_writer_login'), ('proof_fencing_login'),
                          ('proof_other_service_login'), ('proof_shared_login'), ('pg_signal_backend'))
SELECT 'CATALOG_JSON role_matrix ' || json_agg(json_build_object(
  'role', r, 'target', t,
  'member', pg_has_role(r, t, 'MEMBER'),
  'usage', pg_has_role(r, t, 'USAGE'),
  'set', pg_has_role(r, t, 'SET')) ORDER BY r, t)
FROM runtime, target WHERE r <> t;

SELECT 'CATALOG_JSON databases ' || json_agg(json_build_object(
  'datname', datname, 'datacl', datacl::text,
  'public_connect', has_database_privilege('public', datname, 'CONNECT')) ORDER BY datname)
FROM pg_database WHERE datname LIKE 'proof\_%' ESCAPE '\';

WITH roles(r) AS (VALUES ('proof_issuer_login'), ('proof_issuer_b_login'), ('proof_writer_login'),
                         ('proof_other_service_login'), ('proof_fencing_login'), ('proof_fencing_direct_login'),
                         ('proof_authority_owner'), ('proof_fence_owner'), ('proof_shared_login'))
SELECT 'CATALOG_JSON db_connect ' || json_agg(json_build_object(
  'role', r, 'datname', d.datname,
  'connect', has_database_privilege(r, d.datname, 'CONNECT')) ORDER BY r, d.datname)
FROM roles, pg_database d WHERE d.datname LIKE 'proof\_%' ESCAPE '\';

SELECT 'CATALOG_JSON functions ' || json_agg(json_build_object(
  'function', p.oid::regprocedure::text, 'owner', o.rolname,
  'prosecdef', p.prosecdef, 'proconfig', p.proconfig, 'proacl', p.proacl::text,
  'public_execute', EXISTS (SELECT 1 FROM aclexplode(coalesce(p.proacl, acldefault('f', p.proowner))) a
                             WHERE a.grantee = 0 AND a.privilege_type = 'EXECUTE')) ORDER BY 1)
FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace JOIN pg_roles o ON o.oid = p.proowner
WHERE n.nspname IN ('proof', 'proof_fence');

WITH roles(r) AS (VALUES ('proof_issuer_login'), ('proof_issuer_b_login'), ('proof_writer_login'),
                         ('proof_other_service_login'), ('proof_fencing_login'))
SELECT 'CATALOG_JSON function_execute ' || json_agg(json_build_object(
  'role', r, 'function', p.oid::regprocedure::text,
  'execute', has_function_privilege(r, p.oid, 'EXECUTE')) ORDER BY r, p.oid::regprocedure::text)
FROM roles, pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname IN ('proof', 'proof_fence');

WITH roles(r) AS (VALUES ('proof_issuer_login'), ('proof_issuer_b_login'), ('proof_writer_login'),
                         ('proof_other_service_login'), ('proof_fencing_login'))
SELECT 'CATALOG_JSON table_privileges ' || json_agg(json_build_object(
  'role', r, 'table', c.oid::regclass::text,
  'select', has_table_privilege(r, c.oid, 'SELECT'),
  'insert', has_table_privilege(r, c.oid, 'INSERT'),
  'update', has_table_privilege(r, c.oid, 'UPDATE'),
  'delete', has_table_privilege(r, c.oid, 'DELETE'),
  'truncate', has_table_privilege(r, c.oid, 'TRUNCATE')) ORDER BY r, c.oid::regclass::text)
FROM roles, pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname IN ('proof', 'proof_fence') AND c.relkind = 'r';

WITH roles(r) AS (VALUES ('proof_issuer_login'), ('proof_issuer_b_login'), ('proof_writer_login'),
                         ('proof_other_service_login'), ('proof_fencing_login'))
SELECT 'CATALOG_JSON column_privileges ' || json_agg(json_build_object(
  'role', r, 'column', col,
  'update', has_column_privilege(r, 'proof.business_rows', col, 'UPDATE')) ORDER BY r, col)
FROM roles, unnest(ARRAY['id', 'owner_epoch', 'note']) AS col;

WITH roles(r) AS (VALUES ('proof_issuer_login'), ('proof_issuer_b_login'), ('proof_writer_login'),
                         ('proof_other_service_login'), ('proof_fencing_login'))
SELECT 'CATALOG_JSON schema_privileges ' || json_agg(json_build_object(
  'role', r, 'schema', s,
  'usage', has_schema_privilege(r, s, 'USAGE'),
  'create', has_schema_privilege(r, s, 'CREATE')) ORDER BY r, s)
FROM roles, unnest(ARRAY['proof', 'proof_fence', 'public']) AS s;

SELECT 'CATALOG_JSON hba ' || json_agg(json_build_object(
  'line', line_number, 'type', type, 'database', database, 'user_name', user_name,
  'address', address, 'auth_method', auth_method, 'error', error) ORDER BY line_number)
FROM pg_hba_file_rules;

SELECT 'CATALOG_JSON server ' || json_build_object(
  'server_version', current_setting('server_version'),
  'server_version_num', current_setting('server_version_num'),
  'listen_addresses', current_setting('listen_addresses'),
  'log_connections', current_setting('log_connections'),
  'password_encryption', current_setting('password_encryption'));
