# FINANCE-0.1A gaps

```text
TOTAL_GAPS=18
BLOCKING_GAPS=12
LEGAL_COMPLIANCE_RESEARCH_REQUIRED=YES
```

Blocking means the gap stops a later finance stage from treating the current chain as accounting truth or as two-sided forwarder finance. Bank and 1C absence is recorded and is not marked blocking until a stage authorizes that integration.

## Register

### FIN-SET-001

- AREA: Freight settlement
- DESCRIPTION: One shipment can have only one settlement.
- CURRENT_STATE: `uq_freight_settlement_shipment` on `(tenant_id, shipment_id)`. Create returns the existing row.
- SEVERITY: HIGH
- BLOCKING: YES
- OWNER_AGENT: G
- EVIDENCE: `infrastructure/migrations/000042_freight_settlement_v1.7.up.sql`; `FreightSettlementRepository.CreateSettlement`
- RECOMMENDED_STAGE: FINANCE-0.1B

### FIN-SET-002

- AREA: Settlement money type
- DESCRIPTION: Settlement and billing domain amounts are `float64` while storage is `NUMERIC`.
- CURRENT_STATE: Principal is parsed as decimal, then domain structs and register inclusion use `float64`.
- SEVERITY: HIGH
- BLOCKING: YES
- OWNER_AGENT: G
- EVIDENCE: `internal/domain/freight_settlement.go`; `internal/domain/money_policy.go`; `billing_settlement_link_repository.go`
- RECOMMENDED_STAGE: FINANCE-0.1B

### FIN-SET-003

- AREA: Snapshot tax
- DESCRIPTION: Rate snapshot has no VAT columns. Settlement stores VAT amount 0. Freight base is not repriced later.
- CURRENT_STATE: Snapshot or award `total_amount` is copied once. `FREIGHT_BASE_HISTORICAL_REPRICING_ALLOWED=NO`. Gross versus net is unspecified. `HISTORICAL_TAX_BASIS_STABILITY=UNSAFE_NEEDS_HARDENING` because a later billing step can recalculate VAT. This gap does not choose a VAT policy.
- SEVERITY: HIGH
- BLOCKING: YES
- OWNER_AGENT: G
- EVIDENCE: `000051_transport_order_rate_snapshot_v2.0C.up.sql`; `LoadShipmentContext`
- RECOMMENDED_STAGE: FINANCE-0.1B

### FIN-BIL-001

- AREA: Billing line source
- DESCRIPTION: The service still accepts client line amounts.
- CURRENT_STATE: `POST /v1/billing-registers/{id}/items` uses client `base_amount`, extras, penalties, and VAT rate. Gateway does not mount that route. OpenAPI lists it.
- SEVERITY: HIGH
- BLOCKING: YES
- OWNER_AGENT: G
- EVIDENCE: `billing_register_handler.go`; `packages/openapi/billing-register-service.yaml`; gateway `router.go`
- RECOMMENDED_STAGE: FINANCE-0.1B

### FIN-BIL-002

- AREA: Billing VAT fallback
- DESCRIPTION: When settlement VAT is missing, billing recalculates tax from the register VAT rate.
- CURRENT_STATE: `BILLING_VAT_RECALCULATION_FROM_REGISTER_RATE_WHEN_SETTLEMENT_RATE_MISSING=YES`. `IncludeSettlement` calls `CalculateItemAmounts` with that fallback and `float64` rounding. The register rate is client-supplied at register create. This gap does not choose a VAT policy.
- SEVERITY: HIGH
- BLOCKING: YES
- OWNER_AGENT: G
- EVIDENCE: `billing_settlement_link_repository.go`; `CreateBillingRegisterInput.VATRate`
- RECOMMENDED_STAGE: FINANCE-0.1B

### FIN-BIL-003

- AREA: Billing line history
- DESCRIPTION: Removing a settlement hard-deletes the register item.
- CURRENT_STATE: Audit event remains. The line row does not.
- SEVERITY: MEDIUM
- BLOCKING: NO
- OWNER_AGENT: G
- EVIDENCE: `BillingRegisterRepository.RemoveSettlement`
- RECOMMENDED_STAGE: FINANCE-0.1B

### FIN-DOC-001

