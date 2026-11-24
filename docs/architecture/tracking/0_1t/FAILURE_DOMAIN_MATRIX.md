# 0.1T: Failure-domain matrix

Status: read-only analysis. Nothing here was measured on real infrastructure. In the evidence so far, every "independent" component ran on one Windows host:
- 0.1R `FAILURE_DOMAIN.md` (PR #388);
- 0.1S `HIGH_WATER_INDEPENDENCE=LOGICAL_ONLY` (PR #389).

## 1. Inventory

| Object | Backup / restore | Rollback | Corruption | Durability | History / timeline | Privileges (who can change it) | Current evidence | Requirement for independence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| PostgreSQL ticket DB | Dump/restore, PITR, volume snapshot. 0.1R and 0.1S restored a dump with `pg_restore` | Any restore to an older point removes tickets and tombstones; 0.1S `STALE_PG_COMBINED` and P3B | Not tested | Not tested beyond container lifetime | A promoted replica or PITR creates a new timeline; an old primary may still accept writes (0.1P, 0.1Q `T06-E1-LOCK-NOT-FENCE`) | Issuer login, business writer, migrations owner, **shared `freight` login**, superuser (0.1Q F3) | Test containers only; `PRODUCTION_FREIGHT_PRIVILEGES=NOT_VERIFIED` | Must not be backed up, snapshotted, or restored together with the witness or the anchor (0.1R R1) |
| Witness journal | File copy in the local model | Detected against the high-water mark (0.1S T2–T6, J-WIT-001) unless both are rolled back (P3A) | Detected on load (hash chain, MAC) | fsync per append; no directory fsync; no power-loss test | Hash chain from GENESIS with a random incarnation | Witness process; anyone with file access; in 0.1S the harness held every key | Local model only | Separate storage and administration from PostgreSQL (R2); asymmetric keys per principal (R12) |
| High-water store | File copy in the local model | Rolling back the HW store alone fails closed (0.1S P2A/P2B); rolling back both stores together is undetected (P3A) | Detected on load (`HW_HASH_MISMATCH`, `HW_REQUEST_MAC_INVALID`, `HW_RECORD_REQUEST_MISMATCH`) | fsync per append | Append-only history of `(seq, digest)` pins | Only `witness_svc` writes; same host and administrator as the journal | Local model only | Not required to be independent if a real anchor exists; it provides fast local detection |
| Recovery anchor | Not built | Must not be restorable to an older position by any single administrative domain | Must be detectable by its own certificate and history verification | Must survive power loss before acknowledging | Contract v1: one record per append (`GROUP_COMMIT=DISABLED`, `MAX_SEQUENCE_INCREMENT=1`); each append names its immediate predecessor; a pin for every acknowledged record (`REQUIRED_PINS=EVERY_ACKNOWLEDGED_RECORD`) | Witness service (append); anchor administrators (operate only); no BINTRANS credential | None (`ANCHOR_FAILURE_DOMAIN=NOT_PROVEN`) | Disjoint host, storage, backup system, administrative accounts, and signing keys from all three stores above |
| Credentials | Per-attempt random keys in tests | A restored credential store could re-enable a retired credential | Not tested | — | Retired key IDs must stay recorded (0.1P) | Whoever administers the secret store | 0.1S: symmetric HMAC, all keys in one harness process | One credential per principal per store; none valid in two stores; anchor keys outside BINTRANS administration |
| Controller (who reopens issuance) | — | A restored gate or handover state could reopen the gate | — | — | Gate decisions logged in the witness | `binding_authority_fencing` (0.1P), not yet existing | 0.1Q: direct CREATEROLE controller takes over the issuer (F1); shared superuser reopens the gate (F3) | Opens only after the phase E observations **and** a fresh anchor completeness check; no CREATEROLE or ADMIN on issuer roles; cannot write to the anchor |

## 2. Common-cause failures

Each row asks whether the cause can affect several states at once, what that does, and what the design needs to stay safe.

| Common cause | States affected together | Effect without an independent anchor | Effect with the proposed anchor | Required separation | Status |
| --- | --- | --- | --- | --- | --- |
| One host | All of them in the local models | Host loss stops everything (safe, unavailable). A host-level snapshot restore rolls back every store consistently (P3B) | Detected if the anchor is on another host (Scenario A) | Anchor on a host BINTRANS operators cannot snapshot or restore | `NOT_PROVEN` |
| One physical disk or one storage system | Same as one host | Same; plus correlated corruption | Same as above | Separate storage system and storage administration | `NOT_PROVEN` |
| One administrative account | Any store that account can reach | One person can restore, edit, or delete every store (P3B; 0.1Q F3) | That account must not reach the anchor; otherwise Scenario B | Disjoint admin accounts; two-domain approval for any anchor invalidation (CI4) | `NOT_PROVEN` |
| One backup and restore system | Every store it backs up | Restoring "everything to last night" is P3B | Anchor excluded from that system; its own backup cannot restore it to a lower position alone | Separate backup system; anchor restore only to a position at or above the last certified one, verified against an external copy of the certificate | `NOT_PROVEN` |
| Shared root access | Every store on hosts with that root | Same as one admin account | Same | No shared root between BINTRANS hosts and anchor hosts | `NOT_PROVEN` |
| Shared storage system (SAN, cloud volume account) | Every volume in it | Volume-group snapshot restore is P3B | Same | Separate storage account / tenancy for the anchor | `NOT_PROVEN` |
| Single signing key | Every record signed with it | Holder forges any principal (0.1S, all keys in one process) | Holder can forge journal records but not anchor certificates if the anchor key is separate; cannot lower anchor positions | Asymmetric keys per principal; anchor key in the anchor domain only | `NOT_PROVEN` |
| Mistaken restore operation | Whatever the operator restores | P3B by accident | Detected (Scenario A) if the anchor was not part of the restore | The anchor is not in any BINTRANS restore runbook | `NOT_PROVEN` |
| Compromise of the control plane | Gate, handover, epoch, possibly credentials | Gate reopened without proof (0.1Q F1, F3) | Controller can still open the gate wrongly unless the issuer independently checks the anchor before every issuance (as 0.1S issuers checked the high-water mark) | Issuer-side anchor check independent of the controller; controller has no anchor write right | `NOT_PROVEN` |
| Clock | Lease expiry and timeouts | A lease timeout is not a fence (0.1P) | Unchanged; the anchor protocol does not depend on time | No time-based release of reservations | Design rule (0.1P) |
| Network partition between witness and anchor | Availability only | — | Gate CLOSED (`ANCHOR_UNAVAILABLE`); no acknowledgement | None; fail closed by design | `NOT_TESTED` |

## 3. Minimum independence claim a future stage would have to prove

An anchor is "independent" for this design only when a written analysis **and** fault tests show all of the following:
1. No host, disk, storage account, backup system, root account, or BINTRANS administrative account can modify or restore it.
2. Its signing key is not available to any BINTRANS service or operator.
3. A restore of every BINTRANS store, including volume-group and host-level restores, leaves the anchor unchanged.
4. A lost anchor fails closed: issuance stops, and nothing re-initializes.

Until then `ANCHOR_FAILURE_DOMAIN=NOT_PROVEN` and `WITNESS_FAILURE_DOMAIN=NOT_PROVEN`.
