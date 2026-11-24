# EDO signing recovery 0.1 — test plan

No test in this table was executed for this stage. Status is `NOT_RUN` or `NOT_IMPLEMENTED`. A passing name in the tree is not a result of this pass.

Local stand-ins are in-process fakes and the existing embedded-Postgres style used by `services/document-service/internal/integration/edo03schema`. They must not call an operator, use a real private key, or open a staging network.

| Test group | Required | Status | What the test has to show |
| --- | --- | --- | --- |
| Crash recovery | YES | `NOT_IMPLEMENTED` for a crash injection. Domain recovery tests exist and were `NOT_RUN` | F01 and F02 leave either the previous commit or one committed signature, never both a provider success and an empty local row marked failed |
| Duplicate requests | YES | `NOT_RUN` | The same operation id returns one signature |
| Concurrent signing | YES | `NOT_RUN`. Repository locks exist | Two creates produce one active session. Two signatures on the last slot produce one `COMPLETED` session |
| Tenant isolation | YES | `NOT_RUN` | A foreign tenant receives not-found and `STORE_WRITES=0` |
| Authorization revocation | YES | `NOT_IMPLEMENTED` | A signer removed after preparation cannot open a new session. An in-flight `RESULT_UNKNOWN` is not retried as that signer |
| Document integrity | YES | `NOT_IMPLEMENTED` for a content hash | A version or hash mismatch does not sign |
| Unknown external result | YES | `NOT_IMPLEMENTED` | Timeout stores `RESULT_UNKNOWN`, not `FAILED`, and does not call the mock provider a second time |
| Retry safety | YES | `NOT_RUN` for HTTP replay | Replay writes nothing. A new attempt after `EXPIRED` uses a new session id |
| Audit completeness | YES | `NOT_IMPLEMENTED` | Each transition above inserts one event with the fields in the security note and no key material |
| Database transaction failure | YES | `NOT_RUN` | Rollback leaves the prior status. A lost commit acknowledgement becomes `RESULT_UNKNOWN` only for an external call |
| Restart recovery | YES | `NOT_IMPLEMENTED` | Startup does not sign. It only reads |
| Provider callback disorder | YES | `NOT_IMPLEMENTED` | A late failure for another operation id does not change this document |
| Signature validation | YES | `NOT_RUN` for the current `VALID` write. `NOT_IMPLEMENTED` for a real verifier | A failed check does not set `VALID` and does not set document `SIGNED` |

## Audit events the tests must see

Each event records actor, tenant, company, operation id, document id, document version, timestamp, correlation id, previous state, next state, evidence source, and reason code. The actor is the verified user or the service name of the reconciler. It is not a raw client header.

| Event | When |
| --- | --- |
| `SIGNING_OPERATION_CREATED` | Session or operation row commits |
| `SIGNING_PREPARED` | Hash and version are stored |
| `SIGNING_STARTED` | A signature attempt or provider call starts |
| `SIGNING_RESULT_UNKNOWN` | The external outcome is not proven |
| `SIGNING_RESULT_RECONCILED` | Provider evidence selects a proven state |
| `SIGNING_CONFIRMED` | Operator confirmation commits |
| `SIGNING_RETRY_REQUESTED` | A caller asks to retry |
| `SIGNING_RETRY_REJECTED` | The retry is refused |
| `SIGNING_MANUAL_REVIEW_REQUIRED` | Evidence is insufficient or contradictory |
| `SIGNING_RECOVERY_COMPLETED` | The document returns to `READY_FOR_SIGNING` or the review closes |

`SIGNING_RECOVERY_COMPLETED` for the current in-process function, if it is ever exposed, must say the session stayed `EXPIRED` and no signature was inserted.

## Commands not run

`go test` for `services/document-service` and `services/api-gateway` was `NOT_RUN`. Reason: this stage is documentation only, and the assignment forbids treating an unread test as a pass. The commands that a later stage would run are the existing package tests plus new tests named by the groups above. They are not a result.
