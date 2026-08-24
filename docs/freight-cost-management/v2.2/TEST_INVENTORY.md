# FREIGHT COST INTELLIGENCE v2.2 — Test Inventory

**Status:** v2.2G.1 remediation  
**Date:** 2026-08-24  
**Scope:** FC22B/C/D/E/F/G tests discovered in repository

Legend: **v2.2G** = added or materially extended in v2.2G closure work.

---

## Integration & E2E

| ID | Area | Test file | Test function | Layer | Invariant | CI job | Status |
|----|------|-----------|---------------|-------|-----------|--------|--------|
| FC22B-EQV-001 | Projection core | `services/freight-cost-service/internal/integration/analytics/projection_integration_test.go` | `TestFC22BEqvRebuildMatchesIncremental` | integration | Full tenant rebuild ≡ incremental dirty batch | `freight-cost-analytics-final-e2e` | implemented |
| FC22B-IDEM-001 | Projection core | same | `TestFC22BRebuildIdempotent` | integration | Repeated rebuild produces identical period totals | `freight-cost-analytics-final-e2e` | implemented |
| FC22B-CUR-001 | Projection core | same | `TestFC22BRebuildTenantCurrencySeparated` | integration | Tenant A rebuild never includes tenant B currency rows | `freight-cost-analytics-final-e2e` | implemented |
| FC22C-LP-001 | Lane/carrier | `services/freight-cost-service/internal/integration/analytics/lane_carrier_integration_test.go` | `TestFC22CLP001OneLaneOneCurrency` | integration | Single lane cohort aggregates one currency | `freight-cost-analytics-final-e2e` | implemented |
| FC22C-LP-004 | Lane/carrier | same | `TestFC22CLP004MultipleCurrenciesSeparate` | integration | RUB/EUR lane rows remain separate | `freight-cost-analytics-final-e2e` | implemented |
| FC22C-EQV-001 | Lane/carrier | same | `TestFC22CEqv001RebuildMatchesIncremental` | integration | Lane/carrier full rebuild ≡ incremental | `freight-cost-analytics-final-e2e` | implemented |
| FC22C-SEC-001 | Lane/carrier | same | `TestFC22CSEC001TenantIsolation` | integration | Tenant B lane data absent after tenant A rebuild | `freight-cost-analytics-final-e2e` | implemented |
| FC22D-ACC-001 | Accessorial | `services/freight-cost-service/internal/integration/analytics/accessorial_enrichment_integration_test.go` | `TestFC22DACC001ApprovedAccessorialAggregation` | integration | Approved-only accessorial aggregation by category | `freight-cost-analytics-final-e2e` | implemented |
| FC22D-REC-001 | Accessorial | same | `TestFC22DREC001ReconciliationWithSettlementApprovedTotal` | integration | Period total reconciles settlement approved total | `freight-cost-analytics-final-e2e` | implemented |
| FC22D-ENRICH-002 | Accessorial | same | `TestFC22DENRICH002OrderReference` | integration | Order reference + carrier display snapshots populated | `freight-cost-analytics-final-e2e` | implemented |
| FC22D-EQV-001 | Accessorial | same | `TestFC22DEQV001RebuildMatchesIncremental` | integration | Accessorial full rebuild ≡ incremental | `freight-cost-analytics-final-e2e` | implemented |
| FC22E-BM-001 | Benchmark | `services/freight-cost-service/internal/integration/analytics/benchmark_opportunity_integration_test.go` | `TestFC22EBM001FiveValueMedian` | integration | Five-order cohort median computed correctly | `freight-cost-analytics-final-e2e` | implemented |
| FC22E-BM-003 | Benchmark | same | `TestFC22EBM003P25P75P90` | integration | P25/P75/P90 percentiles on eligible cohort | `freight-cost-analytics-final-e2e` | implemented |
| FC22E-BM-008 | Benchmark | same | `TestFC22EBM008SampleThreshold` | integration | Cohort below min sample → `INSUFFICIENT_SAMPLE` | `freight-cost-analytics-final-e2e` | implemented |
| FC22E-CUR-001 | Benchmark | same | `TestFC22ECUR001CurrencyIsolation` | integration | RUB/EUR benchmarks isolated | `freight-cost-analytics-final-e2e` | implemented |
| FC22E-SEC-001 | Benchmark | same | `TestFC22ESEC001CrossTenantIsolation` | integration | Tenant A benchmarks never include tenant B orders | `freight-cost-analytics-final-e2e` | implemented |
| FC22E-SEC-002 | Benchmark | same | `TestFC22ESEC002CrossCompanyIsolation` | integration | Buyer company X never affects company Y cohort | `freight-cost-analytics-final-e2e` | implemented |
| FC22E-OPP-001 | Opportunity | same | `TestFC22EOPP001DeterministicID` | integration | Opportunity ID deterministic for same inputs | `freight-cost-analytics-final-e2e` | implemented |
| FC22E-OPP-004 | Opportunity | same | `TestFC22EOPP004EstimatedDelta` | integration | Estimated delta uses observed/baseline amounts | `freight-cost-analytics-final-e2e` | implemented |
| FC22E-EQV-001 | Benchmark | same | `TestFC22EEQV001RebuildMatchesIncremental` | integration | Benchmark/opportunity full rebuild ≡ incremental | `freight-cost-analytics-final-e2e` | implemented |
| FC22F-SEC-002 | Public API | `services/api-gateway/internal/integration/freightcostpublic/security_integration_test.go` | `TestFC22F_SEC_002_ValidBuyerAnalyticsRoutes` | integration | Buyer receives 200 on all analytics routes; S2S forwarded | `freight-cost-analytics-final-e2e` | implemented |
| FC22F-SEC-003 | Public API | same | `TestFC22F_SEC_003_CarrierDeniedAnalyticsOverview` | integration | Carrier denied analytics overview (403) | `freight-cost-analytics-final-e2e` | implemented |
| FC22F-SEC-ALL | Public API | same | `TestFC22F_SEC_CarrierDeniedAllBuyerAnalyticsRoutes` | integration | Carrier denied all five buyer analytics routes | `freight-cost-analytics-final-e2e` | implemented |
| FC22F-SEC-008 | Public API | same | `TestFC22F_SEC_008_ForeignCompanyMembershipDenied` | integration | Foreign company membership → 403 on analytics | `freight-cost-analytics-final-e2e` | implemented |
| FC22F-SEC-009 | Public API | same | `TestFC22F_SEC_009_CrossTenantSpoofDenied` | integration | Cross-tenant analytics → 404 | `freight-cost-analytics-final-e2e` | implemented |
| FC22F-SEC-012 | Public API | same | `TestFC22F_SEC_012_InternalRouteNotPubliclyExposed` | integration | `/internal/v1/freight-costs/analytics/*` not exposed via gateway | `freight-cost-analytics-final-e2e` | implemented |
| FC22F-SEC-AUTH | Public API | same | `TestFC22F_SEC_AnalyticsUnauthenticated` | integration | Unauthenticated analytics → 401 | `freight-cost-analytics-final-e2e` | implemented |
| FC-D-SEC-011 | Security closure | same | `TestFC_D_SEC_011_CarrierDeniedAnalyticsOverview` | integration | Carrier denied analytics/overview | `freight-cost-analytics-final-e2e` | implemented **v2.2G** |
| FC-D-SEC-012 | Security closure | same | `TestFC_D_SEC_012_CarrierDeniedOpportunities` | integration | Carrier denied opportunities | `freight-cost-analytics-final-e2e` | implemented **v2.2G** |
| FC-D-SEC-013 | Security closure | same | `TestFC_D_SEC_013_ForeignCompanyDeniedAnalytics` | integration | Buyer A cannot read buyer B company scope | `freight-cost-analytics-final-e2e` | implemented **v2.2G** |
| FC-D-SEC-014 | Security closure | same | `TestFC_D_SEC_014_CrossTenantAnalyticsDenied` | integration | Cross-tenant projection query → 404 | `freight-cost-analytics-final-e2e` | implemented **v2.2G** |
| FC-D-SEC-015 | Security closure | same | `TestFC_D_SEC_015_CarrierAnalyticsBodyOmitsBenchmarkFields` | integration | Benchmark/opportunity fields absent in carrier body | `freight-cost-analytics-final-e2e` | implemented **v2.2G** |
| FC22G-SEC-INT | Internal boundary | same | `TestFC22G_CarrierDeniedInternalAnalyticsRoutes` | integration | Internal rebuild/benchmark/opportunity routes not public | `freight-cost-analytics-final-e2e` | implemented **v2.2G** |
| FC22G-N+1-001 | Performance | `services/freight-cost-service/internal/integration/analytics/n_plus_one_integration_test.go` | `TestFC22G_NPlusOne001EnrichmentUsesBatchNotPerOrder` | integration | 120-order rebuild uses O(1) batch enrichment calls | `freight-cost-analytics-final-e2e` | implemented **v2.2G** |
| FC22G-REBUILD-CONC | Rebuild | `services/freight-cost-service/internal/integration/analytics/rebuild_recovery_integration_test.go` | `TestFC22G_ConcurrentRebuildSameTenantSerialized` | integration | Concurrent tenant rebuilds serialize via advisory lock | `freight-cost-analytics-final-e2e` | implemented **v2.2G** |
| FC22G-EQV-FULL | Rebuild | same | `TestFC22G_FullStackRebuildIncrementalEquivalence` | integration | Full-stack B→E rebuild ≡ incremental | `freight-cost-analytics-final-e2e` | implemented **v2.2G** |
| FC22G-MIG-001 | Migration | `services/freight-cost-service/internal/integration/analytics/migration_gate_integration_test.go` | `TestFC22G_MigrationGateV22UpDown` | integration | Migrations 000061–000064 up/down reversible | `freight-cost-analytics-final-e2e` | implemented **v2.2G** |
| FC22G1-DR-001 | DR recovery | `full_projection_dr_integration_test.go` | `TestFC22G1_FullProjectionLossAndRebuildRestoresBusinessState` | integration | Full derived loss → checksum-equal rebuild | `freight-cost-analytics-final-e2e` | implemented **v2.2G.1** |
| FC22G1-DR-002 | DR failure | same | `TestFC22G1_FailedRebuildDoesNotPublishPartialFreshState` | integration | Failed rebuild rolls back; no false fresh state | `freight-cost-analytics-final-e2e` | implemented **v2.2G.1** |
| FC22G1-DR-003 | DR retry | same | `TestFC22G1_RetryAfterFailureRestoresBusinessState` | integration | Retry after injected failure succeeds | `freight-cost-analytics-final-e2e` | implemented **v2.2G.1** |
| FC22G1-PERF-001 | Performance | `performance_100k_integration_test.go` | `TestFC22G1_PERF001_100kAnalyticsRebuild` | integration | 100k rebuild + public query timing (`PERF_100K=1`) | `freight-cost-analytics-100k-gate` | **PASS** run 32760275797 **v2.2G.1** |
| FC22G1-UI-001..008 | Live browser | `apps/web-procurement/e2e/freight-cost-intelligence/*.spec.ts` | Playwright buyer + feature-flag flows | browser | Real stack, no HTTP mocks on primary path | `freight-cost-intelligence-browser-e2e` | **PASS** run 32760275797 **v2.2G.1** |

