# MASTER SYSTEM TEST PLAN — Freight Platform v1

**Status:** DESIGN READY  
**Baseline:** `37c2eb62ccf9377359eb5c2fdf6f71eb9d187140`  
**Scope:** Full platform QA architecture — NOT v2.3 feature work, NOT production deployment.

---

## 1. Purpose

This plan defines **what**, **how**, **when**, and **by whom** the Freight Platform is tested as an integrated business system spanning procurement, execution, finance, intelligence, and operations.

It answers:

1. What must be tested?
2. In what sequence?
3. By which business role?
4. With which tenant/company context?
5. Which services participate?
6. Which APIs/events/databases are involved?
7. What test data is required?
8. What is the expected business result?
9. What is the negative/security result?
10. Which tests are automated?
11. Which require staging?
12. Which require human UAT?
13. Which are blockers for pilot?
14. Which are blockers for production?
15. How do we know the whole platform is ready?

---

## 2. Repository Inventory (Discovery)

### 2.1 Backend Services

| Service | Port | Domain | Compose default |
|---------|------|--------|-----------------|
| api-gateway | 8080 | Gateway, BFF (CT, tracking, driver, freight-cost, rates) | yes |
| identity-service | 8081 | Auth, users, roles | yes |
| company-service | 8082 | Companies, memberships | yes |
| transport-order-service | 8083 | Transport orders, locations, cargo, rate snapshots | yes |
| rfx-service | 8084 | RFx, tenders, freight requests, bids, award | yes |
| shipment-service | 8085 | Shipments, drivers, vehicles, outbox→Kafka | yes |
| document-service | 8086 | Documents, signing (mock EDO) | yes |
| billing-register-service | 8087 | Billing registers, settlements, UPD/closing docs | yes |
| payment-service | 8090 | Payment obligations, allocations, reconciliation | partial |
| contract-rate-service | 8091 | Contract/rate (internal S2S) | no |
| freight-cost-service | 8092 | Freight cost, analytics, intelligence projections | no |
| control-tower-read-model-service | 8089 | CT shipment status projection consumer | profile `read-model` |
| tracking-service | — | GPS/ETA/slots (via gateway BFF) | no |
| low-code-service | 8088 | Custom fields, form templates | yes |
| localization-service | — | Translations helper | no |

### 2.2 Frontend Apps

| App | Port | Status |
|-----|------|--------|
| web-admin | 3000 | Primary operator UI — CI build + lint |
| web-shipper | 3001 | Skeleton |
| web-carrier | 3002 | Skeleton |
| web-consignee | 3003 | Skeleton |
| web-finance | 3004 | Skeleton |
| web-procurement | 3005 | Procurement + Freight Cost Intelligence Playwright E2E |
| driver-mobile | — | Ionic/Capacitor — Vitest unit tests, Android build |

### 2.3 Database Schemas (PostgreSQL)

`core`, `transport`, `rfx`, `documents`, `billing`, `lowcode`, `control_tower`, `tracking`, `contract_rate`, `freight_cost` — **64 migration pairs** (`000001`–`000064`).

### 2.4 Event Bus

- **Broker:** Redpanda (profile `messaging`)
- **Topic:** `shipment.status.v1`
- **Publisher:** shipment-service transactional outbox (`transport.shipment_event_outbox`)
- **Consumers:** control-tower-read-model-service; payment outbox (billing sync) in payment-service

### 2.5 Existing Test Frameworks

| Layer | Framework | Location |
|-------|-----------|----------|
| Unit | Go `testing` | `services/**`, `packages/shared-go` |
| Integration | Go `-tags=integration` | per-service `internal/integration/` |
| API smoke | bash/curl/jq | `tests/integration/smoke-test.sh` |
| Load | k6 | `tests/performance/k6/` |
| Browser E2E | Playwright | `apps/web-procurement/e2e/freight-cost-intelligence/` |
| Driver mobile | Vitest | `apps/driver-mobile/tests/` |
| Security | Go unit + gateway RBAC tests | `*_rbac/*_test.go` |
| System catalog | YAML | `tests/system/test-catalog.yaml` (this plan) |

