# FINANCE-PILOT-MONEY-IDENTITY-0.2B

Agent G evidence for one synthetic shipment and one currency:

`transport.transport_order_rate_snapshots.total_amount` → `billing.freight_settlements.base_freight_amount` → `billing.billing_register_items.base_amount` → `billing.acts.total_amount` and `billing.vat_invoices.amount_without_vat`

No production handler, repository, VAT, tariff, payment, or schema change is included.

## Precheck

| Check | Result |
| --- | --- |
| Worktree | `D:\Projects\freight-platform-finance-money-identity-v0.2b` |
| Branch | `test/finance-pilot-money-identity-v0.2b` |
| Base SHA | `1902a2b4a77e9f2dcf823dae34f85328e8903117` |
| Expected main at start | same SHA |
| `origin/main` at finish | `67d2f394aa838ff259b65c628713cdf87ffb22d4` |
| Commits after the base | PR #338, customer-portal walkthrough docs and portal e2e only |
| Financial files changed on main after the base | none |
| Rebase / reset / force-push | not performed |
| Conflicting 0.2B branch or PR at start | none |
| Migration head | `infrastructure/migrations/000105_platform_postgres_role_foundation_v0_2_i1.up.sql` |
| New migrations or DDL | none |

`docs/engineering/FINANCE_COMMERCIAL_READINESS_DISCOVERY_0.2A.md` is not on this branch. It remains only in the Agent G working tree at `D:\Projects\freight-platform\docs\engineering\FINANCE_COMMERCIAL_READINESS_DISCOVERY_0.2A.md`. SHA256 `69398EF89BC9FCC489D071CE5AC10CB33ACEFB8AFC6D674A113654CF41338C7A`. That file was not modified or deleted.

The isolated billing harness applies migrations through `000056_payment_outbox_aggregate_version_v2.1B.up.sql`. The snapshot tables used here are `000051` and `000052`, inside that window.

## Source of truth

| Stage | Stored field | How this proof reads it |
| --- | --- | --- |
| Rate snapshot | `transport.transport_order_rate_snapshots.total_amount NUMERIC(18,2)`, `currency_code CHAR(3)`, `pricing_source` | `pricing_model_version = SNAPSHOT_V1`. `base_amount` is null because `component_breakdown_status = UNAVAILABLE`. The snapshot table has no `vat_rate` column. |
| Settlement | `billing.freight_settlements.base_freight_amount`, `currency_code`, `vat_rate`, `vat_amount`, `total_without_vat`, `total_with_vat`, `rate_snapshot_id`, `pricing_source`, `approved_accessorial_total` | Existing `FreightSettlementRepository` reads `total_amount::text` and stores `AgreedFreightAmount.StringFixed(2)`. The SNAPSHOT_V1 query does not set `VATRate`. |
| Register line | `billing.billing_register_items.base_amount`, `extra_charges`, `penalties`, `amount_without_vat`, `vat_rate`, `vat_amount`, `amount_with_vat` | Existing include path. One line. No accessorial, penalty, or withholding. |
| Register totals | `billing.billing_registers.total_without_vat`, `vat_amount`, `total_with_vat`, `vat_rate`, `currency_code` | Existing calculate and approve. |
| Act | `billing.acts.total_amount`, `currency_code`, `seller_company_id`, `buyer_company_id` | The act table has no separate net amount. The existing insert copies `reg.TotalWithVAT`. |
| VAT invoice | `billing.vat_invoices.amount_without_vat`, `vat_rate`, `vat_amount`, `amount_with_vat`, `seller_company_id`, `buyer_company_id` | The existing insert copies the register net, VAT, and gross fields. This table has no currency column. |

The award row remains `100000`. The settlement base is the snapshot `1234.56`, so the proof does not use the award amount or contract-rate resolution.

`SNAPSHOT_AMOUNT_BASIS=UNKNOWN`. Nothing in the snapshot row states whether `total_amount` is net or gross. The observed equality of `total_with_vat` to the snapshot is a consequence of a nil VAT rate, not a confirmed gross basis.

## Scenario

One synthetic tenant, shipment, transport order, rate snapshot, currency `RUB`, settlement, register line, act, and VAT invoice. Amount `1234.56`. `pricing_source=RFQ_AWARD`. No extras, penalties, withholdings, or disputes.

Transitions used: settlement `DRAFT → UNDER_REVIEW → APPROVED`; register `DRAFT → CALCULATED → APPROVED → CLOSING_DOCUMENTS_CREATED`.

Not called: payment, `mark-paid`, EDO exchange, register close, contract-rate resolve, freight-cost consumer, outbox worker. Pending `billing.freight_cost_outbox` rows created by the existing settlement path were left pending. `billing.payment_obligations` stayed empty.

## Isolated run

Database: embedded Postgres 16 from the existing e2e harness library, loopback only, empty data directory. Not staging and not production.

Command:

`go test -tags integration -count=1 -timeout 300s -v -run TestMoneyIdentitySnapshotToClosingDocuments ./internal/integration/freightbillingclosing/`

Exit code `0`. Result `PASS` in 5.67s.

Stored identifiers from that run:

