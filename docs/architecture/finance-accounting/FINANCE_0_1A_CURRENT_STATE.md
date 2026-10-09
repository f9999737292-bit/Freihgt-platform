# FINANCE-0.1A current state

```text
STAGE=FINANCE-0.1A
KIND=DISCOVERY
BASE_SHA=2e408a103c3a552fc85fda06325162bf595b654a
WORKTREE=D:\Projects\freight-platform-wt\finance-accounting-current-state-v0.1a
BRANCH=discovery/finance-accounting-current-state-v0.1a
RUNTIME_QUERIES=NOT_RUN
TESTS=NOT_RUN
LEGAL_COMPLIANCE_RESEARCH_REQUIRED=YES
PRODUCT_CODE_CHANGED=NO
```

Evidence is the tree at that SHA. A table or status name is not treated as a finished capability.

## Chain

Commercial price (`contract-rate-service`, decimal) and RFx or spot award amounts can be frozen on `transport.transport_order_rate_snapshots` (`000051`). Freight settlement copies that snapshot `total_amount`, or a legacy award amount, into `billing.freight_settlements`. A billing register includes eligible settlements for one customer and contractor pair and one currency. Closing rows copy register header totals. `document_id` stays null. `mark-signed` can ensure one payment obligation per register. Manual payments allocate to that obligation. Reconcile is an operator command after full allocation. An internal sync can mark the register paid. Register `CLOSED` is the next status after `PAID`.

## Capability inventory

| Capability | Class | Evidence |
| --- | --- | --- |
| Contract rate | IMPLEMENTED | `contract-rate-service`, migrations `000048` `000049`, decimal money, public `POST /api/v1/rates/resolve` |
| RFx / spot price | IMPLEMENTED | Award and snapshot sources `RFQ_AWARD`, `SPOT_BID`, `MANUAL_SPOT`. RFx ERP draft import is not a finance price |
| Rate snapshot | IMPLEMENTED | One row per tenant and transport order. `base_amount`, `total_amount` `NUMERIC(18,2)`. No VAT columns |
| Freight settlement | PARTIAL | Real create, status machine, idempotency, server amount. Unique `(tenant_id, shipment_id)`. Domain money is `float64`. Snapshot path stores VAT 0 |
| Accessorial | IMPLEMENTED | `billing.settlement_accessorials`, propose, approve, reject. Non-negative `NUMERIC` |
| Adjustment | NOT_IMPLEMENTED | No adjustment aggregate. Penalties exist only as a register-item column |
| Dispute | IMPLEMENTED | `billing.settlement_disputes`. Open dispute blocks register inclusion |
| Billing register | PARTIAL | Create, include, calculate, approve, period and currency checks. Totals scanned into `float64`. Item removal hard-deletes the line |
| Billing calculation | UNSAFE_NEEDS_HARDENING | Inclusion uses `CalculateItemAmounts` (`float64`). Nil settlement VAT falls back to register `vat_rate` |
| Billing approval | IMPLEMENTED | Approve only from `CALCULATED` when `total_with_vat > 0`, buyer actor |
| Invoice / Счёт | PARTIAL | `billing.invoices` header. Amount is register `total_with_vat`. No lines. `document_id` null |
| Act / Акт | PARTIAL | `billing.acts` header. Same amount source. Optional `service_description` is not written by `ensureActTx` |
| VAT invoice / Счёт-фактура | PARTIAL | Header without VAT, one rate, VAT, with VAT, copied from the register |
| UPD / УПД | PARTIAL | Same money header. `function_code` constrained to `СЧФ`, `СЧФДОП`, `ДОП` |
| Closing package | PARTIAL | Package row and type check. No document member list on the billing package |
| EDO handoff | MOCK | `MarkSentToEDO` sets `SENT_TO_EDO` with no document and no operator call |
| Signature status | MOCK | `MarkSigned` sets `SIGNED_BY_COUNTERPARTY` on the register. It does not read a signature |
| Document binding | NOT_IMPLEMENTED | Inserts omit `document_id`. No FK to `documents.documents` |
| Payment obligation | IMPLEMENTED | One row per register. Statuses `OPEN`, `PARTIALLY_PAID`, `PAID`, `CANCELLED`, `VOIDED`. Decimal amounts |
| Payment | IMPLEMENTED | Manual create only. SQL also names bank and ERP sources that have no writer |
| Payment allocation | IMPLEMENTED | Allocation to an obligation, currency and party checks, void metadata |
| Payment void / reversal | PARTIAL | Payment void and allocation void exist. No credit note and no accounting reversal document |
| Reconciliation | PARTIAL | Manual reconcile after `FULLY_ALLOCATED`. No statement match |
| Paid projection | IMPLEMENTED | Payment outbox `000046`. Billing internal `sync-paid` and obligation lookup |
| Financial close | PARTIAL | Register status `CLOSED` after `PAID`. No journal and no period close |
| Planned freight cost | IMPLEMENTED | `freight_cost.cost_entry` kind `PLANNED_COST_SNAPSHOT`, append-only `NUMERIC` |
| Actual freight cost | IMPLEMENTED | Accrual, current actual, and final actual snapshot kinds |
| Planned-vs-actual | IMPLEMENTED | Variance attribution and procurement UI pages. This is cost variance, not margin |
| Accounts receivable | NOT_IMPLEMENTED | No AR aggregate |
| Accounts payable | NOT_IMPLEMENTED | No AP aggregate. A payable snapshot kind is a cost-ledger amount, not an AP subledger |
| Receivable | ARCHITECTURE_ONLY | ADR-EDO-005. No table |
| Payable | NOT_IMPLEMENTED | No payable aggregate distinct from the obligation |
| Debtor / creditor | NOT_IMPLEMENTED | Payer and payee on the obligation are companies, not a debtor or creditor account |
| Overdue | PARTIAL | `IsObligationOverdue` has no caller. `OVERDUE` is not a stored status |
| Credit note | NOT_IMPLEMENTED | No credit-note table or route |
| Correction invoice | NOT_IMPLEMENTED | No correction row |
| UKD / correction UPD | ARCHITECTURE_ONLY | ADR-EDO-002 describes a future document. No billing UKD table |
| Factoring | ARCHITECTURE_ONLY | ADR-EDO-005 sketch only |
| Bank integration | NOT_IMPLEMENTED | No provider client |
| Bank statement import | NOT_IMPLEMENTED | Source check value only |
| Auto reconciliation | NOT_IMPLEMENTED | No matcher |
| 1C / ERP finance | NOT_IMPLEMENTED | `ERP_1C` is a payment source name only |
| Finance export | NOT_IMPLEMENTED | No finance export writer |
| Finance import | NOT_IMPLEMENTED | No finance import writer |

