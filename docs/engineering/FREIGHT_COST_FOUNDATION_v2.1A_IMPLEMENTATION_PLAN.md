# Freight Cost Foundation v2.1A — Implementation Plan & Contract Freeze

**Status:** PLANNING / CONTRACT FREEZE (no runtime in this document)  
**Base SHA:** `d1b581c94d685365dc1ffdde07dffbf465649db6`  
**Architecture baseline:**
- `docs/engineering/FREIGHT_COST_MANAGEMENT_v2.1_ARCHITECTURE.md`
- `docs/architecture/FREIGHT_COST_MANAGEMENT_v2.1_FINAL_REVIEW.md`

**Purpose:** Make subsequent v2.1A implementation mechanical and unambiguous. This slice defines contracts only — **do not implement runtime in the planning PR**.

---

## 1. Objective

Deliver **Freight Cost Foundation (v2.1A)**: a minimal runnable `freight-cost-service` with frozen domain semantics, security boundary, canonical read provider interfaces, and an internal planned-cost read API — **without** ledger, event ingestion, migrations, public gateway routes, or frontend.

v2.1A establishes the **read orchestration and pure-domain contract layer** that v2.1B will persist and ingest.

---

## 2. Architecture baseline

### 2.1 Precondition verification (frozen at `d1b581c`)

| Gate | Value |
|------|-------|
| Architecture artifacts present | YES |
| `PLANNED_COST_OWNER` | transport-order-service |
| `ACTUAL_COST_OWNER` | billing-register-service |
| `FINAL_ACTUAL_STATUS` | READY_FOR_PAYMENT |
| `ACCRUAL_OWNER` | freight-cost-service (derived; not canonical writer) |
| `LEDGER_AUTHORITY` | DERIVED_EVENT_JOURNAL |
| `LEDGER_SECOND_SSOT` | NO |
| `REBUILD_ROOT` | canonical domain read APIs |
| `MIXED_CURRENCY_AGGREGATION` | DENY |
| `PLANNED_ACTUAL_TAX_BASIS` | EX-VAT compatible |
| `CROSS_COMPANY_COST_ACCESS` | DENY |
| `OPEN_BLOCKER` | 0 |
| `OPEN_HIGH` | 0 |

### 2.2 Architecture slice reference

Architecture §46 defines v2.1A as: domain types, finality enums, source reference model, invariants, internal read API skeleton, tenant/company isolation design.

---

## 3. Exact scope — `V2_1A_SCOPE`

v2.1A implementation **SHOULD include:**

| # | Deliverable |
|---|-------------|
| 1 | New `services/freight-cost-service` — minimal runnable Go service |
| 2 | Domain types: `Money`, source refs, finality, reconciliation enums, actor/view scope |
| 3 | Pure-domain functions: finality, accrual/forecast **calculation contracts**, billing reconciliation rules |
| 4 | NULL ≠ ZERO semantics enforced in domain + JSON DTOs |
| 5 | Money/currency validation at service boundary (`decimal.Decimal`, scale 2) |
| 6 | Tenant + company isolation in service layer |
| 7 | Trusted internal actor context (no client-spoofed identity without S2S gate) |
| 8 | Provider interfaces for canonical reads (transport required; settlement/billing/payment stubbed) |
| 9 | HTTP client adapters calling downstream **internal APIs only** |
| 10 | Internal read API: planned-cost summary for transport order |
| 11 | Health, metrics, structured logging (no money in labels/logs) |
| 12 | Unit, security, source-adapter, API contract tests |
| 13 | CI matrix entry for `services/freight-cost-service` |
| 14 | **One cross-service addition:** transport-order internal rate-snapshot read endpoint |

v2.1A implements **planned-cost read orchestration** and **pure financial semantics**. It does **not** populate accrual/actual/payment fields from live sources except where domain stubs return explicit `NULL`.

---

## 4. Explicit out-of-scope — `V2_1A_OUT_OF_SCOPE`

| Area | Deferred to |
|------|-------------|
| Cost ledger / `cost_entry` table | v2.1B |
| Event outbox consumers (settlement, billing, payment) | v2.1B |
| Settlement/billing outbox changes | v2.1B |
| Payment event ingestion | v2.1B |
| Accrual event processing / persistence | v2.1B |
| Projection rebuild engine | v2.1B |
| Variance engine (derived amounts in API) | v2.1C |
| Public `/api/v1` gateway routes | v2.1E |
| Frontend workspace | v2.1D+ |
| OpenAPI public runtime changes | v2.1E |
| FX conversion | Out of v2.1 |
| `charge_code` semantic classification | v2.1C (OQ-005 LOW) |
| Reconciliation background jobs | v2.1C |
| Database migrations / freight_cost schema | v2.1B |
| Docker compose production wiring (optional dev-only in v2.1A impl PR) | v2.1A impl follow-up |

---

## 5. Service boundary

### 5.1 Identity

| Field | Frozen value |
|-------|--------------|
| `SERVICE_NAME` | `freight-cost-service` |
| `SERVICE_PATH` | `services/freight-cost-service` |
| Go module | `github.com/freight-platform/freight-cost-service` |

### 5.2 Port allocation (discovered — do not invent)

Existing ports in `infrastructure/docker-compose/docker-compose.yml`:

| Port | Service |
|------|---------|
| 8080 | api-gateway |
| 8081 | identity-service |
| 8082 | company-service |
| 8083 | transport-order-service |
| 8084 | rfx-service |
| 8085 | shipment-service |
| 8086 | document-service |
| 8087 | billing-register-service |
| 8088 | low-code-service |
| 8089 | control-tower-read-model-service |
| **8090** | **payment-service** (already allocated) |
| **8091** | **contract-rate-service** (`services/contract-rate-service/internal/config/config.go`, default `8091`) |

| Field | Frozen value |
|-------|--------------|
| `SERVICE_PORT` | **8092** |
| Env var | `FREIGHT_COST_SERVICE_PORT` (fallback `HTTP_PORT`, default `8092`) |

**Note:** Do not allocate `8091` — already owned by contract-rate-service per v2.0A implementation docs.

### 5.3 Routes

| Route | Purpose |
|-------|---------|
| `HEALTH_ROUTE` | `/health` (via `shared-go/observability.Mount`) |
| `METRICS_ROUTE` | `/metrics` (same mount) |
| `INTERNAL_ROUTE_PREFIX` | `/internal/v1/freight-cost` |

### 5.4 Bootstrap decision

**`SERVICE_BOOTSTRAP_DECISION` = C**

Minimal runnable service with:
- health + metrics
- internal API (planned-cost read)
- provider interfaces + transport adapter
- domain + security + tests

**No ledger. No database.**

Template service: `control-tower-read-model-service` (chi router, observability mount, internal routes) — but **without** DB/consumer in v2.1A.

---

## 6. Domain model

### 6.1 Core types

#### `Money` — **IMPLEMENT_V2_1A**

`Money` always represents a **known** monetary fact. Unknown monetary state is **never** embedded inside `Money`.