- AREA: Billing to EDO reference
- DESCRIPTION: Closing documents never store `document_id`, and operator status does not require it.
- CURRENT_STATE: Column is nullable and omitted on insert. `MarkSentToEDO` only changes register status.
- SEVERITY: HIGH
- BLOCKING: YES
- OWNER_AGENT: G
- EVIDENCE: `closing_document_idempotent.go`; `BillingRegisterService.MarkSentToEDO`; ADR-EDO-002
- RECOMMENDED_STAGE: FINANCE-0.2A billing to EDO reference contract, with Agent B for the legal object

### FIN-DOC-002

- AREA: Mock signature status
- DESCRIPTION: `mark-signed` sets `SIGNED_BY_COUNTERPARTY` without a signature result.
- CURRENT_STATE: Allowed from `SENT_TO_EDO` or `CLOSING_DOCUMENTS_CREATED`.
- SEVERITY: HIGH
- BLOCKING: YES
- OWNER_AGENT: G
- EVIDENCE: `ValidateMarkSignedStatus`; `MarkSigned`
- RECOMMENDED_STAGE: FINANCE-0.2A

### FIN-DOC-003

- AREA: Legal artifact and qualified signature
- DESCRIPTION: No statutory XML or PDF generator is bound to billing, and qualified verification is unavailable.
- CURRENT_STATE: Document-service can store caller-supplied paths. `UnavailableSignatureVerifier` returns `VERIFIER_UNAVAILABLE`. ADR-EDO-011 says qualified trust is not ready.
- SEVERITY: HIGH
- BLOCKING: YES
- OWNER_AGENT: B
- EVIDENCE: `signature_verifier.go`; `docs/adr/ADR-EDO-011-production-gost-verifier.md`
- RECOMMENDED_STAGE: EDO verifier stage owned by Agent B, after FINANCE-0.2A defines the reference

### FIN-DOC-004

- AREA: Accounting document body
- DESCRIPTION: Invoice, act, VAT invoice, and UPD are register-level headers. Lines, quantity, unit price, correction, and payment reference are absent.
- CURRENT_STATE: See the field map. Legal mandatory set is not decided in this repository.
- SEVERITY: HIGH
- BLOCKING: YES
- OWNER_AGENT: G
- EVIDENCE: `000006_create_billing_tables.up.sql`; `FINANCE_0_1A_ACCOUNTING_DOCUMENT_MAP.md`
- RECOMMENDED_STAGE: FINANCE-0.1C after legal research

### FIN-DOC-005

- AREA: Closing routes
- DESCRIPTION: OpenAPI lists invoice, act, VAT invoice, and UPD routes. The gateway route table does not.
- CURRENT_STATE: Service handlers exist. Procurement UI creates a closing package only.
- SEVERITY: MEDIUM
- BLOCKING: NO
- OWNER_AGENT: A
- EVIDENCE: `packages/openapi/billing-register-service.yaml`; `services/api-gateway/internal/http/router.go`
- RECOMMENDED_STAGE: Gateway route review with Agent A when G publishes the binding contract

### FIN-TAX-001

- AREA: Russian tax policy
- DESCRIPTION: Repository evidence is not sufficient to choose VAT rates, exemption, correction, or statutory fields.
- CURRENT_STATE: `LEGAL_COMPLIANCE_RESEARCH_REQUIRED=YES`. Code can store one rate and can compute zero VAT when the rate is nil.
- SEVERITY: HIGH
- BLOCKING: YES
- OWNER_AGENT: G
- EVIDENCE: Snapshot table has no VAT. No exempt flag. No UKD table.
- RECOMMENDED_STAGE: FINANCE-0.1C legal research, docs only until a controller authorizes schema

### FIN-PAY-001

- AREA: Receivable
- DESCRIPTION: Payment obligation is not a receivable, and no receivable aggregate exists.
- CURRENT_STATE: ADR-EDO-005 is design only. No factoring tables.
- SEVERITY: HIGH
- BLOCKING: NO
- OWNER_AGENT: G
- EVIDENCE: `docs/adr/ADR-EDO-005-receivable-vs-payment-obligation.md`; `chk_payment_obligation_source_type`
- RECOMMENDED_STAGE: FINANCE-0.4A AR/AP architecture

### FIN-PAY-002

- AREA: Overdue
- DESCRIPTION: Overdue is a pure function with no caller and no stored status.
- CURRENT_STATE: Due date can be patched. Aging reports do not exist.
- SEVERITY: MEDIUM
- BLOCKING: NO
- OWNER_AGENT: G
- EVIDENCE: `IsObligationOverdue` in `payment_obligation.go`
- RECOMMENDED_STAGE: FINANCE-0.4A

