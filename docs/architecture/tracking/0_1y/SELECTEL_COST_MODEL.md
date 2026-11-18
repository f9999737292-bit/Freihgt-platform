# Selectel cost model 0.1Y

Prices read 2026-11-18 from https://selectel.ru/prices/ . The page states that the listed prices include VAT at 22%. No VAT is added again here. No calculator was submitted, and no quote was requested.

## Prices that rendered

| Item | Published price, VAT included | Also published |
| --- | --- | --- |
| S3 Vault storage | 1.43 ₽ per GB per month | 0.0000019 ₽ per MB per hour |
| S3 Vault egress | 1.96 ₽ per GB | 0.0019 ₽ per MB |

The S3 Vault product page states that infrastructure rental is billed separately, and that S3 traffic and S3 requests are not included in the storage price. The prices page did not render a request price, an infrastructure price, a minimum fee, or a replication surcharge. The ordinary S3 section and the Managed PostgreSQL section were still marked as refreshing, so no unit price from those sections is used.

A promotional line on the product page offers new customers 1.14 ₽ per GB instead of 1.43 ₽ per GB, fixed for one year, after an application. That offer is not the list price and is not used below.

Managed PostgreSQL is pay-as-you-go by vCPU, RAM, disk, and node count. The cloud platform includes 3 TB of external traffic per account per month, then charges further traffic. Those statements do not produce a number for an anchor. https://docs.selectel.ru/managed-databases/about/payment/

## What cannot be computed

```text
REQUEST_PRICE=NOT_PUBLISHED
VAULT_INFRASTRUCTURE_PRICE=NOT_PUBLISHED
MINIMUM_FEE=NOT_PUBLISHED
REPLICATION_SURCHARGE=NOT_PUBLISHED
MANAGED_POSTGRESQL_CONFIGURATION_PRICE=NOT_PUBLISHED
GB_DEFINITION=NOT_STATED
FULL_COST=NOT_VERIFIED
```

The page prints both an hourly MB rate and a monthly GB rate. This note does not reconcile them into a third rate.

## Storage of new bytes only

This block is not a system price. It prices only the published S3 Vault storage line, for new bytes in one month, with no retention carry-over, no requests, no egress, no infrastructure, and no replication.

Assumption, carried from the earlier confirmation-size model and not measured at Selectel: one reservation is at most 1 KiB, which is 1024 bytes.

| Reservations in the month | New bytes | If 1 GB is 10^9 bytes, storage at 1.43 ₽/GB | If 1 GB is 2^30 bytes, storage at 1.43 ₽/GB |
| --- | --- | --- | --- |
| 10 000 | 10 240 000 | 0.015 ₽ | 0.014 ₽ |
| 100 000 | 102 400 000 | 0.146 ₽ | 0.136 ₽ |
| 1 000 000 | 1 024 000 000 | 1.464 ₽ | 1.364 ₽ |

The two GB readings are both shown because the price page does not define GB. Keeping every past month multiplies the stored volume. The retention depth is chosen by the customer and is not a single published term, so the cumulative storage price is not a single number.

Egress of those same bytes, if they were transferred once at the published 1.96 ₽/GB, would be a few rubles at one million 1 KiB records. A recovery that transfers the retained history, or a channel rented for the vault, is not that figure. Infrastructure rental is unknown and can dominate.

`PRICING_VERIFIED=PARTIAL`

`FULL_COST=NOT_VERIFIED`