```go
type Money struct {
    Amount   decimal.Decimal // canonical; never float64 in domain
    Currency string          // ISO 4217, required
}
```

| Rule | Value |
|------|-------|
| `MONEY_STRUCT_CAN_BE_UNKNOWN` | **NO** |
| `OPTIONAL_MONEY_USES_POINTER` | **YES** — use `*Money` at aggregate/function boundaries |
| `UNKNOWN_MONEY` | `nil` (`*Money == nil`) |
| `KNOWN_ZERO_MONEY` | `&Money{Amount: decimal.Zero, Currency: "RUB"}` (non-nil) |

- `Money.IsZero()` — known zero amount (distinct from unknown).
- Optional fields in aggregates: `PlannedAmount *Money`, `AccruedAmount *Money`, etc.

#### `CostSourceRef` (canonical fact pointer) — **IMPLEMENT_V2_1A**

For v2.1A read-only facts (no event identity required yet):

```go
type CanonicalSourceRef struct {
    SourceService        string     // e.g. "transport-order-service"
    SourceType           string     // TO_RATE_SNAPSHOT | FREIGHT_SETTLEMENT | ...
    SourceID             uuid.UUID
    SourceVersion        *int       // nil = N/A; never invent a synthetic version
    PricingModelVersion  *string    // e.g. "SNAPSHOT_V1" for TO snapshots; distinct from aggregate revision
}
```

**Source version semantics (frozen):**

| Field | TO rate snapshot |
|-------|------------------|
| `SourceVersion` | **nil / omitted** — `RateSnapshot` has no aggregate revision field |
| `PricingModelVersion` | `"SNAPSHOT_V1"` from `transport_orders.pricing_model_version` |

Do **not** send `"source_version": 1` merely because the pricing model is v1.

#### `EventSourceRef` — **RESERVE_FOR_V2_1B**

```go
type EventSourceRef struct {
    CanonicalSourceRef
    SourceEventID   uuid.UUID
    SourceOccurredAt time.Time
    SourceRevision  int
}
```

Not required in v2.1A HTTP responses or persistence.

#### `FinancialFinality` — **IMPLEMENT_V2_1A**

Enum describing settlement-side confidence (local canonical strings — **no import** of billing-register Go package):

```text
FINANCIAL_FINALITY_NOT_EVALUATED    // settlement source not loaded (v2.1A default)
FINANCIAL_FINALITY_DRAFT            // settlement loaded, not financially accepted
FINANCIAL_FINALITY_CURRENT_ACTUAL   // CURRENT_ACTUAL available
FINANCIAL_FINALITY_FINAL_ACTUAL     // FINAL_ACTUAL available
FINANCIAL_FINALITY_CANCELLED        // settlement cancelled
```

**v2.1A HTTP value:** `NOT_EVALUATED` — means settlement provider was **not called**, not that settlement does not exist.
Do **not** claim absence of settlement unless the settlement provider was queried.

#### `BillingReconciliationStatus` — **IMPLEMENT_V2_1A** (pure function only)

```text
MATCH | MISMATCH | UNLINKED
```

#### `CostViewScope` — **IMPLEMENT_V2_1A**

```text
BUYER_COST_VIEW | CARRIER_RECEIVABLE_VIEW
```

(`PLATFORM_ADMIN_VIEW` deferred to v2.1E — denied at v2.1A HTTP boundary.)

### 6.2 Summary aggregate — field disposition

`CostSummary` is the internal API DTO aggregate. Fields marked **v2.1A** appear in API with correct null semantics; **v2.1B+** are reserved (omitted or always `null` in v2.1A).

| Field | Slice | v2.1A API |
|-------|-------|-----------|
| `TenantID` | v2.1A | YES |
| `TransportOrderID` | v2.1A | YES |
| `ShipmentID` | v2.1A | nullable (from TO/shipment link when known) |
| `BuyerCompanyID` | v2.1A | YES |
| `CarrierCompanyID` | v2.1A | YES |
| `CurrencyCode` | v2.1A | YES (from snapshot) |
| `PlannedAmount` | v2.1A | YES (from snapshot `total_amount`) |
| `PlannedSourceRef` | v2.1A | YES |
| `DataStage` | v2.1A | YES (`PLANNED_ONLY` in v2.1A) |
| `FinancialFinality` | v2.1A | YES (`NOT_EVALUATED` — settlement source not loaded) |
| `AccruedAmount` | v2.1B | **null** in v2.1A |
| `ForecastExposure` | v2.1B | **null** in v2.1A |
| `CurrentActualAmount` | v2.1B | **null** in v2.1A |
| `FinalActualAmount` | v2.1B | **null** in v2.1A |
| `BillingRegisterAmount` | v2.1B | **null** in v2.1A |
| `PaidAmount` | v2.1B | **null** in v2.1A |
| `CurrentVarianceAmount` | v2.1C | **null** in v2.1A |
| `FinalVarianceAmount` | v2.1C | **null** in v2.1A |
| `BillingReconciliationStatus` | v2.1B | **null** in v2.1A |
| `SourcesAvailable` | v2.1A | YES — bitmask/list e.g. `["TO_RATE_SNAPSHOT"]` |

**Rule:** v2.1A MUST NOT return `0.00` for unimplemented future fields.

### 6.3 Planned / actual type aliases

| Type | Slice | Notes |
|------|-------|-------|
| `PlannedCost` | v2.1A | Wrapper over `Money` + `CanonicalSourceRef` |
| `CurrentActualCost` | v2.1B | Populated from settlement provider |
| `FinalActualCost` | v2.1B | Populated when finality = FINAL |
| `AccruedCost` | v2.1B | Derived; domain calc frozen in v2.1A |
| `ForecastExposure` | v2.1B | Non-canonical KPI |

---

## 7. NULL / zero semantics

**Principle:** `NULL != ZERO`. Unknown financial state is **never** coerced to `decimal.Zero`.

| Field | Required | Nullable | Known-zero allowed | Unavailable meaning |
|-------|----------|----------|--------------------|---------------------|
| `planned_amount` | When TO has snapshot | NO (TO missing → 404; unpriced TO → 409) | **YES** — DB allows `total_amount >= 0` (`000051` `chk_snapshot_total_nonneg`) | N/A when snapshot present |
| `accrued_amount` | NO | YES | YES (explicit zero accrual addon) | Settlement/accessorial source not loaded (v2.1A: always unavailable) |
| `forecast_exposure` | NO | YES | YES | No proposed accessorials |
| `current_actual_amount` | NO | YES | NO | Settlement missing, wrong status, or open dispute |
| `final_actual_amount` | NO | YES | NO | Not READY_FOR_PAYMENT |
| `billing_amount` | NO | YES | NO | Not linked to register |
| `paid_amount` | NO | YES | YES (obligation exists, nothing paid) | No obligation |
| `current_variance` | NO | YES | YES (explicit zero variance) | Planned or current actual unavailable |
| `final_variance` | NO | YES | YES | Planned or final actual unavailable |