### FIN-BANK-001

- AREA: Bank
- DESCRIPTION: No bank provider, statement import, webhook, or automatic reconciliation.
- CURRENT_STATE: Manual reconcile after full allocation. Source names `BANK_STATEMENT` and `BANK_API` are unused.
- SEVERITY: HIGH
- BLOCKING: NO
- OWNER_AGENT: G
- EVIDENCE: `payment.go` `ValidateManualPaymentSource`; `000045` source check
- RECOMMENDED_STAGE: FINANCE-0.5A bank statement architecture

### FIN-1C-001

- AREA: 1C / ERP finance
- DESCRIPTION: No finance export or import. RFx ERP is a different integration.
- CURRENT_STATE: `ERP_1C` and `ERP_SAP` are unused payment source names.
- SEVERITY: HIGH
- BLOCKING: NO
- OWNER_AGENT: G
- EVIDENCE: `payment-service` has no 1C adapter. `rfx-service` `/v1/integrations/erp` is tender draft exchange.
- RECOMMENDED_STAGE: FINANCE-0.6A 1C architecture, after document and payment identities are stable

### FIN-FWD-001

- AREA: Two-sided forwarder finance
- DESCRIPTION: Customer revenue and carrier cost cannot be separate settlements, registers, or EDO chains for one execution.
- CURRENT_STATE: Markers in `FINANCE_0_1A_FORWARDER_FINANCE.md` are NO. Forwarder company type is a buyer actor.
- SEVERITY: HIGH
- BLOCKING: YES
- OWNER_AGENT: G
- EVIDENCE: `uq_freight_settlement_shipment`; `DeriveSettlementActorKind`
- RECOMMENDED_STAGE: FINANCE-0.3A dual-chain architecture

### FIN-FWD-002

- AREA: Margin
- DESCRIPTION: No canonical forwarder margin fact.
- CURRENT_STATE: Cost snapshots exist for one buyer and carrier. No revenue fact. No formula is authorized.
- SEVERITY: HIGH
- BLOCKING: NO
- OWNER_AGENT: G
- EVIDENCE: `freight_cost` entry kinds; absence of a margin aggregate
- RECOMMENDED_STAGE: FINANCE-0.3A, policy only, after the two chains exist

### FIN-SEC-001

- AREA: Header trust
- DESCRIPTION: Public client `X-Tenant-ID` is not authority. The finance service does not validate JWT and trusts the tenant header supplied by the upstream gateway.
- CURRENT_STATE: `PUBLIC_CLIENT_TENANT_HEADER_AUTHORITY=NO`. Gateway auth strips untrusted identity headers and writes tenant and user from token claims. `DOWNSTREAM_FINANCE_SERVICE_TRUSTS_GATEWAY_TENANT_HEADER=YES`. Company header must match membership. Platform admin cannot skip membership. `DIRECT_FINANCE_SERVICE_EXPOSURE_VERIFIED=NO`. No public exploit is claimed.
- SEVERITY: HIGH
- BLOCKING: YES
- OWNER_AGENT: A
- EVIDENCE: `services/billing-register-service/internal/http/handlers/identity.go`
- RECOMMENDED_STAGE: Agent A gateway and identity boundary review before finance production exposure

## Recommended order

1. FINANCE-0.1B architecture freeze for money type, VAT source, settlement identity, and the ban on client line totals. Docs only.
2. FINANCE-0.1C Russian accounting document research. Docs only. No invented tax rules.
3. FINANCE-0.2A billing `document_id` contract with Agent B. G owns the financial reference. B owns XML, signature, and operator state.
4. FINANCE-0.3A two-chain forwarder model and a decision to defer margin recognition until both chains exist.
5. FINANCE-0.4A receivable and payable as separate aggregates from payment obligation, including overdue.
6. FINANCE-0.5A bank statement and manual-to-auto reconciliation boundary.
7. FINANCE-0.6A 1C finance export and import, after document and payment identities are stable.

Agent E does not own these amounts. Agent F owns portal presentation after the facts exist. Agent C continues to own shipment and transport-order execution facts that settlement only references. Agent A owns deployment and the gateway identity boundary in FIN-SEC-001 and FIN-DOC-005.

```text
RECOMMENDED_NEXT_STAGE=FINANCE-0.1B
```
