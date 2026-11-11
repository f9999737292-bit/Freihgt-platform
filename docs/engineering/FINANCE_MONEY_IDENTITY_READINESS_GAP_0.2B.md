# FINANCE MONEY IDENTITY 0.2B — readiness gap

Agent G contract review of one synthetic shipment. This document does not change billing handlers, VAT calculation, migrations, tariffs, payment code, or staging.

`COMMERCIAL_PILOT_READY=NO`

`FINAL_VERDICT=MONEY_IDENTITY_ASSESSED/VAT_POLICY_BLOCKED`

## Overlap with PR #341

PR #341 remains the money-identity proof. It is a draft, open, and unmerged.

| Field | Value |
| --- | --- |
| URL | https://github.com/f9999737292-bit/Freihgt-platform/pull/341 |
| Head | `5444800f2b9e1df8a2bdedf33ca28c15eebeedb8` |
| Base at review | `main` |
| Merge state | `mergeable=true`, `mergeable_state=behind` |
| Latest `origin/main` | `3f306fc407aeda4bdc652d825954292f4a54fad6` |

Files owned by #341:

- `docs/engineering/FINANCE_PILOT_MONEY_IDENTITY_0.2B.md`
- `services/billing-register-service/internal/domain/money_identity_v02b_test.go`
- `services/billing-register-service/internal/integration/freightbillingclosing/helpers_test.go`
- `services/billing-register-service/internal/integration/freightbillingclosing/money_identity_v02b_integration_test.go`
- `tests/bintrans-e2e-transport-flow/financial_closing.go`

This review adds only `docs/engineering/FINANCE_MONEY_IDENTITY_READINESS_GAP_0.2B.md`. That path is absent from #341 and from `origin/main`. No file in #341 is edited here. #341 was not rebased.

`git diff --stat 1902a2b4a77e9f2dcf823dae34f85328e8903117..origin/main` is empty for `services/billing-register-service`, `services/transport-order-service`, `tests/bintrans-e2e-transport-flow`, and `infrastructure/migrations`. The billing code assessed by #341 is the billing code on current `main`.

The commit-status API for `5444800f` returned `state=pending` and an empty status list. The check-run list timed out during this review. CI on #341 is not a pass.

## Chain

The assessed chain is one tenant, one shipment, one transport order, one `SNAPSHOT_V1` snapshot, one currency, one settlement, one register line, one act, and one VAT invoice.

| Step | Stored source |
| --- | --- |
| Snapshot | `transport.transport_order_rate_snapshots.total_amount`, `currency_code`, `pricing_source` |
| Settlement | `billing.freight_settlements.base_freight_amount`, `rate_snapshot_id`, `pricing_source`, `currency_code` |
| Register line | `billing.billing_register_items.base_amount` |
| Register | `billing.billing_registers.total_without_vat`, `total_with_vat`, `currency_code` |
| Act | `billing.acts.total_amount`, `currency_code` |
| VAT invoice | `billing.vat_invoices.amount_without_vat`, `amount_with_vat` |

`SNAPSHOT_V1` settlement reads `total_amount::text` and stores `StringFixed(2)`. It does not read `rfx.rfx_award_transport_orders.amount` and it does not call contract-rate resolution. The proof fixture kept the award amount at `100000` while the snapshot amount was `1234.56`.

The act table has no net column. Its insert copies register `TotalWithVAT`. The VAT invoice insert copies register net, VAT, and gross fields. The VAT invoice table has no currency column.

This review did not repeat the database execution. The execution recorded on #341, against unchanged billing code, is the confirmation:

| Object | ID |
| --- | --- |
| Snapshot | `b1c0ffc6-56b0-48fe-bea0-e6ca2c501b2c` |
| Settlement | `309e9444-e930-42aa-b904-b3f6819e1016` |
| Register | `e97bb29b-345b-4b7a-9c30-10e2f6670c10` |
| Register item | `b518f94e-527a-44f0-bd06-55721d5a8afa` |
| Act | `4aa6de4e-664e-4164-be3c-edc1bd398773` |
| VAT invoice | `20400da8-f08f-4bb4-a76f-4591d00aaf7e` |

Command: `go test -tags integration -count=1 -timeout 300s -v -run TestMoneyIdentitySnapshotToClosingDocuments ./internal/integration/freightbillingclosing/`

Exit code `0`.

## Amount `1234.56 RUB`

Stored texts compared with `decimal.Decimal`, not `float64` equality:

- snapshot `total_amount`
- settlement `base_freight_amount`
- register item `base_amount`
- register `total_without_vat`
- VAT invoice `amount_without_vat`

All equal `1234.56`. Currency `RUB` on the snapshot, settlement, register, and act. Extras, penalties, and approved accessorials are `0.00`.