RFx `/v1/integrations/erp` is a tender draft preview and commit behind `RfxErpIntegrationEnabled`. It is not finance 1C.

## Payment trace

`BillingRegisterService.MarkSigned` moves the register to `SIGNED_BY_COUNTERPARTY` and, when a payment client is configured, calls `EnsurePaymentObligation`. Obligation source type is only `BILLING_REGISTER`. Unique `(tenant_id, source_type, source_id)` blocks a second obligation for that register.

`PaymentService` create rejects any source other than blank or `MANUAL`, then stores `MANUAL`. Allocation updates paid and outstanding on the obligation and allocated and unallocated on the payment. Over-allocation and paid above original fail closed. `PARTIALLY_PAID` and `PARTIALLY_ALLOCATED` are real statuses.

`ReconcilePayment` requires `FULLY_ALLOCATED` and a relational snapshot with no tenant, currency, party, or void-metadata violations. It sets `RECONCILED`. It does not read a bank line.

Payment outbox migration `000046` and the payment outbox worker emit paid-projection events. Billing `POST /internal/v1/billing-registers/{id}/sync-paid` checks obligation preconditions and sets the register `PAID`. `MarkPaid` is a separate actor transition with the same precondition when the lookup is configured. Register `Close` then requires `PAID`.

```text
PAYMENT_OBLIGATION_IMPLEMENTED=YES
PAYMENT_IMPLEMENTED=YES
ALLOCATION_IMPLEMENTED=YES
RECONCILIATION_IMPLEMENTED=PARTIAL
PARTIAL_PAYMENT_SUPPORTED=YES
OVERPAYMENT_SUPPORTED=NO
VOID_SUPPORTED=YES
IDEMPOTENT_PAYMENT_CREATE=PARTIAL
PAID_EVENT_IMPLEMENTED=YES
BILLING_SYNC_PAID_IMPLEMENTED=YES
```

