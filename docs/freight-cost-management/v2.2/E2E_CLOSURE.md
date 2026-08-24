# FREIGHT COST INTELLIGENCE v2.2G — E2E Closure

**Status:** Closed at integration scale  
**Date:** 2026-08-23  
**CI job:** `freight-cost-analytics-final-e2e` (+ `freight-cost-public-e2e` for v2.1E baseline)

---

## 1. Buyer E2E chain

| ID | Route | Test file | Test function | Assertion |
|----|-------|-----------|---------------|-----------|
| E2E-001 | `GET /api/v1/freight-costs/summary` | `services/api-gateway/internal/integration/freightcostpublic/security_integration_test.go` | `TestE2E001BuyerSummaryChain` | 200; `planned_total` present; trusted company forwarded downstream |
| E2E-002 | `GET /api/v1/freight-costs?limit=100` | same | `TestE2E002BuyerLedgerPagination` | 200; limit=100 honored |
| FC22F-SEC-002 | All analytics routes | same | `TestFC22F_SEC_002_ValidBuyerAnalyticsRoutes` | Buyer 200 on overview/lanes/carriers/accessorials/opportunities |

Downstream capture verifies S2S token injection and `ActorKind=BUYER`.

---

## 2. Carrier deny

| ID | Scope | Test function | Assertion |
|----|-------|---------------|-----------|
| E2E-003 | Legacy summary | `TestE2E003CarrierMaskHTTP` | Carrier summary 200; no `accrued_total` leak |
| FC-D-SEC-006 | Legacy analytics routes | `TestFC_D_SEC_006_CarrierDeniedBuyerAnalyticsEndpoints` | variance-detail, accessorial summary → 403 |
| FC-D-SEC-010 | Legacy list | `TestFC_D_SEC_010_CarrierJSONOmitsBuyerOnlyFields` | No `accrued_amount` in carrier JSON |
| FC22F-SEC-003 | Analytics overview | `TestFC22F_SEC_003_CarrierDeniedAnalyticsOverview` | 403 |
| FC22F-SEC-ALL | All analytics + opportunities | `TestFC22F_SEC_CarrierDeniedAllBuyerAnalyticsRoutes` | 403 on all five routes |
| FC-D-SEC-011..015 | v2.2G security closure | `TestFC_D_SEC_011_*` … `TestFC_D_SEC_015_*` | See `SECURITY_CLOSURE.md` |

---

## 3. Cross-tenant / cross-company

| ID | Scenario | Test function | Expected |
|----|----------|---------------|----------|
| E2E-004 | Cross-tenant legacy | `TestE2E004CrossTenantIsolation` | 404 summary/detail |
| FC-D-SEC-009 | Cross-tenant legacy | `TestFC_D_SEC_009_CrossTenantDeny` | 404 |
| FC22F-SEC-009 | Cross-tenant analytics | `TestFC22F_SEC_009_CrossTenantSpoofDenied` | 404 overview |
| FC-D-SEC-014 | Cross-tenant analytics | `TestFC_D_SEC_014_CrossTenantAnalyticsDenied` | 404 |
| FC22F-SEC-008 | Foreign company | `TestFC22F_SEC_008_ForeignCompanyMembershipDenied` | 403 all analytics routes |
| FC-D-SEC-013 | Foreign company | `TestFC_D_SEC_013_ForeignCompanyDeniedAnalytics` | 403 |
| FC22E-SEC-001 | Projection layer tenant | `TestFC22ESEC001CrossTenantIsolation` | Tenant B absent from tenant A benchmarks |
| FC22E-SEC-002 | Projection layer company | `TestFC22ESEC002CrossCompanyIsolation` | Company isolation within tenant |
| Spoof headers | Header injection | `TestPublicSecurityHeaderSpoofingIgnored` | JWT identity wins |

---

## 4. Currency

| ID | Layer | Test function | Assertion |
|----|-------|---------------|-----------|
| FC22E-CUR-001 | Projection | `TestFC22ECUR001CurrencyIsolation` | RUB/EUR separate benchmark cohorts |
| FC22B-CUR-001 | Projection | `TestFC22BRebuildTenantCurrencySeparated` | Tenant-scoped currency rows |
| FC22C-LP-004 | Lane | `TestFC22CLP004MultipleCurrenciesSeparate` | Lane aggregates per currency |
| FC22G-CUR-VAL | Public API | `TestFC22G_ParseAnalyticsPublicQueryInvalidCurrency` | Invalid ISO rejected |

Frontend mixed-currency UX: `apps/web-procurement/tests/freightCostIntelligence.test.ts` — `FC-D-INT-VST-003`, `FC-D-INT-VST-004`, `FC-D-INT-MON-001`.

---

## 5. Data quality

