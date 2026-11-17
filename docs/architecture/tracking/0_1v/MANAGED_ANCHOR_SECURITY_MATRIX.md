# 0.1V: Security matrix

Checked 2026-11-17. Labels are `DOCUMENTED`, `PARTIAL`, `NOT_VERIFIED`, or `UNSUPPORTED`. A `PARTIAL` row is not a pass. Sources are the shortlist.

| ID | Requirement | C1 Azure Confidential Ledger | C2 DynamoDB | C3 S3 Object Lock compliance | C4 Cloud Storage locked retention | C5 Public Rekor |
| --- | --- | --- | --- | --- | --- | --- |
| A1 | Independent failure domain | `NOT_VERIFIED`. Another provider is not, by itself, another domain from the single provider already holding the three servers. | `NOT_VERIFIED` | `NOT_VERIFIED` | `NOT_VERIFIED` | `PARTIAL`. The public log operator is not BINTRANS. Shared fate with that operator is `NOT_VERIFIED`. |
| A2 | Separate administration | `PARTIAL`. Possible only if BINTRANS does not own the subscription. | `PARTIAL` on the same condition, and the owner remains all-powerful. | `PARTIAL` on the same condition. | `PARTIAL` on the same condition. | `DOCUMENTED` for the public deployment: BINTRANS does not run it. |
| A3 | Conditional write of the exact next sequence | `UNSUPPORTED`. No caller sequence compare-and-swap is documented. | `PARTIAL`. A condition can reject a put of an existing key, or require an attribute value. The caller must invent the sequence encoding. | `PARTIAL`. `If-None-Match` and ETag preconditions cover one object key, not a namespace sequence. | `PARTIAL`. `ifGenerationMatch`, including `0`, covers one object name. A locked retention policy also blocks replacement of that object until it ages out, so a single mutable checkpoint key conflicts with the lock. | `UNSUPPORTED`. The log assigns the index. |
| A4 | Silent rollback of history | `PARTIAL`. Data-plane entries are immutable. Control-plane hard delete removes the ledger. | `UNSUPPORTED`. PITR restore and `DeleteItem` are account operations. | `PARTIAL`. A compliance-locked version remains. A delete marker can hide the current name. Closing the account is the documented way to remove a locked version early. | `PARTIAL`. Locked objects cannot be replaced early. New names can still be added. Project-level restore is `NOT_VERIFIED`. | `PARTIAL`. A consistency proof detects a mutation after a witness kept the old checkpoint. The specification says the log is not immutable by itself. |
| A5 | Durable ACK before PostgreSQL `BEGIN` | `NOT_VERIFIED`. A receipt proves a commit after the ledger accepted it. No page states that the HTTP response is durable before return, or how the issuer must wait. | `NOT_VERIFIED`. A successful conditional write is the API success. Durability before the response is not stated on the pages read. | `NOT_VERIFIED` | `NOT_VERIFIED` | `PARTIAL`. A signed entry timestamp is described as a promise to include the entry. An inclusion proof is the later commitment. That is not the BINTRANS predecessor rule. |
| A6 | Confirm namespace, incarnation, seq, digest | `PARTIAL`. Claims can carry application bytes, and the receipt binds `claimsDigest`. The four fields are not a documented schema. | `UNSUPPORTED` as a receipt. The item can store the bytes. Nothing signs them. | `UNSUPPORTED` as a receipt. | `UNSUPPORTED` as a receipt. | `PARTIAL`. HashedRekord stores a digest. It does not store this tuple, and the log index is not the caller sequence. |
| A7 | Predecessor check and replay protection | `NOT_VERIFIED` | `PARTIAL` if the condition names the predecessor. Replay of a successful put is an application problem. | `NOT_VERIFIED` | `NOT_VERIFIED` | `UNSUPPORTED` for a predecessor sequence. An inclusion proof is replayable as evidence of the same entry, which is what a log is for. |
| A8 | Fail closed when unavailable | `NOT_VERIFIED`. Nothing in the product opens a BINTRANS gate. The issuer would have to refuse. The product does not document that refusal. | `NOT_VERIFIED` | `NOT_VERIFIED` | `NOT_VERIFIED` | `NOT_VERIFIED`. The public service has no BINTRANS availability contract on the pages read. |
| A9 | Restore without losing acknowledged history | `PARTIAL`. Hard delete loses it. A successful ledger, if not deleted, keeps entries. Whether disaster-recovery copies can drop a suffix is `NOT_VERIFIED`. | `UNSUPPORTED`. Restore is how history is moved backward. | `PARTIAL` for versions still inside retention. | `PARTIAL` while retention holds. | `PARTIAL` if witnesses still have the checkpoint that included the entry. |
| A10 | BINTRANS administrator cannot delete or rewrite | `UNSUPPORTED` when BINTRANS owns the subscription: the control plane can hard-delete. `PARTIAL` when another organization owns it and BINTRANS is only Contributor. That owner can still hard-delete. | `UNSUPPORTED` for the account owner. | `UNSUPPORTED` as independence. Root cannot delete the locked version in place. The same account can close itself, and a delete marker changes the current view. | `PARTIAL` during retention, because the bucket cannot be deleted early. The project owner remains the administrator. Whether the project can be deleted is `NOT_VERIFIED`. | `PARTIAL`. A BINTRANS caller cannot remove an entry. The log operator is not bound by a BINTRANS contract on the pages read. |