Act `total_amount` equals register `total_with_vat`. In that run the gross texts are also `1234.56` because `vat_amount` is `0.00` and `vat_rate` is null. That equality is an observation. It is not evidence that the snapshot is gross.

`SNAPSHOT_AMOUNT_BASIS=UNKNOWN`

`SNAPSHOT_PRICING_SOURCE=RFQ_AWARD`

`pricing_model_version=SNAPSHOT_V1`

## Float64 rounding, `1.255`

`services/billing-register-service/internal/domain/billing_register_item.go` `round2` is `math.Round(value*100) / 100`.

`TestLegacyFloatRound2DivergesOnThreeDecimalHalf` records:

| Input text | `decimal.Round(2)` | `round2` after `decimal.Float64()` |
| --- | --- | --- |
| `1.255` | `1.26` | `1.25` |

`NUMERIC(18,2)` cannot store `1.255`. The snapshot proof amount `1234.56` survives the same float path, together with `0.01`, `0.10`, `10.01`, `8.10`, `0.29`, and `100000.10`.

Impact: a value that is not already on a two-decimal boundary can lose one kopeck before it is written, if it enters `CalculateItemAmounts` or `CalculateSettlementTotals` as `float64`. The snapshot insert path writes `StringFixed(2)` before storage. This review does not change the helper. A later remediation has to be a separate change with its own acceptance test.

## VAT options

No option below is selected. A missing `vat_rate` is not a zero rate and not a confirmed exemption. Current runtime sets `VATAmount=0` when the rate pointer is nil.

`OBSERVED_RUNTIME_VAT_ZERO=YES`

`VAT_POLICY_CONFIRMED=NO`

`VAT_POLICY_UNRESOLVED=YES`

`COMMERCIAL_VAT_GATE=BLOCKED`

| Option | Owner statement required before use | What the current code would do if this option were later adopted |
| --- | --- | --- |
| N, snapshot is net | Tax category, applicable rate, and the rounding rule for tax and the gross total | A nil rate still stores VAT `0` and copies the snapshot into both net and gross. A future rate would be added on top of `total_amount`. |
| G, snapshot is gross | Tax category, applicable rate, and a rule for extracting VAT from `total_amount` | The loader does not extract VAT. A non-nil rate is added on top. Gross-inclusive interpretation is not implemented. |
| Z, VAT does not apply | A documented legal basis for non-application, distinct from an empty column | The snapshot table has no VAT column. Stored `vat_rate` null is the current absence of a rate, not an explicit exemption marker. |
| U, leave unresolved | None beyond keeping the commercial gate closed | Principal identity can stay assessed. Commercial finance stays blocked. |

The same owner decision has to name the rounding rule. Keeping today's `round2` is one possible engineering consequence of that decision. It is not itself the tax policy.

## Authority

### Rate snapshot

The money-identity test inserts the snapshot row. It does not call `CreatePricedTransportOrder`. Snapshot creation authority is therefore outside the `1234.56` proof.

On current `main`:

- `POST /v1/transport-orders` takes the tenant from the verified tenant header, rejects a different body tenant, and reads the actor from `X-User-ID`, `X-Company-ID`, and `X-Actor-Kind`.
- `ValidateCreatePricedTransportOrderInput` requires those actor fields and an idempotency key. It does not query company membership.
- Award-scope creation is `POST /internal/v1/transport-orders/from-award-scope`, behind the internal service token. It forces actor kind `BUYER` and then uses the same priced-create path. Contract-rate resolution receives the forwarded actor kind.
- Snapshot read is `GET /internal/v1/transport-orders/{transportOrderId}/rate-snapshot`, behind the internal token, scoped by the verified tenant. A missing `SNAPSHOT_V1` order returns a conflict, not another tenant's row.
- `transport.transport_order_rate_snapshots` rejects update and delete through `transport.deny_transport_order_rate_snapshot_mutation`.

Open authority point: transport-order public create does not itself prove that the company header belongs to the user. That check is not established by the money-identity test.

### Closing documents

`POST /v1/billing-registers/{id}/closing-document-package` resolves the actor through `ResolveTrustedSettlementActor` and membership. `EnforceOptionalBodyTenant` rejects a mismatched body tenant. `CreatePackageForActor` replaces the input tenant with the actor tenant. `ValidateRegisterBuyerMutation` allows the buyer that owns the register customer company.

Invoice, act, VAT invoice, and UPD creation overwrite seller with `reg.ContractorCompanyID` and buyer with `reg.CustomerCompanyID`. The #341 run observed those parties on the act and VAT invoice.

The same run denied a foreign tenant read, a foreign company include, and a spoofed actor tenant. Those denials added no register item and did not change register version, register audit count, or settlement outbox count. Repeating settlement create, include, and package kept one row of each.

## Acceptance criteria

