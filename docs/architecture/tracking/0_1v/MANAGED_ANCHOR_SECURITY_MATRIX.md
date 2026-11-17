# 0.1V: Security matrix

Checked 2026-11-17. The C1 A5 and C4 A10 rows were re-read on 2026-11-18. Labels are `DOCUMENTED`, `PARTIAL`, `NOT_VERIFIED`, or `UNSUPPORTED`. A `PARTIAL` row is not a pass. Sources are the shortlist, plus the two pages cited in the remediation notes.

| ID | Requirement | C1 Azure Confidential Ledger | C2 DynamoDB | C3 S3 Object Lock compliance | C4 Cloud Storage locked retention | C5 Public Rekor |
| --- | --- | --- | --- | --- | --- | --- |
| A1 | Independent failure domain | `NOT_VERIFIED`. Another provider is not, by itself, another domain from the single provider already holding the three servers. | `NOT_VERIFIED` | `NOT_VERIFIED` | `NOT_VERIFIED` | `PARTIAL`. The public log operator is not BINTRANS. Shared fate with that operator is `NOT_VERIFIED`. |
| A2 | Separate administration | `PARTIAL`. Possible only if BINTRANS does not own the subscription. | `PARTIAL` on the same condition, and the owner remains all-powerful. | `PARTIAL` on the same condition. | `PARTIAL` on the same condition. | `DOCUMENTED` for the public deployment: BINTRANS does not run it. |
| A3 | Conditional write of the exact next sequence | `UNSUPPORTED`. No caller sequence compare-and-swap is documented. | `PARTIAL`. A condition can reject a put of an existing key, or require an attribute value. The caller must invent the sequence encoding. | `PARTIAL`. `If-None-Match` and ETag preconditions cover one object key, not a namespace sequence. | `PARTIAL`. `ifGenerationMatch`, including `0`, covers one object name. A locked retention policy also blocks replacement of that object until it ages out, so a single mutable checkpoint key conflicts with the lock. | `UNSUPPORTED`. The log assigns the index. |
| A4 | Silent rollback of history | `PARTIAL`. Data-plane entries are immutable. Control-plane hard delete removes the ledger. | `UNSUPPORTED`. PITR restore and `DeleteItem` are account operations. | `PARTIAL`. A compliance-locked version remains. A delete marker can hide the current name. Closing the account is the documented way to remove a locked version early. | `PARTIAL`. Locked objects cannot be replaced early. New names can still be added. Project-level restore is `NOT_VERIFIED`. | `PARTIAL`. A consistency proof detects a mutation after a witness kept the old checkpoint. The specification says the log is not immutable by itself. |
| A5 | Durable ACK before PostgreSQL `BEGIN` | `PARTIAL`. See the C1 note. `AZURE_DURABLE_ACK=PARTIAL`. | `NOT_VERIFIED`. A successful conditional write is the API success. Durability before the response is not stated on the pages read. | `NOT_VERIFIED` | `NOT_VERIFIED` | `PARTIAL`. A signed entry timestamp is described as a promise to include the entry. An inclusion proof is the later commitment. That is not the BINTRANS predecessor rule. |
| A6 | Confirm namespace, incarnation, seq, digest | `PARTIAL`. Claims can carry application bytes, and the receipt binds `claimsDigest`. The four fields are not a documented schema. | `UNSUPPORTED` as a receipt. The item can store the bytes. Nothing signs them. | `UNSUPPORTED` as a receipt. | `UNSUPPORTED` as a receipt. | `PARTIAL`. HashedRekord stores a digest. It does not store this tuple, and the log index is not the caller sequence. |
| A7 | Predecessor check and replay protection | `NOT_VERIFIED` | `PARTIAL` if the condition names the predecessor. Replay of a successful put is an application problem. | `NOT_VERIFIED` | `NOT_VERIFIED` | `UNSUPPORTED` for a predecessor sequence. An inclusion proof is replayable as evidence of the same entry, which is what a log is for. |
| A8 | Fail closed when unavailable | `NOT_VERIFIED`. Nothing in the product opens a BINTRANS gate. The issuer would have to refuse. The product does not document that refusal. | `NOT_VERIFIED` | `NOT_VERIFIED` | `NOT_VERIFIED` | `NOT_VERIFIED`. The public service has no BINTRANS availability contract on the pages read. |
| A9 | Restore without losing acknowledged history | `PARTIAL`. Hard delete loses it. A successful ledger, if not deleted, keeps entries. Whether disaster-recovery copies can drop a suffix is `NOT_VERIFIED`. | `UNSUPPORTED`. Restore is how history is moved backward. | `PARTIAL` for versions still inside retention. | `PARTIAL` while retention holds. | `PARTIAL` if witnesses still have the checkpoint that included the entry. |
| A10 | BINTRANS administrator cannot delete or rewrite | `UNSUPPORTED` when BINTRANS owns the subscription: the control plane can hard-delete. `PARTIAL` when another organization owns it and BINTRANS is only Contributor. That owner can still hard-delete. `AZURE_HARD_DELETE_RISK=OPEN`. | `UNSUPPORTED` for the account owner. | `UNSUPPORTED` as independence. Root cannot delete the locked version in place. The same account can close itself, and a delete marker changes the current view. | `NOT_SATISFIED_FOR_BINTRANS`. See the C4 note. | `PARTIAL`. A BINTRANS caller cannot remove an entry. The log operator is not bound by a BINTRANS contract on the pages read. |