## Scenario A and Scenario B

Scenario A is a rollback of PostgreSQL, the witness journal, and the high-water store, while the managed store keeps the later acknowledgement. Scenario B is the same rollback plus a rollback or deletion of the managed store by an operator or a compromised administrator. Replication across zones is not evidence for either row.

| Candidate | Scenario A | Scenario B |
| --- | --- | --- |
| C1 | `NOT_PROVEN`. The receipt would let an issuer see a later entry only while the ledger still exists. No test was run. | `NOT_PROVEN`. The documented control-plane hard delete removes the history. There is no documented mechanism that stops the subscription owner. |
| C2 | `NOT_PROVEN`. | `NOT_PROVEN`. PITR restore is a logical rollback the account can perform. |
| C3 | `NOT_PROVEN`. A retained version can still be read by version id after a delete marker. The issuer has no documented rule that does so. | `NOT_PROVEN`. Account closure is a documented deletion path. A delete marker changes what a current-key read sees. |
| C4 | `NOT_PROVEN`. | `NOT_PROVEN`. Early replacement is blocked. Deletion of the enclosing project, and any snapshot of it, were not shown to be impossible. |
| C5 | `NOT_PROVEN`. | `NOT_PROVEN`. Detection by a witness is not a refusal the issuer can rely on unless the witness is itself an independent anchor. |

`P3B_SCENARIO_A=NOT_PROVEN`. `P3B_SCENARIO_B=NOT_PROVEN`.

## Matrix

| Candidate | Independence | Monotonic write | Rollback protection | Admin separation | Price | Verdict |
| --- | --- | --- | --- | --- | --- | --- |
| C1 Azure Confidential Ledger | `NOT_VERIFIED` | `UNSUPPORTED` for a caller sequence | Data plane `DOCUMENTED`, owner delete `DOCUMENTED` | Separate subscription only | `PRICE_NOT_VERIFIED` | `REQUIRES_PROVIDER_CONFIRMATION` |
| C2 DynamoDB | `NOT_VERIFIED` | `PARTIAL` | `UNSUPPORTED` | Separate account is still an owner | Example rates published | `NOT_SUITABLE` |
| C3 S3 Object Lock compliance | `NOT_VERIFIED` | `PARTIAL` | `PARTIAL` | Account closure remains | GET rate only | `NOT_SUITABLE` |
| C4 Cloud Storage locked retention | `NOT_VERIFIED` | `PARTIAL` | `PARTIAL` | Project owner remains | Published, see the cost model | `REQUIRES_PROVIDER_CONFIRMATION` |
| C5 Public Rekor | `PARTIAL` | `UNSUPPORTED` | Detection, not prevention | Public operator | `PRICE_NOT_VERIFIED` | `NOT_SUITABLE` |

`BEST_DOCUMENTED_CANDIDATE=C1`. The receipt and the data-plane immutability statement are the closest match to a verifiable acknowledgement. C1 does not document A3, and its owner can hard-delete the ledger.

`LOWEST_VERIFIED_COST=C2`. That rank is a price rank. C2 is `NOT_SUITABLE`.

`SECURITY_GAPS=A3,A5,A7,A8,A10`. No candidate documents a conditional exact-next sequence, a durability statement tied to the HTTP acknowledgement, predecessor replay rules, fail-closed issuance, and a ban on the account owner deleting history.

`SUITABLE_FOR_ISOLATED_PILOT=NO`.