### 2.6 OpenAPI

Source of truth: `packages/openapi/openapi.yaml` — served at `http://localhost:8080/docs`.

---

## 3. Business Domain Map

| Domain | Services | Public API prefix | DB schema | Events | Status |
|--------|----------|-------------------|-----------|--------|--------|
| Identity / Auth | identity, api-gateway | `/api/v1/auth`, `/users`, `/roles` | core | — | IMPLEMENTED |
| Company / Tenant | company | `/api/v1/companies` | core | — | IMPLEMENTED |
| Transport Order | transport-order | `/api/v1/transport-orders`, `/locations`, `/cargoes` | transport | — | IMPLEMENTED |
| RFx / Tender / Bid | rfx | `/api/v1/rfx-*`, `/freight-requests`, `/bids`, `/carrier/*` | rfx | — | IMPLEMENTED |
| Evaluation / Award | rfx | `/bids`, award endpoints | rfx | — | IMPLEMENTED |
| Contract / Rate | contract-rate (+ gateway public rates) | `/api/v1/contracts`, `/rates` (gateway) | contract_rate | — | IMPLEMENTED (internal + public gateway) |
| Order Execution | shipment | `/api/v1/order-execution`, `/carrier/transport-orders` | transport | — | IMPLEMENTED |
| Shipment | shipment | `/api/v1/shipments` | transport | outbox→Kafka | IMPLEMENTED |
| Driver / Vehicle | shipment, api-gateway driver BFF | `/api/v1/drivers`, `/vehicles`, `/api/v1/driver/*` | transport | — | IMPLEMENTED |
| Tracking / ETA | tracking (+ gateway) | `/api/v1/shipments/{id}/tracking` | tracking | — | IMPLEMENTED (partial — no live GPS vendor) |
| Control Tower | api-gateway BFF, CT read-model | `/api/v1/control-tower/*` | control_tower | Kafka consumer | IMPLEMENTED |
| Documents / POD | document | `/api/v1/documents`, `/signing-sessions` | documents | — | IMPLEMENTED |
| Settlement | billing-register | `/api/v1/freight-settlements` | billing | — | IMPLEMENTED |
| Billing / Closing | billing-register | `/api/v1/billing-registers` | billing | — | IMPLEMENTED |
| Payments | payment | `/api/v1/payments`, `/payment-obligations` | billing | payment outbox | IMPLEMENTED |
| Freight Cost | freight-cost (+ gateway) | `/api/v1/freight-costs/*` | freight_cost | projection workers | IMPLEMENTED |
| Freight Cost Intelligence | freight-cost analytics | `/api/v1/freight-costs/analytics/*`, `/opportunities` | freight_cost | ingest/projection | IMPLEMENTED (v2.2 technical closure) |
| Low-code / Admin | low-code | `/api/v1/low-code` | lowcode | — | IMPLEMENTED |
| Localization | localization, packages/i18n | internal | core | — | PARTIAL |

**Not available on main:** dedicated settlement microservice (settlement lives in billing-register-service); production EDO/1C; live GPS telemetry vendor.

---

## 4. Test Level Hierarchy (L0–L14)

