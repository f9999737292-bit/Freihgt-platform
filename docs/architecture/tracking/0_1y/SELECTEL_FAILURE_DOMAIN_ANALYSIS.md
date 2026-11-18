# Selectel failure-domain analysis 0.1Y

Read 2026-11-18. One legal provider is not, by itself, declared to be one failure domain. Separate accounts are not, by themselves, declared to be independence.

## What is separated, and what is shared

| Layer | What the pages show |
| --- | --- |
| Legal provider | АО «Селектел» is the counterparty of the public offer. S3 Vault is an additional agreement billed to the main account. |
| Physical sites | The group's data centers are described in Saint Petersburg, Moscow, and Leningrad region. The S3 Vault page does not name a site that is disjoint from the sites of the three existing servers. |
| Storage systems | S3 Vault stores object copies. Managed PostgreSQL stores a database and WAL. These are different products. |
| Backup domains | Vault copies run on a schedule the customer chooses. Managed PostgreSQL keeps 7 days of WAL-G backups, isolated from other customers' backup files. Neither page says the backup survives an action by the provider's control plane. |
| Administrative accounts | The vault service account has no customer login. The database cluster is administered from the customer panel. These are different logins. |
| Provider control plane | Selectel engineers maintain the vault account. A support ticket restores API access to locked objects after a project deletion. A ticket is also the path for superuser work on Managed PostgreSQL. The public offer lets the provider suspend service on a state order and delete customer information when the agreement ends or a bill is unpaid. |
| Shared accident or bad restore | Not disproven. A scheduled vault can be restored from the same operator that restores the servers. Managed PostgreSQL point-in-time recovery is itself a customer-selected earlier point. |

`SELECTEL_FAILURE_DOMAIN=SAME_LEGAL_PROVIDER_INDEPENDENCE_NOT_PROVEN`

`SELECTEL_ADMIN_DOMAIN=SERVICE_ACCOUNT_SEPARATES_LOGIN_NOT_THE_PROVIDER`

`SELECTEL_BACKUP_DOMAIN=SCHEDULED_COPY_AND_7_DAY_PITR_NOT_AN_INDEPENDENT_HISTORY`

## Scenario A

The three BINTRANS servers and their local histories are restored to an older point. The Selectel copy still holds a later digest.

The documented way to see that copy is an official read request: company letterhead or electronic document exchange, a confirmation call, then temporary read-only access to the buckets. There is no published online comparison, no signed sequence receipt, and no stated bound on how far the schedule may lag. If the later record was acknowledged after the last successful copy, the vault does not contain it.

`P3B_SCENARIO_A=MANUAL_READ_AFTER_OFFICIAL_REQUEST_NOT_AN_ONLINE_GATE`

That manual comparison is not the 0.1T gate. The 0.1S result remains: when the witness journal, the high-water store, and PostgreSQL were restored together, a second ticket was issued. A vault that is not on the commit path does not close that gate.

## Scenario B

An operator, or a shared administrator, restores or deletes both the local history and the anchor.

Published paths that allow this, rather than block it:

- The customer orders deletion of vault buckets, objects, and backup jobs.
- Selectel engineers already operate the vault account.
- The customer restores a Managed PostgreSQL cluster to an earlier point in the 7-day window, or deletes the cluster.
- The account owner bypasses Governance retention, or turns a legal hold off.
- The public offer allows suspension on a state order and deletion of customer information on termination or non-payment.
- Project deletion hides locked objects until a ticket. The ticket is a provider action, so the provider remains able to change whether the history is visible.

No published control says that a Selectel administrator who can act on the three servers cannot also act on the vault.

`P3B_SCENARIO_B=NOT_BLOCKED_BY_A_DOCUMENTED_CONTROL`

`P3B_ALL_STORES_ROLLBACK=NOT_FIXED`

## Second provider

A service account inside Selectel separates a login. It does not satisfy the 0.1T rule that the witness key and the anchor key are held in different administrative domains, because both domains are operated by the same provider. A second provider is still required for that separation. No second provider is selected, and no fourth server is authorized.