**Planned zero (frozen):**

| Rule | Value |
|------|-------|
| `PLANNED_ZERO_ALLOWED` | **YES_AS_CANONICAL_KNOWN_ZERO** |
| Evidence | Migration `000051`: `chk_snapshot_total_nonneg CHECK (total_amount >= 0)` — zero permitted, not forbidden |
| Cost service rule | Faithfully reflect canonical snapshot; `0.00` ≠ NULL |

**JSON rule (frozen):**

| Rule | Value |
|------|-------|
| `UNKNOWN_AMOUNT_JSON` | explicit JSON `null` |
| `UNKNOWN_FIELD_OMITTED` | **NO** for reserved monetary fields |
| `STABLE_SUMMARY_SHAPE` | **YES** — all reserved fields present in every 200 response |
| `KNOWN_ZERO_JSON` | decimal string `"0.00"` (never null) |

Wire DTO example — **no `omitempty`** on reserved nullable amounts:

```go
AccruedAmount *string `json:"accrued_amount"` // nil → JSON null
```

**Domain rule:** use `*Money` for unknown; never default nil to zero inside financial derivations.

---

## 8. Money contract

| Decision | Value |
|----------|-------|
| `CANONICAL_MONEY_TYPE` | `github.com/shopspring/decimal.Decimal` |
| `FLOAT64_CANONICAL_MONEY` | **NO** |
| `JSON_NUMBER_CANONICAL_MONEY` | **NO** |
| DB (v2.1B+) | `NUMERIC(18,2)` |
| JSON internal API | **decimal string** e.g. `"1250.00"` |
| `DECIMAL_SCALE` | 2 |
| `ROUNDING_POLICY` | Half-up, 2 dp — `decimal.Round(2)` (matches transport-order `MoneyScale=2` and billing `CalculateSettlementTotalsDecimal`) |

### 8.1 Domain vs wire separation (frozen)

| Layer | Type |
|-------|------|
| Domain `Money.Amount` | `decimal.Decimal` |
| HTTP/API amount fields | `*string` decimal string (e.g. `"1250.00"`) |

**Explicit helpers — IMPLEMENT_V2_1A:**

```go
func FormatMoneyAmount(d decimal.Decimal) string   // scale 2, no scientific notation
func ParseMoneyAmount(s string) (decimal.Decimal, error) // fail closed; reject float/scientific
```

Do **not** rely on `decimal.Decimal` default JSON marshaling as the API contract.

Rules: exact 2 dp formatting; no float64; fail closed on invalid decimal; no scientific notation.

---

## 9. Currency contract

| Rule | Value |
|------|-------|
| `currency_code` | Required on any summary with monetary facts |
| Format | ISO 4217, 3-letter uppercase |
| Single currency per summary | YES |
| `MIXED_CURRENCY_SUM` | **DENY** — return validation error |
| `CROSS_CURRENCY_VARIANCE` | **DENY** |
| `NULL_CURRENCY` | **DENY** when any amount is present |
| FX | Out of scope |

**Validation:** implement local `ValidateCurrencyCode(code string) error` in freight-cost-service (regex `^[A-Z]{3}$`). No shared-go currency helper exists today — do not duplicate across services later; extract to shared-go in v2.1B if reused.

**Accrual/forecast edge:** if planned currency ≠ accessorial currency → **fail closed** (validation error, not silent conversion).

---

## 10. Source reference model

### 10.1 Permitted source types (foundation)

| `SourceType` | Owner service | v2.1A |
|--------------|---------------|-------|
| `TO_RATE_SNAPSHOT` | transport-order-service | **READ** |
| `FREIGHT_SETTLEMENT` | billing-register-service | interface only |
| `BILLING_REGISTER` | billing-register-service | interface only |
| `INVOICE` | billing-register-service | interface only |
| `PAYMENT_OBLIGATION` | payment-service | interface only |

No speculative source types in v2.1A.

### 10.2 Provider interfaces — **IMPLEMENT_V2_1A** (interfaces + transport adapter)

```go
// TransportOrderPricingProvider — REQUIRED v2.1A
type TransportOrderPricingProvider interface {
    GetRateSnapshot(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*RateSnapshotFact, error)
}

// SettlementCostProvider — interface v2.1A, impl v2.1B
type SettlementCostProvider interface {
    GetByTransportOrder(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*SettlementCostFact, error)
}

// BillingAmountProvider — v2.1B
type BillingAmountProvider interface {
    GetRegisterSnapshot(ctx context.Context, tenantID, billingRegisterID uuid.UUID) (*BillingRegisterFact, error)
}

// PaymentCostProvider — v2.1B
type PaymentCostProvider interface {
    GetObligationByBillingRegister(ctx context.Context, tenantID, billingRegisterID uuid.UUID) (*PaymentObligationFact, error)
}
```

**`CROSS_SERVICE_DB_READS` = NO** — providers call HTTP internal APIs only.

---

## 11. Financial finality contract

Pure-domain functions in `internal/domain/finality.go` — **no billing package import**.

### 11.1 Inputs (local DTO)

```go
type SettlementFinancialInput struct {
    Status           string // DRAFT | UNDER_REVIEW | DISPUTED | APPROVED | DOCUMENTS_READY | READY_FOR_PAYMENT | CANCELLED
    OpenDisputeCount int
    TotalWithoutVAT  *decimal.Decimal // nil if settlement not loaded
}
```

### 11.2 Functions — **IMPLEMENT_V2_1A**

```go
func IsCurrentActualAvailable(in SettlementFinancialInput) bool
func IsFinalActual(in SettlementFinancialInput) bool
func NormalizeSettlementFinancialState(in SettlementFinancialInput) FinancialFinality
func CurrentActualAmount(in SettlementFinancialInput) *decimal.Decimal  // nil or pointer to amount
func FinalActualAmount(in SettlementFinancialInput) *decimal.Decimal
```

### 11.3 Frozen rules

```text
CURRENT_ACTUAL available when:
  status ∈ {APPROVED, DOCUMENTS_READY, READY_FOR_PAYMENT}
  AND open_disputes == 0

FINAL_ACTUAL available when:
  status == READY_FOR_PAYMENT

NULL actual when:
  status ∈ {DRAFT, UNDER_REVIEW, DISPUTED, CANCELLED}
  OR open_disputes > 0
  OR settlement absent

APPROVED with disputes → CURRENT_ACTUAL = NULL (not final)
APPROVED without disputes → CURRENT_ACTUAL available, FINAL_ACTUAL still NULL
```

Evidence: `billing-register-service/internal/domain/freight_settlement.go` — `ValidateSettlementTransition`.

---

## 12. Accrual contract

### 12.1 Frozen business rules — domain frozen in v2.1A

