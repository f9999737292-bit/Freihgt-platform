# Business E2E Catalog — Freight Platform v1

Catalog of end-to-end business scenarios. Status values: `PLANNED`, `IMPLEMENTED_NOT_EXECUTED`, `BLOCKED_STAGING`, `PASS` (only with execution evidence).

Machine-readable index: [`tests/system/test-catalog.yaml`](../../tests/system/test-catalog.yaml)

---

## Golden Scenarios (Minimum Required)

### FP-E2E-GOLDEN-001 — Procurement → Execution → Finance → Intelligence

**Priority:** P0 | **Automation:** AUTO_STAGING | **Status:** IMPLEMENTED_NOT_EXECUTED (skeleton)

| Step | Actor | UI/API | Input | Expected state | Expected event | Negative check |
|------|-------|--------|-------|----------------|----------------|----------------|
| 1 | SHIPPER_LOGIST (Tenant A) | POST `/api/v1/auth/login` | credentials | JWT issued | — | invalid → 401 |
| 2 | SHIPPER_LOGIST | POST freight-request / rfx | lanes, currency RUB | RFx DRAFT | — | wrong tenant → 403/404 |
| 3 | SHIPPER_LOGIST | invite carriers A1, A2 | participant IDs | participants linked | — | cross-company invite rules |
| 4 | SHIPPER_LOGIST | publish | deadline | PUBLISHED/BIDDING | — | carrier cannot publish |
| 5 | CARRIER_DISPATCHER A1 | POST bid | price, lanes | bid SUBMITTED | — | wrong tenant → 404 |
| 6 | CARRIER_DISPATCHER A2 | POST bid | competing price | bid SUBMITTED | — | — |
| 7 | PROCUREMENT_MANAGER | evaluate/compare | — | comparison visible | — | driver cannot evaluate |
| 8 | PROCUREMENT_MANAGER | award to A1 | bid ID | AWARDED, winner A1 | award audit | loser cannot execute |
| 9 | System / buyer | contract+rate from award | effective dates | ACTIVE rate version | — | expired overlap rejected |
| 10 | SHIPPER_LOGIST | create transport order | award ref | order + rate snapshot | — | expired rate → 409 |
| 11 | CARRIER_DISPATCHER | create shipment | order ID | CARRIER_ASSIGNED | — | loser carrier → 403 |
| 12 | CARRIER_DISPATCHER | assign driver+vehicle | fleet IDs | DRIVER_ASSIGNED | — | cross-tenant IDs → 404 |
| 13 | DRIVER | accept + milestones | milestone events | through IN_TRANSIT | outbox events | illegal transition → 409 |
| 14 | CONTROL_TOWER_OPERATOR | GET summary | — | projection updated | Kafka consumed | Tenant B → empty/deny |
| 15 | DRIVER | delivery milestones + POD | POD doc | DELIVERED→READY_FOR_BILLING | — | wrong shipment → 404 |
| 16 | FINANCE_MANAGER | create settlement | shipment facts | settlement PLANNED/ACCRUED | — | cross-company → 403 |
| 17 | FINANCE_MANAGER | billing register + UPD | period | register APPROVED | — | wrong totals rejected |
| 18 | System | freight cost ingest | settlement/billing | cost ledger updated | projection job | no cross-currency sum |
| 19 | FINANCE_MANAGER | analytics overview | filters | labels, money, DQ | — | carrier masked fields |
| 20 | SHIPPER_LOGIST | variance/benchmark | eligible lane | opportunity explain | — | no frontend recompute |

**Business traceability chain (required at end):**

```
TENDER/RFx_ID → AWARD_ID → CONTRACT/RATE_ID → TRANSPORT_ORDER_ID → SHIPMENT_ID
→ DRIVER_ASSIGNMENT → POD_DOC_ID → SETTLEMENT_ID → BILLING_REGISTER_ID
→ FREIGHT_COST_REF → ANALYTICS_PROJECTION_REF
```

**Automation:** `tests/system/golden/fp_e2e_golden_001.sh` (skeleton — extends smoke-test pattern).

---

### FP-E2E-GOLDEN-002 — Spot / Mini Tender to Delivery

**Priority:** P1 | **Status:** PLANNED  
Uses freight-request spot path (confirmed in integration smoke).

---

### FP-E2E-NEG-001 — Losing Carrier Cannot Execute Winner Actions

**Priority:** P0 | **Status:** PLANNED

After award to Carrier A1, Carrier A2 attempts accept/status/driver/POD/settlement → 403/404.

---

### FP-E2E-SEC-001 — Cross-Tenant Isolation

**Priority:** P0 | **Automation:** AUTO_CI (partial) + AUTO_STAGING

Tenant B JWT against Tenant A IDs for tender, bid, order, shipment, settlement, billing, freight cost, analytics → fail safely.

---

### FP-E2E-SEC-002 — Cross-Company Isolation

**Priority:** P0 | Carrier A1 vs A2 within Tenant A.

---

### FP-E2E-DRV-001 / FP-E2E-DRV-002

Driver normal delivery and delay/problem — milestones from `apps/driver-mobile/src/utils/milestones.ts`.

---

### FP-E2E-FIN-001 / FP-E2E-COST-001 / FP-E2E-INTEL-001

Settlement→billing (partial smoke); rate→cost→variance (CI e2e); analytics (CI PASS v2.2G.1).

---

## Role Abuse (FP-SEC-010..018)

See [ROLE_RBAC_TEST_MATRIX.md](./ROLE_RBAC_TEST_MATRIX.md).