| Quality enum | Backend test | Frontend test |
|--------------|--------------|---------------|
| `INSUFFICIENT_SAMPLE` | `TestFC22EBM008SampleThreshold` | `FC-D-INT-VST-001`, `FC-D-INT-VST-006` |
| `NOT_AVAILABLE` | `TestLaneAndAccessorialNotAvailableSemantics` (legacy routes) | `FC-D-INT-VST-005` |
| `STALE` | Projection freshness fields in DTO | `FC-D-INT-VST-002`, `FC-D-INT-VST-006` |
| `MIXED_CURRENCY` | Multi-currency cohort separation | `FC-D-INT-VST-003`, `FC-D-INT-VST-004` |

Opportunity evidence always includes `sample_size` + `currency_code` (`TestFC22EOPP004EstimatedDelta`).

---

## 6. Opportunity E2E

| ID | Test | Assertion |
|----|------|-----------|
| FC22E-OPP-001 | `TestFC22EOPP001DeterministicID` | Stable opportunity ID |
| FC22E-OPP-004 | `TestFC22EOPP004EstimatedDelta` | Delta from observed/baseline, not client-computed |
| FC-D-SEC-012 | `TestFC_D_SEC_012_CarrierDeniedOpportunities` | Carrier cannot list opportunities |
| FC22G-SEC-INT | `TestFC22G_CarrierDeniedInternalAnalyticsRoutes` | Internal opportunity projection not public |

Frontend savings display (no client-side arithmetic):

- `FC-D-INT-SAV-001` — uses backend `estimated_delta`
- `FC-D-INT-SAV-002` — DTO money fields only
- `FC-D-INT-SAV-003` — composable documents backend-only savings

---

## 7. Frontend tests reference

| File | Scope |
|------|-------|
| `apps/web-procurement/tests/freightCostIntelligence.test.ts` | Intelligence tabs, data quality states, money formatting, i18n (RU/EN/ZH), data source paths |
| `apps/web-procurement/tests/freightCostWorkspace.test.ts` | Workspace security UX (`FC-D-SEC-001..005`), permissions, carrier mask |

Key intelligence tests:

- **Navigation:** `FC-D-INT-NAV-001/002` — buyer sees opportunities tab; carrier hidden
- **Data source:** `FC-D-INT-DS-001/002/003` — production paths include `/analytics/overview`
- **Carrier display:** `FC-D-INT-CAR-001..003` — UUID not shown as label; snapshot name used

Vitest runs in web-procurement CI; mocked adapters — **not** live browser E2E.

---

## 8. Live browser E2E (v2.2G.1)

| ID | Route | Framework | Assertion |
|----|-------|-----------|-----------|
| FC22G1-UI-001 | Overview | Playwright | Real `/api/v1/freight-costs/analytics/overview` 200; fixture planned total visible |
| FC22G1-UI-002..005 | Lanes/Carriers/Accessorials/Opportunities | Playwright | Real gateway-backed responses |
| FC22G1-UI-006 | Filters | Playwright | Currency query changes network request |
| FC22G1-UI-007 | Pagination | Playwright | `limit=1` honored |
| FC22G1-UI-008 | Feature flag off | Playwright | `/freight-costs` → `/freight-costs/unavailable`; env hint visible |

**Orchestrator:** `TestFC22G1_BrowserE2E_LiveBuyerFlow` (`BROWSER_E2E=1`).  
**Specs:** `apps/web-procurement/e2e/freight-cost-intelligence/`.  
**Mode:** `LIVE_BROWSER_BACKEND_MODE=REAL_LOCAL_STACK` — no HTTP mocks on primary path.

**Green CI evidence:** [run 32760275797](https://github.com/f9999737292-bit/Freihgt-platform/actions/runs/32760275797) job `freight-cost-intelligence-browser-e2e` (`97537223665`) @ `1394462` — UI-001..008 PASS in 36.8s.

---

## 9. Error handling

| ID | Test | Assertion |
|----|------|-----------|
| E2E-005 | `TestE2E005ServiceUnavailable` | Downstream unavailable → 502/503; no SQL leak |
| FC22F-SEC-AUTH | `TestFC22F_SEC_AnalyticsUnauthenticated` | Missing JWT → 401 |
| FC22F-SEC-012 | `TestFC22F_SEC_012_InternalRouteNotPubliclyExposed` | Internal path → 404 |

---

## 10. Closure verdict (v2.2G.1)

Gateway/service integration E2E from v2.2G remains valid. **Live browser buyer E2E** is green on PR #60 CI.

| Flag | Value |
|------|-------|
| `LIVE_BROWSER_E2E_READY` | **YES** |
| CI run | [32760275797](https://github.com/f9999737292-bit/Freihgt-platform/actions/runs/32760275797) |
| Browser job | `97537223665` |

**References:** `TEST_INVENTORY.md`, `SECURITY_CLOSURE.md`, `FINAL_CLOSURE.md`.
