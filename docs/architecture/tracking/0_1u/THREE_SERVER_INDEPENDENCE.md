# 0.1U supplement: three existing servers

Confirmed inputs for this check:

```text
EXISTING_SERVERS=3
PROVIDER_COUNT=1
PROVIDER_NAME=NOT_VERIFIED
INDEPENDENT_FAILURE_DOMAINS=NOT_PROVEN
FOURTH_SERVER_PURCHASE=NOT_AUTHORIZED
```

No server was contacted. No panel was opened. No secret value was read. No resource was created.

A capture note names a vendor for `161.104.53.221` only. That name is not promoted. The confirmed provider property of the set is count `1` and name `NOT_VERIFIED`.

## 1. The three servers

| ID | Address | Role in operator inventory | Placement | Storage | Backup | Administration | Restore procedure |
| --- | --- | --- | --- | --- | --- | --- | --- |
| S1 | `161.104.53.221` | Old shared VPS `gpt-docker`. The dedicated-staging runbook tells operators not to use it as the current staging host. Local SSH alias `7rights-staging`. | Site, zone, and hypervisor `NOT_VERIFIED`. | An early capture records 500 GB SSD for this address, before Docker was installed. Current disk `NOT_VERIFIED`. | That capture records a daily backup, retention 7 days, for this address only. Whether it still runs, and whether it is a separate system from S2 and S3: `NOT_VERIFIED`. | `User root`. Key file name `private3.pem`. | `NOT_VERIFIED` as a current procedure. |
| S2 | `161.104.57.152` | Current control-tower staging, hostname `bintrans-control-tower-staging`. Alias `bintrans-ct-staging`. | Site, zone, and hypervisor `NOT_VERIFIED`. | Compose volumes on that host. Volume product `NOT_VERIFIED`. | `pg_dump` under `/protected/bintrans/backups` (override `BINTRANS_BACKUP_DIR`). A deploy-snapshot path is also recorded. Both are operator paths, not a second provider. | `User root`. Key file name `bintrans_selectel_staging`. | Operator runs the staging backup script and sets `BACKUP_VERIFIED`. |
| S3 | `161.104.49.146` | E2E host `bintrans-test-2-e-2`. Prior operator specification: Ubuntu 24.04.4 LTS, created from the staging image. Alias `bintrans-e2e`. | Site, zone, and hypervisor `NOT_VERIFIED`. | Disk product `NOT_VERIFIED`. Image lineage is the staging image, so the initial contents are a copy of staging, not a separate history. | Backup product and schedule `NOT_VERIFIED`. | `User root`. Key file name `bintrans_e2e_ed25519_v2`. | The recorded creation path is an image of staging. A later restore of that image is the same operator's action. |

The three `IdentityFile` names differ. All three `Host` entries are on one workstation and all three log in as `root`. Different key files are not different administrators.

## 2. Independence

| Property | Result |
| --- | --- |
| Physical site | `NOT_VERIFIED`. Three addresses in `161.104.0.0/16` do not prove three sites or one site. |
| Storage system | `NOT_VERIFIED` as separate products. S3 starts from a staging image. |
| Backup system | `NOT_VERIFIED` as independent. S2's dump path is an operator directory. S1's 7-day backup is a single-host capture, not a proof about S3. |
| Administrative authority | Shared. One workstation holds root login material for all three. |
| Restore procedure | Shared operator. S3 exists because staging was imaged. That procedure can recreate a host; it does not protect a later sequence from the same operator. |
| Failure domain | `NOT_PROVEN`, matching the confirmed input. |
| Provider | Count `1`. Name `NOT_VERIFIED`. |

S3 does not meet the anchor requirements. It has no synchronous acknowledgement of `(namespace, incarnation, seq, digest)`, no proven monotonic log, and no administrator who cannot also restore S1 and S2. A rollback that includes S3 is Scenario B.

## 3. Two options

| | A. S3 as the recovery anchor | B. Managed anchor at a second provider, no fourth server |
| --- | --- | --- |
| Independence required | Separate site, storage, backup, admin, and restore | Same properties, without a BINTRANS virtual machine |
| What is confirmed | S3 is the third server of the single provider, created from the staging image, with root held on the same workstation | No second provider, no product, no API, and no price is in the inventory |
| Scenario A | `NOT_SUITABLE`. A restore available to this operator can include S3. | `NOT_VERIFIED`. Possible only if that provider's administrators are not BINTRANS operators. |
| Scenario B | Expected while admin and provider stay shared. | Still open if BINTRANS can rewrite or restore the log. The class does not close it. |
| Confirmed hardware | Prior OS string only. CPU, RAM, and disk of S3 are `NOT_VERIFIED`. | No service specification. |
| Confirmed price | None. | None. |
| Fourth server | Not used. | `NOT_AUTHORIZED`, and not required by this option. |
| Label | `NOT_SUITABLE` | `PENDING` |

## 4. Recommendation

Do not use S3 as the anchor. Its independence is not proven on placement, storage, backup, or restore, and its administration is the same root custody as S1 and S2.

The remaining class is a managed append-only log at a provider other than the one that already holds these three servers. No such service is identified, so there is no confirmed specification and no confirmed price. Until those facts exist, the decision stays pending. Renting a fourth server is not authorized and would not by itself create a second provider.

```text
OPTION_A_THIRD_SERVER=NOT_SUITABLE
OPTION_B_MANAGED_SECOND_PROVIDER=PENDING
INFRASTRUCTURE_DECISION=PENDING
P3B_ALL_STORES_ROLLBACK=NOT_FIXED
ANCHOR_FAILURE_DOMAIN=NOT_PROVEN
IMPLEMENTATION_GATE=CLOSED
```