`IDEMPOTENT_PAYMENT_CREATE=PARTIAL` because uniqueness is the payment number and, when present, `external_id`. A retry that mints a new number can create a second payment. Settlement create is stronger: unique shipment and idempotency key. Closing-document create returns the existing register row.

## Bank

```text
BANK_PROVIDER_IMPLEMENTED=NO
BANK_STATEMENT_IMPORT=NO
BANK_STATEMENT_FORMATS=NONE
AUTO_RECONCILIATION=NO
MANUAL_RECONCILIATION=YES
BANK_WEBHOOK=NO
```

`chk_payment_source` allows `BANK_STATEMENT` and `BANK_API`. No payment-service code writes those values.

## 1C / ERP

```text
ONE_C_INTEGRATION_IMPLEMENTED=NO
ERP_FINANCE_EXPORT_IMPLEMENTED=NO
ERP_FINANCE_IMPORT_IMPLEMENTED=NO
```

## Money model

```text
MONEY_MODEL_CANONICAL=MIXED
FLOAT64_FINANCIAL_DEBT_PRESENT=YES
MONEY_SCALE=2
ROUNDING_POLICY=MIXED
CURRENCY_VALIDATION=PARTIAL
FREIGHT_BASE_HISTORICAL_REPRICING_ALLOWED=NO
BILLING_VAT_RECALCULATION_FROM_REGISTER_RATE_WHEN_SETTLEMENT_RATE_MISSING=YES
HISTORICAL_TAX_BASIS_STABILITY=UNSAFE_NEEDS_HARDENING
```

PostgreSQL amount columns in these domains are `NUMERIC(18,2)`. Rates are `NUMERIC(5,2)`.

| Path | In-process type | Storage |
| --- | --- | --- |
| Contract rate | `shopspring/decimal` | `NUMERIC` |
| Rate snapshot amounts | decimal string at the internal read API | `NUMERIC` |
| Settlement principal parse | `decimal.NewFromString` | then written through domain totals |
| Settlement and register domain structs | `float64` | `NUMERIC` scanned into `float64` |
| Settlement VAT helper | `decimal` body, rate via `decimal.NewFromFloat` | `NUMERIC` |
| Register line inclusion | `CalculateItemAmounts` `float64` and `math.Round` | `NUMERIC` |
| Register total sum | SQL `SUM` scanned into `float64` | `NUMERIC` |
| Closing document amounts | `float64` fields | `NUMERIC` |
| Payment and allocation | `shopspring/decimal` scale 2 | `NUMERIC` |
| Freight-cost entry | `shopspring/decimal`; missing amount is `UNAVAILABLE` | `NUMERIC`, append-only |
| web-admin `revenueTotal` | JS number sum of loaded `total_with_vat` | presentation only |

Authoritative `float64` paths:

- `services/billing-register-service/internal/domain/freight_settlement.go` amount fields and `CalculateSettlementTotals`
- `CalculateSettlementTotalsDecimal` rate conversion with `decimal.NewFromFloat`
- `billing_register.go` totals and `vat_rate`
- `billing_register_item.go` `CalculateItemAmounts` and `round2`
- `invoice.go`, `act.go`, `vat_invoice.go`, `upd.go` amounts
- `billing_register_repository.go` `registerTotals`
- `billing_settlement_link_repository.go` inclusion, which recomputes the billed line from `float64` settlement fields

`money_policy.go` documents this float64 boundary as legacy and states half-away-from-zero to 2 places. The inclusion path uses `math.Round`. The settlement decimal path uses `decimal.Round(2)`. Those are two calculators.

Payment `ValidateCurrencyCode` requires a 3-character code. Register inclusion rejects a currency mismatch. Snapshot and settlement copy the source currency. A register can still be created with a client currency and a client VAT rate.

Settlement base freight is copied from the historical snapshot or award source. A later contract-rate change does not reprice that freight basis. `FREIGHT_BASE_HISTORICAL_REPRICING_ALLOWED=NO`.

When the settlement VAT rate is nil, register inclusion recalculates tax from the register VAT rate. That rate is supplied when the register is created. The billed tax can then differ from the VAT amount stored on the settlement. `BILLING_VAT_RECALCULATION_FROM_REGISTER_RATE_WHEN_SETTLEMENT_RATE_MISSING=YES`. This discovery does not choose the correct VAT policy. `HISTORICAL_TAX_BASIS_STABILITY=UNSAFE_NEEDS_HARDENING`.