| Object | ID |
| --- | --- |
| Snapshot | `b1c0ffc6-56b0-48fe-bea0-e6ca2c501b2c` |
| Settlement | `309e9444-e930-42aa-b904-b3f6819e1016` |
| Register | `e97bb29b-345b-4b7a-9c30-10e2f6670c10` |
| Register item | `b518f94e-527a-44f0-bd06-55721d5a8afa` |
| Act | `4aa6de4e-664e-4164-be3c-edc1bd398773` |
| VAT invoice | `20400da8-f08f-4bb4-a76f-4591d00aaf7e` |

The test reads `::text` from PostgreSQL and compares those texts with `decimal.Decimal`, not `float64` equality. Compared equal to `1234.56`: snapshot `total_amount`, settlement `base_freight_amount`, item `base_amount`, register `total_without_vat`, VAT invoice `amount_without_vat`. Act `total_amount` equals register `total_with_vat`. In this run those gross texts are also `1234.56` because every stored VAT amount is `0.00` and every `vat_rate` is null.

Currencies `RUB`: snapshot, settlement, register, act. Accessorial, extra, and penalty texts are `0.00`. Parties: act and VAT invoice seller = carrier, buyer = customer. Package input carried another tenant id; the stored package tenant is the actor tenant.

Denied foreign-tenant read, foreign-company include, and spoofed actor tenant created no register item and did not change register version, register audit count, or settlement outbox count. Repeating settlement create, include, and closing package kept one settlement, one item, one act, and one VAT invoice.

## VAT gate

`VAT_POLICY_CONFIRMED=NO`. No owner confirmation of tax category, rate, snapshot net/gross basis, or rounding rule is present. The system default was not treated as policy.

`VAT_RATE_SOURCE=ABSENT`. The snapshot has no rate column. The SNAPSHOT_V1 loader does not assign `ctxOut.VATRate`.

`CalculateSettlementTotalsDecimal` and `CalculateItemAmounts` set VAT to zero when the rate pointer is nil. The database run stored `vat_amount=0.00` and `vat_rate` null.

`OBSERVED_RUNTIME_VAT_ZERO=YES`

`VAT_POLICY_UNRESOLVED=YES`

`COMMERCIAL_VAT_GATE=BLOCKED`

`COMMERCIAL_PILOT_READY=NO`

## Precision

`TestStoredKopeckAmountsSurviveFloatRound2` keeps these stored two-decimal texts through `round2(float64)` and `CalculateItemAmounts`: `1234.56`, `0.01`, `0.10`, `10.01`, `8.10`, `0.29`, `100000.10`.

`TestLegacyFloatRound2DivergesOnThreeDecimalHalf` reproduces the current helper:

| Input | Decimal `Round(2)` | `round2(float64)` |
| --- | --- | --- |
| `1.255` | `1.26` | `1.25` |

`NUMERIC(18,2)` cannot store `1.255`. The snapshot proof uses `1234.56` and does not hit this input. The helper was not changed.

## Checks

| Command | Exit | Result |
| --- | --- | --- |
| `go test ./internal/domain/ -count=1 -timeout 60s` | 0 | PASS |
| `go test -tags integration -count=1 -timeout 300s -v -run TestMoneyIdentitySnapshotToClosingDocuments ./internal/integration/freightbillingclosing/` | 0 | PASS |
| `go test -tags integration -count=1 -timeout 300s ./internal/integration/freightbillingclosing/` | 0 | PASS, 55.943s, after the harness recognizes SQLSTATE `42P07`, `42701`, `42710`, and `23505` |
| `go test ./internal/domain/ ./internal/http/handlers/ -count=1 -timeout 180s` | 0 | PASS |
| `go test ./internal/service/ ./internal/repository/ ./internal/platform/... -count=1 -timeout 180s` | 0 | PASS; repository and platform packages have no test files |
| `go test -tags integration -count=1 -timeout 180s -run "^$" .` in `tests/bintrans-e2e-transport-flow` | 0 | compile only, no tests executed |

The first full-package attempt on the already migrated database exited `1` before the harness change. Postgres reported duplicate relations in Russian (`SQLSTATE 42P07`), and the helper only matched the English text `already exists`. That helper change is test-only.

## Existing coverage cited

- `tests/bintrans-e2e-transport-flow/financial_closing.go` `assertRateSnapshot` already compares snapshot total, settlement base, item base, and register totals for the HTTP flow. `assertClosingDocumentMoney` adds act and VAT invoice text, currency, party, and single-row checks. The package compiles. The full HTTP stand was not started.
- `Test06CrossTenantInclusionDenied`, `Test07CrossCompanyInclusionDenied`, `Test10SpoofedTenantDenied`, `Test12DuplicateSettlementInclusionIdempotent`, and `Test19DuplicateClosingPackageRetry` ran in the passing package.
- `domain.EnforceOptionalBodyTenant` is covered by `trusted_identity_test.go`.

## Not run

The shared `bintrans-e2e-transport-flow` HTTP stand, payment transitions, EDO operator, bank calls, freight-cost consumer, and the live outbox worker were not started.

## Verdict

`MONEY_IDENTITY_VERDICT=PASS` for the isolated snapshot chain and the fixed amount `1234.56`.

`COMMERCIAL_PILOT_READY=NO` because the VAT policy is unconfirmed and the act amount is the register gross total.
