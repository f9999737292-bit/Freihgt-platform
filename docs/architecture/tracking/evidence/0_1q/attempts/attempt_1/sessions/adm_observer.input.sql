-- step 1 [adm_observer backend pid] sent 2026-11-17T16:12:52.797332+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 60 [victim still present after denied raw terminate] sent 2026-11-17T16:12:59.513188+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 289))::text AS v;
-- step 61 [harness seeds namespace ns-t03] sent 2026-11-17T16:12:59.999073+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-t03', 1, 'proof_issuer_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-t03', 'OPEN', 'OPEN');
-- step 65 [issued_by of i-ok] sent 2026-11-17T16:13:00.258839+00:00
SELECT 'VAL=' || ((SELECT issued_by || ',' || issued_epoch FROM proof.tickets WHERE request_id = 'i-ok'))::text AS v;
-- step 66 [harness seeds namespace ns-main] sent 2026-11-17T16:13:00.490695+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-main', 1, 'proof_issuer_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-main', 'OPEN', 'OPEN');
-- step 67 [harness seeds namespace ns-n4] sent 2026-11-17T16:13:00.550862+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-n4', 1, 'proof_issuer_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-n4', 'OPEN', 'OPEN');
-- step 84 [iss1 pid present after ban] sent 2026-11-17T16:13:01.385677+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 333))::text AS v;
-- step 86 [count req-n4 committed] sent 2026-11-17T16:13:01.396279+00:00
SELECT 'VAL=' || ((SELECT count(*) FROM proof.tickets WHERE request_id = 'req-n4'))::text AS v;
-- step 93 [pids by application_name = proof-issuer] sent 2026-11-17T16:13:02.626408+00:00
SELECT 'VAL=' || ((SELECT array_agg(pid ORDER BY pid) FROM pg_stat_activity WHERE application_name = 'proof-issuer'))::text AS v;
-- step 99 [independent check right after signal for 333] sent 2026-11-17T16:13:02.651742+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 333))::text AS v;
-- step 101 [independent check right after signal for 334] sent 2026-11-17T16:13:02.658638+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 334))::text AS v;
-- step 103 [independent check right after signal for 335] sent 2026-11-17T16:13:02.664658+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 335))::text AS v;
-- step 105 [independent check right after signal for 336] sent 2026-11-17T16:13:02.671799+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 336))::text AS v;
-- step 106 [independent poll #1 pids] sent 2026-11-17T16:13:02.674771+00:00
SELECT 'VAL=' || ((SELECT coalesce(array_agg(pid ORDER BY pid), '{}') FROM pg_stat_activity WHERE pid = ANY('{336,333,334,335}'::int[])))::text AS v;
-- step 111 [req-open-1 rows after termination] sent 2026-11-17T16:13:02.778300+00:00
SELECT 'VAL=' || ((SELECT count(*) FROM proof.tickets WHERE request_id = 'req-open-1'))::text AS v;
-- step 112 [other service pid after termination] sent 2026-11-17T16:13:02.783225+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 343))::text AS v;
-- step 113 [writer pid after termination] sent 2026-11-17T16:13:02.785917+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 344))::text AS v;
-- step 115 [other-in-flight rows] sent 2026-11-17T16:13:02.796912+00:00
SELECT 'VAL=' || ((SELECT count(*) FROM proof.control_rows WHERE note = 'other-in-flight'))::text AS v;
-- step 119 [gate after try_open_gate] sent 2026-11-17T16:13:02.822193+00:00
SELECT 'VAL=' || ((SELECT gate || ',' || state FROM proof.handover_state WHERE issuer_namespace_id = 'ns-main'))::text AS v;
-- step 123 [harness seeds namespace ns-revoke] sent 2026-11-17T16:13:05.246814+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-revoke', 1, 'proof_issuer_b_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-revoke', 'OPEN', 'OPEN');
-- step 125 [REVOKE CONNECT main FROM issuer_b] sent 2026-11-17T16:13:05.594521+00:00
REVOKE CONNECT ON DATABASE "proof_0_1q_99e796c3" FROM proof_issuer_b_login;
-- step 128 [REVOKE CONNECT pub FROM issuer_b] sent 2026-11-17T16:13:05.890154+00:00
REVOKE CONNECT ON DATABASE "proof_pub_99e796c3" FROM proof_issuer_b_login;
-- step 130 [REVOKE CONNECT main FROM shared superuser] sent 2026-11-17T16:13:06.148685+00:00
REVOKE CONNECT ON DATABASE "proof_0_1q_99e796c3" FROM proof_shared_login;
-- step 132 [restore CONNECT] sent 2026-11-17T16:13:06.406233+00:00
GRANT CONNECT ON DATABASE "proof_0_1q_99e796c3" TO proof_issuer_b_login, proof_shared_login;
-- step 133 [harness seeds namespace ns-e1] sent 2026-11-17T16:13:06.640537+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-e1', 1, 'proof_issuer_b_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-e1', 'OPEN', 'OPEN');
-- step 137 [E1 wait_event_type of epoch changer] sent 2026-11-17T16:13:06.947444+00:00
SELECT 'VAL=' || ((SELECT coalesce(wait_event_type, 'none') FROM pg_stat_activity WHERE pid = 450))::text AS v;
-- step 138 [E1 wait_event_type of epoch changer] sent 2026-11-17T16:13:07.050970+00:00
SELECT 'VAL=' || ((SELECT coalesce(wait_event_type, 'none') FROM pg_stat_activity WHERE pid = 450))::text AS v;
-- step 140 [E1 ticket epoch vs current epoch] sent 2026-11-17T16:13:07.610450+00:00
SELECT 'VAL=' || ((SELECT t.issued_epoch || ',' || e.epoch FROM proof.tickets t JOIN proof.issuance_epoch e ON e.issuer_namespace_id = t.issuer_namespace_id WHERE t.request_id = 'req-e1'))::text AS v;
-- step 141 [harness seeds namespace ns-e2] sent 2026-11-17T16:13:07.612632+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-e2', 1, 'proof_issuer_b_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-e2', 'OPEN', 'OPEN');
-- step 143 [E2 epoch change commits] sent 2026-11-17T16:13:07.627538+00:00
UPDATE proof.issuance_epoch SET epoch = 2 WHERE issuer_namespace_id = 'ns-e2';
-- step 146 [E2 ticket epoch vs current] sent 2026-11-17T16:13:07.645377+00:00
SELECT 'VAL=' || ((SELECT t.issued_epoch || ',' || e.epoch FROM proof.tickets t JOIN proof.issuance_epoch e ON e.issuer_namespace_id = t.issuer_namespace_id WHERE t.request_id = 'req-e2'))::text AS v;
-- step 147 [harness seeds namespace ns-e2b] sent 2026-11-17T16:13:07.648727+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-e2b', 1, 'proof_issuer_b_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-e2b', 'OPEN', 'OPEN');
-- step 149 [E2b epoch change commits] sent 2026-11-17T16:13:07.662094+00:00
UPDATE proof.issuance_epoch SET epoch = 2 WHERE issuer_namespace_id = 'ns-e2b';
-- step 152 [harness seeds namespace ns-e2c] sent 2026-11-17T16:13:07.675764+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-e2c', 1, 'proof_issuer_b_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-e2c', 'OPEN', 'OPEN');
-- step 153 [E2c epoch change commits] sent 2026-11-17T16:13:07.686728+00:00
UPDATE proof.issuance_epoch SET epoch = 2 WHERE issuer_namespace_id = 'ns-e2c';
-- step 155 [harness seeds namespace ns-e4] sent 2026-11-17T16:13:08.228083+00:00
INSERT INTO proof.issuance_epoch VALUES ('ns-e4', 1, 'proof_issuer_login');
INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('ns-e4', 'OPEN', 'OPEN');
-- step 159 [E4 harness terminates controller mid-transition] sent 2026-11-17T16:13:08.532453+00:00
SELECT 'VAL=' || (pg_terminate_backend(457, 5000))::text AS v;
-- step 160 [E4 controller pid gone] sent 2026-11-17T16:13:08.636522+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 457))::text AS v;
-- step 162 [E4 persisted state] sent 2026-11-17T16:13:08.664205+00:00
SELECT 'VAL=' || ((SELECT h.state || ',' || h.gate || ',' || e.epoch || ',' || e.holder FROM proof.handover_state h JOIN proof.issuance_epoch e USING (issuer_namespace_id) WHERE issuer_namespace_id = 'ns-e4'))::text AS v;
-- step 170 [create test-only probe: unrestricted terminate via pg_signal_backend] sent 2026-11-17T16:13:09.856803+00:00
SET ROLE proof_fence_owner;
BEGIN;
CREATE FUNCTION proof_fence.probe_terminate_any(p_pid int) RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog AS 'SELECT pg_terminate_backend(p_pid)';
REVOKE ALL ON FUNCTION proof_fence.probe_terminate_any(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION proof_fence.probe_terminate_any(int) TO proof_fencing_login;
COMMIT;
RESET ROLE;
-- step 172 [shared superuser backend still present] sent 2026-11-17T16:13:10.099832+00:00
SELECT 'VAL=' || (EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = 471))::text AS v;
-- step 177 [final state dump] sent 2026-11-17T16:13:11.321929+00:00
SELECT 'STATE_JSON ' || json_build_object('tickets', (SELECT json_agg(t ORDER BY ticket_id) FROM (SELECT ticket_id, issuer_namespace_id, request_id, status, issued_epoch, issued_by FROM proof.tickets) t),'epochs', (SELECT json_agg(e ORDER BY issuer_namespace_id) FROM proof.issuance_epoch e),'handover', (SELECT json_agg(h ORDER BY issuer_namespace_id) FROM proof.handover_state h),'roles', (SELECT json_agg(json_build_object('rolname', rolname, 'rolcanlogin', rolcanlogin) ORDER BY rolname) FROM pg_roles WHERE rolname LIKE 'proof%'))::jsonb::text;
