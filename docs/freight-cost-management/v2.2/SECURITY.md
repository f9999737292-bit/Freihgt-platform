# FREIGHT COST INTELLIGENCE v2.2 — Security

**Status:** Design freeze (v2.2A)  
**Date:** 2026-08-23  
**RBAC source:** `services/api-gateway/internal/freightcostrbac/`

---

## 1. Security Objectives

1. **Tenant isolation** — no cross-tenant data leakage in analytics queries or projections.
2. **Commercial sensitivity** — buyer freight spend, benchmarks, and savings are buyer-confidential.
3. **Carrier least privilege** — carriers see receivable-scoped operational data only.
4. **Server-side enforcement** — redaction in projection/query layer, not UI-only hiding.
5. **No cross-tenant benchmarking** — prohibited in v2.2 runtime.

---

## 2. Identity & Trust Boundaries

### JWT / trusted identity

- Public routes via `api-gateway` with standard JWT validation.
- `X-Tenant-ID` must match token tenant claim (existing platform pattern).
- `X-Company-ID` selects active company context for membership checks.

### Tenant boundary

| Layer | Enforcement |
|-------|-------------|
| API gateway | JWT tenant + company membership |
| freight-cost-service | `TrustedActor.TenantID` on all repository queries |
| Analytics projections | `tenant_id` **first** column in PK and all indexes |
| Benchmark cohorts | Never aggregate across tenants |

**Prohibited:** Unique constraints or query filters on `(lane_key, carrier_company_id)` without `tenant_id`.

### Company scope

| Role context | Scope |
|--------------|-------|
| Buyer company (shipper/forwarder/finance) | Buyer analytics on orders where `buyer_company_id` matches membership OR platform admin |
| Carrier company | Orders where `carrier_company_id` matches membership |
| Platform admin | Full tenant read (existing pattern) |

**Do not conflate** `tenant_id` (platform partition) with `company_id` (commercial party).

---

## 3. RBAC Model (Discovered)

Source: `freightcostrbac/policies.go`

### Policies

| Policy | Purpose |
|--------|---------|
| `PolicyRead` | Workspace list, detail, carrier-scoped views |
| `PolicyBuyerAnalytics` | Variance detail, aggregate buyer KPIs, future analytics/opportunities |

### Roles

**PolicyRead (includes carriers):**

- PLATFORM_ADMIN
- PROCUREMENT_MANAGER, SHIPPER_ADMIN, SHIPPER_LOGIST, FORWARDER_MANAGER, FINANCE_MANAGER
- CARRIER_ADMIN, CARRIER_DISPATCHER, CARRIER_ACCOUNTANT

**PolicyBuyerAnalytics (buyer-side only — no carrier roles):**

- PLATFORM_ADMIN
- PROCUREMENT_MANAGER, SHIPPER_ADMIN, SHIPPER_LOGIST, FORWARDER_MANAGER, FINANCE_MANAGER

---

## 4. Capability Matrix

| Capability | Buyer roles | Carrier roles | Tenant Admin | Notes |
|------------|-------------|---------------|--------------|-------|
| Overview (spend totals) | YES | **NO** (receivable subset) | YES (platform) | Carrier: own receivable lines only |
| Lane Analytics | YES | **NO** | YES | Buyer benchmark confidential |
| Carrier Analytics | YES | **NO** (self only, no benchmark) | YES | No competitor rows |
| Accessorial Analytics | YES | PARTIAL (own approved) | YES | No buyer total spend |
| Tenant Benchmark | YES | **NO** | YES | `Q16=NO` |
| Savings Opportunities | YES | **NO** | YES | Commercial sensitivity |
| Variance drivers | YES | **NO** | YES | v2.1E enforced |
| Reconciliation findings | YES | **NO** | YES | v2.1E enforced |

---

## 5. Carrier Visibility Rules

### Carriers MUST NOT see

- Buyer total freight spend across carriers
- Tenant historical benchmarks
- Competitor carrier prices or performance
- Buyer savings opportunities
- Buyer internal cost / margin / budget
- Lane spend rankings across carrier portfolio (buyer view)

