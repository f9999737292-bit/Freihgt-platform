# FREIGHT COST INTELLIGENCE v2.2G — Security Closure

**Status:** Closed  
**Date:** 2026-08-23  
**Feature flags:** remain default OFF (`FREIGHT_COST_ANALYTICS_PROJECTION_ENABLED=false`, `NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED` off)

---

## 1. Threat model summary

Freight Cost Intelligence v2.2 exposes **buyer-only** tenant benchmarks, lane/carrier spend, accessorial breakdowns, and savings opportunities derived from internal projections. Primary threats:

| Threat | Impact | Control |
|--------|--------|---------|
| **Carrier commercial leakage** | Carrier learns buyer median spend, cohort percentiles, opportunity deltas | Gateway RBAC (`PolicyBuyerAnalytics`); carrier → 403; DTO redaction on legacy workspace routes |
| **Cross-tenant spoofing** | Tenant A reads tenant B analytics | JWT tenant is authoritative; spoofed `X-Tenant-ID` ignored; mismatch → 404 |
| **Cross-company bleed** | Buyer A reads buyer B company within tenant | Gateway validates `X-Company-ID` membership; foreign company → 403 |
| **Internal route exposure** | Public caller reaches S2S rebuild/benchmark reads | Internal paths under `/internal/v1/freight-cost/analytics/tenants/*` not registered on gateway |
| **Query injection / abuse** | Unbounded reads or SQL via sort/filter | Server-side allowlists; pagination max 100; validated ISO currency |
| **Evidence metadata leak** | Raw internal attribution in opportunity JSON | Public DTO allowlists evidence fields only |

Trust boundary: **Browser (JWT) → API Gateway (RBAC + membership) → freight-cost-service (S2S token, tenant+company scope)**.

---

## 2. FC-D-SEC-011..015 closure

| Test ID | Assertion | Test file | Test function | Result |
|---------|-----------|-----------|---------------|--------|
| FC-D-SEC-011 | Carrier denied `/analytics/overview` | `services/api-gateway/internal/integration/freightcostpublic/security_integration_test.go` | `TestFC_D_SEC_011_CarrierDeniedAnalyticsOverview` | **403** |
| FC-D-SEC-012 | Carrier denied `/opportunities` | same | `TestFC_D_SEC_012_CarrierDeniedOpportunities` | **403** |
| FC-D-SEC-013 | Buyer cannot access foreign company analytics | same | `TestFC_D_SEC_013_ForeignCompanyDeniedAnalytics` | **403** all routes |
| FC-D-SEC-014 | Cross-tenant analytics query denied | same | `TestFC_D_SEC_014_CrossTenantAnalyticsDenied` | **404** |
| FC-D-SEC-015 | Carrier response body omits benchmark/opportunity fields | same | `TestFC_D_SEC_015_CarrierAnalyticsBodyOmitsBenchmarkFields` | **403**; no `median`, `p25`, `p75`, `p90`, `estimated_delta`, `benchmark` in body |

Supporting v2.2F tests in the same file:

- `TestFC22F_SEC_CarrierDeniedAllBuyerAnalyticsRoutes` — all five analytics routes return 403 for carrier
- `TestFC22F_SEC_002_ValidBuyerAnalyticsRoutes` — buyer 200 + downstream S2S token
- `TestPublicSecurityHeaderSpoofingIgnored` — untrusted identity headers stripped

---

## 3. Carrier commercial leakage

**Controls:**

1. **Gateway policy:** `PolicyBuyerAnalytics` on `/api/v1/freight-costs/analytics/*` and `/opportunities` (see `v2.2F-PUBLIC-API-WORKSPACE.md`).
2. **403 before service:** Carrier never receives benchmark payload; FC-D-SEC-015 additionally asserts response body contains no percentile or delta keys even on denied responses.
3. **Legacy workspace mask:** FC-D-SEC-006/010 (v2.1E) remain in `freight-cost-public-e2e` — carrier list omits `accrued_amount`.

