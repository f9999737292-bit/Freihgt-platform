# 0.1T: P3B recovery decision

Status: design analysis only. No scenario in this file was executed.

## Counterexample under analysis

PR #389 (`ec3af1f09aa1df92c057685a0b9dfbf0e73de3e1`), `docs/architecture/tracking/evidence/0_1s/attempts/attempt_{1,2,3}/traces/P3_BOTH_ROLLED_BACK.json` and assertion `P3B-SECOND-TICKET`:
1. Issue k1. Snapshot the journal, the high-water store, and PostgreSQL.
2. Issue k2.
3. Restore all three to the snapshot.
4. Issue k2 again: `ISSUE_OK` with a different ticket ID, in three of three attempts.

The companion `P3A` (PostgreSQL not restored) showed the witness accepted a second reservation of k2. Only the PostgreSQL row made the gate refuse.

## Scenario A: the anchor survives

PostgreSQL, the witness journal, and the high-water store return to point T0. The independent anchor keeps its position at T1 > T0, which covers k2's anchored reservation and outcome.

| Step | Proposed behaviour | Rule |
| --- | --- | --- |
| Witness start | journal.len(T0) < anchor.seq(T1), so `ROLLBACK_DETECTED source=anchor`, FAILED | `RECOVERY_ANCHOR_PROTOCOL.md` §3 rule 3 |
| High-water comparison | Consistent with the journal (both T0); it does not matter, because the anchor rule fails first | §3 |
| Gate | `HANDOVER_PENDING`, `ISSUER_ISSUANCE_GATE=CLOSED` for **every** `request_id` in the namespace, not only k2, because the anchor stores digests and the missing segment's keys are unknown | §4 step 1 |
| Issuer request for k2 | Denied before PostgreSQL. The issuer's own anchor read fails the completeness check (exported length below the certificate's `seq`) even if the witness were to report OK | §1 "history in use" |
| New ticket | None | Invariant |
| Automatic recovery | **None.** Missing records are not regenerated from PostgreSQL, from logs, or from the issuer's memory | §4 |
| Recovery with a verifiable copy | A copy of the missing segment that chains to the anchored head digest is re-appended byte for byte. k2's reservation and outcome reappear. PostgreSQL lacks k2's ticket, so the key is reconciled to a denial tombstone or `PERMANENTLY_DENIED`, never reissued. The gate opens only through 0.1P phase E | §4 steps 2–6 |
| Recovery without a copy | The namespace stays CLOSED. History completeness cannot be established; `BLOCKED_PROTOCOL_DESIGN` for that path | §4 |

`P3B_WITH_ANCHOR=DETECTED_GATE_CLOSED_BY_DESIGN_NOT_TESTED`.

Precondition: k2's reservation must have been anchored **before** the issuer started its PostgreSQL transaction (T3 before T4). If the anchor lagged, for example by asynchronous or periodic anchoring, k2's records could fall in the unanchored gap and P3B would reappear. Periodic anchoring therefore does not satisfy this design.

## Scenario B: the anchor rolls back too

All four states return to one consistent earlier point T0. This could happen through:
- a common administrator;
- a common backup system;
- a compromise of the anchor domain;
- a mistaken restore that includes the anchor.

| Question | Answer |
| --- | --- |
| Does the local completeness check detect it? | **No.** Journal, high-water, PostgreSQL, and anchor agree at T0. Every rule in §3 passes |
| Is a second ticket possible? | **Yes.** Reissuing k2 behaves exactly like P3B, unless some other surviving fact exposes it |
| What could detect it? | Only a fact the rollback did not reach. None of these is part of the proposed design or proven: (a) an external trusted history outside every rolled-back domain (for example a second anchor in another administrative domain, with a quorum across them); (b) a non-rollbackable mechanism such as a hardware monotonic counter whose properties are proven separately; (c) receipts held by third parties (the gateway, or clients holding the anchor certificate for k2) that are checked at issuance. A receipt only helps when someone presents it |
| Is it fixed? | **No.** `P3B_ALL_STORES_ROLLBACK=NOT_DETECTED_NOT_FIXED` |

A quorum of anchors raises the cost of Scenario B, since the attacker or mistake must reach a majority of independent domains. It does not remove the scenario. The claim stays bounded: safety holds only while at least one anchor domain that holds the latest certified position is neither rolled back nor compromised. No such domain exists today.

## Infrastructure variant classes

No product or provider is selected. Cost and operational complexity are **not stated**, because no verified data exists for them in this repository.

