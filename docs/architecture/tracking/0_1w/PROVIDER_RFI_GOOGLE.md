# RFI-3: Google Cloud — Cloud Storage Bucket Lock

Status: `NOT_SENT`. `PROVIDER_REPLY=NOT_RECEIVED`. Prepared 2026-11-18. Do not send until the controller names a recipient.

`CONTACT_NOT_VERIFIED`. Customer Care is a console case for customers with a support offer. No mailbox for this qualification is published on the support hub.

https://cloud.google.com/support-hub

The source for the lien facts is the Bucket Lock page, re-read 2026-11-18.

https://docs.cloud.google.com/storage/docs/bucket-lock

Preconditions and prices were read on 2026-11-17.

https://docs.cloud.google.com/storage/docs/request-preconditions

https://cloud.google.com/storage/pricing

## What we already take from public documentation

Locking a retention policy is irreversible. The policy cannot be removed or shortened. The period can be increased. Objects younger than the period cannot be deleted or replaced. The bucket cannot be deleted until every object has met the period.

Locking the policy places a lien on the project. The lien blocks `resourcemanager.projects.delete`. The project owner or an organization administrator removes the lien. Removing the lien does not remove the locked retention policy. After removal, the lien no longer blocks project deletion. The page says the Google Cloud account must stay active. It recommends that the person who removes the lien sit outside the IT chain. That recommendation is not a technical prohibition. `GCS_PROJECT_LIEN_REMOVAL=DOCUMENTED`. `GCS_ACCOUNT_OWNER_ROLLBACK_PROTECTION=NOT_PROVEN`. `GCS_A10=NOT_SATISFIED_FOR_BINTRANS`.

`ifGenerationMatch` conditions one object name, including the value `0` for "no live object". It is not an application sequence. A locked policy also blocks replacement of that object, so one mutable checkpoint key conflicts with the lock.

Published unit prices used in PR #392, for a single-region Standard bucket with a flat namespace: Class A $0.005 per 1,000, Class B $0.0004 per 1,000, storage $0.000027397 per GiB-hour, internet egress $0.12 per GiB for the first 10 TiB to worldwide destinations excluding Asia and Australia. Those are page rates, not a quotation for this anchor.

## Questions

### Q1 — Exact successor

Can a write be rejected unless the object body is the next application sequence for a namespace and incarnation? Generation match is not that condition. If the mechanism is one new object name per sequence, how is a second writer stopped from creating a conflicting name, and how does a reader know which name is the latest without trusting an administrator-controlled pointer?

Evidence: the API and the race between two writers.

### Q2 — Durable acknowledgement

Which successful response survives a zone failure and a recovery of the bucket? Retention lock is not, by itself, the durability statement for the acknowledgement.

Evidence: the durability class of a Standard single-region object after HTTP 200.

### Q3 — Unknown outcome

If the client loses the response, can a caller-supplied idempotency key show whether that sequence was stored? A retry that creates a second object name is not idempotent.

Evidence: the key field and the retry result.

### Q4 — Account-owner deletion

The lien path is already documented. Confirm whether, after the project owner or an organization administrator removes the lien, project deletion destroys objects that have not met retention. Can a Google operator do that without the customer removing the lien? Can a second organization forbid lien removal?

Evidence: the project-delete result while objects are inside retention, and the identity that can remove the lien.

### Q5 — Rollback and restore

Can a snapshot, a soft-delete restore, or a support operation replace the live history with an earlier copy so that a previously acknowledged sequence is no longer readable? Removing the lien must not be described as unlocking the bucket policy.

Evidence: the restore operation and its effect on objects still inside retention.

### Q6 — Independent administrator

Can a project and organization be owned by a custodian who is not a BINTRANS administrator, with BINTRANS limited to object create and object read, and with no right to remove the lien, change the retention period, or delete the project?

Evidence: the roles. The page's advice to place lien removal outside IT is not this split unless the product enforces it.

### Q7 — Verifiable evidence

Does Cloud Storage sign a receipt over the application digest that a third party can verify without the project? An MD5 or CRC32C of the object is not that receipt unless the page says it is bound to a Google-operated key that the custodian does not hold.

Evidence: the signed fields, if any.

### Q8 — Availability and partition

What error means the write did not happen? Is a missing object defined as sequence zero?

Evidence: the codes the issuer must treat as refusal.

### Q9 — Recovery

After a failure that leaves the bucket in place, can every object written under retention be listed and read with the digest that was acknowledged?

Evidence: the list order and any object a recovery may omit.

### Q10 — Pilot and cost

Confirm or replace the unit prices cited above, and add:

- regions;
- minimum charge;
- the cost of 10,000, 100,000, and 1,000,000 confirmations, stating the operations each confirmation uses;
- Always Free limits and why they are not a pilot commitment;
- the price of Cloud Audit Logs for lien removal and project deletion;
- the price of a second organization.

Evidence: SKU names. Do not send a sales proposal in place of the SKUs.