## Scenario A and Scenario B

Scenario A is a rollback of PostgreSQL, the witness journal, and the high-water store, while the managed store keeps the later acknowledgement. Scenario B is the same rollback plus a rollback or deletion of the managed store by an operator or a compromised administrator. Replication across zones is not evidence for either row.

| Candidate | Scenario A | Scenario B |
| --- | --- | --- |
| C1 | `NOT_PROVEN`. The receipt would let an issuer see a later entry only while the ledger still exists. No test was run. | `NOT_PROVEN`. The documented control-plane hard delete removes the history. There is no documented mechanism that stops the subscription owner. |
| C2 | `NOT_PROVEN`. | `NOT_PROVEN`. PITR restore is a logical rollback the account can perform. |
| C3 | `NOT_PROVEN`. A retained version can still be read by version id after a delete marker. The issuer has no documented rule that does so. | `NOT_PROVEN`. Account closure is a documented deletion path. A delete marker changes what a current-key read sees. |
| C4 | `NOT_PROVEN`. | `NOT_PROVEN`. Early object replacement stays blocked. The project owner or an organization administrator can remove the project lien and then delete the project. Removing the lien does not unlock the bucket policy. A snapshot restore is still `NOT_VERIFIED`. `GCS_ACCOUNT_OWNER_ROLLBACK_PROTECTION=NOT_PROVEN`. |
| C5 | `NOT_PROVEN`. | `NOT_PROVEN`. Detection by a witness is not a refusal the issuer can rely on unless the witness is itself an independent anchor. |

`P3B_SCENARIO_A=NOT_PROVEN`. `P3B_SCENARIO_B=NOT_PROVEN`.

## Matrix

| Candidate | Independence | Monotonic write | Rollback protection | Admin separation | Price | Verdict |
| --- | --- | --- | --- | --- | --- | --- |
| C1 Azure Confidential Ledger | `NOT_VERIFIED` | `UNSUPPORTED` for a caller sequence | Data plane `DOCUMENTED`, owner delete `DOCUMENTED` | Separate subscription only | `PRICE_NOT_VERIFIED` | `REQUIRES_PROVIDER_CONFIRMATION` |
| C2 DynamoDB | `NOT_VERIFIED` | `PARTIAL` | `UNSUPPORTED` | Separate account is still an owner | Example rates published | `NOT_SUITABLE` |
| C3 S3 Object Lock compliance | `NOT_VERIFIED` | `PARTIAL` | `PARTIAL` | Account closure remains | GET rate only | `NOT_SUITABLE` |
| C4 Cloud Storage locked retention | `NOT_VERIFIED` | `PARTIAL` | `PARTIAL`. Lien removal is documented. Owner rollback protection is `NOT_PROVEN`. | Project owner or organization administrator can remove the lien | Published, see the cost model | `REQUIRES_PROVIDER_CONFIRMATION` |
| C5 Public Rekor | `PARTIAL` | `UNSUPPORTED` | Detection, not prevention | Public operator | `PRICE_NOT_VERIFIED` | `NOT_SUITABLE` |