```text
FINANCIAL_ACCRUAL (ex-VAT) =
  PLANNED_COST
  + SUM(approved execution accessorials ex-VAT)
  when settlement exists AND currencies match

ACCRUAL_INCLUDES_PROPOSED = NO
ACCRUAL_INCLUDES_APPROVED = YES

FORECAST_EXPOSURE (non-canonical) =
  PLANNED_COST + SUM(PROPOSed accessorials ex-VAT)
```

### 12.2 Pure functions — **IMPLEMENT_V2_1A**

```go
func CalculateAccrual(planned *Money, approvedAccessorials []Money) (*Money, error)
func CalculateForecastExposure(planned *Money, proposedAccessorials []Money) (*Money, error)
```

| Flag | Value |
|------|-------|
| `DOMAIN_RULE_IMPLEMENT_V2_1A` | YES |
| `EVENT_DRIVEN_ACCRUAL_IMPLEMENT_V2_1B` | YES |

### 12.3 Edge cases

| Case | Behavior |
|------|----------|
| `planned` NULL | Return NULL accrual |
| Currency mismatch | Error `CURRENCY_MISMATCH` |
| Approved total NULL (no lines) | Treat as zero **approved addon** — accrual = planned |
| Approved total zero | Accrual = planned (known zero addon) |

v2.1A does **not** call these from HTTP until settlement provider exists (v2.1B).

---

## 13. Billing reconciliation contract

Enum: `MATCH` | `MISMATCH` | `UNLINKED`

Pure function — **IMPLEMENT_V2_1A**:

```go
type BillingReconciliationInput struct {
    SettlementLinked        bool
    SettlementTotalExVAT    *decimal.Decimal
    SettlementCurrency      string
    SettlementStatus        string
    OpenDisputeCount        int
    BilledLineAmountExVAT   *decimal.Decimal
    BilledLineCurrency      string
}

func DetermineBillingReconciliation(in BillingReconciliationInput) BillingReconciliationStatus
```

### 13.1 Deterministic rules

| Status | Condition |
|--------|-----------|
| `UNLINKED` | `SettlementLinked == false` OR `billing_register_id` nil |
| `MISMATCH` | Linked AND (amounts differ OR currencies differ OR `open_disputes > 0` OR settlement status == DISPUTED) |
| `MATCH` | Linked AND amounts equal (scale 2) AND currencies equal AND `open_disputes == 0` AND settlement status ∉ {DISPUTED, CANCELLED} |

**Disputed-but-same-amount → MISMATCH** (architecture §7: post-include dispute while register item frozen).

No reconciliation job in v2.1A.

---

## 14. Security actor model

### 14.1 Internal auth threat model — **FROZEN**

Repository truth: `packages/shared-go/internalauth/auth.go` validates only `X-Internal-Service-Token` via shared configured token and constant-time comparison. It does **not** cryptographically bind actor headers to caller identity.

| Rule | Value |
|------|-------|
| `S2S_TOKEN_AUTHENTICATES_CALLER_CLASS` | YES — proves caller is trusted internal service class |
| `S2S_TOKEN_BINDS_ACTOR_HEADERS` | **NO** |
| `IDENTITY_HEADERS_WITHOUT_VALID_S2S_TOKEN` | **DENY** → 401 |
| `IDENTITY_HEADERS_WITH_VALID_S2S_TOKEN` | **TRUSTED_FORWARDED_CONTEXT** |
| `PUBLIC_CLIENT_IDENTITY_HEADERS_TRUSTED` | **NO** |

**Threat model:**

- Browser/public clients **never** reach `/internal/v1/freight-cost/*` directly.
- Actor headers are trusted **only after** `internalauth.Middleware` succeeds.
- Authenticity of user/company membership is the **upstream caller's responsibility** (future api-gateway v2.1E derives headers from verified JWT + company membership/RBAC — never client-supplied headers).
- freight-cost-service **cannot** detect forged actor headers from a caller that already possesses the valid shared S2S token. Do not claim otherwise in tests or docs.

### 14.2 Trusted actor headers — **IMPLEMENT_V2_1A**

After successful S2S auth, require:

| Header | Validation |
|--------|------------|
| `X-Tenant-ID` | Valid non-zero UUID |
| `X-User-ID` | Valid non-zero UUID (required for audit consistency with gateway/settlement contracts even though v2.1A is stateless) |
| `X-Company-ID` | Valid non-zero UUID |
| `X-Actor-Kind` | `BUYER` \| `CARRIER` only in v2.1A HTTP |

**Forbidden identity sources:** query params, request body, URL path (except resource IDs).

Malformed/missing actor headers after S2S auth → **400** `VALIDATION_ERROR`.

### 14.3 Platform admin policy — **FROZEN**

| Rule | Value |
|------|-------|
| `PLATFORM_ADMIN_DOMAIN_SCOPE` | DEFINED (future v2.1E gateway RBAC) |
| `PLATFORM_ADMIN_HTTP_ACCESS_V2_1A` | **DENY** |

v2.1A HTTP rejects `X-Actor-Kind: PLATFORM_ADMIN` with **400** — no stronger upstream proof mechanism exists today. Defer platform-admin reads to v2.1E gateway RBAC.

### 14.4 Authorization flow — **FROZEN**

```text
1. Authenticate S2S token (internalauth middleware)
2. Parse + validate trusted actor headers
3. Call tenant-scoped transport provider (tenantID + transportOrderID)
4. Receive canonical buyer_company_id / carrier_company_id from snapshot
5. Authorize actor company against canonical facts
6. Apply view-scope filter
7. Build response DTO
```

| Rule | Value |
|------|-------|
| `AUTHORIZATION_USES_CANONICAL_COMPANY_FACTS` | YES |
| `NO_CROSS_TENANT_EXISTENCE_PROBE` | YES — never global unscoped lookup |

### 14.5 HTTP error semantics — **FROZEN**

| Scenario | HTTP | Code |
|----------|------|------|
| Missing/invalid S2S token | 401 | UNAUTHORIZED |
| Malformed/missing actor headers (after S2S) | 400 | VALIDATION_ERROR |
| Resource UUID belongs to **another tenant** | **404** | NOT_FOUND |
| Same tenant, wrong buyer/carrier company | **403** | FORBIDDEN |
| Resource not found within tenant scope | **404** | NOT_FOUND |
| Invalid `X-Actor-Kind` (incl. PLATFORM_ADMIN) | **400** | VALIDATION_ERROR |

**Cross-tenant → 404**, not 403 — tenant-scoped provider lookup must not distinguish "exists in another tenant" from "does not exist". No global fallback queries.

Future public chain (v2.1E): browser → api-gateway (JWT + membership) → server-side internal call with token + server-derived headers.

---

## 15. Buyer vs carrier view matrix

Field visibility enforced in service layer **before** DTO serialization. Tests must use **populated synthetic fixtures** — v2.1A null fields alone do not prove masking.

