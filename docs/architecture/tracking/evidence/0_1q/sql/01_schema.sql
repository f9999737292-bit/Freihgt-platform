-- BINTRANS 0.1Q isolated proof: test-only authority schema.
-- This is a model of the 0.1P contract (PR #386 @ 994bc8a2), not product DDL and
-- not a platform migration. Runs as proof_admin in the disposable database.
-- Every function is created and has PUBLIC EXECUTE revoked inside the same
-- transaction, so no PUBLIC window exists. proof.unprotected_issue() is the one
-- deliberate exception (counterexample for a forgotten REVOKE).

\set ON_ERROR_STOP on
BEGIN;

CREATE SCHEMA proof AUTHORIZATION proof_authority_owner;
CREATE SCHEMA proof_fence AUTHORIZATION proof_fence_owner;
REVOKE ALL ON SCHEMA public FROM PUBLIC;

SET LOCAL ROLE proof_authority_owner;

CREATE TABLE proof.issuance_epoch (
  issuer_namespace_id text PRIMARY KEY,
  epoch bigint NOT NULL,
  holder name NOT NULL
);

CREATE TABLE proof.handover_state (
  issuer_namespace_id text PRIMARY KEY,
  state text NOT NULL,
  gate text NOT NULL CHECK (gate IN ('OPEN', 'CLOSED')),
  -- Never set to true in 0.1Q: no external witness exists in this model.
  witness_confirmed boolean NOT NULL DEFAULT false,
  old_holder name,
  new_holder name
);

CREATE TABLE proof.tickets (
  ticket_id bigserial PRIMARY KEY,
  issuer_namespace_id text NOT NULL,
  request_id text NOT NULL,
  payload_hash text NOT NULL,
  status text NOT NULL DEFAULT 'ISSUED',
  issued_epoch bigint NOT NULL,
  issued_by name NOT NULL,
  issued_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE (issuer_namespace_id, request_id)
);

CREATE TABLE proof.business_rows (
  id int PRIMARY KEY,
  owner_epoch bigint NOT NULL,
  note text
);

CREATE TABLE proof.control_rows (
  id bigserial PRIMARY KEY,
  written_by name NOT NULL DEFAULT session_user,
  note text
);

INSERT INTO proof.business_rows VALUES (1, 1, 'seed');

CREATE FUNCTION proof.issue_ticket(p_ns text, p_req text, p_hash text, p_expected_epoch bigint)
RETURNS bigint LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
  v_gate text;
  v_epoch bigint;
  v_holder name;
  v_id bigint;
BEGIN
  IF p_hash IS NULL OR length(p_hash) <> 64 THEN
    RAISE EXCEPTION 'ISSUE_DENIED_AUTH missing_digest' USING ERRCODE = 'P0001';
  END IF;
  SELECT gate INTO v_gate FROM proof.handover_state WHERE issuer_namespace_id = p_ns;
  IF v_gate IS DISTINCT FROM 'OPEN' THEN
    RAISE EXCEPTION 'GATE_CLOSED ns=%', p_ns USING ERRCODE = 'P0001';
  END IF;
  SELECT epoch, holder INTO v_epoch, v_holder
    FROM proof.issuance_epoch WHERE issuer_namespace_id = p_ns FOR UPDATE;
  IF v_epoch IS DISTINCT FROM p_expected_epoch THEN
    RAISE EXCEPTION 'STALE_EPOCH expected=% current=%', p_expected_epoch, v_epoch USING ERRCODE = 'P0001';
  END IF;
  IF v_holder IS DISTINCT FROM session_user THEN
    RAISE EXCEPTION 'NOT_EPOCH_HOLDER holder=% session=%', v_holder, session_user USING ERRCODE = 'P0001';
  END IF;
  INSERT INTO proof.tickets (issuer_namespace_id, request_id, payload_hash, issued_epoch, issued_by)
  VALUES (p_ns, p_req, p_hash, v_epoch, session_user)
  RETURNING ticket_id INTO v_id;
  RETURN v_id;
END $$;
REVOKE ALL ON FUNCTION proof.issue_ticket(text, text, text, bigint) FROM PUBLIC;

-- Same checks, but the epoch is read without FOR UPDATE (snapshot read).
CREATE FUNCTION proof.issue_ticket_snapshot(p_ns text, p_req text, p_hash text, p_expected_epoch bigint)
RETURNS bigint LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
  v_gate text;
  v_epoch bigint;
  v_holder name;
  v_id bigint;
BEGIN
  IF p_hash IS NULL OR length(p_hash) <> 64 THEN
    RAISE EXCEPTION 'ISSUE_DENIED_AUTH missing_digest' USING ERRCODE = 'P0001';
  END IF;
  SELECT gate INTO v_gate FROM proof.handover_state WHERE issuer_namespace_id = p_ns;
  IF v_gate IS DISTINCT FROM 'OPEN' THEN
    RAISE EXCEPTION 'GATE_CLOSED ns=%', p_ns USING ERRCODE = 'P0001';
  END IF;
  SELECT epoch, holder INTO v_epoch, v_holder
    FROM proof.issuance_epoch WHERE issuer_namespace_id = p_ns;
  IF v_epoch IS DISTINCT FROM p_expected_epoch THEN
    RAISE EXCEPTION 'STALE_EPOCH expected=% current=%', p_expected_epoch, v_epoch USING ERRCODE = 'P0001';
  END IF;
  IF v_holder IS DISTINCT FROM session_user THEN
    RAISE EXCEPTION 'NOT_EPOCH_HOLDER holder=% session=%', v_holder, session_user USING ERRCODE = 'P0001';
  END IF;
  INSERT INTO proof.tickets (issuer_namespace_id, request_id, payload_hash, issued_epoch, issued_by)
  VALUES (p_ns, p_req, p_hash, v_epoch, session_user)
  RETURNING ticket_id INTO v_id;
  RETURN v_id;
END $$;
REVOKE ALL ON FUNCTION proof.issue_ticket_snapshot(text, text, text, bigint) FROM PUBLIC;

CREATE FUNCTION proof.consume_ticket(p_ticket bigint)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
  UPDATE proof.tickets SET status = 'CONSUMED' WHERE ticket_id = p_ticket AND status = 'ISSUED';
  RETURN FOUND;
END $$;
REVOKE ALL ON FUNCTION proof.consume_ticket(bigint) FROM PUBLIC;

-- Phase A / C bookkeeping: any state change closes the gate.
CREATE FUNCTION proof.set_handover(p_ns text, p_state text)
RETURNS text LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
  IF p_state NOT IN ('HANDOVER_PENDING', 'RECONNECT_DENIED', 'QUIESCED') THEN
    RAISE EXCEPTION 'BAD_STATE %', p_state USING ERRCODE = 'P0001';
  END IF;
  UPDATE proof.handover_state SET state = p_state, gate = 'CLOSED' WHERE issuer_namespace_id = p_ns;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'UNKNOWN_NAMESPACE %', p_ns USING ERRCODE = 'P0001';
  END IF;
  RETURN p_state || ':CLOSED';
END $$;
REVOKE ALL ON FUNCTION proof.set_handover(text, text) FROM PUBLIC;

-- Phase D. The backend and rolcanlogin checks run before the epoch row lock;
-- they are only meaningful because reconnect was denied first (phase B).
CREATE FUNCTION proof.transition_epoch(p_ns text, p_expected_epoch bigint, p_new_holder name)
RETURNS bigint LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
  v_state text;
  v_old name;
  v_epoch bigint;
  v_n int;
BEGIN
  PERFORM pg_stat_clear_snapshot();
  SELECT state INTO v_state FROM proof.handover_state WHERE issuer_namespace_id = p_ns FOR UPDATE;
  IF v_state IS DISTINCT FROM 'QUIESCED' THEN
    RAISE EXCEPTION 'STATE_NOT_QUIESCED state=%', v_state USING ERRCODE = 'P0001';
  END IF;
  SELECT holder INTO v_old FROM proof.issuance_epoch WHERE issuer_namespace_id = p_ns;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = v_old AND rolcanlogin) THEN
    RAISE EXCEPTION 'OLD_HOLDER_CAN_LOGIN holder=%', v_old USING ERRCODE = 'P0001';
  END IF;
  SELECT count(*) INTO v_n FROM pg_stat_activity
   WHERE usesysid = (SELECT oid FROM pg_roles WHERE rolname = v_old);
  IF v_n > 0 THEN
    RAISE EXCEPTION 'OLD_BACKENDS_PRESENT holder=% count=%', v_old, v_n USING ERRCODE = 'P0001';
  END IF;
  SELECT epoch INTO v_epoch FROM proof.issuance_epoch WHERE issuer_namespace_id = p_ns FOR UPDATE;
  IF v_epoch IS DISTINCT FROM p_expected_epoch THEN
    RAISE EXCEPTION 'STALE_EPOCH expected=% current=%', p_expected_epoch, v_epoch USING ERRCODE = 'P0001';
  END IF;
  UPDATE proof.issuance_epoch SET epoch = epoch + 1, holder = p_new_holder WHERE issuer_namespace_id = p_ns;
  UPDATE proof.handover_state
     SET state = 'TRANSITIONED', gate = 'CLOSED', old_holder = v_old, new_holder = p_new_holder
   WHERE issuer_namespace_id = p_ns;
  RETURN v_epoch + 1;
