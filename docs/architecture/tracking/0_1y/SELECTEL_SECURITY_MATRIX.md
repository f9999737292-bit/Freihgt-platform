# Selectel security matrix 0.1Y

Read 2026-11-18. Values are `DOCUMENTED`, `PARTIAL`, `NOT_VERIFIED`, or `UNSUPPORTED`. A cell describes the published product against the 0.1T requirement. It is not a provider answer.

## S3 Vault

| ID | Requirement | Value | Source |
| --- | --- | --- | --- |
| A1 | Independent failure domain | `NOT_VERIFIED` | The vault is a separate service account at the same legal provider. The page does not place it in a failure domain that survives a Selectel-wide incident. https://selectel.ru/solutions/s3-vault/ |
| A2 | Independent administration | `PARTIAL` | The customer has no login. Selectel engineers do. The customer can still order deletion of buckets, objects, and jobs. |
| A3 | Atomic exact-successor | `UNSUPPORTED` | The service copies buckets. It has no sequence API. |
| A4 | Rollback-resistant history | `PARTIAL` | The page says data are protected from accidental deletion and ransomware. Official deletion by request is also documented. |
| A5 | Synchronous durable ACK | `NOT_VERIFIED` | Copying is scheduled. No per-record synchronous acknowledgement is described. |
| A6 | Namespace, incarnation, seq, digest | `UNSUPPORTED` | The copied unit is an object in a bucket, not the anchor tuple. |
| A7 | Predecessor check | `UNSUPPORTED` | The integrity check compares ETag, size, and block hashes of a file copy. |
| A8 | Fail-closed | `UNSUPPORTED` | The copy is off the issuer's commit path. A missed copy does not reject a local commit. |
| A9 | Recovery of acknowledged history | `PARTIAL` | Copied objects can be read after an official request. The maximum lag of the schedule is not published, so the latest acknowledgement may be absent. |
| A10 | BINTRANS owner cannot quietly delete or roll back | `PARTIAL` | There is no direct login. Deletion is available through an official request, which is a deliberate path, not a technical prohibition. |

## S3 Object Lock

| ID | Requirement | Value | Source |
| --- | --- | --- | --- |
| A1 | Independent failure domain | `NOT_VERIFIED` | The bucket remains a Selectel S3 resource of the customer account. https://docs.selectel.ru/s3/buckets/object-lock/ |
| A2 | Independent administration | `UNSUPPORTED` | The account owner and a `member` can bypass Governance. Compliance limits the customer until the date, and the provider's support path restores access after a project is deleted. |
| A3 | Atomic exact-successor | `UNSUPPORTED` | `If-Match` compares an ETag. `If-None-Match` checks that the key has no active version. Neither is `current_sequence + 1`. https://docs.selectel.ru/s3/objects/conditional-requests/ |
| A4 | Rollback-resistant history | `PARTIAL` | A Compliance-locked version cannot be deleted by anyone until the date. A new version of the same key, and a delete marker on an unlocked current version, are not forbidden by that rule. |
| A5 | Synchronous durable ACK | `NOT_VERIFIED` | `PutObject` returns a result for that object. The page does not define it as a durable anchor acknowledgement that must precede PostgreSQL `BEGIN`. |
| A6 | Namespace, incarnation, seq, digest | `UNSUPPORTED` | No such record type is published. |
| A7 | Predecessor check | `PARTIAL` | The predecessor that is checked is an ETag or the absence of a key, not the digest pinned at `seq - 1`. |
| A8 | Fail-closed | `NOT_VERIFIED` | A failed condition returns 412 for that request. The pages do not say the issuer must stop. |
| A9 | Recovery of acknowledged history | `PARTIAL` | A locked version is kept through project deletion, then hidden until a ticket. |
| A10 | BINTRANS owner cannot quietly delete or roll back | `PARTIAL` | Compliance says nobody deletes the locked version before the date. Legal hold can be turned off. Governance can be bypassed by the account owner. The public offer allows deletion of customer information when the agreement ends or a bill is unpaid. https://selectel.ru/about/documents/user-agreement/ |

## Managed PostgreSQL

| ID | Requirement | Value | Source |
| --- | --- | --- | --- |
| A1 | Independent failure domain | `NOT_VERIFIED` | The cluster is another Selectel service. Isolation of backup files from other customers is not isolation from Selectel. https://docs.selectel.ru/managed-databases/postgresql/backups/ |
| A2 | Independent administration | `PARTIAL` | Superuser is not granted. The panel owner creates users, deletes the cluster, and restores it. Superuser work is requested by ticket. https://docs.selectel.ru/managed-databases/postgresql/configure-roles-and-privileges/ |
| A3 | Atomic exact-successor | `POSSIBLE` | A customer can implement an atomic conditional sequence write in transactional SQL. Selectel does not publish a recovery-anchor API that performs that check. `MANAGED_POSTGRES_SQL_CAS=POSSIBLE`. `MANAGED_POSTGRES_SERVICE_CAS=CUSTOMER_SQL_AVAILABLE`. |
| A4 | Rollback-resistant history | `NOT_PROVEN` | ACID and `COMMIT` acknowledge one transaction. They do not keep that history when the panel owner restores the cluster to an earlier point in the 7-day window. `ROLLBACK_RESISTANT_ANCHOR=NOT_PROVEN`. |
| A5 | Synchronous durable ACK | `PARTIAL` | `COMMIT` is the acknowledgement of that transaction. A later point-in-time restore can replace the committed history. |
| A6 | Namespace, incarnation, seq, digest | `UNSUPPORTED` | Not a published record type. |
| A7 | Predecessor check | `UNSUPPORTED` | Not a published anchor operation. |
| A8 | Fail-closed | `NOT_VERIFIED` | Not described for an issuer gate. |
| A9 | Recovery of acknowledged history | `PARTIAL` | Recovery is a new cluster from at most 7 days of WAL. It is a restore to a chosen earlier time, not a proof that the later acknowledgement survived. |
| A10 | BINTRANS owner cannot quietly delete or roll back | `UNSUPPORTED` | The panel restore and cluster delete are customer operations. |

## Reading

```text
MANAGED_POSTGRES_SQL_CAS=POSSIBLE
MANAGED_POSTGRES_SERVICE_CAS=CUSTOMER_SQL_AVAILABLE
MANAGED_POSTGRES_ROLLBACK_RESISTANCE=NOT_PROVEN
MANAGED_POSTGRES_AS_INDEPENDENT_ANCHOR=NOT_SUITABLE_FOR_ANCHOR
```

The SQL compare-and-set is customer SQL inside the same cluster. It is not a documented Selectel recovery-anchor service, and the 7-day restore path means the committed history is not shown to survive an administrative rollback. Object Lock Compliance remains the strongest published restriction on deleting one object version, and it does not implement the anchor contract.
