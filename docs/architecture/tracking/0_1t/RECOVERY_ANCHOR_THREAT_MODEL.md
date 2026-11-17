# 0.1T: Recovery anchor threat model

Stage: `TRACKING-BINDING-INDEPENDENT-RECOVERY-ANCHOR-DISCOVERY-0.1T` (BINTRANS, Agent C).
Reviewer: Agent J.
Type: read-only discovery and design. No code was written, no test was run, and no infrastructure was created. Every mechanism below is `PROPOSED_NOT_IMPLEMENTED`.

## Inputs

| Source | Location | Used for |
| --- | --- | --- |
| 0.1P trusted issuer contract (handover phases A–E, restore rules) | PR #386, `docs/architecture/tracking/TRACKING_BINDING_0_1P_TRUSTED_ISSUER_CONTRACT.md` | `HANDOVER_PENDING`, history completeness definition, fencing controller role |
| 0.1Q DB identity fencing proof (F1–F6) | PR #387, `docs/architecture/tracking/evidence/0_1q/README.md` | Credential and shared-login findings |
| 0.1R external witness proof | PR #388, `docs/architecture/tracking/evidence/0_1r/FUTURE_WITNESS_REQUIREMENTS.md`, `FAILURE_DOMAIN.md` | R1–R15, local failure-domain limits |
| 0.1S rollback detection | PR #389 at `ec3af1f09aa1df92c057685a0b9dfbf0e73de3e1`, `docs/architecture/tracking/evidence/0_1s/` (`HIGH_WATER_PROTOCOL.md`, `PRIVILEGED_BYPASS.md`, `REMAINING_UNPROVEN.md`, P3A/P3B traces) | High-water protocol and counterexample P3B |

None of these PRs is merged, and none of them was changed by this stage.

## Invariant

> If a previously confirmed ticket is missing from every restored local store, no old `(issuer_namespace_id, request_id)` may receive a new issuance permission until an independent anchor has confirmed that the history is complete.

The corollary is a gate rule: whenever history completeness is unknown, `ISSUER_ISSUANCE_GATE=CLOSED`.

This stage does **not** claim protection against simultaneous compromise or rollback of every store, including the anchor.

## Why 0.1S is not enough (P3B)

In 0.1S, `P3B_BOTH_ROLLED_BACK` restored three stores together:
- the witness journal;
- the high-water store;
- the PostgreSQL ticket database.

All three were restored to the same earlier point. In three of three attempts, k2 then received `ISSUE_OK` with a second, different ticket ID.

Every check in that model compares local stores with each other, and each store agreed with the others. No component held an earlier fact that the rollback could not reach.

## Assets

| Asset | Owner (proposed) | Loss means |
| --- | --- | --- |
| A1 Committed tickets and tombstones | PostgreSQL ticket database | A missing tombstone looks like a free `request_id` |
| A2 Witness journal (reservations and outcomes) | Witness | A missing reservation lets the same key be reserved again |
| A3 High-water mark (local monotonic position of A2) | High-water authority | A journal rollback on one side goes undetected |
| A4 Recovery anchor (independent monotonic position and digest of A2) | Anchor service | Rollback of A1, A2, and A3 together goes undetected |
| A5 Credentials and signing keys | Each principal | Forged records, forged marks, or a lowered anchor |
| A6 Gate / handover state | Fencing controller, recorded in the witness | Issuance reopened without proof |

## Actors and threat sources

| Actor | Legitimate power | Threat if misused or compromised |
| --- | --- | --- |
| Issuer | Reserve a key, append its own outcomes, insert a ticket in its own transaction | Reserve again after a rollback; forge outcomes for keys it does not own |
| Business writer (`shipment-service`) | Consume tickets | Insert tickets directly if it gets privileges (0.1Q F4) |
| Witness service (`witness_svc`) | Append to the journal; advance the high-water mark; submit records to the anchor | Advance the anchor to a forged head (0.1S P4 shows a stolen `witness_svc` credential can move the high-water mark forward, though not back) |
| Fencing controller (`binding_authority_fencing`, 0.1P) | Set `HANDOVER_PENDING`, transition the epoch, open the gate after the phase E observations | Open the gate without proof; with a direct CREATEROLE grant, take over the issuer identity (0.1Q F1) |
| Recovery operator | Restore backups; record comparisons | Restore every store to one consistent earlier point (P3B); try to lower the anchor |
| Anchor administrator | Operate the anchor service | Roll back or rewrite the anchor (Scenario B) |
| Shared privileged login (`freight` / superuser) | Not legitimate for this path | Bypass every PostgreSQL control (0.1Q F3, `SHARED_LOGIN_BYPASS=REPRODUCED_IN_TEST_MODEL`) |
| Infrastructure fault | — | Host, disk, or power loss; snapshot restore of a volume group; operator error |

## Threats

