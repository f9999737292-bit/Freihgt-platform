-- BINTRANS 0.1S isolated proof: test-only ticket table in a disposable database.
-- Not product DDL and not a platform migration. The row is both the ticket and
-- the idempotency tombstone for (issuer_namespace_id, request_id).
-- Runs as the disposable bootstrap superuser wproof_admin. The role
-- wproof_issuer is created once per container by run_proof.py.

\set ON_ERROR_STOP on
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO wproof_issuer;

CREATE TABLE public.tickets (
  ticket_id uuid PRIMARY KEY,
  issuer_namespace_id text NOT NULL,
  request_id text NOT NULL,
  payload_hash text NOT NULL CHECK (length(payload_hash) = 64),
  status text NOT NULL DEFAULT 'ISSUED',
  witness_seq bigint NOT NULL,
  issued_by text NOT NULL,
  issued_epoch bigint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE (issuer_namespace_id, request_id)
);

GRANT SELECT, INSERT ON public.tickets TO wproof_issuer;
