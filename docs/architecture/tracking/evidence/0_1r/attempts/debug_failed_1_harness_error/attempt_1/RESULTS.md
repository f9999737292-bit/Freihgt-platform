# Attempt 1 results

- PostgreSQL container: `bintrans-proof-0-1r-952bb461` (network `none`, no published ports), removed: True
- Server: PostgreSQL 16.15 (Debian 16.15-1.pgdg13+2) on x86_64-pc-linux-gnu, compiled by gcc (Debian 14.2.0-19) 14.2.0, 64-bit
- Started 2026-11-17T17:34:20.318756+00:00, finished 2026-11-17T17:34:30.203959+00:00
- Harness error: TypeError: wcall() got multiple values for argument 'name'
- Counts: {"FAIL": 1, "PASS": 26}
- Findings: []

`COUNTEREXAMPLE_REPRODUCED` with PASS means the predicted unsafe or unproven behavior was observed.

| ID | Scenario | Kind | Result | Description |
| --- | --- | --- | --- | --- |
| `W1-RESERVE` | W1_STATE_MACHINE | POSITIVE_CONTROL | PASS | First reserve of (ns, k1) is RESERVED |
| `W1-NO-SECOND-RESERVE` | W1_STATE_MACHINE | DENIAL | PASS | Second reserve of the same key is DUPLICATE and appends nothing |
| `W1-NON-HOLDER-RESERVE` | W1_STATE_MACHINE | DENIAL | PASS | A non-holder issuer cannot reserve |
| `W1-OPERATOR-CANNOT-RESERVE` | W1_STATE_MACHINE | DENIAL | PASS | operator cannot reserve |
| `W1-AUDITOR-CANNOT-RESERVE` | W1_STATE_MACHINE | DENIAL | PASS | auditor cannot reserve |
| `W1-BAD-SIGNATURE` | W1_STATE_MACHINE | DENIAL | PASS | A request signed with the wrong key is AUTH_FAILED |
| `W1-TICKET-MISMATCH` | W1_STATE_MACHINE | DENIAL | PASS | Outcome citing a different ticket_id is rejected |
| `W1-NOT-OWNER` | W1_STATE_MACHINE | DENIAL | PASS | Another issuer cannot append the outcome |
| `W1-EVIDENCE-REQUIRED` | W1_STATE_MACHINE | DENIAL | PASS | COMMIT_CONFIRMED without a commit acknowledgement is rejected |
| `W1-COMMIT` | W1_STATE_MACHINE | POSITIVE_CONTROL | PASS | Owner appends COMMIT_CONFIRMED for its reservation |
| `W1-OUTCOME-IDEMPOTENT` | W1_STATE_MACHINE | POSITIVE_CONTROL | PASS | Repeating the same outcome is IDEMPOTENT and appends nothing |
| `W1-NO-DELETE` | W1_STATE_MACHINE | DENIAL | PASS | There is no 'delete' operation (UNSUPPORTED_OP) |
| `W1-NO-UPDATE` | W1_STATE_MACHINE | DENIAL | PASS | There is no 'update' operation (UNSUPPORTED_OP) |
| `W1-NO-TRUNCATE` | W1_STATE_MACHINE | DENIAL | PASS | There is no 'truncate' operation (UNSUPPORTED_OP) |
| `W1-NO-REWRITE` | W1_STATE_MACHINE | DENIAL | PASS | There is no 'rewrite' operation (UNSUPPORTED_OP) |
| `W1-NO-RELEASE` | W1_STATE_MACHINE | DENIAL | PASS | There is no 'release' operation (UNSUPPORTED_OP) |
| `W1-NO-RESERVATION` | W1_STATE_MACHINE | DENIAL | PASS | Outcome without a reservation is rejected |
| `W1-EPOCH-MONOTONIC` | W1_STATE_MACHINE | DENIAL | PASS | Epoch cannot be set to a value not above the current one |
| `W1-EPOCH-ROLE` | W1_STATE_MACHINE | DENIAL | PASS | An issuer cannot set the fencing epoch |
| `W1-CHECKPOINT` | W1_STATE_MACHINE | POSITIVE_CONTROL | PASS | Operator records the accepted sequence |
| `W1-CHECKPOINT-NO-LOWER` | W1_STATE_MACHINE | DENIAL | PASS | The accepted sequence cannot be lowered |
| `W1-CHECKPOINT-HASH` | W1_STATE_MACHINE | DENIAL | PASS | A checkpoint with a wrong head hash is rejected |
| `W1-CHECKPOINT-ROLE` | W1_STATE_MACHINE | DENIAL | PASS | An issuer cannot move the checkpoint |
| `HARNESS-ERROR` | HARNESS | POSITIVE_CONTROL | FAIL | Harness completed without exception |
| `HYGIENE-CONTAINER-REMOVED` | HYGIENE | POSITIVE_CONTROL | PASS | PostgreSQL container removed |
| `HYGIENE-TMP-REMOVED` | HYGIENE | POSITIVE_CONTROL | PASS | Witness temp directory removed |
| `HYGIENE-SECRET-SCAN` | HYGIENE | POSITIVE_CONTROL | PASS | No generated key, pipe authkey, or password in any evidence file |
