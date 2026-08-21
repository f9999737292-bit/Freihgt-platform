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

| Field | Frozen value |
|-------|--------------|
| `SERVICE_PORT` | **8091** |
| Env var | `FREIGHT_COST_SERVICE_PORT` (fallback `HTTP_PORT`, default `8091`) |

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

```go
type Money struct {
    Amount   decimal.Decimal // canonical; never float64 in domain
    Currency string          // ISO 4217, required when Amount is present
}
```

- `IsZero()` allowed only when explicitly known-zero (e.g. approved accessorial sum with no lines).
- `IsUnknown()` when amount pointer is nil — distinct from zero.

#### `CostSourceRef` (canonical fact pointer) — **IMPLEMENT_V2_1A**

For v2.1A read-only facts (no event identity required yet):

```go
type CanonicalSourceRef struct {
    SourceService string    // e.g. "transport-order-service"
    SourceType    string    // TO_RATE_SNAPSHOT | FREIGHT_SETTLEMENT | ...
    SourceID      uuid.UUID
    SourceVersion int       // aggregate version when available; 0 if N/A
}
```

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
FINANCIAL_FINALITY_UNKNOWN          // no settlement
FINANCIAL_FINALITY_DRAFT            // settlement exists, not financially accepted
FINANCIAL_FINALITY_CURRENT_ACTUAL   // CURRENT_ACTUAL available
FINANCIAL_FINALITY_FINAL_ACTUAL     // FINAL_ACTUAL available
FINANCIAL_FINALITY_CANCELLED        // settlement cancelled
```

#### `BillingReconciliationStatus` — **IMPLEMENT_V2_1A** (pure function only)

```text
MATCH | MISMATCH | UNLINKED
```

#### `CostViewScope` — **IMPLEMENT_V2_1A**

```text
BUYER_COST_VIEW | CARRIER_RECEIVABLE_VIEW | PLATFORM_ADMIN_VIEW
```

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
| `FinancialFinality` | v2.1A | YES (`UNKNOWN` until settlement provider wired) |
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
| `planned_amount` | When TO has snapshot | NO (if TO exists without snapshot → 404/409) | NO | N/A — snapshot mandatory for priced TO |
| `accrued_amount` | NO | YES | YES (explicit zero accrual addon) | Settlement/accessorial source not loaded (v2.1A: always unavailable) |
| `forecast_exposure` | NO | YES | YES | No proposed accessorials |
| `current_actual_amount` | NO | YES | NO | Settlement missing, wrong status, or open dispute |
| `final_actual_amount` | NO | YES | NO | Not READY_FOR_PAYMENT |
| `billing_amount` | NO | YES | NO | Not linked to register |
| `paid_amount` | NO | YES | YES (obligation exists, nothing paid) | No obligation |
| `current_variance` | NO | YES | YES (explicit zero variance) | Planned or current actual unavailable |
| `final_variance` | NO | YES | YES | Planned or final actual unavailable |

**JSON rule:** unavailable → JSON `null` (pointer omitted in Go struct tags `omitempty` only when semantically unknown; never serialize `"0.00"` for unknown).

**Domain rule:** functions accepting `*decimal.Decimal` — nil means unknown; never default nil to zero inside financial derivations.

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

**Downstream boundary:** billing-register settlement HTTP DTOs currently expose `float64`. freight-cost-service **must parse at adapter boundary** into `decimal.Decimal` with fail-closed validation. Log parse failures; never silently round float64 into canonical money without explicit conversion helper.

Reuse pattern from `transport-order-service/internal/domain/rate_snapshot.go` (`TotalAmount decimal.Decimal`).

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
func CalculateAccrual(planned Money, approvedAccessorials []Money) (*Money, error)
func CalculateForecastExposure(planned Money, proposedAccessorials []Money) (*Money, error)
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

### 14.1 Trusted context — **IMPLEMENT_V2_1A**

v2.1A is **internal-only**. Pattern matches settlement/billing actor model + S2S gate:

| Dimension | Source |
|-----------|--------|
| S2S auth | `X-Internal-Service-Token` via `shared-go/internalauth` |
| `TenantID` | `X-Tenant-ID` header (required) |
| `UserID` | `X-User-ID` header (required for audited reads) |
| `CompanyID` | `X-Company-ID` header (required) |
| `ActorKind` | `X-Actor-Kind` header: `BUYER` \| `CARRIER` \| `PLATFORM_ADMIN` |

**Do not trust** identity from query params or request body.

Future public chain (v2.1E): browser → api-gateway (JWT + membership) → server-side internal call with token + derived headers.

### 14.2 Authorization rules

| Check | Behavior |
|-------|----------|
| Cross-tenant | 403 Forbidden |
| Same-tenant, buyer actor, wrong `buyer_company_id` | **403 Forbidden** (align with `ValidateSettlementAccess`) |
| Same-tenant, carrier actor, wrong `carrier_company_id` | **403 Forbidden** |
| Resource not found within tenant | **404 Not Found** |
| Spoofed token | 401 Unauthorized |

Platform admin: may read any company within tenant (v2.1E hardening may add explicit permission code `freight_cost.read`).

---

## 15. Buyer vs carrier view matrix

Field visibility for **future** projection/API layers. v2.1A internal API returns full struct; **view filter applied in service layer** before response.

| Field | Buyer | Carrier | Platform Admin |
|-------|-------|---------|----------------|
| `planned_amount` | YES | YES (agreed freight) | YES |
| `accrued_amount` | YES | NO | YES |
| `forecast_exposure` | YES | NO | YES |
| `current_actual_amount` | YES | YES (receivable context) | YES |
| `final_actual_amount` | YES | YES | YES |
| `current_variance_amount` | YES | **NO** | YES |
| `final_variance_amount` | YES | **NO** | YES |
| `billing_reconciliation_status` | YES | YES (payable state) | YES |
| `paid_amount` | YES | YES | YES |
| Buyer internal benchmark / cross-carrier analytics | YES | **NO** | YES |

**Carrier must never receive buyer-internal variance or forecast.**

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
Authorization: X-Internal-Service-Token
X-Tenant-ID: {tenant_uuid}
```

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
  "resolved_at": "2026-08-21T12:00:00Z",
  "version": 1
}
```

Implementation note: reuse `priced_order_repository.getSnapshotByID` / snapshot by transport order lookup.

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
X-Actor-Kind: BUYER|CARRIER|PLATFORM_ADMIN
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
  "financial_finality": "UNKNOWN",
  "sources_available": ["TO_RATE_SNAPSHOT"],
  "planned_amount": "150000.00",
  "planned_source": {
    "source_service": "transport-order-service",
    "source_type": "TO_RATE_SNAPSHOT",
    "source_id": "...",
    "source_version": 1
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
| 400 | Validation (bad UUID, missing headers) |
| 401 | Invalid/missing internal token |
| 403 | Tenant/company authorization failure |
| 404 | Transport order or snapshot not found in tenant |
| 409 | TO exists but pricing snapshot missing (unpriced TO) |
| 503 | Downstream transport-order unavailable |

---

## 19. Partial data semantics

| `data_stage` | Meaning |
|--------------|---------|
| `PLANNED_ONLY` | Snapshot loaded; no settlement chain (v2.1A default) |
| `ACCRUAL_AVAILABLE` | v2.1B+ |
| `CURRENT_ACTUAL_AVAILABLE` | v2.1B+ |
| `FINAL_ACTUAL_AVAILABLE` | v2.1B+ |
| `BILLING_LINKED` | v2.1B+ |
| `PAID` | v2.1B+ |

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
| `FREIGHT_COST_SERVICE_PORT` | NO | `8091` |
| `ENVIRONMENT` | NO | `development` |
| `LOG_LEVEL` | NO | `info` |
| `INTERNAL_SERVICE_TOKEN` | YES (non-empty in non-dev) | — |
| `TRANSPORT_ORDER_SERVICE_URL` | YES | `http://transport-order-service:8083` |

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
| Forbidden tenant/company | `FORBIDDEN` | 403 |
| Not found | `NOT_FOUND` | 404 |
| Unpriced TO | `CONFLICT` | 409 |
| Downstream timeout/5xx | `SERVICE_UNAVAILABLE` | 503 |
| Currency mismatch | `VALIDATION_ERROR` | 400 |
| Downstream invalid decimal | `INTERNAL_ERROR` | 502/500 (fail closed, log) |

