# Freight Billing & Closing Documents Architecture (v1.8)

## Canonical commercial chain

```
RFx Award
  → Transport Order
  → Shipment / Execution / POD
  → Freight Settlement          (commercial amount SSOT)
  → Billing Register / Batch    (closing aggregate SSOT)
  → Invoice / Act / VAT / UPD   (document metadata SSOT)
  → EDO (mock/manual)           (future: real provider)
  → Payment (mock/manual)       (future: bank reconciliation)
```

## Source of truth

| Concept | Owner | Notes |
|---------|-------|-------|
| Commercial award amount | `rfx.rfx_award_transport_orders` | Loaded when settlement is created |
| Settled payable amount | `billing.freight_settlements` | Base freight + approved accessorials; disputes block billing |
| Billing register total | `billing.billing_registers` | Server-derived sum of register items |
| Register line amounts | `billing.billing_register_items` | Derived from linked settlement at inclusion time |
| Invoice / act / UPD amounts | `billing.invoices`, `billing.acts`, etc. | Copied from approved register totals |
| Document binary files | `document-service` / `documents.documents` | Not duplicated in billing-register-service |
| EDO / payment status | Register status fields | Manual/mock transitions only in v1.8 |

## Architecture decision (Option B)

Freight Settlement v1.7 introduced register inclusion via settlement mutation.
v1.8 **does not** introduce a second authoritative register model.

- **Freight Settlement** remains SSOT for commercial amounts and eligibility.
- **billing-register-service billing register** remains SSOT for the closing batch.
- v1.7 `IncludeInRegister` is preserved for backward compatibility; v1.8 adds explicit
  `billing_register_items.settlement_id` and `POST /billing-registers/{id}/settlements`.

## Settlement → register link

- `billing_register_items.settlement_id` → `freight_settlements.id`
- Partial unique index `(tenant_id, settlement_id)` prevents duplicate billing
- Settlement stores `billing_register_id` / `billing_register_item_id` for reverse lookup

## Eligibility

Settlements are eligible for billing inclusion when:

- Status is `APPROVED`, `DOCUMENTS_READY`, or `READY_FOR_PAYMENT`
- No open dispute
- Not already linked to a register
- Register currency matches settlement currency
- Buyer/carrier match register parties

Server exposes `eligible_for_billing` and `billing_block_reason` on settlement detail.

## Security boundary

Public clients reach billing APIs through **api-gateway** with JWT auth.
Gateway sets trusted `X-Tenant-ID`, `X-User-ID` headers after authentication.
Actor company and role come from verified query params (`actor`, `company_id`) aligned with v1.7 settlement RBAC.

Financial amounts and party IDs are **never** accepted as authoritative from browser JSON for inclusion;
inclusion is driven by `settlement_id` and persisted settlement state.

## Money

- PostgreSQL: `NUMERIC(18,2)` for persisted amounts
- Go service layer: `float64` with `round2()` policy (documented in `money_policy.go`)
- New v1.8 boundaries use server derivation; legacy float64 debt remains isolated

## EDO / payment

v1.8 supports secure **manual/mock** status transitions (`mark-sent-to-edo`, `mark-signed`, `mark-paid`, `close`).
No real Diadoc, SBIS, 1C, or bank integration is implemented.

## Legacy behavior changed in v1.8

- Register list/detail require actor-scoped authorization (buyer/carrier isolation)
- Register creation requires buyer actor; tenant derived from verified headers
- Settlement inclusion via register API sets `settlement_id` on items
- Register audit events persisted for settlement inclusion/removal