| Field | Buyer | Carrier | v2.1A HTTP |
|-------|-------|---------|------------|
| `planned_amount` | YES | YES (only if `actor.company_id == carrier_company_id`) | YES |
| `accrued_amount` | YES | **NO** (null/stripped) | null in v2.1A |
| `forecast_exposure` | YES | **NO** | null in v2.1A |
| `current_actual_amount` | YES | YES (receivable context) | null in v2.1A |
| `final_actual_amount` | YES | YES | null in v2.1A |
| `current_variance_amount` | YES | **NO** | null in v2.1A |
| `final_variance_amount` | YES | **NO** | null in v2.1A |
| `billing_reconciliation_status` | YES | YES (payable state) | null in v2.1A |
| `paid_amount` | YES | YES | null in v2.1A |

**FC-A-SEC-003:** use synthetic `CostSummary` with populated buyer-only values; assert carrier view removes/nulls them.

**Carrier must never receive buyer-internal variance or forecast** — even when values are non-null in domain assembly pre-filter.

---

## 16. Tenant / company isolation

### 16.1 Lookup keys

Every read MUST include `tenantID`. Service methods:

```go
GetCostSummaryByTransportOrder(ctx, actor TrustedActor, transportOrderID uuid.UUID) (*CostSummary, error)
```

**Forbidden:** `GetByTransportOrderID(id)` without tenant + actor authorization.

### 16.2 Provider calls

Downstream internal calls MUST pass:
- `X-Internal-Service-Token`
- `X-Tenant-ID`
- Other actor headers as required by downstream contract

### 16.3 Wrong-company semantics

Align with billing-register: **403 Forbidden** (not 404) when resource exists but actor company cannot access — prevents cross-company existence leak within tenant.

---

## 17. Canonical read providers — gap analysis

### 17.1 Transport order pricing — **GAP (implementation required in v2.1A)**

**Current state:** `transport-order-service/internal/http/router.go` exposes:
- `POST /internal/v1/transport-orders/from-award-scope` (create only)
- `GET /v1/transport-orders/{id}` — **no rate snapshot**

**Required new endpoint (transport-order-service — cross-service change):**

```http
GET /internal/v1/transport-orders/{transportOrderId}/rate-snapshot
X-Internal-Service-Token: {shared token}
X-Tenant-ID: {tenant_uuid}
```

**Middleware:** reuse existing `shared-go/internalauth` on `/internal/v1` route group (same as `from-award-scope`).

**Lookup strategy (frozen — enables 404 vs 409):**

1. Tenant-scoped TO lookup by `(tenant_id, transport_order_id)`.
2. If TO missing → **404** `NOT_FOUND`.
3. If TO exists, inspect `pricing_model_version`:
   - `NULL` (legacy) or not `SNAPSHOT_V1` → **409** `CONFLICT` (unpriced TO).
   - `SNAPSHOT_V1` but snapshot row missing → **409** `CONFLICT` (data inconsistency).
4. If snapshot found → **200**.

| HTTP | Meaning |
|------|---------|
| `TO_MISSING_HTTP` | **404** |
| `TO_UNPRICED_HTTP` | **409** |

No global cross-tenant fallback. Snapshot query always includes `tenant_id`.

**Response DTO (minimal):**

```json
{
  "transport_order_id": "uuid",
  "tenant_id": "uuid",
  "buyer_company_id": "uuid",
  "carrier_company_id": "uuid",
  "snapshot_id": "uuid",
  "currency_code": "RUB",
  "total_amount": "150000.00",
  "pricing_source": "CONTRACT_RATE",
  "pricing_model_version": "SNAPSHOT_V1",
  "resolved_at": "2026-08-21T12:00:00Z"
}
```

`tenant_id` included in body for downstream audit/logging consistency (request is already tenant-scoped via header).

| Field | Value |
|------|-------|
| `TO_RATE_SNAPSHOT_SOURCE_VERSION` | **N/A** — omit `source_version`; use `pricing_model_version` separately |

### 17.2 Settlement — **NOT_FOUND internal read (defer impl to v2.1B)**

Public `GET /v1/freight-settlements/{id}` requires actor headers — not suitable for unattended S2S.

**Planned v2.1B endpoint:**

```http
GET /internal/v1/freight-settlements/by-transport-order/{transportOrderId}
```

### 17.3 Billing — **NOT_FOUND internal read (v2.1B)**

Planned: internal register snapshot by ID for reconciliation inputs.

### 17.4 Payment — **PARTIAL (repo only, no HTTP read)**

`payment-service` has `GetByBillingRegister` in repository; internal routes are write-only (`POST /internal/v1/payment-obligations/ensure`).

**v2.1A:** payment provider interface stub only — **payment-service not required at runtime for v2.1A**.

---

## 18. Internal API — freight-cost-service v2.1A

### 18.1 Endpoint

```http
GET /internal/v1/freight-cost/transport-orders/{transportOrderId}
X-Internal-Service-Token: ***
X-Tenant-ID: {uuid}
X-User-ID: {uuid}
X-Company-ID: {uuid}
X-Actor-Kind: BUYER|CARRIER
```

### 18.2 Response — planned-only stage

```json
{
  "tenant_id": "...",
  "transport_order_id": "...",
  "buyer_company_id": "...",
  "carrier_company_id": "...",
  "currency_code": "RUB",
  "data_stage": "PLANNED_ONLY",
  "financial_finality": "NOT_EVALUATED",
  "sources_available": ["TO_RATE_SNAPSHOT"],
  "planned_amount": "150000.00",
  "planned_source": {
    "source_service": "transport-order-service",
    "source_type": "TO_RATE_SNAPSHOT",
    "source_id": "...",
    "pricing_model_version": "SNAPSHOT_V1"
  },
  "accrued_amount": null,
  "forecast_exposure": null,
  "current_actual_amount": null,
  "final_actual_amount": null,
  "billing_register_amount": null,
  "paid_amount": null,
  "current_variance_amount": null,
  "final_variance_amount": null,
  "billing_reconciliation_status": null
}
```

Carrier view: same shape but variance/forecast/accrual fields **stripped or null** per §15.

### 18.3 Status codes

| Code | When |
|------|------|
| 200 | Success |
| 400 | Validation (bad UUID, missing/malformed actor headers, invalid actor kind) |
| 401 | Invalid/missing internal token |
| 403 | Same-tenant wrong buyer/carrier company (after canonical facts loaded) |
| 404 | Transport order not found **within tenant** (includes cross-tenant UUID) |
| 409 | TO exists but unpriced / snapshot missing (propagate from transport provider) |
| 502 | Downstream responded but violated trusted contract (invalid decimal, malformed DTO) |
| 503 | Downstream transport-order unavailable (timeout/connection/5xx) |

---

## 19. Partial data semantics

| `data_stage` | Meaning |
|--------------|---------|
| `PLANNED_ONLY` | Snapshot loaded; settlement chain **not loaded** (v2.1A default) |
| `ACCRUAL_AVAILABLE` | v2.1B+ |
| `CURRENT_ACTUAL_AVAILABLE` | v2.1B+ |
| `FINAL_ACTUAL_AVAILABLE` | v2.1B+ |
| `BILLING_LINKED` | v2.1B+ |
| `PAID` | v2.1B+ |

