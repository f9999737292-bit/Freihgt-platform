# 0.1V: Managed anchor provider shortlist

Checked on 2026-11-17. No account was opened. No resource was created. No price was inferred beyond a figure printed on the cited page.

Confirmed limits from PR #391: three servers, one provider, provider name `NOT_VERIFIED`, the third server is `NOT_SUITABLE`, a fourth server is `NOT_AUTHORIZED`. The protocol is PR #390 contract v1: one durable acknowledgement of `(namespace, incarnation, seq, digest)` before PostgreSQL `BEGIN`, exact successor only, fail closed.

Amazon QLDB is not a candidate. AWS lists it in full shutdown with end of support on 2025-07-31.

https://docs.aws.amazon.com/general/latest/gr/full_shutdown_services.html

No managed product was found whose only job is a monotonic checkpoint of an application sequence. The four candidates below are the closest real services in the requested classes.

## Candidates

| ID | Provider | Product | Class | Region in the cited pages | Primary sources, checked 2026-11-17 |
| --- | --- | --- | --- | --- | --- |
| C1 | Microsoft | Azure Confidential Ledger | Managed append-only ledger with a receipt | The overview says availability is limited and points to a residency page. A single region was not stated on the pages read. | https://learn.microsoft.com/en-us/azure/confidential-ledger/overview ; https://learn.microsoft.com/en-us/azure/confidential-ledger/write-transaction-receipts ; https://learn.microsoft.com/en-us/azure/confidential-ledger/verify-write-transaction-receipts |
| C2 | Amazon | DynamoDB on-demand, condition expressions | Strongly consistent key-value store | Price examples used here are US East (N. Virginia). | https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Expressions.ConditionExpressions.html ; https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_TransactWriteItems.html ; https://aws.amazon.com/dynamodb/pricing/ |
| C3 | Amazon | S3 Object Lock, compliance mode, plus conditional requests | Object store with a retention lock | Object Lock is documented as an S3 feature. The GET price example below is US East (N. Virginia). The regional storage table did not render as text on the pricing page. | https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock.html ; https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-requests.html ; https://aws.amazon.com/s3/pricing/ |
| C4 | Google | Cloud Storage, locked bucket retention, plus request preconditions | Object store with an irreversible retention policy | Operation prices below are for a bucket in one region. Always Free storage is aggregated across `US-WEST1`, `US-CENTRAL1`, and `US-EAST1`. | https://docs.cloud.google.com/storage/docs/bucket-lock ; https://docs.cloud.google.com/storage/docs/request-preconditions ; https://cloud.google.com/storage/pricing |
| C5 | Sigstore | Public Rekor transparency log at `rekor.sigstore.dev` | Public append-only log with inclusion proofs | The public deployment's site is not stated in the specification. | https://docs.sigstore.dev/logging/overview/ ; https://github.com/sigstore/architecture-docs/blob/main/rekor-spec.md |

C5 is listed so the public-log class is not skipped. It is not a private sequence store. Entries are signed artifacts of a pluggable type. The specification says a transparency log does not itself make history immutable. It makes a mutation detectable to a witness that kept an older checkpoint. Witnesses must sit in a trust domain separate from the log operator.

## What each product actually documents

C1 documents an append-only data plane, a Merkle receipt, and verification of `writeSetDigest`, `commitEvidence`, and `claimsDigest`. A committed entry cannot be deleted through the data plane. The same overview says a control-plane administrator can delete the ledger resource, and that deletion is a hard delete with no recovery. User roles inside the ledger are Reader, Contributor, and Administrator. That Administrator manages ledger users. It is not a second cloud account. Collection IDs group records. The ledger assigns the transaction. No page documents a compare-and-swap on a caller-supplied sequence.

C2 documents a condition expression on the item named by the request key. `attribute_not_exists` rejects a second put of the same key. `TransactWriteItems` is atomic across its actions and fails when a condition fails. The account that owns the table can delete the table. Point-in-time restore is a documented, billed operation on the same pricing page. Neither fact is an anchor.

C3 documents that, in compliance mode, no user including the account root can overwrite or delete a protected object version, shorten its retention, or change its mode, until the retain-until date. The documented early-deletion path is deletion of the AWS account. A simple `DELETE` without a version id still returns success and inserts a delete marker, which becomes the current version. Conditional writes fail the request when the precondition does not hold, at the normal request price, including when the request fails. There is no API for `(namespace, incarnation, seq, digest)`.

C4 documents a retention policy that, once locked, cannot be removed or shortened. The retention period can be increased. The bucket cannot be deleted until every object has aged past the policy. Objects cannot be deleted or replaced before that age. `ifGenerationMatch` makes one object write conditional on that object's generation, including the special value `0` for "no live object of this name". It does not name an application sequence. A new object name is a different object.

C5 documents inclusion proofs, signed checkpoints, and consistency proofs between two tree sizes. The log index is assigned by the log. A new shard starts its index at zero. The public API accepts proposed entries of defined types and can return `409` on conflict. It does not document a private namespace whose next sequence is conditional.

## Authority

| Question | C1 | C2 | C3 | C4 | C5 |
| --- | --- | --- | --- | --- | --- |
| Who creates it | A customer subscription | An AWS account | An AWS account | A Google Cloud project | The Sigstore operators, for the public log |
| Who holds credentials | Entra ID or a certificate, managed inside the ledger | The AWS account | The AWS account | The Google Cloud project | Callers of the public API. A private key for the checkpoint stays with the log |
| Who can delete one record | Not the data plane, after commit | Any principal allowed to `DeleteItem` | Not a compliance-locked version. A delete marker can hide the current name | Not until retention age is reached | Not by a BINTRANS caller. The log operator is outside BINTRANS |
| Who can delete the store | The control-plane administrator of the subscription | The account that can `DeleteTable` | The account, by closing the account. The bucket is not a delete path for a locked version | The bucket, only after every object has passed retention. The project lien can be removed by the project owner or an organization administrator, which unblocks project deletion. Removing the lien does not unlock the bucket policy. | Not documented as a BINTRANS action |
| Who can restore a snapshot | Hard delete does not restore data. Disaster-recovery files are documented; a rewind of committed transactions is `NOT_VERIFIED` | PITR and on-demand backup restore are documented | Not a version that is still under compliance retention. A delete marker changes the current name | `NOT_VERIFIED` for project-level restore | A witness detects a rewrite. It does not stop the operator from publishing a new tree |
| Who can turn retention off | No retention switch is documented. Deleting the resource removes the ledger | Backup plans are optional and account-controlled | Compliance retention cannot be shortened by any user | A locked policy cannot be removed or shortened | Not a retention product |
| Who can change IAM | Ledger Administrator for data-plane users. Subscription owner for the resource | The AWS account | The AWS account | The Google Cloud project | The log operator |
| Who signs the acknowledgement | The ledger node, over the Merkle root | No ledger signature is documented | No ledger signature is documented | No ledger signature is documented | The log signs the checkpoint |
| Can the anchor owner be a different organization from BINTRANS | Yes, if the subscription is not BINTRANS. The owner of that subscription can still hard-delete | Yes, and that owner can still delete and restore the table | Yes, and that owner can still close the account | Yes, and that owner still administers the project | The public log already has a different operator |

Ordinary IAM inside one account is not separation from the account owner. A different organization moves the owner. It does not remove one.