---

## Unit

| ID | Area | Test file | Test function | Layer | Invariant | CI job | Status |
|----|------|-----------|---------------|-------|-----------|--------|--------|
| FC22C-LANE-001..012 | Lane key | `services/freight-cost-service/internal/domain/lane_key_test.go` | `TestFC22CLane001MoscowToSPB` … `TestFC22CLane012SameInputsSameKey` | unit | Lane key normalization, directionality, exclusions | default `go test` | implemented |
| FC22G-BATCH-500 | Enrichment | `services/freight-cost-service/internal/client/company/client_batch_test.go` | `TestFC22G_BatchGetCompanyDisplayChunksAt500` | unit | Company batch API chunks at 500 IDs | default `go test` | implemented **v2.2G** |
| FC22G-BATCH-EMPTY | Enrichment | same | `TestFC22G_BatchGetCompanyDisplayEmpty` | unit | Empty batch is no-op | default `go test` | implemented **v2.2G** |
| FC22G-BATCH-BODY | Enrichment | same | `TestFC22G_BatchGetCompanyDisplayReadsBody` | unit | Batch request sends JSON body | default `go test` | implemented **v2.2G** |
| FC22G-SORT-VAL | Public query | `services/freight-cost-service/internal/service/analytics_public_service_test.go` | `TestFC22G_ParseAnalyticsPublicQuerySortInjection` | unit | Sort allowlist rejects injection | default `go test` | implemented **v2.2G** |
| FC22G-PAGE-VAL | Public query | same | `TestFC22G_ParseAnalyticsPublicQueryPaginationAbuse` | unit | Limit capped at 100; invalid offset rejected | default `go test` | implemented **v2.2G** |
| FC22G-CUR-VAL | Public query | same | `TestFC22G_ParseAnalyticsPublicQueryInvalidCurrency` | unit | Invalid ISO currency rejected | default `go test` | implemented **v2.2G** |