| Level | Purpose | Environment | Automation | Pilot blocker | Entry | Exit |
|-------|---------|-------------|------------|---------------|-------|------|
| **L0** Static / repo quality | No secrets, OpenAPI valid, scripts syntax | CI | AUTO_CI | yes | PR opened | openapi-check + repository-safety PASS |
| **L1** Unit | Domain logic, RBAC guards, money | local/CI | AUTO_CI | yes | code compiles | `go test ./...` PASS per module |
| **L2** Component | Handler + repo with mocks | local/CI | AUTO_CI | no | L1 pass | component tests PASS |
| **L3** Service integration | Postgres, single service | CI disposable DB | AUTO_CI | yes (critical domains) | migrations applied | integration tags PASS |
| **L4** Contract/API | OpenAPI drift, request/response | CI | AUTO_CI | yes | spec generated | contract tests PASS |
| **L5** Cross-service integration | 2+ services, S2S tokens | disposable compose | AUTO_CI / AUTO_STAGING | yes | platform-up | smoke + outbox e2e PASS |
| **L6** Browser/mobile E2E | UI flows | CI / staging | AUTO_CI (FC intel), AUTO_STAGING | P1 | gateway + web build | Playwright/Vitest PASS |
| **L7** Full business E2E | Golden path procurement→finance | disposable / staging | AUTO_STAGING | yes | seed tenants | FP-E2E-GOLDEN-* PASS |
| **L8** Performance/load | k6 profiles S1–S4 | staging / isolated | PERFORMANCE | no (pilot) | stable env | metrics within documented targets |
| **L9** Security/adversarial | BOLA, tenant escape, header spoof | CI + staging | SECURITY | yes | auth enabled | FP-SEC-* PASS |
| **L10** Recovery/DR | restart, rebuild, replay | staging | DR | production blocker | ops access | rebuild acceptance PASS |
| **L11** Staging acceptance | live stack validation | staging | AUTO_STAGING | yes | SSH/staging ready | staging pack PASS |
| **L12** UAT | business sign-off | UAT | MANUAL_UAT | yes | L7 + L11 | persona checklists signed |
| **L13** Pilot readiness | cohort ops | pilot | MANUAL_QA | — | staging PASS + blockers closed | pilot gate |
| **L14** Production readiness | go-live | prod | MANUAL + DR | — | pilot complete | exec sign-off |

---

## 5. Master Business Flows

Detailed step/state definitions: [BUSINESS_E2E_CATALOG.md](./BUSINESS_E2E_CATALOG.md).

### Flow 1 — Procurement to Award

`RFx/tender → invite → publish → bid → evaluate → award`

**State transitions (rfx):** DRAFT → PUBLISHED → BIDDING → EVALUATION → AWARDED (derive exact enums from `rfx-service/internal/domain`).

**Required tests:** happy path, two-carrier competition, late bid, withdrawn bid, unauthorized carrier, wrong tenant/company, duplicate submission, invalid currency/lane, deadline boundary, award idempotency, audit trail.

### Flow 2 — Award to Contract / Rate

`Award → contract → rate version → ACTIVE`

Contract-rate-service: DRAFT/ACTIVE/SUSPENDED/TERMINATED/EXPIRED/CANCELLED. Historical rate versions append-only — **no retroactive financial mutation**.

### Flow 3 — Order Creation

`award/rate → transport order (+ pricing snapshot)`

**Invariant:** execution order preserves commercial snapshot (`transport-order-service` rate snapshot integration tests exist).

### Flow 4 — Shipment Execution

**State machine** (from `shipment-service/internal/domain/shipment.go`):

```
CARRIER_ASSIGNED → ACCEPTED_BY_CARRIER → (VEHICLE_ASSIGNED|DRIVER_ASSIGNED)
→ PICKUP_SLOT_BOOKED → IN_PICKUP → LOADED → IN_TRANSIT
→ ARRIVED_AT_CONSIGNEE → UNLOADING → DELIVERED → DELIVERY_CONFIRMED
→ DOCUMENTS_COMPLETED → READY_FOR_BILLING → INCLUDED_IN_BILLING_REGISTER → FINANCIALLY_CLOSED
```

Cancel forbidden from DELIVERED onward.

### Flow 5 — Driver / Vehicle

Assignment scoped to tenant/carrier. Driver milestones (driver-mobile): `ARRIVED_AT_PICKUP`, `LOADING_STARTED`, `PICKUP_COMPLETED`, `DEPARTED_PICKUP`, `ARRIVED_AT_DELIVERY`, `UNLOADING_STARTED`, `DELIVERY_COMPLETED`, plus delay/problem/POD.

### Flow 6 — Control Tower

Driver/backend events → outbox → Kafka → CT projection → operator UI. Tests: delay, problem, risk, critical exception, acknowledge, case, deduplication, tenant isolation.

### Flow 7 — POD / Delivery

Document attach + signing mock → shipment status sync.

### Flow 8 — Settlement → Billing → Payment

Execution facts → freight settlement → billing register → UPD → payment obligation → PAID projection.

### Flow 9 — Freight Cost → Intelligence