## VAT behavior in code

```text
VAT_RATE_SOURCE=NONE_ON_SNAPSHOT; REGISTER_CLIENT_RATE_WHEN_SETTLEMENT_RATE_NIL
VAT_CALCULATION_OWNER=billing-register-service
VAT_ROUNDING_POLICY=SCALE_2_MIXED_FLOAT64_AND_DECIMAL
ZERO_VAT_SUPPORTED=YES
VAT_EXEMPT_SUPPORTED=UNKNOWN
MULTI_RATE_DOCUMENT_SUPPORTED=NO
CORRECTION_SUPPORTED=NO
```

`ZERO_VAT_SUPPORTED=YES` means a nil or zero rate produces a zero VAT amount in the calculator. It is not a classified tax exemption. `VAT_EXEMPT_SUPPORTED=UNKNOWN` because the repository has no exempt flag and this discovery does not supply a legal rule.

The snapshot query in `LoadShipmentContext` does not select a VAT rate. The legacy award query selects `NULL::float8`. Settlement therefore persists VAT amount 0 and sets with-VAT equal to the copied principal. Whether snapshot `total_amount` is gross or net is not defined on `transport.transport_order_rate_snapshots`.

## Receivable and payable

```text
PAYMENT_OBLIGATION_IS_RECEIVABLE=NO
CANONICAL_RECEIVABLE_IMPLEMENTED=NO
CANONICAL_PAYABLE_IMPLEMENTED=NO
AR_AGING_IMPLEMENTED=NO
AP_AGING_IMPLEMENTED=NO
FACTORING_IMPLEMENTED=NO
```

ADR-EDO-005 keeps `PaymentObligation` as payment-execution intent. Freight settlement, billing register, obligation, payment, and receivable remain different objects in that ADR. Only the first four have runtime tables, and receivable does not.

## Security

```text
FINANCE_TENANT_ISOLATION=PARTIAL
FINANCE_COMPANY_ISOLATION=PARTIAL
PUBLIC_CLIENT_TENANT_HEADER_AUTHORITY=NO
DOWNSTREAM_FINANCE_SERVICE_TRUSTS_GATEWAY_TENANT_HEADER=YES
DIRECT_FINANCE_SERVICE_EXPOSURE_VERIFIED=NO
CLIENT_COMPANY_HEADER_IS_AUTHORITY=NO
FINANCE_IDENTITY_SPOOF_RISK=PARTIAL
```

A browser or other public client value of `X-Tenant-ID` is not canonical tenant authority. When API Gateway auth is enabled, `Auth` deletes untrusted identity headers, including `X-Tenant-ID`, `X-User-ID`, `X-Company-ID`, and `X-Actor-Kind`, and writes `X-Tenant-ID` and `X-User-ID` from the verified token claims (`services/api-gateway/internal/http/middleware/auth.go`). Billing guards then use that gateway context (`billingrbac.Guard` through `routeauth.BuildRequestContext`).

`billing-register-service` has no JWT middleware. `resolveVerifiedTenant` and `resolveVerifiedUser` read `X-Tenant-ID` and `X-User-ID` and treat them as the upstream tenant and user. `DOWNSTREAM_FINANCE_SERVICE_TRUSTS_GATEWAY_TENANT_HEADER=YES`. `X-Company-ID` and `X-Actor-Kind` must still match a membership, and actor kind is derived from company type and roles. A body `tenant_id` or `approved_by` that differs from those headers is rejected. Platform admin without a matching membership is rejected (`settlement_actor_context.go`). Queries cannot override the resolved company or actor. `CLIENT_COMPANY_HEADER_IS_AUTHORITY=NO` because membership must agree.

Direct exposure of the finance service port, bypassing the gateway, was `NOT_RUN`. `DIRECT_FINANCE_SERVICE_EXPOSURE_VERIFIED=NO`. This inventory does not claim a demonstrated public exploit. FIN-SEC-001 stays open because the finance service itself does not validate a token and depends on that upstream boundary.

Closing-document seller and buyer on the actor path are overwritten from the register: contractor is seller, customer is buyer.

