# 0.1U: Failure-domain verification

Status: documentary check only. No fault was injected. No host was restored. PR #390 `FAILURE_DOMAIN_MATRIX.md` already marks every real-infrastructure row `NOT_PROVEN`. This file checks whether the inventory can move any of those rows.

Reference heads, unchanged:

| PR | Head | Limit that still stands |
| --- | --- | --- |
| #387 | `4c296a57a2a8366019aac693ae2f883c9d8d624d` | Fencing is not proven. Shared superuser and CREATEROLE takeover were reproduced in a disposable model. |
| #388 | `12560d702bec29c1ed6b764c64e955ac52f937f7` | A local witness is the same failure domain as PostgreSQL. Tail loss after the last checkpoint was not detected. |
| #389 | `ec3af1f09aa1df92c057685a0b9dfbf0e73de3e1` | High-water independence is logical only. Rolling back the journal and the high-water store together is undetected (P3A). Rolling back those two plus PostgreSQL issues a second ticket (P3B). |
| #390 | `cd22a3789725cf49a32a8f5bfc3a58980a9ce7dd` | Scenario A is a design if an anchor survives. Scenario B, rollback of the anchor as well, is not fixed by the anchor class. |

## 1. Claims that stay closed

| Claim | Value | Why this inventory does not change it |
| --- | --- | --- |
| `P3B_ALL_STORES_ROLLBACK` | `NOT_FIXED` | No store outside the operator domain was found. |
| `ANCHOR_FAILURE_DOMAIN` | `NOT_PROVEN` | No anchor exists. |
| `OLD_PRIMARY_FENCING` | `NOT_PROVEN` | An anchor does not fence an old primary. #390 leaves this to a separate test. |
| `J_DML_001` | `NOT_CLOSED` | Direct PostgreSQL writes stay detectable at most, not prevented. |
| `ISSUER_ISSUANCE_GATE` | `CLOSED` | Unchanged. |
| `IMPLEMENTATION_GATE` | `CLOSED` | Unchanged. |
| `DECISION_CONSUMPTION_GATE` | `CLOSED` | Unchanged. |
| `PRODUCTION_CONSUMER_READINESS` | `BLOCKED` | Unchanged. |

## 2. Independence tests applied to the inventory

PR #390 requires all of the following before an anchor may be called independent. Each line is the result of reading the inventory, not of a live test.

| # | Required fact | Result |
| --- | --- | --- |
| 1 | No BINTRANS host, disk, storage account, backup system, root account, or operator account can modify or restore the anchor. | `NOT_VERIFIED` for any future service, and `FAIL` for S1, S2, R1, and O1 as they are documented: they sit in the BINTRANS Selectel operating set or are not built. |
| 2 | The anchor signing key is not available to any BINTRANS service or operator. | `NOT_VERIFIED`. No such key exists. Staging SSH and registry credentials are BINTRANS-operated. |
| 3 | Restoring every BINTRANS store leaves the anchor unchanged. | `NOT_VERIFIED`. There is no anchor to leave unchanged. A single documented snapshot path (`/protected/bintrans/deploy-snapshots/...`) does not prove the reverse either. |
| 4 | Loss of the anchor closes issuance and does not re-initialize it. | Design rule only (`PROPOSED_NOT_IMPLEMENTED`). No service implements it. |

## 3. What must not be inferred

- Two IP addresses are not two failure domains.
- A Compose project name is not a failure domain. `bintrans-e2e-stand-v01a` can be deleted without touching `bintrans-ct-staging` on the same Docker engine, and both still die together if the engine's disk is restored.
- `--network none` between containers on one machine is not an independent domain. That is the 0.1R/0.1S result.
- Selectel as a brand on both the VM notes and `cr.selcloud.ru` is evidence of a shared provider family. It is not, by itself, proof of one account. It is also not proof of separation. Account boundary: `NOT_VERIFIED`.
- The E2E runbook's rule "do not copy the staging key onto the E2E host" is a procedure. It is not evidence that the procedure is in force, and it does not put the host outside the operator's restore rights.

## 4. Scenario mapping

| Scenario | Existing documented resources | What would count as evidence later |
| --- | --- | --- |
| A. PostgreSQL, journal, and high-water return to T0; the anchor stays at T1. | No resource can play the anchor. S1 next to S2 is not that resource. | A restore of the BINTRANS side that leaves an externally administered log at the higher sequence, checked by digest. |
| B. The anchor returns to T0 as well. | Any backup or admin right that covers the candidate anchor together with S2 makes B ordinary, not exceptional. | A written control that BINTRANS operators cannot restore the anchor, plus a test where their restore action does not move it. |

Until those tests exist, `ANCHOR_FAILURE_DOMAIN=NOT_PROVEN`.