**Never leak:** SQL, tokens, cross-tenant existence, raw downstream payloads.

---

## 24. Test contract

Prefix families: `FC-A-DOM-*`, `FC-A-SEC-*`, `FC-A-API-*`, `FC-A-SRC-*`, `FC-A-E2E-*`

| ID | Description | Slice |
|----|-------------|-------|
| FC-A-DOM-001 | Planned decimal parsing | v2.1A |
| FC-A-DOM-002 | NULL ≠ zero | v2.1A |
| FC-A-DOM-003 | Current actual APPROVED, no disputes | v2.1A |
| FC-A-DOM-004 | Current actual DISPUTED → NULL | v2.1A |
| FC-A-DOM-005 | Final actual READY_FOR_PAYMENT | v2.1A |
| FC-A-DOM-006 | APPROVED is not final | v2.1A |
| FC-A-DOM-007 | Mixed currency deny | v2.1A |
| FC-A-DOM-008 | Billing reconciliation MATCH | v2.1A |
| FC-A-DOM-009 | Billing reconciliation MISMATCH | v2.1A |
| FC-A-DOM-010 | Billing reconciliation UNLINKED | v2.1A |
| FC-A-SEC-001 | Cross-tenant deny | v2.1A |
| FC-A-SEC-002 | Same-tenant cross buyer company deny | v2.1A |
| FC-A-SEC-003 | Carrier buyer-internal view deny | v2.1A |
| FC-A-SEC-004 | Spoofed tenant/user/company denied | v2.1A |
| FC-A-SRC-001 | Transport snapshot canonical total | v2.1A |
| FC-A-SRC-002 | Downstream unavailable → 503 | v2.1A |
| FC-A-SRC-003 | Invalid decimal from downstream → fail closed | v2.1A |
| FC-A-API-001 | Planned-only summary; nullable unknown fields | v2.1A |
| FC-A-API-002 | Unknown amount never serialized as `"0.00"` | v2.1A |
| FC-A-E2E-001 | Tenant/company scoped planned cost read | v2.1A |