| | Class 1: independent journal with proven durability and rollback protection | Class 2: separate consensus store with monotonic conditional operations | Class 3: external append-only audit / anchor service with independent administration |
| --- | --- | --- | --- |
| What it holds | The full journal (records), so it is both witness and anchor | Records or at least `(namespace, seq, digest, request key)` per entry, written with compare-and-set on the current position | `(namespace, incarnation, seq, head_digest)` pins and signed certificates; optionally request keys |
| Security preconditions | Storage that cannot be rolled back by BINTRANS operators; durable before acknowledgement; append-only for every principal (R5) | Linearizable conditional writes; durable commit on a quorum before acknowledgement; membership changes that do not lose acknowledged writes; administrators separate from BINTRANS | Operated by a different administrative domain; append-only and non-rewritable for its own administrators, or tamper-evident through public or third-party verifiable logs; certificate key outside BINTRANS |
| Shared failure domain with BINTRANS | Low only if hosted and administered separately; otherwise it is the same as today | Low only if the quorum members are outside the BINTRANS host, storage, and admin domains; a cluster on BINTRANS hosts is the same domain | Lowest by construction, if administration is truly separate |
| Rollback possibility | By anyone who can restore its storage | By anyone who can restore a majority of members or rewrite membership | By the service operator, unless its log is externally verifiable |
| Behaviour on failure | Unavailable means the gate is CLOSED | Quorum lost means the gate is CLOSED; minority partition cannot acknowledge | Unreachable means the gate is CLOSED |
| Quorum | Not applicable for a single instance; replication needs its own analysis | Required; majority of `2f+1` members tolerates `f` failures; members must sit in separate failure domains | Not inherent; can be combined into a quorum of several services |
| Recovery without full history (§4) | Possible: it holds the records | Possible if it stores records or request keys | Not possible with digest-only pins; possible if request keys are stored |
| Remaining unproven | Durability, rollback resistance, admin separation | Linearizability under faults, durable membership changes, admin separation, latency per reservation | Independence of the operator, non-rewritability, certificate key custody, availability |

## Feasibility gate

| Option | Decision |
| --- | --- |
| `FEASIBLE_FOR_ISOLATED_IMPLEMENTATION` | Not chosen. A local isolated model could repeat 0.1S with a third store, but on one host it would prove only logical behaviour again, which 0.1S already shows for two stores. It cannot address P3B, because P3B is a failure-domain problem |
| `REQUIRES_INFRASTRUCTURE_DECISION` | **Chosen.** The acknowledgement boundary and the detection and gate rules are designable without atomicity claims (`RECOVERY_ANCHOR_PROTOCOL.md` §2–3). Closing Scenario A needs an anchor in a separate, proven failure domain, and one of the variant classes must be chosen, owned, and verified. That decision is outside this stage and outside Agent C's authority |
| `BLOCKED_PROTOCOL_DESIGN` | Applies **only** to recovery without a complete, verifiable history, when the anchor holds digests only. That path has no safe way to reopen old `request_id` values; the namespace stays CLOSED. Scenario B is also unresolved by design. Neither blocks the acknowledgement boundary |

`FEASIBILITY_GATE=REQUIRES_INFRASTRUCTURE_DECISION`.

`ISSUER_ISSUANCE_GATE=CLOSED`, `IMPLEMENTATION_GATE=CLOSED`, `DECISION_CONSUMPTION_GATE=CLOSED`, `PRODUCTION_CONSUMER_READINESS=BLOCKED`.

## Open risks carried from 0.1Q–0.1S

| Risk | Origin | Status after 0.1T |
| --- | --- | --- |
| Shared superuser / `freight` login bypasses every PostgreSQL control | 0.1Q F3 | OPEN. The anchor allows detection afterwards, not prevention |
| Direct CREATEROLE controller takes over the issuer | 0.1Q F1 | Design rule: controller has no CREATEROLE or ADMIN on issuer roles; NOT_PROVEN in product |
| `pg_signal_backend` not role-scoped | 0.1Q F2 | Unchanged |
| In-flight COMMIT before quiesce; epoch row lock is not a fence | 0.1Q `T04-N4-INFLIGHT-COMMIT`, `T06-E1-LOCK-NOT-FENCE` | `OLD_PRIMARY_FENCING=NOT_PROVEN` |
| Witness tail loss after the last checkpoint | 0.1R F-TAIL-LOSS / J-WIT-001 | Closed only in the 0.1S local model (`J_WIT_001=LOCAL_MODEL_ONLY`) |
| Direct PostgreSQL writes | 0.1R `SS-BYPASS-DETECTED-NOT-PREVENTED` | `DIRECT_PG_BYPASS=OPEN` |
| Both witness stores rolled back together | 0.1S P3A | Addressed by Scenario A, by design only |
| All local stores rolled back together | 0.1S P3B | Scenario A by design only; Scenario B OPEN |
| `hw_first` order not live | 0.1S CW4 | Not used; the anchor follows the append-first order |
| Symmetric keys; all keys in one process | 0.1R/0.1S | Requirement CI2; NOT_PROVEN |
| Power-loss durability | 0.1R/0.1S | NOT_PROVEN |
