-- step 1 [adm_observer backend pid] sent 2026-11-17T16:13:54.832555+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 60 [victim still present after denied raw terminate] sent 2026-11-17T16:14:01.745643+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 299))::text AS v;
-- step 61 [harness seeds namespace ns-t03] sent 2026-11-17T16:14:02.273193+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-t03', 1, 'proof_issuer_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-t03', 'OPEN', 'OPEN');
-- step 65 [issued_by of i-ok] sent 2026-11-17T16:14:02.511543+00:00
SELECT 'VAL=' || ((SELECT issued_by || ',' || issued_epoch FROM proof.tickets WHERE request_id = 'i-ok'))::text AS v;
-- step 66 [harness seeds namespace ns-main] sent 2026-11-17T16:14:02.738882+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-main', 1, 'proof_issuer_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-main', 'OPEN', 'OPEN');
-- step 67 [harness seeds namespace ns-n4] sent 2026-11-17T16:14:02.755000+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-n4', 1, 'proof_issuer_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-n4', 'OPEN', 'OPEN');
-- step 84 [iss1 pid present after ban] sent 2026-11-17T16:14:03.569463+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 337))::text AS v;
-- step 86 [count req-n4 committed] sent 2026-11-17T16:14:03.579989+00:00
SELECT 'VAL=' || ((SELECT count(*) FROM proof.tickets WHERE request_id = 'req-n4'))::text AS v;
-- step 93 [pids by application_name = proof-issuer] sent 2026-11-17T16:14:04.727522+00:00
SELECT 'VAL=' || ((SELECT array_agg(pid ORDER BY pid) FROM pg_stat_activity WHERE application_name = 'proof-issuer'))::text AS v;
-- step 99 [independent check right after signal for 337] sent 2026-11-17T16:14:04.756158+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 337))::text AS v;
-- step 101 [independent check right after signal for 340] sent 2026-11-17T16:14:04.763015+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 340))::text AS v;
-- step 103 [independent check right after signal for 351] sent 2026-11-17T16:14:04.768724+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 351))::text AS v;
-- step 105 [independent check right after signal for 352] sent 2026-11-17T16:14:04.775486+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 352))::text AS v;
-- step 106 [independent poll #1 pids] sent 2026-11-17T16:14:04.778182+00:00
SELECT 'VAL=' || ((SELECT coalesce(array_agg(pid ORDER BY pid), '{}') FROM pg_stat_activity WHERE pid = ANY('{352,337,340,351}'::int[])))::text AS v;
-- step 111 [req-open-1 rows after termination] sent 2026-11-17T16:14:04.884713+00:00
SELECT 'VAL=' || ((SELECT count(*) FROM proof.tickets WHERE request_id = 'req-open-1'))::text AS v;
-- step 112 [other service pid after termination] sent 2026-11-17T16:14:04.888765+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 353))::text AS v;
-- step 113 [writer pid after termination] sent 2026-11-17T16:14:04.891521+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 355))::text AS v;
-- step 115 [other-in-flight rows] sent 2026-11-17T16:14:04.949968+00:00
SELECT 'VAL=' || ((SELECT count(*) FROM proof.control_rows WHERE note = 'other-in-flight'))::text AS v;
-- step 119 [gate after try_open_gate] sent 2026-11-17T16:14:04.976702+00:00
SELECT 'VAL=' || ((SELECT gate || ',' || state FROM proof.handover_state WHERE issuer_namespace_id = 'ns-main'))::text AS v;
-- step 123 [harness seeds namespace ns-revoke] sent 2026-11-17T16:14:07.486041+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-revoke', 1, 'proof_issuer_b_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-revoke', 'OPEN', 'OPEN');
-- step 125 [REVOKE CONNECT main FROM issuer_b] sent 2026-11-17T16:14:07.765602+00:00
REVOKE CONNECT ON DATABASE "proof_0_1q_a08c1aa0" FROM proof_issuer_b_login;
-- step 128 [REVOKE CONNECT pub FROM issuer_b] sent 2026-11-17T16:14:08.041405+00:00
REVOKE CONNECT ON DATABASE "proof_pub_a08c1aa0" FROM proof_issuer_b_login;
-- step 130 [REVOKE CONNECT main FROM shared superuser] sent 2026-11-17T16:14:08.320520+00:00
REVOKE CONNECT ON DATABASE "proof_0_1q_a08c1aa0" FROM proof_shared_login;
-- step 132 [restore CONNECT] sent 2026-11-17T16:14:08.608163+00:00
GRANT CONNECT ON DATABASE "proof_0_1q_a08c1aa0" TO proof_issuer_b_login, proof_shared_login;
-- step 133 [harness seeds namespace ns-e1] sent 2026-11-17T16:14:08.843506+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-e1', 1, 'proof_issuer_b_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-e1', 'OPEN', 'OPEN');
-- step 137 [E1 wait_event_type of epoch changer] sent 2026-11-17T16:14:09.148235+00:00
SELECT 'VAL=' || ((SELECT coalesce(wait_event_type, 'none') FROM pg_stat_activity WHERE pid = 460))::text AS v;
-- step 138 [E1 wait_event_type of epoch changer] sent 2026-11-17T16:14:09.252453+00:00
SELECT 'VAL=' || ((SELECT coalesce(wait_event_type, 'none') FROM pg_stat_activity WHERE pid = 460))::text AS v;
-- step 140 [E1 ticket epoch vs current epoch] sent 2026-11-17T16:14:09.767650+00:00
SELECT 'VAL=' || ((SELECT t.issued_epoch || ',' || e.epoch FROM proof.tickets t JOIN proof.issuance_epoch e ON e.issuer_namespace_id = t.issuer_namespace_id WHERE t.request_id = 'req-e1'))::text AS v;
-- step 141 [harness seeds namespace ns-e2] sent 2026-11-17T16:14:09.773611+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-e2', 1, 'proof_issuer_b_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-e2', 'OPEN', 'OPEN');
-- step 143 [E2 epoch change commits] sent 2026-11-17T16:14:09.792326+00:00
UPDATE proof.issuance_epoch SET epoch = 2 WHERE issuer_namespace_id = 'ns-e2';
-- step 146 [E2 ticket epoch vs current] sent 2026-11-17T16:14:09.814831+00:00
SELECT 'VAL=' || ((SELECT t.issued_epoch || ',' || e.epoch FROM proof.tickets t JOIN proof.issuance_epoch e ON e.issuer_namespace_id = t.issuer_namespace_id WHERE t.request_id = 'req-e2'))::text AS v;
-- step 147 [harness seeds namespace ns-e2b] sent 2026-11-17T16:14:09.818355+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-e2b', 1, 'proof_issuer_b_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-e2b', 'OPEN', 'OPEN');
-- step 149 [E2b epoch change commits] sent 2026-11-17T16:14:09.835203+00:00
UPDATE proof.issuance_epoch SET epoch = 2 WHERE issuer_namespace_id = 'ns-e2b';
-- step 152 [harness seeds namespace ns-e2c] sent 2026-11-17T16:14:09.850130+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-e2c', 1, 'proof_issuer_b_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-e2c', 'OPEN', 'OPEN');
-- step 153 [E2c epoch change commits] sent 2026-11-17T16:14:09.862813+00:00
UPDATE proof.issuance_epoch SET epoch = 2 WHERE issuer_namespace_id = 'ns-e2c';
-- step 155 [harness seeds namespace ns-e4] sent 2026-11-17T16:14:10.351884+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-e4', 1, 'proof_issuer_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-e4', 'OPEN', 'OPEN');
-- step 159 [E4 harness terminates controller mid-transition] sent 2026-11-17T16:14:10.640166+00:00
SELECT 'VAL=' || (pg_terminate_backend(467, 5000))::text AS v;
-- step 160 [E4 controller pid gone] sent 2026-11-17T16:14:10.744055+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 467))::text AS v;
-- step 162 [E4 persisted state] sent 2026-11-17T16:14:10.774993+00:00
SELECT 'VAL=' || ((SELECT h.state || ',' || h.gate || ',' || e.epoch || ',' || e.holder FROM proof.handover_state h JOIN proof.issuance_epoch e USING (issuer_namespace_id) WHERE issuer_namespace_id = 'ns-e4'))::text AS v;
-- step 170 [create test-only probe: unrestricted terminate via pg_signal_backend] sent 2026-11-17T16:14:12.010705+00:00
SET ROLE proof_fence_owner;
BEGIN;
CREATE FUNCTION proof_fence.probe_terminate_any(p_pid int) RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog AS 'SELECT pg_terminate_backend(p_pid)';
REVOKE ALL ON FUNCTION proof_fence.probe_terminate_any(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION proof_fence.probe_terminate_any(int) TO proof_fencing_login;
COMMIT;
RESET ROLE;
-- step 172 [shared superuser backend still present] sent 2026-11-17T16:14:12.275623+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 481))::text AS v;
-- step 177 [final state dump] sent 2026-11-17T16:14:13.591718+00:00
SELECT 'STATE_JSON ' || json_build_object('tickets', (SELECT json_agg(t ORDER BY ticket_id) FROM (SELECT ticket_id, issuer_namespace_id, request_id, status, issued_epoch, issued_by FROM proof.tickets) t),'epochs', (SELECT json_agg(e ORDER BY issuer_namespace_id) FROM proof.issuance_epoch e),'handover', (SELECT json_agg(h ORDER BY issuer_namespace_id) FROM proof.handover_state h),'roles', (SELECT json_agg(json_build_object('rolname', rolname, 'rolcanlogin', rolcanlogin) ORDER BY rolname) FROM pg_roles WHERE rolname LIKE 'proof%'))::jsonb::text;
