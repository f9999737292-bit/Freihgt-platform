# Attempt 1 results

- Container: `bintrans-proof-0-1q-99e796c3` (network `none`, no published ports), removed: True
- Databases: `proof_0_1q_99e796c3`, `proof_pub_99e796c3`
- Server: PostgreSQL 16.15 (Debian 16.15-1.pgdg13+2) on x86_64-pc-linux-gnu, compiled by gcc (Debian 14.2.0-19) 14.2.0, 64-bit
- Started: 2026-11-17T16:12:44.474637+00:00; finished: 2026-11-17T16:13:12.170499+00:00
- Harness error: None
- Assertion counts: {"OBSERVED": 2, "PASS": 119}

`COUNTEREXAMPLE_REPRODUCED` with PASS means the unsafe behavior was observed as predicted. `OBSERVATION` rows record an outcome without a pass/fail expectation.

| ID | Kind | Result | Description |
| --- | --- | --- | --- |
| `T01-LOGIN-proof_issuer_login` | POSITIVE_CONTROL | PASS | proof_issuer_login logs in with its own credentials; session_user = current_user = proof_issuer_login |
| `T01-LOGIN-proof_issuer_b_login` | POSITIVE_CONTROL | PASS | proof_issuer_b_login logs in with its own credentials; session_user = current_user = proof_issuer_b_login |
| `T01-LOGIN-proof_writer_login` | POSITIVE_CONTROL | PASS | proof_writer_login logs in with its own credentials; session_user = current_user = proof_writer_login |
| `T01-LOGIN-proof_other_service_login` | POSITIVE_CONTROL | PASS | proof_other_service_login logs in with its own credentials; session_user = current_user = proof_other_service_login |
| `T01-LOGIN-proof_fencing_login` | POSITIVE_CONTROL | PASS | proof_fencing_login logs in with its own credentials; session_user = current_user = proof_fencing_login |
| `T01-LOGIN-proof_fencing_direct_login` | POSITIVE_CONTROL | PASS | proof_fencing_direct_login logs in with its own credentials; session_user = current_user = proof_fencing_direct_login |
| `T01-LOGIN-proof_shared_login` | POSITIVE_CONTROL | PASS | proof_shared_login logs in with its own credentials; session_user = current_user = proof_shared_login |
| `T01-NOLOGIN-proof_authority_owner` | DENIAL | PASS | proof_authority_owner is NOLOGIN; login is rejected with 28000 |
| `T01-NOLOGIN-proof_fence_owner` | DENIAL | PASS | proof_fence_owner is NOLOGIN; login is rejected with 28000 |
| `T01-CAT-RUNTIME-ATTRS` | POSITIVE_CONTROL | PASS | Runtime logins (issuer, issuer_b, writer, other service, fencing) are LOGIN and have no SUPERUSER/CREATEROLE/CREATEDB/REPLICATION/BYPASSRLS |
| `T01-CAT-OWNERS-NOLOGIN` | POSITIVE_CONTROL | PASS | Owner roles are NOLOGIN |
| `T01-CAT-NO-MEMBERSHIP` | POSITIVE_CONTROL | PASS | No runtime login is a member of, can use the privileges of, or can SET ROLE to any owner, issuer, writer, fencing, shared, or pg_signal_backend role |
| `T01-CAT-NO-AUTH-MEMBERS` | POSITIVE_CONTROL | PASS | pg_auth_members has no rows whose member is a runtime login |
| `T01-CAT-SECDEF-SEARCH-PATH` | POSITIVE_CONTROL | PASS | Every proof/proof_fence function is SECURITY DEFINER with proconfig = {search_path=pg_catalog} |
| `T01-CAT-PUBLIC-EXECUTE` | POSITIVE_CONTROL | PASS | Only the deliberate counterexample proof.unprotected_issue(text) keeps PUBLIC EXECUTE |
| `T01-CAT-DB-CONNECT` | POSITIVE_CONTROL | PASS | Main database has no PUBLIC CONNECT; the counterexample database keeps PUBLIC CONNECT |
| `T01-CAT-NO-PROTECTED-DML` | POSITIVE_CONTROL | PASS | No runtime login has INSERT/UPDATE/DELETE/TRUNCATE on tickets, issuance_epoch, handover_state, allowlist |
| `T01-CAT-ISSUER-NO-BUSINESS` | POSITIVE_CONTROL | PASS | Issuer logins can only SELECT proof.issuance_epoch; no business-table rights |
| `T01-CAT-WRITER-COLUMNS` | POSITIVE_CONTROL | PASS | Writer may UPDATE business_rows.note only, not owner_epoch or id |
| `T01-CAT-EXECUTE-MATRIX` | POSITIVE_CONTROL | PASS | EXECUTE matrix matches the model: issuer=issue only, writer=consume only, fencing=handover/fence only, other service=none (plus PUBLIC counterexample) |
| `T01-CAT-NO-SCHEMA-CREATE` | POSITIVE_CONTROL | PASS | No runtime login has CREATE on proof, proof_fence, or public |
| `T02-proof_writer_login-1` | DENIAL | PASS | proof_writer_login: SET ROLE proof_issuer_login is rejected with 42501 |
| `T02-proof_writer_login-2` | DENIAL | PASS | proof_writer_login: SET ROLE proof_authority_owner is rejected with 42501 |
| `T02-proof_writer_login-3` | DENIAL | PASS | proof_writer_login: SET ROLE proof_fencing_login is rejected with 42501 |
| `T02-proof_writer_login-4` | DENIAL | PASS | proof_writer_login: SET ROLE proof_fence_owner is rejected with 42501 |
| `T02-proof_writer_login-5` | DENIAL | PASS | proof_writer_login: SET SESSION AUTHORIZATION proof_issuer_login is rejected with 42501 |
| `T02-proof_writer_login-IDENTITY` | POSITIVE_CONTROL | PASS | proof_writer_login: session_user and current_user unchanged after denied SET ROLE |
| `T02-proof_issuer_login-1` | DENIAL | PASS | proof_issuer_login: SET ROLE proof_authority_owner is rejected with 42501 |
| `T02-proof_issuer_login-2` | DENIAL | PASS | proof_issuer_login: SET ROLE proof_writer_login is rejected with 42501 |
| `T02-proof_issuer_login-3` | DENIAL | PASS | proof_issuer_login: SET ROLE proof_fence_owner is rejected with 42501 |
| `T02-proof_issuer_login-IDENTITY` | POSITIVE_CONTROL | PASS | proof_issuer_login: session_user and current_user unchanged after denied SET ROLE |
| `T02-proof_fencing_login-1` | DENIAL | PASS | proof_fencing_login: SET ROLE proof_issuer_login is rejected with 42501 |
| `T02-proof_fencing_login-2` | DENIAL | PASS | proof_fencing_login: SET ROLE proof_fence_owner is rejected with 42501 |
| `T02-proof_fencing_login-3` | DENIAL | PASS | proof_fencing_login: SET ROLE proof_authority_owner is rejected with 42501 |
| `T02-proof_fencing_login-IDENTITY` | POSITIVE_CONTROL | PASS | proof_fencing_login: session_user and current_user unchanged after denied SET ROLE |
| `T02-proof_fencing_direct_login-1` | DENIAL | PASS | proof_fencing_direct_login: SET ROLE proof_issuer_b_login is rejected with 42501 |
| `T02-proof_fencing_direct_login-IDENTITY` | POSITIVE_CONTROL | PASS | proof_fencing_direct_login: session_user and current_user unchanged after denied SET ROLE |
| `T02-proof_other_service_login-1` | DENIAL | PASS | proof_other_service_login: SET ROLE proof_issuer_login is rejected with 42501 |
| `T02-proof_other_service_login-2` | DENIAL | PASS | proof_other_service_login: SET ROLE proof_writer_login is rejected with 42501 |
| `T02-proof_other_service_login-IDENTITY` | POSITIVE_CONTROL | PASS | proof_other_service_login: session_user and current_user unchanged after denied SET ROLE |
| `T03-W-INSERT-TICKET` | DENIAL | PASS | writer INSERT into proof.tickets -> 42501 |
| `T03-W-UPDATE-TICKET` | DENIAL | PASS | writer UPDATE proof.tickets status -> 42501 |
| `T03-W-CALL-ISSUE` | DENIAL | PASS | writer calls proof.issue_ticket -> 42501 |
| `T03-W-CALL-ISSUE-SNAP` | DENIAL | PASS | writer calls proof.issue_ticket_snapshot -> 42501 |
| `T03-W-UPDATE-EPOCH-COL` | DENIAL | PASS | writer UPDATE business_rows.owner_epoch (sensitive column) -> 42501 |
| `T03-W-UPDATE-NOTE` | POSITIVE_CONTROL | PASS | writer UPDATE business_rows.note (granted column) -> succeeds |
| `T03-W-DELETE` | DENIAL | PASS | writer DELETE business_rows -> 42501 |
| `T03-W-EPOCH` | DENIAL | PASS | writer UPDATE proof.issuance_epoch -> 42501 |
| `T03-W-HANDOVER` | DENIAL | PASS | writer calls proof.set_handover -> 42501 |
| `T03-W-FENCE` | DENIAL | PASS | writer calls proof_fence.deny_login -> 42501 |
| `T03-W-CREATE-FN` | DENIAL | PASS | writer CREATE FUNCTION in schema proof -> 42501 |
| `T03-W-CONSUME` | POSITIVE_CONTROL | PASS | writer calls proof.consume_ticket (granted) -> succeeds |
| `T03-W-UNPROTECTED` | COUNTEREXAMPLE_REPRODUCED | PASS | COUNTEREXAMPLE: writer calls proof.unprotected_issue (PUBLIC EXECUTE left at default) -> succeeds |
| `T03-I-INSERT-TICKET` | DENIAL | PASS | issuer INSERT into proof.tickets directly -> 42501 |
| `T03-I-UPDATE-EPOCH` | DENIAL | PASS | issuer UPDATE proof.issuance_epoch directly -> 42501 |
| `T03-I-SELECT-BUSINESS` | DENIAL | PASS | issuer SELECT proof.business_rows -> 42501 |
| `T03-I-UPDATE-BUSINESS` | DENIAL | PASS | issuer UPDATE proof.business_rows -> 42501 |
| `T03-I-CONSUME` | DENIAL | PASS | issuer calls proof.consume_ticket -> 42501 |
| `T03-I-FENCE` | DENIAL | PASS | issuer calls proof_fence.census -> 42501 |
| `T03-O-CALL-ISSUE` | DENIAL | PASS | other service calls proof.issue_ticket -> 42501 |
| `T03-O-SELECT-TICKETS` | DENIAL | PASS | other service SELECT proof.tickets -> 42501 |
| `T03-O-CONTROL-INSERT` | POSITIVE_CONTROL | PASS | other service INSERT into its own control table -> succeeds |
| `T03-O-UNPROTECTED` | COUNTEREXAMPLE_REPRODUCED | PASS | COUNTEREXAMPLE: other service calls proof.unprotected_issue via PUBLIC -> succeeds |
| `T03-F-INSERT-TICKET` | DENIAL | PASS | fencing INSERT into proof.tickets -> 42501 |
| `T03-F-CALL-ISSUE` | DENIAL | PASS | fencing calls proof.issue_ticket -> 42501 |
| `T03-F-ALTER-PASSWORD` | DENIAL | PASS | fencing ALTER ROLE proof_issuer_login PASSWORD (takeover attempt) -> 42501 |
| `T03-F-ALTER-NOLOGIN-DIRECT` | DENIAL | PASS | fencing ALTER ROLE proof_issuer_login NOLOGIN directly (outside definer) -> 42501 |
| `T03-F-SELF-GRANT` | DENIAL | PASS | fencing GRANT proof_issuer_login TO itself -> 42501 |
| `T03-F-TERMINATE-RAW` | DENIAL | PASS | fencing calls pg_terminate_backend directly on a live other-service backend (no pg_signal_backend) -> 42501 |
| `T03-F-TERMINATE-RAW-VICTIM-ALIVE` | POSITIVE_CONTROL | PASS | The other-service backend survives the denied raw terminate |
| `T03-I-NO-DIGEST` | DENIAL | PASS | issuer issue_ticket without payload digest -> ISSUE_DENIED_AUTH -> P0001 |
| `T03-I-ISSUE-OK` | POSITIVE_CONTROL | PASS | issuer issue_ticket with digest and current epoch (granted path) -> succeeds |
| `T03-I-DUPLICATE` | DENIAL | PASS | issuer issue_ticket same (ns, request_id) again -> unique violation -> 23505 |
| `T03-I-ISSUED-BY` | POSITIVE_CONTROL | PASS | Ticket records issued_by = session_user of the issuer login (not the definer owner) |
| `T04-PRE-ISS1-OPEN-TXN` | POSITIVE_CONTROL | PASS | Old issuer opens a transaction that issues a ticket |
| `T04-PRE-ISS4-OPEN-TXN` | POSITIVE_CONTROL | PASS | Second old-issuer transaction issues a ticket |
| `T04-PRE-OTHER-OPEN-TXN` | POSITIVE_CONTROL | PASS | Other service holds an open transaction |
| `T04-A-CLOSE` | POSITIVE_CONTROL | PASS | Fencing controller closes the issuance gates |
| `T04-A-NEW-ISSUE-DENIED` | DENIAL | PASS | A new issuance attempt after CLOSE fails with GATE_CLOSED |
| `T04-B-DENY` | POSITIVE_CONTROL | PASS | Fencing controller sets NOLOGIN on the old issuer via the definer function |
| `T04-B-RECONNECT-DENIED` | DENIAL | PASS | Old issuer reconnect after ban fails with 28000 (not permitted to log in) |
| `T04-B-EXISTING-SURVIVES-BAN` | COUNTEREXAMPLE_REPRODUCED | PASS | Existing old-issuer backend with an open transaction stays alive and usable after NOLOGIN |
| `T04-N4-INFLIGHT-COMMIT` | COUNTEREXAMPLE_REPRODUCED | PASS | Old issuer COMMIT before quiesce succeeds even after CLOSE and DENY (gate/ban are not commit fences) |
| `T04-D-POOL-RECONNECT` | DENIAL | PASS | Five pool reconnects (3 tcp, 2 unix socket) are all denied with 28000 |
| `T04-C-CENSUS-EXACT` | POSITIVE_CONTROL | PASS | Census by role OID returns exactly the four old-issuer backends |
| `T04-C-CENSUS-EXCLUDES-OTHERS` | POSITIVE_CONTROL | PASS | Census excludes the other service (spoofed application_name), writer, and controller |
| `T04-ORDER-DENY-BEFORE-CENSUS` | ORDERING | PASS | Deny committed, then reconnect denial observed, then census started |
| `T04-C-APPNAME-SPOOF` | COUNTEREXAMPLE_REPRODUCED | PASS | A census by application_name would include the other service (spoofed name) and is unsafe |
| `T04-C-NO-EPOCH-WITH-OLD-BACKENDS` | DENIAL | PASS | New epoch is refused while old-issuer backends remain (OLD_BACKENDS_PRESENT) |
| `T04-C-WRONG-PID-REFUSED` | DENIAL | PASS | Controller refuses to terminate a pid not owned by the target role |
| `T04-C-TARGET-NOT-ALLOWLISTED` | DENIAL | PASS | Controller refuses a non-allowlisted target role |
| `T04-C-SIGNALS-SENT` | POSITIVE_CONTROL | PASS | pg_terminate_backend returned true for every census pid |
| `T04-C-SIGNAL-VS-GONE` | OBSERVATION | OBSERVED | Was the pid still visible to an independent observer immediately after terminate returned true? |
| `T04-C-BACKENDS-GONE` | POSITIVE_CONTROL | PASS | Independent observer (separate admin session) confirms all old-issuer pids are gone |
| `T04-C-CLIENTS-SEE-TERMINATION` | POSITIVE_CONTROL | PASS | Every terminated client observes connection loss (FATAL 57P01 or lost connection) and psql exits |
| `T04-B-ISS1-57P01` | POSITIVE_CONTROL | PASS | The open-transaction client reports FATAL 57P01 (administrator termination) on COMMIT |
| `T04-B-ROLLBACK` | POSITIVE_CONTROL | PASS | The terminated open transaction was rolled back (0 rows for req-open-1) |
| `T04-C-OTHER-SERVICES-UNAFFECTED` | POSITIVE_CONTROL | PASS | Other service (spoofed application_name) and writer sessions survive and commit |
| `T04-D-TRANSITION` | POSITIVE_CONTROL | PASS | After ban + confirmed termination, transition to epoch 2 / proof_issuer_b_login succeeds |
| `T04-E-GATE-STAYS-CLOSED` | DENIAL | PASS | Gate stays CLOSED because no external witness confirmation exists (witness NOT_RUN) |
| `T04-E-NEW-HOLDER-BLOCKED` | DENIAL | PASS | New epoch holder cannot issue while the gate is CLOSED |
| `T04-E-OLD-IDENTITY-REJECTED` | DENIAL | PASS | Old issuer credentials cannot open a session after handover |
| `T04-C-CLIENT-EXIT-CODES` | POSITIVE_CONTROL | PASS | Terminated psql clients exit with code 2 (connection lost) |
| `T05-N1-REVOKE-ACTIVE-BACKEND` | COUNTEREXAMPLE_REPRODUCED | PASS | REVOKE CONNECT does not affect an already-open backend; it still issues and commits |
| `T05-N1-REVOKE-BLOCKS-NEW` | DENIAL | PASS | New connection after REVOKE CONNECT fails with 42501 |
| `T05-N2-INSUFFICIENT-REVOKE` | COUNTEREXAMPLE_REPRODUCED | PASS | REVOKE CONNECT from one role is insufficient while PUBLIC keeps CONNECT: connection succeeds |
| `T05-N3-SUPERUSER-IGNORES-REVOKE` | COUNTEREXAMPLE_REPRODUCED | PASS | A superuser login ignores REVOKE CONNECT |
| `T06-E1-LOCK-NOT-FENCE` | COUNTEREXAMPLE_REPRODUCED | PASS | Epoch change waits on the holder's FOR UPDATE lock; the holder still commits under epoch 1, then the epoch becomes 2 |
| `T06-E2-STALE-SNAPSHOT-COMMIT` | COUNTEREXAMPLE_REPRODUCED | PASS | A snapshot read of the epoch commits a ticket under epoch 1 after epoch 2 was committed |
| `T06-E2B-RR-FOR-UPDATE` | DENIAL | PASS | FOR UPDATE on a concurrently changed epoch row under RR fails with 40001 |
| `T06-E2C-RC-STALE-REJECTED` | DENIAL | PASS | A statement that starts after the epoch change is rejected (STALE_EPOCH) |
| `T06-E4-UNKNOWN-OUTCOME` | POSITIVE_CONTROL | PASS | Controller terminated mid-transition: client never sees COMMIT, epoch unchanged, state HANDOVER_PENDING, gate CLOSED |
| `T07-E-SHARED-LOGIN-BYPASS` | COUNTEREXAMPLE_REPRODUCED | PASS | A shared superuser login bypasses every control: forged direct INSERT, gate reopened, issuer identity assumed, NOLOGIN undone and old issuer reconnects |
| `T07-E-SUPERUSER-NOT-TERMINABLE` | COUNTEREXAMPLE_REPRODUCED | PASS | pg_signal_backend cannot terminate a superuser backend (42501); the shared login stays connected |
| `T07-F-DIRECT-CONTROLLER-TAKEOVER` | COUNTEREXAMPLE_REPRODUCED | PASS | A controller holding CREATEROLE + ADMIN on an issuer role can reset its password and log in as that issuer |
| `T07-F-DIRECT-CONTROLLER-SELF-GRANT` | OBSERVATION | OBSERVED | Outcome of a CREATEROLE + ADMIN controller granting the issuer role to itself WITH SET TRUE, then SET ROLE |
| `T08-LOG-28000` | POSITIVE_CONTROL | PASS | Server log records exactly 8 old-issuer reconnect denials with SQLSTATE 28000 (1 phase-B verification + 5 pool reconnects + 2 post-handover attempts) |
| `T08-LOG-42501` | POSITIVE_CONTROL | PASS | Server log records the REVOKE CONNECT denial for issuer_b with SQLSTATE 42501 |
| `T08-LOG-57P01` | POSITIVE_CONTROL | PASS | Server log records administrator termination (57P01) for exactly the four old-issuer backends |
| `T09-CONTAINER-REMOVED` | POSITIVE_CONTROL | PASS | The attempt container was removed and no container with its name remains |
| `T09-SECRET-SCAN` | POSITIVE_CONTROL | PASS | No generated password value appears in any evidence file of this attempt |