**Counts:** DOM=10, SEC=4, SRC=3, API=2, E2E=1 (total 20 planned)

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
| `internal/domain/money.go` | Money, currency validation | NEW | FC-A-DOM-001,007 |
| `internal/domain/nullable.go` | NULL vs zero helpers | NEW | FC-A-DOM-002 |
| `internal/domain/source_ref.go` | CanonicalSourceRef | NEW | — |
| `internal/domain/finality.go` | Finality pure functions | NEW | FC-A-DOM-003..006 |
| `internal/domain/accrual.go` | Accrual/forecast calc | NEW | FC-A-DOM-007 |
| `internal/domain/reconciliation.go` | MATCH/MISMATCH/UNLINKED | NEW | FC-A-DOM-008..010 |
| `internal/domain/cost_summary.go` | Aggregate + data_stage | NEW | — |
| `internal/domain/view_scope.go` | Buyer/carrier field mask | NEW | FC-A-SEC-003 |
| `internal/security/actor.go` | TrustedActor from headers | NEW | FC-A-SEC-001..004 |
| `internal/security/access.go` | Tenant/company checks | NEW | FC-A-SEC-* |
| `internal/provider/transport_order.go` | Provider interface + DTO | NEW | — |
| `internal/client/transport_order/client.go` | HTTP adapter | NEW | FC-A-SRC-* |
| `internal/service/cost_service.go` | Orchestration | NEW | — |
| `internal/http/router.go` | chi routes | NEW | — |
| `internal/http/handlers/cost_handler.go` | Internal API | NEW | FC-A-API-* |
| `internal/http/handlers/tenant.go` | Header parsing | NEW | — |
| `internal/platform/errors/errors.go` | AppError | NEW | — |
| `internal/platform/respond/respond.go` | JSON envelope | NEW | — |
| `internal/platform/logger/logger.go` | slog setup | NEW | — |
| `internal/platform/metrics/metrics.go` | Domain counters | NEW | — |
| `internal/integration/planned_cost/planned_cost_test.go` | E2E with mock TO | NEW | FC-A-E2E-001 |
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
| RISK-A-002 | HIGH | Transport internal snapshot API missing | Add GET in transport-order-service as v2.1A dependency | Block impl until endpoint exists |
| RISK-A-003 | MEDIUM | Same-tenant wrong-company authorization | Mirror billing 403 semantics; FC-A-SEC-002 | SEC tests |
| RISK-A-004 | HIGH | Partial summaries misread as zero | Explicit `data_stage`; null JSON; FC-A-API-002 | API tests |
| RISK-A-005 | LOW | Premature DB schema | V2_1A_DATABASE_REQUIRED=NO | Planning gate |
| RISK-A-006 | HIGH | Carrier receives buyer variance | View scope filter in service layer; FC-A-SEC-003 | SEC tests |

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
| New service | **freight-cost-service** @ port **8091** |
| v2.1A persistence | **NO** (stateless) |
| Ledger in v2.1A | **NO** |
| Event ingestion in v2.1A | **NO** |
| Public API in v2.1A | **NO** |
| Canonical money | **shopspring/decimal.Decimal**, scale 2, JSON string |
| Planned source | transport-order-service `TO_RATE_SNAPSHOT.total_amount` |
| Current actual source | billing-register-service `freight_settlements.total_without_vat` (v2.1B) |
| Final actual source | Same field, status READY_FOR_PAYMENT (v2.1B) |
| Final actual status | **READY_FOR_PAYMENT** |
| Accrual rule | planned ex-VAT + approved accessorials ex-VAT |
| Forecast rule | planned + proposed accessorials (non-canonical) |
| Cross-service DB reads | **NO** |
| Buyer/carrier projection split | **YES** — service-layer view mask |
| Internal auth pattern | **X-Internal-Service-Token** + trusted actor headers |
| Mixed currency | **DENY** |
| Payment provider in v2.1A | **NO** |
| Transport internal API | **NEW** GET rate-snapshot |
| OPEN BLOCKER/HIGH | **0** |

---

## 35. Planning PR metadata

- **Branch:** `arch/freight-cost-foundation-v2.1A-plan`
- **Files changed:** this document only
- **Do not merge** in planning task
- **Do not implement** v2.1A runtime in planning task

---

*End of contract freeze.*