These criteria describe the proof that already passed and the checks still required. They do not authorize a VAT implementation.

### Regression, no database

| ID | Pass condition |
| --- | --- |
| AC-R1 | For `1234.56`, `0.01`, `0.10`, `10.01`, `8.10`, `0.29`, and `100000.10`, `decimal` text after `round2(float64)` equals the input text at two places. |
| AC-R2 | Text `1.255` rounds to `1.26` with `decimal.Round(2)` and to `1.25` with `round2`. The test stays red if those two results become equal without a separate remediation note. |
| AC-R3 | `CalculateSettlementTotalsDecimal("1234.56", 0, nil)` returns net `1234.56`, VAT `0`, gross `1234.56`. The assertion records the nil-rate observation. |

### Integration, one isolated database

| ID | Pass condition |
| --- | --- |
| AC-I1 | One shipment, one order with `pricing_model_version=SNAPSHOT_V1`, one snapshot whose `total_amount` text is `1234.56` and whose currency is `RUB`. The award amount text is different. |
| AC-I2 | After `DRAFT → UNDER_REVIEW → APPROVED`, settlement `base_freight_amount` text equals the snapshot total, `rate_snapshot_id` matches, currency matches, `vat_rate` is null, and `vat_amount` text is `0.00`. |
| AC-I3 | One register item. Its `base_amount` text equals the settlement base. Extra charges and penalties are `0.00`. |
| AC-I4 | Register `total_without_vat` text equals the item base. Currency is `RUB`. Status becomes `CLOSING_DOCUMENTS_CREATED` only after the package. Status is not `PAID` or `CLOSED`. |
| AC-I5 | Act `total_amount` text equals register `total_with_vat`. VAT invoice `amount_without_vat` text equals register `total_without_vat`. VAT invoice `amount_with_vat` text equals register `total_with_vat`. Invoice `vat_rate` is null. |
| AC-I6 | Act and VAT invoice seller equal the register contractor. Buyer equals the register customer. |
| AC-I7 | Foreign tenant, foreign company, and spoofed actor tenant are denied. The denial creates no financial row and does not change version, audit, or outbox. |
| AC-I8 | Repeating settlement create, register include, and closing package leaves one settlement, one item, one act, and one VAT invoice. |
| AC-I9 | `billing.payment_obligations` for the tenant stays empty. The test does not call mark-paid, EDO, bank, or the freight-cost consumer. |

### Authority still outside the amount proof

| ID | Pass condition |
| --- | --- |
| AC-A1 | Closing-package HTTP denies a buyer of another company and denies a carrier mutation. |
| AC-A2 | A body tenant different from the trusted tenant is forbidden and creates no package. |
| AC-A3 | Internal snapshot read requires the internal token and the verified tenant. Another tenant does not receive the row. |
| AC-A4 | Update and delete of `transport.transport_order_rate_snapshots` raise the immutability exception. |
| AC-A5 | Public priced create rejects a mismatched body tenant before a snapshot is written. A company header without membership remains an open check until a test shows the write is refused. |

### Commercial gate

| ID | Pass condition |
| --- | --- |
| AC-C1 | `COMMERCIAL_PILOT_READY` stays `NO` until the owner records the tax category, the rate or the documented basis for non-application, the choice among options N, G, and Z, and the rounding rule. An empty `vat_rate` does not satisfy this criterion. |

## Engineering questions and customer decisions

Engineering, already evidenced or still testable without a tax ruling:

- The principal text `1234.56` is preserved on the snapshot, settlement, register line, register net, and VAT invoice net.
- Currency identity holds for the columns that exist. The VAT invoice has no currency column.
- The act stores the register gross total.
- Nil rate produces stored VAT `0.00`.
- `round2` changes `1.255` to `1.25`.
- Closing-document mutations use the trusted buyer and the register parties.
- Snapshot immutability is a database trigger. Public snapshot create does not itself prove company membership.

Customer legal and accounting decisions, not selected here:

- Tax category of this freight operation.
- Applicable VAT rate, or the document that states VAT does not apply.
- Whether `total_amount` on the snapshot is net, gross, or neither until a later document.
- The rounding rule the accountant requires for tax and for the document total.
- Whether an act that stores only a gross total is acceptable for the customer's document form.
- Whether a null rate may appear on a commercial VAT invoice. Current storage does that when the rate is absent. That behavior is not an acceptance of the document.

`PAYMENT_EXECUTED=NO`

`BANK_CONNECTED=NO`

`EDO_OPERATOR_CONNECTED=NO`

`LIVE_COST_CONSUMER_STARTED=NO`

`MIGRATIONS_CREATED=NO`

`PRODUCT_BEHAVIOR_CHANGED=NO`

`STAGING_CHANGED=NO`

`PRODUCTION_CHANGED=NO`