**Evidence:** `security_integration_test.go` — FC-D-SEC-011..015, FC22F-SEC-003, FC22F-SEC-ALL.

---

## 4. Spoofing

| Vector | Mitigation | Test |
|--------|------------|------|
| `X-Tenant-ID` header | Gateway uses JWT tenant only | `TestPublicSecurityHeaderSpoofingIgnored`, `TestFC22F_SEC_009_CrossTenantSpoofDenied` |
| `X-User-ID` / `X-Actor-Kind` | Stripped; actor derived from membership | `TestPublicSecurityHeaderSpoofingIgnored` |
| `X-Internal-Service-Token` | Never accepted from browser; gateway injects S2S token | `TestPublicSecurityHeaderSpoofingIgnored` |
| Cross-tenant resource access | 404 (not 403) to avoid existence leak | `TestFC_D_SEC_009_CrossTenantDeny`, `TestFC_D_SEC_014_CrossTenantAnalyticsDenied` |

---

## 5. Internal route boundary

Internal analytics endpoints (rebuild, benchmarks, opportunities, lane/carrier reads) live at:

```text
/internal/v1/freight-cost/analytics/tenants/{tenantId}/...
```

**Not exposed** via API gateway public router.

| Test | Assertion |
|------|-----------|
| `TestFC22F_SEC_012_InternalRouteNotPubliclyExposed` | `GET /internal/v1/freight-costs/analytics/overview` → 404 via gateway |
| `TestFC22G_CarrierDeniedInternalAnalyticsRoutes` | Benchmarks, opportunities, rebuild internal paths → 404 via gateway |

Service-side handlers: `internal/http/handlers/analytics_projection_handler.go`, `internal/http/router.go` (internal-only group).

---

## 6. Opportunity evidence allowlist

Internal storage (`domain.OpportunityEvidence`) may contain attribution metadata. Public API maps through `AnalyticsOpportunityEvidenceDTO` with fixed JSON keys only:

- `observed_cost`, `baseline_cost`, `potential_delta`
- `sample_size`, `currency_code`, `lane_key`
- `cohort_median`, `cohort_p90`
- `carrier_company_id`, `reason_code`, `occurrence_count`
- `accessorial_rate`, `baseline_p75_rate`

Mapping: `services/freight-cost-service/internal/http/dto/analytics_public.go` — `ToOpportunityEvidence`. No raw `evidence_json` passthrough; `schema_version` and internal rule metadata excluded from public DTO.

Benchmark isolation at projection layer: `TestFC22ESEC001CrossTenantIsolation`, `TestFC22ESEC002CrossCompanyIsolation` in `benchmark_opportunity_integration_test.go`.

---

## 7. Sort / filter validation

Public list endpoints parse query via `ParseAnalyticsPublicQuery` (`analytics_public_service.go`):

| Parameter | Rule | Test |
|-----------|------|------|
| `sort` | Allowlist map only; `-field` for descending | `TestFC22G_ParseAnalyticsPublicQuerySortInjection` |
| `limit` | Default 20, max **100**; over-max capped; ≤0 rejected | `TestFC22G_ParseAnalyticsPublicQueryPaginationAbuse` |
| `offset` | Non-negative | same |
| `from` / `to` | `from <= to` | `TestParseAnalyticsPublicQueryInvalidRange` |
| `currency` | ISO 4217 `[A-Z]{3}` | `TestFC22G_ParseAnalyticsPublicQueryInvalidCurrency` |

Injection attempts (`foo;drop table`, `' OR 1=1`, etc.) return validation errors — never reach SQL builder.

---

## 8. Residual risk & rollout

- **CRITICAL/HIGH findings:** 0 for v2.2G scope (automated gates above).
- **Production enablement:** NOT authorized — feature flags remain OFF pending controlled rollout decision (`FINAL_CLOSURE.md`).
- **Deferred:** `CLASSIFICATION_ANOMALY` opportunity type remains NOT_AVAILABLE (no public surface).

**References:** `SECURITY.md` §11, `TEST_INVENTORY.md`, `E2E_CLOSURE.md`.
