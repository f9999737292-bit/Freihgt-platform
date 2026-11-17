# 0.1V: Questions for providers

Status: `NOT_SENT`. Do not send this list until the controller authorizes a named recipient.

The questions are the gaps in `MANAGED_ANCHOR_SECURITY_MATRIX.md`. A marketing claim of immutability is not an answer. The answer needs the control that enforces it, and what the account owner can still do.

## C1 — Microsoft, Azure Confidential Ledger

1. After the write API returns success, is the entry durable on a majority of ledger nodes before that response, and which receipt field is the acknowledgement?
2. Can a caller condition the write on an application predecessor, so sequence `N+1` is accepted only when the stored record is exactly `(namespace, incarnation, N, predecessor_digest)`?
3. Can any identity, including the subscription Owner and a Microsoft operator, restore the ledger to an earlier transaction or delete a committed entry without deleting the whole resource?
4. The overview says control-plane deletion is a hard delete. Is there a lock, legal hold, or support process that prevents the subscription Owner from deleting the resource for a stated retention period?
5. On disaster recovery from the replicated files, can the recovered ledger omit a transaction that already had a receipt?
6. Which regions can host a ledger, and what is the published USD price per instance hour and per GB-month after the included 100 GB?
7. Is there a non-production ledger whose writes are refused by production BINTRANS, and what does it cost?

## C2 — Amazon DynamoDB

No suitability question remains. PITR restore and table deletion are already documented. Send nothing unless the controller wants those pages re-checked.

## C3 — Amazon S3 Object Lock, compliance mode

1. After `PutObject` returns success for a compliance-retained object, which request establishes that the version cannot be hidden from a current-key read? The object-lock page says a simple `DELETE` inserts a delete marker and returns success.
2. Can the account root close the account while compliance retention is in force, and what happens to the locked versions?
3. Is there a conditional request whose precondition is an application sequence rather than an ETag or a key?
4. Publish the current US East (N. Virginia) Standard storage price per GB-month and the Standard PUT price per 1,000 requests. The fetched pricing page did not render those two tables.

## C4 — Google Cloud Storage, locked retention

1. Can the project owner, or a Google operator, delete the project or restore a snapshot while a locked bucket still holds objects inside their retention period?
2. Can a write be rejected unless the object body contains the exact next application sequence, rather than only matching an object generation?
3. If retention forbids replacing an object, how should a checkpoint advance without creating a second name that an administrator can point readers at?
4. On a regional outage, can recovered objects omit a write that had already returned success?

## C5 — Sigstore public Rekor

1. Does any entry type accept an arbitrary `(namespace, incarnation, seq, digest)` and reject it unless `seq` is the exact successor of that namespace?
2. Who is contractually forbidden from starting a new shard or a new tree that omits earlier entries, and is that obligation available to a private customer?
3. What availability and rate limit apply to `rekor.sigstore.dev`, and is there a price for a dedicated log?

## Stop

A reply that confirms zone redundancy, backups, or a WORM mode, and does not answer the predecessor rule and the account-owner delete path, does not open the pilot.