END $$;
REVOKE ALL ON FUNCTION proof.transition_epoch(text, bigint, name) FROM PUBLIC;

-- Phase E. Opens only when every condition holds. witness_confirmed is never
-- set in 0.1Q, so this function cannot open the gate here.
CREATE FUNCTION proof.try_open_gate(p_ns text)
RETURNS text LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
  r proof.handover_state%ROWTYPE;
  v_n int;
BEGIN
  PERFORM pg_stat_clear_snapshot();
  SELECT * INTO r FROM proof.handover_state WHERE issuer_namespace_id = p_ns FOR UPDATE;
  IF r.state IS DISTINCT FROM 'TRANSITIONED' THEN
    RETURN 'CLOSED:STATE_' || coalesce(r.state, 'UNKNOWN');
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = r.old_holder AND rolcanlogin) THEN
    RETURN 'CLOSED:OLD_HOLDER_CAN_LOGIN';
  END IF;
  SELECT count(*) INTO v_n FROM pg_stat_activity
   WHERE usesysid = (SELECT oid FROM pg_roles WHERE rolname = r.old_holder);
  IF v_n > 0 THEN
    RETURN 'CLOSED:OLD_BACKENDS_PRESENT';
  END IF;
  IF NOT r.witness_confirmed THEN
    RETURN 'CLOSED:WITNESS_NOT_CONFIRMED';
  END IF;
  UPDATE proof.handover_state SET gate = 'OPEN', state = 'OPEN' WHERE issuer_namespace_id = p_ns;
  RETURN 'OPEN';