`BEST_DOCUMENTED_CANDIDATE=AZURE_CONFIDENTIAL_LEDGER_NOT_APPROVED`. The receipt and the data-plane immutability statement are the closest match to a verifiable acknowledgement. `waitForCommit=true` does not approve it. C1 does not document an exact successor, and its subscription owner can hard-delete the ledger.

`LOWEST_VERIFIED_COST=C2`. That rank is a price rank for one standard write plus one strongly consistent read. `DYNAMODB_0_75_USD_PER_MILLION=ONE_WRITE_PLUS_ONE_STRONG_READ_ONLY`. `DYNAMODB_FULL_ANCHOR_COST=NOT_VERIFIED`. C2 is `NOT_SUITABLE`.

`SECURITY_GAPS=A3,A5,A7,A8,A10`. No candidate documents an exact-next sequence, a proven durable acknowledgement through a dropped connection, predecessor replay rules, fail-closed issuance, and a ban on the managing account deleting history. C1 documents a preview response-after-commit. That preview is not the whole of A5.

`SUITABLE_FOR_ISOLATED_PILOT=NO`.

`A1_A10_COMPLETE=NO_CANDIDATE_ALL_DOCUMENTED`.
`PROVIDERS_RESEARCHED=4`.
`PRODUCTS_RESEARCHED=6`.
`ACTIVE_CANDIDATES=5`.
`AMAZON_QLDB=EXCLUDED_SHUTDOWN`.
`PILOT_RECOMMENDATION=DO_NOT_START`.

## C1 A5, re-read 2026-11-18

Source: [Write transaction receipts](https://learn.microsoft.com/en-us/azure/confidential-ledger/write-transaction-receipts).

`J_PROV_002=FIXED_IN_DOCS`.

The page documents a preview. From REST API version `2026-07-31-preview`, `POST /app/transactions` accepts `waitForCommit=true`. The ledger then responds after the transaction is committed and includes the receipt. Azure support must enable the option for that ledger before it is used. Preview terms apply. Availability without that enablement is not stated.

A response that arrives under that option is a commit acknowledgement. The receipt remains a Merkle proof of that committed transaction. The page does not state a durability guarantee beyond "committed", and it does not say the acknowledgement survives a later hard delete of the ledger resource.

If the connection drops before the response, the write might still be committed. Retrying can create a duplicate entry. The page tells the caller to use the default flow when that distinction matters: the transaction id returns when the write is accepted, and the caller polls until the status is `Committed`. `AZURE_RETRY_IDEMPOTENCY=NOT_PROVEN`.

No field on this page is a caller-defined exact successor. `AZURE_EXACT_SUCCESSOR=NOT_DOCUMENTED`. `AZURE_WAIT_FOR_COMMIT=DOCUMENTED_PREVIEW`. `AZURE_DURABLE_ACK=PARTIAL`. `AZURE_HARD_DELETE_RISK=OPEN`. `AZURE_PRICE=PRICE_NOT_VERIFIED`. The preview parameter does not make C1 suitable for a pilot.

## C4 A10, re-read 2026-11-18

Source: [Bucket Lock](https://docs.cloud.google.com/storage/docs/bucket-lock).

`J_PROV_001=FIXED_IN_DOCS`.

These are separate facts:

1. Locking a retention policy is irreversible. The policy cannot be removed or shortened. The period can still be increased. An attempt to remove or shorten it returns `400 BadRequestException`.
2. While an object is younger than the retention period, it cannot be deleted or replaced. The bucket itself cannot be deleted until every object has met the period.
3. Locking the policy causes Cloud Storage to place a lien on the project. The lien blocks `resourcemanager.projects.delete`. The project owner or an organization administrator removes the lien. `GCS_PROJECT_LIEN_REMOVAL=DOCUMENTED`.
4. After that removal, the lien no longer blocks project deletion. The page states that the lien must be removed in order to delete the project. Removing the lien does not remove or shorten the locked retention policy on the bucket. The bucket-deletion limit remains.
5. The same page says the Google Cloud account must stay active. It recommends that the person who removes the lien sit outside the IT accountability chain. That is not a demonstrated control. A managing account that is allowed to remove the lien can take the documented path. `GCS_ACCOUNT_OWNER_ROLLBACK_PROTECTION=NOT_PROVEN`. `GCS_A10=NOT_SATISFIED_FOR_BINTRANS`.