---

## Related (v2.1E baseline, not FC22-prefixed)

| ID | Area | Test file | Test function | Layer | CI job |
|----|------|-----------|---------------|-------|--------|
| FC-D-SEC-006..010 | Workspace security | `services/api-gateway/internal/integration/freightcostpublic/security_integration_test.go` | `TestFC_D_SEC_006_*` … `TestFC_D_SEC_010_*` | integration | `freight-cost-public-e2e` |
| E2E-001..005 | Buyer/carrier chain | same | `TestE2E001BuyerSummaryChain` … `TestE2E005ServiceUnavailable` | integration | `freight-cost-public-e2e` |
| FC-D-INT-* | Frontend intelligence | `apps/web-procurement/tests/freightCostIntelligence.test.ts` | Vitest `it('FC-D-INT-*')` | frontend | web-procurement unit CI |

---

## CI job summary

| Job | Package / filter |
|-----|------------------|
| `freight-cost-ledger-integration` | `go test -tags=integration ./internal/integration/analytics/...` (full analytics suite) |
| `freight-cost-analytics-race-gate` | `go test -race` on service/domain/security/company client + gateway freightcost |
| `freight-cost-analytics-final-e2e` | Analytics: `-run 'FC22G\|FC22B\|FC22C\|FC22D\|FC22E\|FC22F'`; Gateway: `-run 'FC22F\|FC_D_SEC\|FC22G'` |
| `freight-cost-analytics-100k-gate` | `-run TestFC22G1_PERF001_100kAnalyticsRebuild` with `PERF_100K=1` |
| `freight-cost-intelligence-browser-e2e` | `-run TestFC22G1_BrowserE2E_LiveBuyerFlow` with `BROWSER_E2E=1` |
| `freight-cost-public-e2e` | Full `freightcostpublic` integration suite |

**Green closure CI:** [run 32760275797](https://github.com/f9999737292-bit/Freihgt-platform/actions/runs/32760275797) @ PR #60 `1394462`.