| v2.1A field | Value | Meaning |
|-------------|-------|---------|
| `DATA_STAGE_V2_1A` | `PLANNED_ONLY` | Only planned source queried |
| `FINANCIAL_FINALITY_V2_1A` | `NOT_EVALUATED` | Settlement provider **not called** — does not assert settlement absence |

### 19.1 Domain rules without live orchestration

Pure-domain functions are implemented in v2.1A even though HTTP does not load settlement data:

| Flag | Value |
|------|-------|
| `FINALITY_DOMAIN_FUNCTIONS` | YES |
| `ACCRUAL_DOMAIN_FUNCTIONS` | YES |
| `RECONCILIATION_DOMAIN_FUNCTIONS` | YES |
| `LIVE_SETTLEMENT_ORCHESTRATION` | NO |
| `LIVE_ACCRUAL_HTTP_OUTPUT` | NO |
| `LIVE_RECONCILIATION_HTTP_OUTPUT` | NO |

Consumers MUST use `data_stage` + `sources_available` — not infer completeness from null fields alone when ambiguous (future stages).

---

## 20. Persistence decision

| Decision | Value |
|----------|-------|
| `V2_1A_DATABASE_REQUIRED` | **NO** |
| `V2_1A_DATABASE` | Stateless read orchestration |
| Rationale | Avoid premature schema lock-in; v2.1B introduces `freight_cost` schema + ledger |

v2.1A service has **no `DATABASE_URL` requirement**. No repository persistence except optional in-memory test doubles.

---

## 21. Observability contract

Minimal metrics (register in `internal/platform/metrics/`):

| Metric | Labels |
|--------|--------|
| `freight_cost_http_requests_total` | `method`, `path`, `status` |
| `freight_cost_source_requests_total` | `source_service`, `operation`, `result` |
| `freight_cost_source_errors_total` | `source_service`, `error_code` |
| `freight_cost_currency_mismatch_total` | `operation` |

Plus shared `http_requests_total` from observability mount.

| Rule | Value |
|------|-------|
| `MONEY_IN_LOGS` | NO |
| `AUTH_TOKEN_IN_LOGS` | NO |

---

## 22. Config contract

Follow `{SERVICE}_SERVICE_PORT` / `{UPSTREAM}_SERVICE_URL` pattern from docker-compose.

| Env var | Required v2.1A | Default |
|---------|----------------|---------|
| `FREIGHT_COST_SERVICE_PORT` | NO | `8092` |
| `ENVIRONMENT` | NO | `development` |
| `LOG_LEVEL` | NO | `info` |
| `INTERNAL_SERVICE_TOKEN` | YES (non-empty in non-dev) | — |
| `TRANSPORT_ORDER_SERVICE_URL` | YES | `http://transport-order-service:8083` |

### 22.1 Internal token direction — **FROZEN**

Repository convention: all services use the **same shared** `INTERNAL_SERVICE_TOKEN` env var (`transport-order-service`, `billing-register-service`, `payment-service`, `rfx-service` clients).

| Config | Value |
|--------|-------|
| `INBOUND_INTERNAL_TOKEN_CONFIG` | `INTERNAL_SERVICE_TOKEN` — validates incoming `/internal/v1/*` |
| `OUTBOUND_TRANSPORT_TOKEN_CONFIG` | Same `INTERNAL_SERVICE_TOKEN` sent as `X-Internal-Service-Token` to transport-order-service |

Do not invent per-service downstream tokens unless repository convention changes.

Not required v2.1A:
- `DATABASE_URL`
- `BILLING_REGISTER_SERVICE_URL`
- `PAYMENT_SERVICE_URL`

---

## 23. Error model

Reuse `internal/platform/errors` + `respond` pattern (copy from control-tower-read-model-service).

| Condition | Code | HTTP |
|-----------|------|------|
| Validation | `VALIDATION_ERROR` | 400 |
| Unauthorized token | `UNAUTHORIZED` | 401 |
| Same-tenant wrong company | `FORBIDDEN` | 403 |
| Not found (incl. cross-tenant) | `NOT_FOUND` | 404 |
| Unpriced TO | `CONFLICT` | 409 |
| Downstream timeout/unavailable | `SERVICE_UNAVAILABLE` | **503** |
| Downstream invalid contract (bad decimal/DTO) | `BAD_GATEWAY` | **502** |
| Currency mismatch | `VALIDATION_ERROR` | 400 |

| Rule | Value |
|------|-------|
| `DOWNSTREAM_UNAVAILABLE_HTTP` | **503** |
| `DOWNSTREAM_INVALID_RESPONSE_HTTP` | **502** |

Do not return ambiguous 500 for downstream contract violations.

**Never leak:** SQL, tokens, cross-tenant existence, raw downstream payloads.

---

## 24. Test contract

Prefix families: `FC-A-DOM-*`, `FC-A-SEC-*`, `FC-A-API-*`, `FC-A-SRC-*`, `FC-A-E2E-*`

| ID | Description | Slice |
|----|-------------|-------|
| FC-A-DOM-001 | Decimal string parse/format helpers | v2.1A |
| FC-A-DOM-002 | nil `*Money` ≠ known zero `*Money` | v2.1A |
| FC-A-DOM-003 | Current actual APPROVED, no disputes | v2.1A |
| FC-A-DOM-004 | Current actual DISPUTED → NULL | v2.1A |
| FC-A-DOM-005 | Final actual READY_FOR_PAYMENT | v2.1A |
| FC-A-DOM-006 | APPROVED is not final | v2.1A |
| FC-A-DOM-007 | Currency mismatch deny | v2.1A |
| FC-A-DOM-008 | Billing reconciliation MATCH | v2.1A |
| FC-A-DOM-009 | Billing reconciliation MISMATCH | v2.1A |
| FC-A-DOM-010 | Billing reconciliation UNLINKED | v2.1A |
| FC-A-SEC-001 | Wrong-tenant resource UUID → 404 (no existence leak) | v2.1A |
| FC-A-SEC-002 | Same-tenant wrong buyer/carrier company → 403 | v2.1A |
| FC-A-SEC-003 | Populated buyer-only fields masked for carrier view | v2.1A |
| FC-A-SEC-004 | Identity headers without valid S2S token → 401 | v2.1A |
| FC-A-SEC-005 | Missing/malformed actor headers after S2S → 400 | v2.1A |
| FC-A-SRC-001 | Snapshot total decimal string preserved exactly | v2.1A |
| FC-A-SRC-002 | Downstream unavailable → 503 | v2.1A |
| FC-A-SRC-003 | Invalid downstream decimal → 502 | v2.1A |
| FC-A-SRC-004 | TO missing (404) vs unpriced (409) per frozen contract | v2.1A |
| FC-A-API-001 | Planned summary stable nullable JSON shape | v2.1A |
| FC-A-API-002 | Unknown financial amounts serialize as JSON `null` | v2.1A |
| FC-A-API-003 | Known zero serializes `"0.00"`, not null | v2.1A |
| FC-A-E2E-001 | Buyer same-company planned read success | v2.1A |
| FC-A-E2E-002 | Carrier same-company planned read success | v2.1A |
| FC-A-E2E-003 | Wrong-tenant resource indistinguishable from not found | v2.1A |