Planned → accrued → billed → analytics overview/lanes/carriers/accessorials/opportunities. **Do not recompute backend metrics in frontend.**

---

## 6. Multi-Tenancy & Security Invariants

Deterministic tenants: **TENANT_A**, **TENANT_B** — see [TEST_DATA_MODEL.md](./TEST_DATA_MODEL.md).

Cross-tenant tests (**FP-E2E-SEC-001**): every read/mutation with Tenant B identity against Tenant A resources must fail with 401/403/404 per contract — no existence leak where concealment is designed.

Cross-company within tenant (**FP-E2E-SEC-002**): carrier A1 cannot read carrier A2 settlement unless permitted.

Header spoof tests: client-supplied `X-Tenant-ID`, `X-Company-ID`, `X-Role`, admin headers — gateway must not trust client overrides.

---

## 7. Financial Test Invariants

| ID | Rule | Evidence |
|----|------|----------|
| FIN-001 | DECIMAL_NOT_FLOAT | PostgreSQL NUMERIC + shopspring/decimal |
| FIN-002 | CURRENCY_REQUIRED | All money fields include currency |
| FIN-003 | NO_CROSS_CURRENCY_SUM | Domain tests in freight-cost |
| FIN-004 | ROUNDING_RULE_EXPLICIT | `"0.00"` vs null semantics (FC-A-API-*) |
| FIN-005 | HISTORICAL_SNAPSHOT_STABLE | Rate snapshot on transport order |
| FIN-006 | FINALIZED_AMOUNT_NOT_SILENTLY_RECALCULATED | Settlement/billing finalization |
| FIN-007 | ACCESSORIAL_TRACEABLE | freight_cost accessorial facts |
| FIN-008 | SETTLEMENT_TRACEABLE | billing-register settlement IDs |
| FIN-009 | BILLING_TRACEABLE | register → closing docs |

Existing freight-cost test IDs (`FC-A-*`) are mapped in traceability matrix — **not renamed**.

---

## 8. Test Identifier Standard

Prefix families:

`FP-AUTH-*`, `FP-TENANT-*`, `FP-RFX-*`, `FP-AWARD-*`, `FP-RATE-*`, `FP-ORDER-*`, `FP-SHIP-*`, `FP-DRV-*`, `FP-CT-*`, `FP-POD-*`, `FP-SETTLE-*`, `FP-BILL-*`, `FP-COST-*`, `FP-INTEL-*`, `FP-E2E-*`, `FP-SEC-*`, `FP-PERF-*`, `FP-DR-*`, `FP-UAT-*`

Legacy IDs preserved: `FC-A-*`, `AUTH-STG-*`.

---

## 9. Priority Model

| Priority | Meaning | Examples |
|----------|---------|----------|
| **P0** | Pilot blocker | auth, tenant isolation, award, order, shipment, driver, POD, settlement, billing, money, security |
| **P1** | Important operational | CT cases, tracking, payments, analytics |
| **P2** | Secondary | i18n edge, low-code extended |
| **P3** | Nice-to-have | extended perf profiles |

---

## 10. Automation Classification

Every catalog entry: `AUTO_CI` | `AUTO_STAGING` | `MANUAL_QA` | `MANUAL_UAT` | `PERFORMANCE` | `SECURITY` | `DR`

---

## 11. Test Waves

| Wave | Focus | Entry | Exit | Current blocker |
|------|-------|-------|------|-----------------|
| **W0** | Repo/CI baseline | branch pushed | CI green on main modules | none |
| **W1** | Identity/RBAC/Tenant | migrations | FP-AUTH + FP-TENANT P0 pass | none (CI) |
| **W2** | Procurement | W1 | FP-RFX/AWARD P0 pass | none (CI + smoke) |
| **W3** | Execution/Driver | W2 | FP-SHIP/DRV P0 pass | device tests need staging |
| **W4** | Control Tower | messaging profile | FP-CT P1 pass | staging for live CT |
| **W5** | Settlement/Billing | W3 | FP-SETTLE/BILL P0 pass | smoke partial (no FINANCIALLY_CLOSED auto) |
| **W6** | Freight Cost | W5 | FP-COST/INTEL P0 pass | CI e2e exists |
| **W7** | Golden E2E | W1–W6 | FP-E2E-GOLDEN-001 pass | disposable stack |
| **W8** | Security | W1 | FP-SEC-* pass | partial CI |
| **W9** | Performance/DR | staging | k6 + rebuild drills | staging blocked |
| **W10** | Staging | SSH restored | staging pack | **F22R001–008, SSH_ACCESS=BLOCKED** |
| **W11** | UAT | W10 | persona sign-off | UAT_READY=NO |