END $$;
REVOKE ALL ON FUNCTION proof.try_open_gate(text) FROM PUBLIC;

-- Counterexample: PUBLIC EXECUTE deliberately left at the default.
CREATE FUNCTION proof.unprotected_issue(p_req text)
RETURNS bigint LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_id bigint;
BEGIN
  INSERT INTO proof.tickets (issuer_namespace_id, request_id, payload_hash, issued_epoch, issued_by)
  VALUES ('ns-unprotected', p_req, repeat('0', 64), 0, session_user)
  RETURNING ticket_id INTO v_id;
  RETURN v_id;
END $$;

GRANT USAGE ON SCHEMA proof TO proof_issuer_login, proof_issuer_b_login, proof_writer_login,
  proof_other_service_login, proof_fencing_login;
GRANT EXECUTE ON FUNCTION proof.issue_ticket(text, text, text, bigint),
  proof.issue_ticket_snapshot(text, text, text, bigint)
  TO proof_issuer_login, proof_issuer_b_login;
GRANT SELECT ON proof.issuance_epoch TO proof_issuer_login, proof_issuer_b_login;
GRANT EXECUTE ON FUNCTION proof.consume_ticket(bigint) TO proof_writer_login;
GRANT SELECT ON proof.business_rows TO proof_writer_login;
GRANT UPDATE (note) ON proof.business_rows TO proof_writer_login;
GRANT EXECUTE ON FUNCTION proof.set_handover(text, text), proof.transition_epoch(text, bigint, name),
  proof.try_open_gate(text) TO proof_fencing_login;
