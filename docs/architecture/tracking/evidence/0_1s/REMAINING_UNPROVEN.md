# 0.1S: Remaining unproven properties

These hold even though every recorded assertion passed:

```text
PRODUCTION_WITNESS=NOT_SELECTED
WITNESS_FAILURE_DOMAIN=NOT_PROVEN
HIGH_WATER_INDEPENDENCE=LOGICAL_ONLY
POWER_LOSS_DURABILITY=NOT_PROVEN
GLOBAL_OLD_PRIMARY_FENCING=NOT_PROVEN
DIRECT_PG_BYPASS=OPEN
PRODUCTION_FREIGHT_PRIVILEGES=NOT_VERIFIED
J_DML_001=NOT_CLOSED
TRUSTED_TENANT_AUTHORITY=NOT_PROVEN
PRESENTER_IDENTITY_BOUND=NOT_IMPLEMENTED
ALL_WRITERS_FENCED=NO
ISSUER_ISSUANCE_GATE=CLOSED
IMPLEMENTATION_GATE=CLOSED
DECISION_CONSUMPTION_GATE=CLOSED
PRODUCTION_CONSUMER_READINESS=BLOCKED
```

| Property | Status | Why |
| --- | --- | --- |
| Physical independence of the high-water authority from the journal and from PostgreSQL | NOT_PROVEN | Same host, disk, OS, operator, and harness (see 0.1R `FAILURE_DOMAIN.md`, PR #388) |
| Detection of a consistent rollback of both stores | NOT_PROVIDED | `P3A` / `P3B` counterexamples |
| Rollback resistance of the authority store itself | NOT_PROVEN (assumed by the model) | It is a local file; P2 shows that rolling back the authority alone fails closed, but nothing anchors the authority |
| Power-loss / OS-crash durability | NOT_PROVEN | Only process crashes were injected; no directory fsync on Windows |
| Production-grade append-only storage | NOT_PROVEN | File lock and hash-chained JSONL are test-only |
| Credential isolation and non-repudiation | NOT_PROVEN | Symmetric HMAC; the witness process holds every verification key; the harness holds every key |
| Liveness of the `hw_first` order | NOT_PROVIDED | CW4 fails closed permanently by design |
| Recovery procedure after `FAILED` | NOT_DEFINED | Only demonstrated by putting back a complete journal copy; authorization and tooling are out of scope |
| Old-primary COMMIT fencing | NOT_PROVEN | Unchanged from 0.1R `SS-OLD-COMMIT-NOT-FENCED` |
| Direct PostgreSQL writes | OPEN | Detected by the issuer gate afterwards, not prevented |
| Network partitions, timeouts, latency, throughput | NOT_TESTED | Local named pipes only |
| Nonce replay across restarts | NOT_TESTED | Nonces are rebuilt from the journal on load (witness) and from the authority log (authority); no cross-restart replay assertion |
| Per-namespace sharded journals | NOT_TESTED | One journal per witness; marks are per journal, not per namespace |

## Status of 0.1R future-witness requirements R1–R4

The source is the 0.1R `FUTURE_WITNESS_REQUIREMENTS.md` (PR #388, `docs/architecture/tracking/evidence/0_1r/`).

| Req | Requirement | 0.1S status |
| --- | --- | --- |
| R1 | Independence from PostgreSQL restore | Logical only. `STALE_PG_COMBINED` shows the witness and the mark are unaffected by a PostgreSQL restore and still prevent the second ticket. All stores share one host, so a host-level restore is untested. `P3B` shows a second ticket when the restore also covers both witness stores |
| R2 | Independent failure domain | NOT_PROVEN. The authority is a separate process, store, lock, and key set on the same machine |
| R3 | Durability across process crash, host crash, and power loss | Process crash only (CW1–CW8). Host crash and power loss are NOT_PROVEN |
| R4 | Rollback / tail-loss detection, including after the last checkpoint | Met in the local model for rollback or rewriting of the **journal alone** (T2–T6, J-WIT-001, P1, P4) and fails closed for the **authority alone** (T8, P2). **Not met** when both are rolled back together (P3A) |
