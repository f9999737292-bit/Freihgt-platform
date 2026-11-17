-- BINTRANS 0.1Q isolated proof: role model.
-- Runs as the disposable cluster bootstrap superuser proof_admin inside a
-- throwaway PostgreSQL 16 container (--network none). Not a platform migration.
-- Passwords are ephemeral, generated per attempt by run_proof.py and read from
-- the container environment with \getenv; they are never written to Git.
-- Required psql variables: main_db, pub_db.

\set ON_ERROR_STOP on
\getenv pw_issuer PROOF_PW_ISSUER
\getenv pw_issuer_b PROOF_PW_ISSUER_B
\getenv pw_writer PROOF_PW_WRITER
\getenv pw_other PROOF_PW_OTHER
\getenv pw_fencing PROOF_PW_FENCING
\getenv pw_fencing_direct PROOF_PW_FENCING_DIRECT
\getenv pw_shared PROOF_PW_SHARED
\getenv pw_owner PROOF_PW_OWNER
\getenv pw_fence_owner PROOF_PW_FENCE_OWNER

-- NOLOGIN owners. They carry passwords only so that a login attempt reaches the
-- rolcanlogin check (SQLSTATE 28000) instead of failing SCRAM first (28P01).
CREATE ROLE proof_authority_owner NOLOGIN NOINHERIT PASSWORD :'pw_owner';
CREATE ROLE proof_fence_owner NOLOGIN NOINHERIT CREATEROLE PASSWORD :'pw_fence_owner';

-- Separate runtime logins.
CREATE ROLE proof_issuer_login LOGIN NOINHERIT PASSWORD :'pw_issuer';
CREATE ROLE proof_issuer_b_login LOGIN NOINHERIT PASSWORD :'pw_issuer_b';
CREATE ROLE proof_writer_login LOGIN NOINHERIT PASSWORD :'pw_writer';
CREATE ROLE proof_other_service_login LOGIN NOINHERIT PASSWORD :'pw_other';

-- Independent handover controller. No CREATEROLE, no membership in any issuer
-- role; it acts only through proof_fence.* SECURITY DEFINER functions.
CREATE ROLE proof_fencing_login LOGIN NOINHERIT PASSWORD :'pw_fencing';

-- Counterexample controller: CREATEROLE + ADMIN on an issuer role, used only to
-- record what a directly privileged controller can do (password takeover).
CREATE ROLE proof_fencing_direct_login LOGIN NOINHERIT CREATEROLE PASSWORD :'pw_fencing_direct';

-- Counterexample shared privileged login (models one login shared by several
-- services with elevated rights). Test-only; not the real freight login.
CREATE ROLE proof_shared_login LOGIN SUPERUSER PASSWORD :'pw_shared';

-- proof_fence_owner may change LOGIN on issuer roles (PG16: CREATEROLE + ADMIN)
-- but cannot SET ROLE to them or inherit their privileges.
GRANT proof_issuer_login TO proof_fence_owner WITH ADMIN TRUE, INHERIT FALSE, SET FALSE;
GRANT proof_issuer_b_login TO proof_fence_owner WITH ADMIN TRUE, INHERIT FALSE, SET FALSE;
GRANT pg_signal_backend TO proof_fence_owner WITH INHERIT TRUE, SET FALSE;

GRANT proof_issuer_b_login TO proof_fencing_direct_login WITH ADMIN TRUE, INHERIT FALSE, SET FALSE;

-- Main database: no PUBLIC CONNECT, explicit CONNECT per login.
SELECT format('REVOKE ALL ON DATABASE %I FROM PUBLIC', :'main_db') \gexec
SELECT format('GRANT CONNECT ON DATABASE %I TO %I', :'main_db', r)
  FROM unnest(ARRAY['proof_issuer_login','proof_issuer_b_login','proof_writer_login',
                    'proof_other_service_login','proof_fencing_login',
                    'proof_fencing_direct_login','proof_shared_login']) AS r \gexec

-- Second database keeps the default PUBLIC CONNECT; it exists only for the
-- "REVOKE CONNECT from one role is insufficient" counterexample.
SELECT format('ALTER DATABASE %I OWNER TO proof_admin', :'pub_db') \gexec