### Carriers MAY see (PolicyRead, scoped)

- Own orders' receivable amounts (current/final actual where carrier is party)
- Own order list with sanitized labels
- Own accessorial lines submitted/approved

**Evidence:** v2.1E — `WorkspaceService.VarianceDetail` returns empty for `ActorKindCarrier`; carrier DTO uses `omitempty` on buyer-only fields; `SanitizeDisplayLabel()` prevents UUID-as-name leakage pattern.

### Server-side redaction

Analytics projection queries for carrier actors must:

1. Filter `carrier_company_id = actor.company_id`
2. Omit benchmark columns at SQL/DTO layer (not JSON null tricks)
3. Exclude opportunity and lane ranking endpoints entirely (403)

```
CARRIER_CAN_SEE_BUYER_BENCHMARK=NO
```

---

## 6. Commercial Sensitivity Classification

| Data element | Sensitivity | Buyer | Carrier | Cross-tenant |
|--------------|-------------|-------|---------|--------------|
| Carrier rate (per order) | HIGH | YES | Own only | NO |
| Lane spend ranking | HIGH | YES | NO | NO |
| Tenant benchmark median | HIGH | YES | NO | NO |
| Variance vs planned | MEDIUM | YES | NO | NO |
| Savings opportunity delta | HIGH | YES | NO | NO |
| Competitor comparison | CRITICAL | YES | NO | **PROHIBITED** |
| Accessorial breakdown | MEDIUM | YES | Partial | NO |
| Reconciliation mismatch | MEDIUM | YES | NO | NO |

---

## 7. Cross-Tenant Policy

| Rule | Value |
|------|-------|
| Cross-tenant analytics query | **PROHIBITED** |
| Cross-tenant benchmark | **PROHIBITED** in v2.2 |
| Cross-tenant pricing exposure | **NO** |

### Future market benchmark prerequisites (documentation only)

- Anonymization pipeline
- Minimum aggregated cohort size (k-anonymity)
- Anti-reidentification review
- Legal / customer consent
- Separate feature flag and security review

**Not in v2.2 scope.**

---

## 8. Projection Security

### Storage

- All analytics projection tables include `tenant_id` in PK.
- Row-level security: service-layer tenant filter (consistent with v2.1).
- No shared materialized views across tenants.

### Rebuild jobs

- Rebuild scoped to single tenant or full platform admin operation.
- Audit log rebuild initiation (pattern: control tower rebuild jobs).

---

## 9. API Security

- All analytics endpoints: `PolicyBuyerAnalytics` unless explicitly carrier-scoped alternate.
- Rate limiting: inherit gateway defaults.
- Pagination bounds prevent enumeration attacks on large tenants.
- No PII beyond company names already in workspace contract.

---

## 10. Data Quality & Information Leakage

Prevent inferential leakage:

| Anti-pattern | Prevention |
|--------------|------------|
| Empty list = "no analytics" | Return `data_capability: NOT_AVAILABLE` |
| Zero = "no spend" | Null amount + quality enum |
| Small sample benchmark | `INSUFFICIENT_SAMPLE` — hide percentiles |
| Mixed currency total | `MIXED_CURRENCY` — no synthetic sum |

---

## 11. Security Test Plan (v2.2G)

Extend v2.1E pattern (`FC-D-SEC-*`):

| Test ID | Assertion |
|---------|-----------|
| FC-D-SEC-011 | Carrier denied analytics/overview |
| FC-D-SEC-012 | Carrier denied opportunities |
| FC-D-SEC-013 | Buyer A cannot read Buyer B company scope |
| FC-D-SEC-014 | Cross-tenant projection query returns empty/403 |
| FC-D-SEC-015 | Benchmark fields absent in carrier response body |

---

## 12. References

- `services/api-gateway/internal/freightcostrbac/policies.go`
- `services/api-gateway/internal/integration/freightcostpublic/` (v2.1E security E2E)
- `docs/freight-cost-management/v2.1e-live-integration.md`