| ID | Threat | Proposed control | Status |
| --- | --- | --- | --- |
| TH1 | Tail loss of the journal only | High-water mark (0.1S) and the anchor | Local model only (0.1S) |
| TH2 | Journal and high-water rolled back together | Anchor position is above the restored journal, so the rollback is detected | `PROPOSED_NOT_IMPLEMENTED` |
| TH3 | Journal, high-water, and PostgreSQL rolled back together (P3B) | Anchor position is above the restored journal, so the gate stays CLOSED and no new ticket is issued | `PROPOSED_NOT_IMPLEMENTED`; Scenario A in `P3B_RECOVERY_DECISION.md` |
| TH4 | All four, including the anchor, rolled back consistently | Not preventable by the anchor alone; needs an external trusted history or a non-rollbackable mechanism | **OPEN**; Scenario B |
| TH5 | Anchor confirms a checkpoint but not the history the issuer uses | The anchor certificate binds namespace, incarnation, sequence, and head digest; the issuer verifies its exported history chains to that digest | `PROPOSED_NOT_IMPLEMENTED` |
| TH6 | Stale or replayed anchor reply | A fresh, challenge-bound anchor read on every gate decision; certificate includes the challenge | `PROPOSED_NOT_IMPLEMENTED` |
| TH7 | Acknowledgement before the anchor confirms | Strict order `APPEND → HW → ANCHOR → ACK`; no reply carries `RESERVED` or `ISSUE_OK` before the anchor certificate covers the record | `PROPOSED_NOT_IMPLEMENTED` |
| TH8 | Forged history accepted by the anchor (stolen writer credential) | Anchor keeps every accepted `(seq, digest)` pin and the predecessor link; it accepts only the exact successor; a forged tail conflicts with earlier pins | Partially shown for high-water in 0.1S P4; anchor `PROPOSED_NOT_IMPLEMENTED` |
| TH9 | Lowering or resetting the anchor by one person | No lower, reset, or delete operation exists; a namespace retirement is a new append that needs two independent approvals and never lowers the position | `PROPOSED_NOT_IMPLEMENTED` |
| TH10 | Shared credentials or keys across stores | Credential isolation (section below) | `NOT_PROVEN` |
| TH11 | Direct PostgreSQL writes bypassing the issuer | Out of the anchor's power; detected afterwards by the gate (0.1R `SS-BYPASS-DETECTED-NOT-PREVENTED`) | `DIRECT_PG_BYPASS=OPEN` |
| TH12 | Old primary commits after handover | Out of the anchor's power (0.1R R9) | `OLD_PRIMARY_FENCING=NOT_PROVEN` |
| TH13 | Automatic "repair" that recreates missing records or tickets | Forbidden; missing anchored records are recovered only from a copy that hash-chains to the anchored digest, otherwise the namespace stays CLOSED | `PROPOSED_NOT_IMPLEMENTED` |

## Separation of powers (section 7)

| Principal | May | Must not |
| --- | --- | --- |
| Issuer | Read the anchor; reserve through the witness; append its own outcomes | Write to the anchor; change the high-water mark; open the gate; set epochs |
| Witness service | Submit the exact successor `(seq, digest, prev)` to the high-water authority and the anchor | Lower, reset, rewrite, or delete any accepted anchor position; open the gate; insert tickets |
| Business writer | Consume tickets | Write to the witness, the high-water mark, or the anchor; issue tickets |
| Fencing controller | Set `HANDOVER_PENDING`; transition epochs; open the gate after the phase E observations **and** a fresh anchor completeness check | Lower or reset the anchor; hold CREATEROLE or ADMIN on issuer roles (0.1Q F1); hold an unscoped `pg_signal_backend` (0.1Q F2) |
| Recovery operator | Restore backups; submit a recovered journal segment for verification; record comparisons | Lower the anchor alone; append reservations; open the gate |
| Anchor administrator | Operate, monitor, and back up the anchor | Hold any BINTRANS application, PostgreSQL, witness, or high-water credential; restore the anchor to an older state alone |
| Auditor | Read every store and the anchor's full history | Write anything |

### Credential isolation requirements

- **CI1.** Each principal has its own credential, and no credential is valid for more than one of these stores: PostgreSQL, witness, high-water authority, anchor. A single shared login (for example `freight`) for any two of them breaks this requirement.
- **CI2.** Anchor writes are authenticated with an asymmetric key held only by the witness service. Anchor certificates are signed with a key held only by the anchor. The 0.1S model used symmetric HMAC and held every key in one process; that does not meet this requirement.
- **CI3.** The anchor's administrative domain (accounts, backup system, storage, host) is disjoint from the BINTRANS platform's administrative domain. No BINTRANS operator account can restore or modify the anchor's storage.
- **CI4.** Any operation that would invalidate an anchor position, such as retiring a namespace after an unrecoverable loss, needs two approvals from different administrative domains. It is recorded as a new appended entry, never as a lower position.
- **CI5.** Compromise of any single service credential must not give control over all four states. The test plan checks this per credential (`FUTURE_ISOLATED_TEST_PLAN.md`, FT-PRIV).

### Audit trail requirements

- Every anchor append records:
  - the authenticated principal;
  - the signed request;
  - `namespace_id`, `incarnation`, `seq`, `head_digest`, `prev_digest`;
  - the anchor's own log position.
- Every gate decision (open or closed) records:
  - the anchor certificate it relied on;
  - the challenge;
  - the journal head it compared.
- The audit trail lives in the anchor's domain. A copy in BINTRANS logs is a convenience, not evidence.

### Privileged bypass through a shared login (kept open)

0.1Q F3 showed that a shared superuser login defeats every PostgreSQL control: it can forge tickets, reopen the gate, and assume the issuer identity. The anchor does not change that for PostgreSQL. It can make a forged or reissued ticket **detectable** afterwards, because the ticket's reservation is missing from an anchored history. It cannot prevent the write.

`PRIVILEGED_BYPASS=OPEN` and `SHARED_FREIGHT_LOGIN=OPEN` remain.