**Counts:** DOM=10, SEC=5, SRC=4, API=3, E2E=3 (**total 25**)

---

## 25. CI plan

Add to `.github/workflows/ci.yml` matrix:

```yaml
- services/freight-cost-service
```

Commands (same as other services):

```bash
cd services/freight-cost-service && go test ./...
```

No PostgreSQL service container for v2.1A (stateless). Integration tests use httptest/mock providers.

Optional: `go vet ./...` if sibling services run it (CI currently `go test` only).

---

## 26. Implementation file plan

| File | Purpose | New | Tests |
|------|---------|-----|-------|
| `services/freight-cost-service/go.mod` | Module | NEW | — |
| `cmd/server/main.go` | Bootstrap, HTTP server | NEW | — |
| `internal/config/config.go` | Env loading | NEW | unit |
| `internal/domain/money.go` | Money, Format/Parse helpers, currency validation | NEW | FC-A-DOM-001,007 |
| `internal/domain/money_test.go` | Money semantics | NEW | FC-A-DOM-001,002 |
| `internal/domain/source_ref.go` | CanonicalSourceRef (no synthetic version) | NEW | — |
| `internal/domain/finality.go` | Finality pure functions | NEW | FC-A-DOM-003..006 |
| `internal/domain/accrual.go` | Accrual/forecast calc (*Money) | NEW | FC-A-DOM-007 |
| `internal/domain/reconciliation.go` | MATCH/MISMATCH/UNLINKED | NEW | FC-A-DOM-008..010 |
| `internal/domain/cost_summary.go` | Domain aggregate (*Money fields) | NEW | — |
| `internal/domain/view_scope.go` | Buyer/carrier field mask | NEW | FC-A-SEC-003 |
| `internal/security/actor.go` | TrustedActor from headers | NEW | FC-A-SEC-004,005 |
| `internal/security/access.go` | Company authorization vs canonical facts | NEW | FC-A-SEC-001,002 |
| `internal/provider/transport_order.go` | Provider interface + DTO | NEW | — |
| `internal/client/transport_order/client.go` | HTTP adapter | NEW | FC-A-SRC-* |
| `internal/service/cost_service.go` | Orchestration | NEW | — |
| `internal/http/router.go` | chi routes | NEW | — |
| `internal/http/handlers/tenant.go` | Header parsing | NEW | — |
| `internal/http/dto/cost_summary.go` | Wire DTOs (*string amounts, no omitempty on reserved nulls) | NEW | FC-A-API-* |
| `internal/http/dto/money.go` | DTO money mapping | NEW | FC-A-API-003 |
| `internal/http/handlers/cost_handler.go` | Internal API | NEW | FC-A-API-* |
| `internal/platform/errors/errors.go` | AppError | NEW | — |
| `internal/platform/respond/respond.go` | JSON envelope | NEW | — |
| `internal/platform/logger/logger.go` | slog setup | NEW | — |
| `internal/platform/metrics/metrics.go` | Domain counters | NEW | — |
| `internal/integration/planned_cost/planned_cost_test.go` | E2E with mock TO | NEW | FC-A-E2E-* |
| `Dockerfile` | Container build | NEW | — |
| `README.md` | Service docs | NEW | — |

**Cross-service (transport-order-service):**

| File | Purpose |
|------|---------|
| `internal/http/handlers/rate_snapshot_internal_handler.go` | Internal GET handler | NEW |
| `internal/http/router.go` | Register GET route | MOD |
| `internal/service/rate_snapshot_read_service.go` | Lookup by TO id | NEW |

---

## 27. Cross-service change plan

| Service | Change required | Why | Runtime/contract | Risk |
|---------|-----------------|-----|------------------|------|
| **transport-order-service** | YES | Expose canonical snapshot via internal GET | RUNTIME | LOW — read-only, tenant-scoped |
| **freight-cost-service** | YES | New service (v2.1A main deliverable) | RUNTIME | MEDIUM — greenfield |
| billing-register-service | NO in v2.1A | Settlement internal read deferred v2.1B | — | — |
| payment-service | NO in v2.1A | Payment read deferred v2.1B | — | — |
| api-gateway | NO in v2.1A | Public routes v2.1E | — | — |
| identity-service | NO | Reuse existing auth/me in v2.1E | — | — |

**Goal:** minimize cross-service changes — **one** new internal read endpoint in transport-order-service.

---

## 28. v2.1A acceptance gates

| Gate | Required |
|------|----------|
| `SERVICE_BUILDS` | YES |
| `SERVICE_TESTS` | PASS |
| `NO_FLOAT_CANONICAL_MONEY` | YES |
| `NULL_ZERO_SEMANTICS` | PASS |
| `FINALITY_SEMANTICS` | PASS |
| `TENANT_ISOLATION` | PASS |
| `COMPANY_ISOLATION` | PASS |
| `CARRIER_INTERNAL_COST_DENY` | PASS |
| `CROSS_SERVICE_DB_READS` | 0 |
| `PUBLIC_API_ADDED` | NO |
| `LEDGER_IMPLEMENTED` | NO |
| `EVENT_INGESTION_IMPLEMENTED` | NO |
| `DATABASE_MIGRATION_COUNT` | 0 |

---

## 29. v2.1B handoff boundary

v2.1A MUST NOT implement:

