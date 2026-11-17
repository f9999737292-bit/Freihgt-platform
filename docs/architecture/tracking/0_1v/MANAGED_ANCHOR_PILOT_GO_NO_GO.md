# 0.1V: Pilot go / no-go

`SUITABLE_FOR_ISOLATED_PILOT=NO`.

`PILOT_RECOMMENDATION=DO_NOT_START`.

Checked 2026-11-17. No provider was contacted. No account was created. A fourth server remains `NOT_AUTHORIZED`.

## Why the pilot does not start

PR #390 requires a durable acknowledgement of the exact next `(namespace, incarnation, seq, digest)` before PostgreSQL `BEGIN`, and a history the BINTRANS administrator cannot lower. The shortlist does not contain a product for which every one of A1–A10 is `DOCUMENTED`.

| Gate | Result |
| --- | --- |
| A product whose data plane rejects a lower sequence | Not documented. DynamoDB and the two object stores can condition a write on a key, an ETag, or a generation. None of those is the BINTRANS sequence. |
| A product whose owner cannot delete or rewind acknowledged history | Not documented. Confidential Ledger hard-deletes from the control plane. DynamoDB restores from PITR. S3 compliance mode still documents account deletion, and a delete marker changes the current name. Cloud Storage blocks early object replacement. Locking the policy adds a project lien, and the project owner or an organization administrator can remove that lien and then delete the project. Removing the lien does not unlock the bucket policy. Rekor detects a split view only for a witness that kept the old checkpoint. |
| A published price for the best-documented candidate | `PRICE_NOT_VERIFIED` for Azure Confidential Ledger. |
| An isolated host | PR #391: `AVAILABLE` isolated hosts were zero, and the third existing server is `NOT_SUITABLE`. |

`BEST_DOCUMENTED_CANDIDATE=AZURE_CONFIDENTIAL_LEDGER_NOT_APPROVED`. The preview parameter `waitForCommit=true` documents a response after commit. It does not approve a pilot. Exact successor, retry idempotency, and subscription hard delete stay open.

`LOWEST_VERIFIED_COST=C2` DynamoDB. `DYNAMODB_0_75_USD_PER_MILLION=ONE_WRITE_PLUS_ONE_STRONG_READ_ONLY` on the published US East (N. Virginia) example rates. `DYNAMODB_FULL_ANCHOR_COST=NOT_VERIFIED`. Price does not make it an anchor. The figures were not recalculated.

## What a later go would require

All of the following, in writing, from the provider, for one named product and one named region:

- the write response is durable before it returns;
- the service rejects any write except the exact next sequence for that namespace and incarnation;
- a BINTRANS identity cannot delete an entry, delete the store, shorten retention, or restore an older snapshot;
- the identity that can do those things is not a BINTRANS operator and is not the same account that runs the three existing servers;
- a receipt binds namespace, incarnation, sequence, digest, and predecessor;
- an outage is an error, and an empty store is not treated as sequence zero;
- a price for 10,000, 100,000, and 1,000,000 confirmations a month.

Until those answers exist, `IMPLEMENTATION_GATE` stays `CLOSED`, `ANCHOR_FAILURE_DOMAIN` stays `NOT_PROVEN`, and `P3B_ALL_STORES_ROLLBACK` stays `NOT_FIXED`.