`POST /v1/billing-registers/{id}/items` still accepts client `base_amount`, `extra_charges`, `penalties`, and `vat_rate`. That route is on the service and in `packages/openapi/billing-register-service.yaml`. It is absent from `services/api-gateway/internal/http/router.go`. The same OpenAPI file lists `/invoices`, `/acts`, `/vat-invoices`, and `/upd`. The gateway route table does not.

## UI

| Workflow | Class | Where |
| --- | --- | --- |
| Settlement | BACKEND_READY_UI_PRESENT | `apps/web-procurement/pages/settlements` and `useSettlementApi.ts` |
| Billing register | BACKEND_READY_UI_PRESENT | Procurement list and detail: create, include, calculate, approve. `apps/web-admin` list and detail are read views |
| Closing package | PARTIAL | Procurement detail calls `createClosingDocumentPackage` with hardcoded `ACT_PLUS_VAT_INVOICE` |
| Invoice, act, VAT invoice, UPD | BACKEND_READY_UI_MISSING | No page or composable call to those routes |
| Payments | BACKEND_READY_UI_PRESENT | `apps/web-procurement/pages/payments`, including reconcile |
| Reconciliation | BACKEND_READY_UI_PRESENT | Manual reconcile on the payment page. No statement UI |
| Freight cost | BACKEND_READY_UI_PRESENT | `apps/web-procurement/pages/freight-costs`, including planned-versus-actual |
| AR/AP | BLOCKED | No aggregate and no page |
| Forwarder margin | BLOCKED | No canonical fact. web-admin `revenueTotal` sums loaded register totals in the browser |

`apps/web-finance/pages/index.vue` states that the finance portal is a skeleton with no business logic.

## Readiness

Score steps are 0, 25, 50, 75, and 100.

- 0 means no runtime path.
- 25 means a schema, label, or manual status without the domain boundary.
- 50 means a real single-chain command path with a known integrity or model gap.
- 75 means fail-closed money and idempotent core commands, with a remaining gap that is outside that dimension.
- 100 means no known blocking gap in that dimension.

| Dimension | Score | Why |
| --- | --- | --- |
| SETTLEMENT | 50 | Server amount, accessorials, disputes, idempotency. One row per shipment. Float64 domain. VAT stored as 0 |
| BILLING_REGISTER | 50 | Include, calculate, approve, currency check. Float64 totals and client VAT fallback |
| CLOSING_DOCUMENTS | 25 | Distinct header rows. No lines, null `document_id`, gateway routes absent |
| EDO_BINDING | 25 | Nullable `document_id` column. Status marks do not bind a document |
| PAYMENTS | 75 | Decimal obligation, payment, allocation, void, partial pay, paid outbox, billing sync. Create idempotency is number-based |
| RECONCILIATION | 50 | Manual full-allocation reconcile. No bank line |
| ACCOUNTING_MASTER_DATA | 25 | `core.companies` has `legal_name`, `tax_id`, `registration_number`, `country_code`. No address, no KPP, and `tax_id` is not labeled ИНН |
| RUSSIAN_TAX_DOCS | 25 | Four document types and UPD function codes. Header money only. Legal research required |
| BANK_INTEGRATION | 0 | No provider, import, or webhook |
| ONE_C_INTEGRATION | 0 | No finance adapter |
| FORWARDER_AR_AP | 25 | One buyer-to-carrier chain can run. Two chains on one execution cannot |
| SECURITY | 50 | Membership and platform-admin checks exist. Public client tenant header is not authority. The finance service trusts the gateway header and does not validate JWT. Direct port exposure was not verified |
| TESTING | 50 | Integration suites exist under the finance services. This stage did not run them |
| DEPLOYMENT | 25 | Compose files wire billing-register-service and payment-service. This stage did not deploy, and deployment is not scored as production-ready |

Sum `475` across 14 dimensions. Mean `475 / 14 = 33.928...`, rounded half up.

```text
FINANCE_ACCOUNTING_OVERALL_READINESS=34%
```

## Tests and deployment evidence

Integration packages exist for freight settlement, billing closing, payment core, contract rate, and freight-cost ledger. `TESTS=NOT_RUN` in this stage. `tests/integration/README.md` says EDO, 1C, and payment-gateway steps are mocked.

Compose wiring is not a finance go-live claim. Staging was not changed.
