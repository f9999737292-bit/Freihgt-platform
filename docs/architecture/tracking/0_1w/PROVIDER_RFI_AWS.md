# RFI-2: AWS — DynamoDB and S3 Object Lock

Status: `NOT_SENT`. `PROVIDER_REPLY=NOT_RECEIVED`. Prepared 2026-11-18. Do not send until the controller names a recipient.

`CONTACT_NOT_VERIFIED`. The public contact page lists technical support as unavailable on the Basic Support Plan and does not name a recipient for this qualification.

https://aws.amazon.com/contact-us/

This request asks whether a composite of DynamoDB and S3 Object Lock can be an independent recovery anchor. It does not ask AWS to propose a new architecture. Public pages already read are the DynamoDB condition-expression and pricing pages, and the S3 Object Lock and conditional-request pages, as recorded in PR #392.

## What we already take from public documentation

DynamoDB evaluates a condition expression against the item named by the request key. A failed condition rejects that write. `TransactWriteItems` is atomic for its actions. The account that owns the table can delete it. Point-in-time restore is a documented operation. The pricing-page examples for US East (N. Virginia), on-demand Standard, are $0.625 per million 1 KB writes and $0.125 per million 1 KB strongly consistent reads. One write plus one strong read at 1,000,000 confirmations is $0.75 on those examples. `DYNAMODB_FULL_ANCHOR_COST=NOT_VERIFIED`. Those facts make DynamoDB alone `NOT_SUITABLE`.

S3 Object Lock compliance mode stops every user, including the account root, from overwriting or deleting a protected object version or from shortening its retention. The documented early-deletion path is deletion of the AWS account. A simple `DELETE` without a version id returns success and inserts a delete marker, which becomes the current version. Conditional writes use the normal request price, including when they fail. The fetched pricing page stated $0.0004 per 1,000 S3 Standard GET requests in US East (N. Virginia). Standard PUT and Standard storage per GB-month did not render. There is no minimum charge.

https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Expressions.ConditionExpressions.html

https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock.html

https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-requests.html

https://aws.amazon.com/dynamodb/pricing/

https://aws.amazon.com/s3/pricing/

## Questions

A composite is in scope only if both halves have one answer. Disaster recovery across regions is not an answer to rollback.

### Q1 — Exact successor

Does any single API, or one atomic transaction across DynamoDB and S3, accept a write only when `(namespace, incarnation, seq_current, predecessor_digest)` matches and then commit `seq_current + 1`? If the caller must build this from `ConditionExpression` and `If-None-Match`, which step can succeed while the other fails?

Evidence: the atomic boundary and the error when two writers use the same predecessor.

### Q2 — Durable acknowledgement

Which response means the conditional item and the compliance-locked object version are both durable after a node or availability-zone failure?

Evidence: the durability statement for a successful `PutItem` and a successful `PutObject` with compliance retention.

### Q3 — Unknown outcome

If the client loses the response, how does it learn whether the sequence advanced? Is there a caller-supplied idempotency key that returns the original write instead of a second sequence?

Evidence: the token field and the retry result.

### Q4 — Account-owner deletion

Can the AWS account root delete the table, restore it to an earlier time, close the account, or insert a delete marker over the current object key? Can a principal outside that account prevent or detect those actions before a later reader accepts a lower sequence?

Evidence: the IAM and account-closure behavior for a compliance-locked version and for point-in-time restore.

### Q5 — Rollback and restore

Point-in-time restore and account closure are already documented. Is there a configuration in which neither the root user nor a backup administrator can move the acknowledged sequence backward or hide it behind a delete marker?

Evidence: the configuration, and what a current-key read returns after a simple `DELETE`.

### Q6 — Independent administrator

Can a separate AWS account own the table and the bucket, with BINTRANS allowed only to call the conditional write and the read, and with no right to `DeleteTable`, backup restore, `PutObjectRetention`, or account closure?

Evidence: the permission boundary. Ordinary IAM inside the BINTRANS account is not sufficient.

### Q7 — Verifiable evidence

Does either service return a proof that a third party can check without trusting the AWS account that holds the data? An ETag or a successful HTTP status is not that proof unless the page says it is signed over the application digest.

Evidence: the signed fields, if any.

### Q8 — Availability and partition

What error is returned when DynamoDB or S3 cannot complete the conditional write? Is an empty table or a missing key defined as sequence zero?

Evidence: the error codes the issuer must treat as "do not issue".

### Q9 — Recovery

After a failure that does not delete the account, can every acknowledged `(seq, digest)` be read, including versions hidden by a delete marker?

Evidence: the read by version id and the gap a restore is allowed to omit.

### Q10 — Pilot and cost

Publish current USD prices for one named region, for 10,000, 100,000, and 1,000,000 confirmations a month, each confirmation being one strong read, one conditional write, and one compliance-locked object of at most 1 KiB:

- DynamoDB on-demand read, write, storage, point-in-time restore, and on-demand backup;
- S3 Standard storage per GB-month, PUT, GET, and any Object Lock charge;
- data transfer out to the internet;
- minimum charges;
- the cost of a second account and of CloudTrail or an equivalent control-plane audit;
- what a free-tier or test account does not include.

Evidence: meter names from the public price list. The $0.75 figure above is only the example read-plus-write total and is not this answer.
