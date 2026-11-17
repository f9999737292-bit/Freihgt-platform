# 0.1U: Prerequisites for a later isolated pilot

Status: `DEFINED_NOT_RUN`. Nothing in this file is deployed. Staging and production are out of scope. The tests below are the 0.1T plan (`FUTURE_ISOLATED_TEST_PLAN.md`, PR #390). They do not run until the infrastructure decision names a provider that passed the seven checks in `INFRASTRUCTURE_DECISION.md`.

## 1. Minimum stand

Three roles, three storage domains, three sets of credentials. A role here is an administrative identity, not a Unix group on one VM.

| Piece | Must be true before the first test | Current inventory |
| --- | --- | --- |
| PostgreSQL | Disposable. Not S2. Not a volume in the BINTRANS snapshot set. | Only disposable local containers have been used (0.1Q–0.1S). |
| Witness journal and high-water store | Separate host and disk from PostgreSQL. | Previously the same machine. `NOT_SUITABLE` to repeat. |
| Anchor | The class selected after provider verification. Not S1, S2, the registry, or an unconfigured bucket. | Absent. |
| Issuer | Reads the anchor itself. Does not hold the anchor admin credential. | Not deployed. |
| Observer | Reads every store. Is not the issuer. Records ticket IDs. | Not deployed. |

`AVAILABLE_ISOLATED_HOSTS=0`. The pilot does not start by ordering a third server into the current Selectel account.

## 2. Administrative identities

Each of these identities exists only for the pilot and is destroyed after it. None of them is `root@161.104.57.152`, the staging registry credential, or the E2E operator if that operator can also restore S2.

| Identity | May | Must not |
| --- | --- | --- |
| `pilot-pg` | Restore and start the disposable database | Read the anchor admin API |
| `pilot-witness` | Append the journal and submit the exact successor | Restore PostgreSQL or the anchor |
| `pilot-anchor` | Operate the anchor | Hold a BINTRANS or `pilot-pg` credential |
| `pilot-observer` | Read | Write |

Credential values are not written into the evidence pack.

## 3. Tests the stand must be able to run

These are gates for the stand, not results. Every row stays `NOT_RUN`.

| ID | What the stand must make possible |
| --- | --- |
| FT-P3B-A | Roll PostgreSQL, the journal, and the high-water store back together. The anchor position does not move. The gate stays closed. No second ticket. |
| FT-P3B-B | Roll the anchor back as well. Record the second ticket as the boundary, not as a pass. |
| FT-PART | Cut the witness off from the anchor, then the issuer off from the anchor. No `RESERVED` and no `ISSUE_OK` during the cut. |
| FT-ACK-A, FT-ACK-B, FT-ACK-C, FT-ACK-PG | Crash or drop the acknowledgement at each protocol point. No issuance while the outcome is unknown. No `waitForTimeout` and no sleep as the proof. |
| FT-SEQ | Reject a gap, a lower sequence, and a repeated sequence with a different digest. |
| Stale restore | Restore one local store to an older point while the anchor stays ahead. |
| Anchor loss | Stop the anchor. Issuance stops. Restart does not create a new empty namespace. |
| Issuer epoch change | A reservation from the previous epoch is rejected. |
| Forced gate close | After every restore in this list the gate is closed until a separate, recorded decision. Restore does not open it. |

`FT-POWER`, `FT-PRIV`, `FT-INDEP`, and the anchor-restore rows in the 0.1T plan stay in that plan. They are not waived.

## 4. Explicit non-actions

- Do not place the anchor on `161.104.53.221` or `161.104.57.152`.
- Do not reuse `cr.selcloud.ru/bintrans-staging`.
- Do not treat the E2E Compose project as the anchor.
- Do not open staging security groups, create a bucket, or order a VM as part of this stage.
- Do not run `run-session.sh --execute`.

Passing a future pilot does not open the implementation gate. That remains a separate decision.
