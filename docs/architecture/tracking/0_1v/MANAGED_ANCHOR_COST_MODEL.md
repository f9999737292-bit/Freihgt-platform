# 0.1V: Cost model

Checked 2026-11-17. Currency is USD. A missing figure is `PRICE_NOT_VERIFIED`. No calculator was signed into, and no quote was requested.

## Assumptions used wherever a product has a published unit price

- One confirmation is one read of the current record and one write of the next record.
- The record is at most 1 KiB, counted as 1024 bytes.
- A failed conditional write is not included. Retries would add requests.
- A month used to turn a per-hour storage rate into a month is 730 hours. That factor is an assumption, not a price.
- The record is stored for the whole month at its end-of-month size. That overstates a month that grows from zero. It is an upper bound for that month, not a forecast.
- The issuer is outside the provider, so internet egress applies where an egress rate was published. Same-region traffic inside the provider is a different price and is not used here.
- Free-tier quantities are shown only where the provider page states them. They are not subtracted from the totals below, except DynamoDB storage, where the cited examples put the first 25 GB at no storage charge.
- Backup, audit logs, and a second account are extra and are listed apart from the request totals.

## Published unit prices

| Item | What the page states | Source |
| --- | --- | --- |
| DynamoDB on-demand Standard, US East (N. Virginia), example | $0.625 per million writes of 1 KB. $0.125 per million strongly consistent reads of 1 KB. Storage $0.25 per GB after the first 25 GB in the examples. On-demand backup example $0.10 per GB-month. PITR example $0.20 per GB-month. Inbound transfer $0. Transfer between DynamoDB and other AWS services in the same region $0. | https://aws.amazon.com/dynamodb/pricing/ |
| DynamoDB transactional write | A transactional write consumes 2 write request units per 1 KB. | Same page, pricing rules above the examples. |
| S3 Standard GET, US East (N. Virginia) | $0.0004 per 1,000 GET requests, in the Object Lambda example. | https://aws.amazon.com/s3/pricing/ |
| S3 Standard PUT and S3 Standard storage per GB-month | The regional tables did not appear as text in the fetched pricing page. | `PRICE_NOT_VERIFIED` |
| S3 minimum charge | No minimum charge. | Same pricing page. |
| S3 new-account free tier | 5 GB Standard, 20,000 GET, 2,000 PUT, and 100 GB data transfer out each month for one year. Unused amounts do not roll over. GovCloud excluded. | https://aws.amazon.com/s3/faqs/ |
| S3 conditional requests | No additional charge. Failed conditional requests are billed as normal requests. | https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-requests.html |
| Cloud Storage Standard, one region, flat namespace | Class A $0.005 per 1,000. Class B $0.0004 per 1,000. Storage $0.000027397 per GiB-hour. | https://cloud.google.com/storage/pricing |
| Cloud Storage internet egress | Worldwide destinations excluding Asia and Australia: $0.12 per GiB for the first 10 TiB. Inbound transfer: free. | Same page, general network usage. |
| Cloud Storage Always Free | 5 GB-months Standard, 5,000 Class A, 50,000 Class B, 100 GB egress from North America to each destination except Australia and China. Aggregated across `US-WEST1`, `US-CENTRAL1`, and `US-EAST1`. | Same page. |
| Azure Confidential Ledger | The public pricing page prints `$-/hour per instance` and `$-/GB per month`. It says the instance price includes the first 100 GB across three nodes, and that further storage uses the standard storage price. Regions are limited. The page also still says the service can be tried at no cost during preview. The rendered page has no numeric rate. | https://azure.microsoft.com/en-us/pricing/details/confidential-ledger/ |
| Public Rekor | No price list was found for `rekor.sigstore.dev`. | `PRICE_NOT_VERIFIED` |

A March 2025 Microsoft community post says the reduced compute price is approximately $3 per day per ledger instance, with storage unchanged. That post is not the pricing page. It is not used in the totals.

https://techcommunity.microsoft.com/blog/azureconfidentialcomputingblog/price-reduction-and-upcoming-features-for-azure-confidential-ledger/4387491

## Scenarios

DynamoDB uses one standard write and one strongly consistent read per confirmation. It does not use `TransactWriteItems`. Storage stays under 25 GB at these volumes, so the example storage charge is $0. Internet egress is not given a DynamoDB-specific rate on the page, so it is omitted.

| Confirmations per month | DynamoDB reads and writes | Cloud Storage operations | Cloud Storage end-size storage, 730-hour upper bound | Cloud Storage egress of the reads | S3 GET only |
| --- | --- | --- | --- | --- | --- |
| 10,000 | $0.0075 | $0.054 | $0.000191 | $0.001144 | $0.004 |
| 100,000 | $0.075 | $0.54 | $0.001907 | $0.01144 | $0.04 |
| 1,000,000 | $0.75 | $5.40 | $0.01907 | $0.11444 | $0.40 |

Cloud Storage all-in under those assumptions: $0.0553, $0.553, and $5.533. A precondition implementation that first reads object metadata adds another Class B request. That second read is not in the table.

S3 has no complete total. PUT and GB-month storage are `PRICE_NOT_VERIFIED`. Object Lock itself is not given a separate fee on the pages read.

Azure Confidential Ledger is `PRICE_NOT_VERIFIED` at 10,000, 100,000, and 1,000,000. The page prices an instance hour, not a confirmation.

Public Rekor is `PRICE_NOT_VERIFIED` at all three volumes.

## Other charges

| Charge | C1 | C2 | C3 | C4 | C5 |
| --- | --- | --- | --- | --- | --- |
| Separate account | An Azure subscription. No published fee for the subscription itself on the ledger page. | An AWS account. No published account fee on the DynamoDB page. | Same AWS account question. | A Google Cloud billing account. No separate fee on the storage page. | Not an account BINTRANS creates. |
| Backup | Included replication is described. A customer-controlled backup price is `PRICE_NOT_VERIFIED`. | Example PITR $0.20 per GB-month. Example on-demand backup $0.10 per GB-month. At 1 GiB those examples are about $0.20 and $0.10. | Versioning stores extra versions at the class rate. The class rate did not render. | No separate bucket-lock fee. Each version is charged as a live object when versioning is on. | Not priced. |
| Audit logs | `PRICE_NOT_VERIFIED` | CloudTrail is a different product and was not priced. | Same. | Cloud Audit Logs were not priced. | The log is the audit record. |

`LOWEST_VERIFIED_COST` is the DynamoDB column. At 1,000,000 confirmations the request total is $0.75 before tax, in US East (N. Virginia), on the example rates. That product remains `NOT_SUITABLE`. The lowest priced candidate is not a recommendation.
