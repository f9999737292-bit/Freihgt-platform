# Selectel anchor decision 0.1Y

Date: 2026-11-18. This is a reading of published pages. It does not start a pilot and it does not buy a service.

Baseline used for the reading: `origin/main` `3f306fc407aeda4bdc652d825954292f4a54fad6`, Draft PR #394 at `e7f69bc7e8f982074d9f9a54daeba1dda6cc331d`, architecture PR #390 at `cd22a3789725cf49a32a8f5bfc3a58980a9ce7dd`, provider research PR #392 at `80747ebe117fa1a9f6d9430381ebd9c7d37c88df`. Those pull requests are unchanged.

## Classification

| Option | Class | Why |
| --- | --- | --- |
| S3 Vault as backup | `SUITABLE_FOR_BACKUP_ONLY` | Scheduled copy, a service account, an integrity check of the copied files, and a documented restore read after an official request. |
| S3 Vault as an online anchor | `NOT_SUITABLE_FOR_ANCHOR` | Synchronous per-record acknowledgement is not documented. A backup copy is not the monotonic log. |
| S3 Object Lock as an anchor component | `NOT_SUITABLE_FOR_ANCHOR` | Compliance retains one object version. It does not provide exact succession, the anchor tuple, or a receipt. Governance and legal hold remain movable by the customer. |
| Managed PostgreSQL as an anchor component | `NOT_SUITABLE_FOR_ANCHOR` | ACID and the absence of superuser do not stop a 7-day point-in-time restore by the panel owner. |
| A composite of these Selectel services | `NOT_SUITABLE_FOR_ANCHOR` | None of the three puts a synchronous exact-successor acknowledgement in front of PostgreSQL `BEGIN`, and all three remain under the Selectel control plane. |
| A second independent provider | Still required, not selected | A separate login at the same provider does not meet the 0.1T split of administrative domains. |

`RECOMMENDED_CLASS=NOT_SUITABLE_FOR_ANCHOR`

`S3_VAULT_SYNC_ANCHOR=NOT_VERIFIED`

## Gates

```text
MANAGED_ANCHOR=NOT_SELECTED
PILOT_RECOMMENDATION=DO_NOT_START
FOURTH_SERVER_PURCHASE=NOT_AUTHORIZED
FOURTH_SERVER_REQUIRED=NOT_DECIDED
P3B_ALL_STORES_ROLLBACK=NOT_FIXED
ANCHOR_FAILURE_DOMAIN=NOT_PROVEN
OLD_PRIMARY_FENCING=NOT_PROVEN
J_DML_001=NOT_CLOSED
ISSUER_ISSUANCE_GATE=CLOSED
IMPLEMENTATION_GATE=CLOSED
DECISION_CONSUMPTION_GATE=CLOSED
PRODUCTION_CONSUMER_READINESS=BLOCKED
PROVIDER_RFI_SENT=NO
FULL_COST=NOT_VERIFIED
```

The prepared Russian questions stay in `SELECTEL_PROVIDER_RFI_RU.md`. Sending them would be a later controller decision. It is not made here.
