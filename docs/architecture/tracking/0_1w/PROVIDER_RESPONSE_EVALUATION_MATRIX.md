# Provider response evaluation

Prepared 2026-11-18. No request has been sent. `PROVIDER_REPLY=NOT_RECEIVED` on every row. A cell in "Official evidence" is a reading of a public page. It is not a provider answer. `CONFIRMED_BY_PROVIDER` is unused.

Labels: `CONFIRMED_DOCUMENTED`, `CONFIRMED_BY_PROVIDER`, `PARTIAL`, `NOT_VERIFIED`, `UNSUPPORTED`.

| Requirement | Provider answer | Official evidence | Verified | Blocker |
| --- | --- | --- | --- | --- |
| C1 exact successor | `NOT_RECEIVED` | No caller-defined compare-and-set is described on the receipts or overview pages. | `UNSUPPORTED` as documented. Whether an undocumented API exists is `NOT_VERIFIED`. | Yes, until a reply shows the condition inside the commit. |
| C1 durable ACK | `NOT_RECEIVED` | Preview `2026-07-31-preview` with `waitForCommit=true` responds after commit and can include the receipt. Support must enable it. A dropped connection can leave the outcome unknown. | `PARTIAL` | Yes. Preview is not a production guarantee. |
| C1 unknown outcome / retry | `NOT_RECEIVED` | The same page says a retry can create a duplicate entry. No idempotency key is named. | `NOT_VERIFIED` | Yes. |
| C1 account-owner delete | `NOT_RECEIVED` | Control-plane deletion is a hard delete. | `CONFIRMED_DOCUMENTED` for the delete. A second administrator who can stop it is `NOT_VERIFIED`. | Yes. `AZURE_HARD_DELETE_RISK=OPEN`. |
| C1 rollback | `NOT_RECEIVED` | Hard delete removes history. A rewind that keeps the resource and drops receipted transactions is `NOT_VERIFIED`. | `NOT_VERIFIED` | Yes. |
| C1 independent administrator | `NOT_RECEIVED` | Ledger users are local Reader, Contributor, and Administrator. The subscription owner remains on the control plane. The FAQ says Microsoft does not manage users after creation. | `PARTIAL` | Yes, until the Owner cannot delete the resource. |
| C1 verifiable receipt | `NOT_RECEIVED` | A receipt binds Merkle fields and a node signature. Offline use after the resource is deleted is `NOT_VERIFIED`. | `PARTIAL` | Yes for the "after deletion" case. |
| C1 partition | `NOT_RECEIVED` | Not stated as a BINTRANS fail-closed rule. | `NOT_VERIFIED` | Yes. |
| C1 continuous recovery | `NOT_RECEIVED` | Entries remain while the ledger exists. A recovery that drops a suffix is `NOT_VERIFIED`. | `PARTIAL` | Yes. |
| C1 price and preview terms | `NOT_RECEIVED` | The pricing page did not render a number. Preview requires support enablement. | `PRICE_NOT_VERIFIED` | Yes. |
| C2 DynamoDB successor | `NOT_RECEIVED` | Condition expressions reject a write when the item condition fails. They do not name this tuple. | `PARTIAL` | Yes. The account can also restore and delete. |
| C2 durable ACK | `NOT_RECEIVED` | A successful write API is documented. Durability before the response was not on the pages read. | `NOT_VERIFIED` | Yes. |
| C2 retry | `NOT_RECEIVED` | No caller idempotency key for this sequence was read. | `NOT_VERIFIED` | Yes. |
| C2 owner delete and PITR | `NOT_RECEIVED` | DeleteTable and point-in-time restore are documented account operations. | `UNSUPPORTED` as an independent anchor. | Yes. This is why DynamoDB alone is `NOT_SUITABLE`. |
| C3 S3 compliance retention | `NOT_RECEIVED` | Root cannot delete a locked version in place. A delete marker hides the current name. Account closure is the early-deletion path. | `PARTIAL` | Yes. |
| C3 S3 successor | `NOT_RECEIVED` | Conditional requests cover an ETag or key existence, not an application sequence. | `PARTIAL` | Yes. |
| C3 signed receipt | `NOT_RECEIVED` | No application receipt is documented. | `UNSUPPORTED` on the pages read. | Yes. |
| C2+C3 composite atomicity | `NOT_RECEIVED` | No page describes one atomic commit across a DynamoDB item and an S3 object. | `NOT_VERIFIED` | Yes. |
| C2+C3 price | `NOT_RECEIVED` | Read-plus-write example is $0.75 per 1,000,000 at the cited rates. S3 PUT and storage did not render. Full anchor cost is not that example. | `PARTIAL` | Yes for the missing meters. |
| C4 successor | `NOT_RECEIVED` | `ifGenerationMatch` covers one object generation. | `PARTIAL` | Yes. |
| C4 lien and project delete | `NOT_RECEIVED` | Lien removal by the project owner or an organization administrator is documented and does not unlock retention. Effect of the following project delete on young objects is `NOT_VERIFIED`. | `CONFIRMED_DOCUMENTED` for the lien path. `NOT_PROVEN` for owner rollback protection. | Yes. `GCS_A10=NOT_SATISFIED_FOR_BINTRANS`. |
| C4 receipt | `NOT_RECEIVED` | No signed application receipt was found. | `UNSUPPORTED` on the pages read. | Yes. |
| C4 price | `NOT_RECEIVED` | Unit prices are published. A provider confirmation of the anchor scenario is not. | `PARTIAL` | No, for the unit rates. Yes, for a pilot budget, until the operation mix is confirmed. |
| C5 successor | `NOT_RECEIVED` | The log assigns the index. Shards restart at zero. | `UNSUPPORTED` | Yes. |
| C5 immutability | `NOT_RECEIVED` | The specification says the log is not immutable by itself. Witnesses detect a split view. | `CONFIRMED_DOCUMENTED` | Yes, unless a witness is itself an independent anchor. |
| C5 corporate offering and price | `NOT_RECEIVED` | No offering or price was found. The security address is for vulnerabilities, not this RFI. | `NOT_VERIFIED` | Yes. |

`P3B_SCENARIO_A=NOT_PROVEN`. `P3B_SCENARIO_B=NOT_PROVEN`. No row above is `CONFIRMED_BY_PROVIDER`.