- `freight_cost` PostgreSQL schema
- `cost_entry` ledger table
- Transactional settlement/billing outbox publishers
- Event consumers (settlement, billing, payment outbox)
- `source_event_id` persistence
- `source_revision` ingest pipeline
- Replay/idempotency (`UNIQUE(tenant_id, source_event_id)`)
- Projection rebuild job
- Accrual persistence / projection tables
- Reconciliation background worker
- Settlement/billing/payment provider **implementations** (interfaces only in v2.1A)
- Internal settlement/billing/payment read endpoints (spec'd here, built in v2.1B)

---

## 30. v2.1C handoff boundary

Leave for v2.1C:

- Current/final variance engine in API responses
- Variance reason attribution
- `charge_code` semantic classification (OQ-005)
- Accessorial double-count classification
- Scheduled reconciliation jobs
- Analytics aggregation / export

---

## 31. Implementation order

1. Service bootstrap (`go.mod`, config, main, observability mount)
2. Domain: money, null, currency validation
3. Domain: finality, accrual, reconciliation pure functions + tests (FC-A-DOM-*)
4. Security: TrustedActor, tenant/company access + tests (FC-A-SEC-*)
5. Provider interfaces
6. Transport-order internal endpoint (cross-service PR or same epic branch)
7. Transport HTTP client adapter + tests (FC-A-SRC-*)
8. Cost service orchestration (planned-only)
9. Internal HTTP handler + router + tests (FC-A-API-*)
10. View scope filtering (carrier mask)
11. Integration test (FC-A-E2E-001)
12. CI matrix entry
13. Dockerfile + README (dev)

**No ledger work at any step.**

---

## 32. Risk register

| ID | Severity | Description | Mitigation | Gate |
|----|----------|-------------|------------|------|
| RISK-A-001 | MEDIUM | billing-register settlement HTTP uses float64 | Parse at adapter boundary; fail closed; v2.1B internal DTO uses decimal strings | FC-A-SRC-003 |
| RISK-A-002 | HIGH | Transport internal snapshot API missing | Add GET in transport-order-service as v2.1A dependency | FC-A-SRC-004 |
| RISK-A-003 | MEDIUM | Same-tenant wrong-company authorization | 403 after canonical facts; FC-A-SEC-002 | SEC tests |
| RISK-A-004 | HIGH | Partial summaries misread as zero | Explicit `data_stage`; JSON null contract; FC-A-API-002 | API tests |
| RISK-A-005 | LOW | Premature DB schema | V2_1A_DATABASE_REQUIRED=NO | Planning gate |
| RISK-A-006 | HIGH | Carrier receives buyer variance | View filter with populated fixtures; FC-A-SEC-003 | SEC tests |
| RISK-A-007 | MEDIUM | Port 8091 conflict with contract-rate-service | Allocate **8092** for freight-cost-service | Config gate |

**Open for implementation review:** RISK-A-002 blocks E2E until transport endpoint ships (can ship in same v2.1A PR series).

---

## 33. Review questions (pre-freeze checklist)

| # | Question | Answer |
|---|----------|--------|
| 1 | Can v2.1A be stateless? | **YES** |
| 2 | Does v2.1A need payment-service yet? | **NO** (interface stub only) |
| 3 | Does v2.1A need settlement source now? | **NO runtime** — planned-only API; settlement provider v2.1B |
| 4 | Accrual too early? | Domain functions YES; event-driven NO |
| 5 | Can carrier receive buyer variance? | **NO** — filtered |
| 6 | Unknown amount → zero? | **NO** |
| 7 | float64 canonical money? | **NO** — decimal.Decimal |
| 8 | Cross-schema SQL? | **NO** |
| 9 | Internal endpoints tenant-scoped? | **YES** |
| 10 | Finality without billing import? | **YES** — local status strings |
| 11 | Accidental ledger work? | **NO** — explicit deferral |
| 12 | v2.1B responsibilities deferred? | **YES** — §29 |

---

## 34. Decision table

| Decision | Value |
|----------|-------|
| New service | **freight-cost-service** @ port **8092** |
| v2.1A persistence | **NO** (stateless) |
| Ledger in v2.1A | **NO** |
| Event ingestion in v2.1A | **NO** |
| Public API in v2.1A | **NO** |
| Canonical money | **shopspring/decimal.Decimal** domain; **string** wire |
| Optional money | **`*Money`** (nil = unknown) |
| Planned source | transport-order-service `TO_RATE_SNAPSHOT.total_amount` |
| Current actual source | billing-register-service (v2.1B) |
| Final actual source | Same field, status READY_FOR_PAYMENT (v2.1B) |
| Final actual status | **READY_FOR_PAYMENT** |
| Accrual rule | planned ex-VAT + approved accessorials ex-VAT |
| Forecast rule | planned + proposed accessorials (non-canonical) |
| Cross-service DB reads | **NO** |
| Buyer/carrier projection split | **YES** — service-layer view mask |
| Internal auth | **X-Internal-Service-Token** + trusted forwarded actor headers |
| S2S binds actor headers | **NO** |
| Platform admin v2.1A HTTP | **DENY** |
| Cross-tenant lookup | **404** |
| Same-tenant wrong company | **403** |
| Mixed currency | **DENY** |
| Payment provider in v2.1A | **NO** |
| Transport internal API | **NEW** GET rate-snapshot (404/409 semantics) |
| Source version | **N/A** (no synthetic version) |
| Downstream invalid DTO | **502** |
| Downstream unavailable | **503** |
| OPEN BLOCKER/HIGH/MEDIUM | **0** |

---

## 35. Planning PR metadata

- **Branch:** `arch/freight-cost-foundation-v2.1A-plan`
- **PR:** #43
- **Files changed:** this document only
- **Status after review:** ready for merge to main

---

## 36. Final implementation contract review

Independent review performed 2026-08-21 against PR #43 head and repository evidence.

### Findings register

| ID | Severity | Original issue | Resolution | Final rule |
|----|----------|----------------|------------|------------|
| F-A-HIGH-001 | HIGH | FC-A-SEC-004 claimed freight-cost-service could detect spoofed actor headers when caller already has valid S2S token | Rewrote threat model per `internalauth` reality | S2S authenticates caller **class** only; actor headers are trusted forwarded context after token gate; FC-A-SEC-004 → missing token → 401 |
| F-A-HIGH-002 | HIGH | Cross-tenant → 403 implies existence probe or global lookup | Tenant-scoped provider only | Cross-tenant UUID → **404**; same-tenant wrong company → **403** |
| F-A-MED-001 | MEDIUM | `Money` struct claimed both non-pointer Amount and nil-unknown semantics | Separated known vs unknown | `Money` = known fact; unknown = `*Money == nil` |
| F-A-MED-002 | MEDIUM | JSON null vs `omitempty` contradiction | Frozen stable wire shape | Reserved fields always present as explicit JSON `null`; no `omitempty` on nullable amounts |
| F-A-MED-003 | MEDIUM | `"source_version": 1` synthetic — no domain revision on RateSnapshot | Removed synthetic version | `SourceVersion` = nil/N/A; separate `pricing_model_version` |
| F-A-MED-004 | MEDIUM | Downstream invalid DTO mapped to 500/502 ambiguously | Single mapping | Invalid contract → **502**; unavailable → **503** |
| F-A-MED-005 | MEDIUM | Port 8091 assigned to freight-cost but already used by contract-rate-service | Rechecked repo | **8092** for freight-cost-service |
| F-A-LOW-001 | LOW | Planned known-zero marked NO without source evidence | Cited migration 000051 | `PLANNED_ZERO_ALLOWED=YES`; `total_amount >= 0` |
| F-A-LOW-002 | LOW | `financial_finality=UNKNOWN` implied no settlement | Renamed semantics | v2.1A → `NOT_EVALUATED` (source not loaded) |
| F-A-LOW-003 | LOW | PLATFORM_ADMIN accepted without proof mechanism | Denied at HTTP | `PLATFORM_ADMIN_HTTP_ACCESS_V2_1A=DENY`; defer to v2.1E |

### Review gate status

| Gate | Count |
|------|-------|
| OPEN_BLOCKER | 0 |
| OPEN_HIGH | 0 |
| OPEN_MEDIUM | 0 |

**Review verdict:** APPROVED for merge — implementation may proceed after PR #43 closes.

---

*End of contract freeze.*
