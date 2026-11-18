# Selectel service inventory 0.1Y

Read 2026-11-18. No account was opened, no bucket was created, and no message was sent.

The anchor contract is 0.1T, PR #390 at `cd22a3789725cf49a32a8f5bfc3a58980a9ce7dd`, file `docs/architecture/tracking/0_1t/RECOVERY_ANCHOR_PROTOCOL.md`. It is `PROPOSED_NOT_IMPLEMENTED`. Contract version 1 requires `GROUP_COMMIT=DISABLED`, `MAX_SEQUENCE_INCREMENT=1`, and `REQUIRED_PINS=EVERY_ACKNOWLEDGED_RECORD`. An append is accepted only as the exact successor or as an idempotent repeat of an already pinned `(seq, digest)`. The anchor must keep every accepted pair and must sign a certificate over `(namespace_id, incarnation, seq, head_digest, anchor_log_position, challenge)`. PostgreSQL `BEGIN` stays after that acknowledgement. The questions Q1–Q10 in the 0.1W documents are unchanged.

The controller's input for this stage is `EXISTING_PROVIDER=SELECTEL`, `EXISTING_SERVERS=3`. A fourth server is not authorized. Another bucket, region, or account inside Selectel is not treated as proof of an independent failure domain.

## S3 Vault

Source: https://selectel.ru/solutions/s3-vault/

S3 Vault is described as scheduled reservation of S3 buckets onto a protected Selectel S3. It can copy from another provider's S3 or from local infrastructure. The published tasks are backup of a primary object store, protection from loss and ransomware, long-term archive, and migration.

What the page documents:

- A separate service account. Only Selectel engineers sign in to maintain it. The customer has no login. Temporary read access to the buckets, without signing into the account, is given after a written request on company letterhead or through electronic document exchange, and after a confirmation call to the named contact. The customer sets the duration.
- The customer supplies the source bucket list, the sync frequency, whether versioning is required, and the retention depth. Selectel configures the copy jobs.
- Copying follows that schedule. A maximum actual lag is not stated.
- Integrity of a copy compares ETag for small files and size plus block hashes for multipart objects. A failed check is retried. There is no signed receipt over `(namespace, incarnation, seq, digest)`.
- Deletion of buckets, objects, and backup jobs is performed only on the customer's official request.
- The service is attached by an additional agreement. Invoices go to the main account. The bill covers infrastructure rental and S3 storage. Traffic and S3 requests are outside that storage price.
- Deployment takes up to five business days. Network connectivity is scheduled separately. The channel is described as up to 10 Gbit/s.

`S3_VAULT_SYNC_ANCHOR=NOT_VERIFIED`. The page does not document a synchronous acknowledgement of one record before the caller continues. S3 Vault is a protected backup of buckets.

## S3 Object Lock

Source: https://docs.selectel.ru/s3/buckets/object-lock/

Object Lock is WORM on object versions. The bucket must have versioning. Enabling Object Lock cannot be undone, and versioning cannot be suspended afterwards. Enabling the feature does not lock objects that are already stored.

| Mode | Who can change it | Who can delete the locked version |
| --- | --- | --- |
| Indefinite legal hold | It can be turned off | Nobody while it is on |
| Governance | The retention can be shortened, extended, or changed to Compliance. The footnote allows the account owner, a `member`, or another S3 role whose bucket policy allows `s3:BypassGovernanceRetention` | Those same users, before the retention date |
| Compliance | The object retention can only be extended | Nobody until the retention date |

The control panel limits a Compliance retention to one year. A longer Compliance retention is set through the API. Governance can be set up to 100 years. A default retention applies to new objects only.

If a project that contains locked objects is deleted, the objects are not deleted while the lock is active. They disappear from the panel and from the API. Restoring access requires a support ticket.

Conditional writes are a separate page, last changed 2026-11-12: https://docs.selectel.ru/s3/objects/conditional-requests/

`PutObject` accepts `If-Match` and `If-None-Match`. `If-Match` writes when the active version's ETag matches. `If-None-Match` writes when the key has no active version, including no delete marker. A failed condition returns `412 Precondition Failed`. The predicate is the object's ETag or its absence. It is not `seq = current_sequence + 1`, and it is not a predecessor digest of the anchor contract. A series of object keys is not a linearizable exact successor.

## Managed PostgreSQL

Backup source, last changed 2026-05-25: https://docs.selectel.ru/managed-databases/postgresql/backups/

Backups are automatic and cannot be disabled or downloaded. WAL-G writes a full backup weekly, a delta daily, and WAL every 10 minutes or when a volume of changes accumulates. Backups are kept for 7 days and are stored apart from other customers' backups. A deleted cluster's backups remain for 7 days from the backup's creation. Restore builds a new cluster at the latest backup or at any point in those 7 days. The new cluster can change name, size upward, node count, network, and database settings. Location, database type, and version stay the same. Node addresses change.

Roles, https://docs.selectel.ru/managed-databases/postgresql/configure-roles-and-privileges/ : custom roles cannot be created, and the superuser role is not granted. The preset roles are `dbaas_admin`, `dbaas_replication`, `pg_read_all_stats`, and `pg_signal_backend`. The product page states that superuser actions, if required, go through a support ticket: https://selectel.ru/services/cloud/managed-databases/postgresql/

The panel user who owns the cluster can create users, restore a cluster, and delete a cluster. Absence of superuser does not remove that restore path. ACID of a customer transaction is not an independent anchor.

## Contract terms that apply across the products

The public offer of АО «Селектел», version of 2026-11-12, https://selectel.ru/about/documents/user-agreement/ :

- The provider may suspend services on an order from a competent state authority.
- On termination of the agreement, or on non-payment, the provider may delete the information the customer processed on the provider's equipment and services.

Those clauses are not a technical lock. They are the contractual path that sits above a bucket and a database cluster.