---

## 12. CI Strategy (Three Lanes)

### Lane 1 — FAST PR GATE (every PR)

- repository-safety, openapi-check
- `go test ./...` matrix (14 modules)
- web-admin build, driver-mobile vitest, scripts-check

### Lane 2 — INTEGRATION GATE (main + QA PRs)

- rfx-deadline-worker-integration (Postgres)
- contract-rate-public-e2e, freight-cost-public-e2e
- freight-cost-analytics-final-e2e
- control-tower integration jobs (where configured)

### Lane 3 — FULL ACCEPTANCE (nightly / release / QA branch)

- `make platform-up` + `integration-smoke-test`
- `outbox-end-to-end-test`, control-tower shadow acceptance
- freight-cost-intelligence-browser-e2e (Playwright)
- k6 smoke (optional, not in default CI)

**Cadence proposal (PROPOSED):** Lane 1 on every PR; Lane 2 on main merge + `test/*` branches; Lane 3 nightly + pre-pilot.

---

## 13. Entry / Exit Criteria

### System testing entry (current)

| Criterion | Status |
|-----------|--------|
| Master test plan documented | YES |
| Test catalog machine-readable | YES |
| CI unit/integration baseline | YES (main CI) |
| Disposable integration path | YES (compose + smoke) |
| Staging environment | NO (external blockers) |

**Verdict:** `READY_FOR_SYSTEM_TESTING=YES`

### System testing exit (PROPOSED thresholds)

| Metric | Target |
|--------|--------|
| P0 tests defined | 100% catalogued |
| P0 pass (disposable + CI) | 100% |
| P1 pass | ≥ 90% PROPOSED |
| Open Critical defects | 0 |
| Open High defects | 0 for pilot PROPOSED |
| Security gate FP-SEC-001/002 | PASS on staging |
| Golden path FP-E2E-GOLDEN-001 | PASS |
| Browser E2E (FC intelligence) | CI PASS (evidence) |
| DR rebuild acceptance | PASS on staging |

---

## 14. Staging Blockers (External)

Operational track: PR #61 (`ops/freight-cost-intelligence-controlled-rollout-v2.2`) — **do not modify in this QA branch**.

Known open: F22R001, F22R002, F22R003, F22R007, F22R008; `SSH_ACCESS=BLOCKED`; `STAGING_ENVIRONMENT_READY=NO`.

Test design proceeds independently.

---

## 15. Evidence Standard

Automated tests record where possible:

`TEST_ID`, `GIT_SHA`, `ENVIRONMENT`, `TIMESTAMP`, `ACTOR_ROLE`, `TENANT`, `RESULT`, `ERROR`, `CI_RUN`, screenshot/trace (browser).

Never log secrets.

---

## 16. Related Documents

- [BUSINESS_E2E_CATALOG.md](./BUSINESS_E2E_CATALOG.md)
- [ROLE_RBAC_TEST_MATRIX.md](./ROLE_RBAC_TEST_MATRIX.md)
- [TEST_DATA_MODEL.md](./TEST_DATA_MODEL.md)
- [SYSTEM_TEST_TRACEABILITY_MATRIX.md](./SYSTEM_TEST_TRACEABILITY_MATRIX.md)
- [UAT_PLAN.md](./UAT_PLAN.md)
- [TEST_ENVIRONMENT_MATRIX.md](./TEST_ENVIRONMENT_MATRIX.md)
- [SYSTEM_TEST_READINESS_REPORT.md](./SYSTEM_TEST_READINESS_REPORT.md)