GRANT INSERT, SELECT ON proof.control_rows TO proof_other_service_login, proof_writer_login;
GRANT USAGE ON SEQUENCE proof.control_rows_id_seq TO proof_other_service_login, proof_writer_login;

RESET ROLE;
SET LOCAL ROLE proof_fence_owner;

CREATE TABLE proof_fence.allowlist (target name PRIMARY KEY);
INSERT INTO proof_fence.allowlist VALUES ('proof_issuer_login'), ('proof_issuer_b_login');

-- Phase B: deny new sessions for an allowlisted issuer login.
CREATE FUNCTION proof_fence.deny_login(p_target name)
RETURNS text LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM proof_fence.allowlist WHERE target = p_target) THEN
    RAISE EXCEPTION 'TARGET_NOT_ALLOWLISTED %', p_target USING ERRCODE = 'P0001';
  END IF;
  EXECUTE format('ALTER ROLE %I NOLOGIN', p_target);
  RETURN 'NOLOGIN:' || p_target;
END $$;
REVOKE ALL ON FUNCTION proof_fence.deny_login(name) FROM PUBLIC;

-- Phase C census: backends by role OID, never by application_name.
CREATE FUNCTION proof_fence.census(p_target name)
RETURNS int[] LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v int[];
BEGIN
  IF NOT EXISTS (SELECT 1 FROM proof_fence.allowlist WHERE target = p_target) THEN
    RAISE EXCEPTION 'TARGET_NOT_ALLOWLISTED %', p_target USING ERRCODE = 'P0001';
  END IF;
  PERFORM pg_stat_clear_snapshot();
  SELECT coalesce(array_agg(pid ORDER BY pid), '{}') INTO v FROM pg_stat_activity
   WHERE usesysid = (SELECT oid FROM pg_roles WHERE rolname = p_target);
  RETURN v;
END $$;
REVOKE ALL ON FUNCTION proof_fence.census(name) FROM PUBLIC;

-- Phase C termination: signals one pid only if it belongs to the target role.
-- The boolean result means "signal sent", not "backend gone".
CREATE FUNCTION proof_fence.terminate(p_target name, p_pid int)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM proof_fence.allowlist WHERE target = p_target) THEN
    RAISE EXCEPTION 'TARGET_NOT_ALLOWLISTED %', p_target USING ERRCODE = 'P0001';
  END IF;
  PERFORM pg_stat_clear_snapshot();
  IF NOT EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = p_pid
                   AND usesysid = (SELECT oid FROM pg_roles WHERE rolname = p_target)) THEN
    RAISE EXCEPTION 'PID_NOT_OWNED_BY_TARGET pid=% target=%', p_pid, p_target USING ERRCODE = 'P0001';
  END IF;
  RETURN pg_terminate_backend(p_pid);
END $$;
REVOKE ALL ON FUNCTION proof_fence.terminate(name, int) FROM PUBLIC;

GRANT USAGE ON SCHEMA proof_fence TO proof_fencing_login;
GRANT EXECUTE ON FUNCTION proof_fence.deny_login(name), proof_fence.census(name),
  proof_fence.terminate(name, int) TO proof_fencing_login;

RESET ROLE;
COMMIT;
