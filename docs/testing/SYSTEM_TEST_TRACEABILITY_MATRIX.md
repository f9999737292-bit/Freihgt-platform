# System Test Traceability Matrix — Freight Platform v1

| Business Capability | Test IDs | Services | UI | APIs | Data | Status |
|---------------------|----------|----------|-----|------|------|--------|
| Login / JWT | FP-AUTH-001..006 | identity, api-gateway | all apps | `/api/v1/auth/login` | core.users | AUTO_CI |
| Tenant isolation | FP-E2E-SEC-001, FP-TENANT-* | all | — | all `/api/v1/*` | core.tenants | PLANNED |
| Company CRUD | FP-TENANT-010 | company | web-admin | `/api/v1/companies` | core.companies | smoke PASS |
| Transport order | FP-ORDER-001..010 | transport-order | web-admin | `/transport-orders` | transport.* | smoke PASS |
| RFx / tender | FP-RFX-001..010 | rfx | web-procurement | `/freight-requests`, `/bids` | rfx.* | smoke PASS |
| Award | FP-AWARD-001..003 | rfx | web-procurement | award endpoints | rfx.bids | PLANNED |
| Contract / rate | FP-RATE-001..007 | contract-rate, gateway | web-admin | `/contracts`, `/rates` | contract_rate.* | PLANNED |
| Rate snapshot on order | FP-ORDER-005, FC-A-SRC-001 | transport-order | — | internal snapshot | transport.orders | AUTO_CI |
| Shipment lifecycle | FP-SHIP-001..005 | shipment | web-admin | `/shipments` | transport.shipments | smoke partial |
| Driver milestones | FP-E2E-DRV-001, FP-DRV-* | shipment, gateway driver | driver-mobile | `/api/v1/driver/*` | transport.drivers | PLANNED |
| Fleet assign | FP-DRV-001 | shipment, fleetrbac | web-admin | `/drivers`, `/vehicles` | transport.* | smoke PASS |
| Outbox / Kafka | FP-CT-001 | shipment, CT read-model | — | Kafka `shipment.status.v1` | outbox, control_tower | AUTO_CI |
| Control Tower | FP-CT-001..005 | api-gateway, CT | web-admin CT | `/control-tower/*` | control_tower.* | BLOCKED_STAGING |
| Tracking / ETA | FP-TRACK-* | tracking, gateway | web-admin | `/shipments/{id}/tracking` | tracking.* | PARTIAL |
| POD / documents | FP-POD-001..005 | document | web-admin, driver-mobile | `/documents` | documents.* | smoke PASS |
| Settlement | FP-SETTLE-001..010 | billing-register | web-finance | `/freight-settlements` | billing.* | PLANNED |
| Billing / UPD | FP-BILL-001..010 | billing-register | web-finance | `/billing-registers` | billing.* | smoke PASS |
| Payments | FP-PAY-001..010 | payment | web-finance | `/payments`, `/payment-obligations` | billing.payment_* | PLANNED |
| Freight cost | FP-COST-001, FC-A-* | freight-cost | web-procurement | `/freight-costs/*` | freight_cost.* | AUTO_CI PASS |
| Intelligence | FP-E2E-INTEL-001, FC-A-* | freight-cost | web-procurement e2e | `/analytics/*`, `/opportunities` | projections | CI PASS |
| Low-code | FP-LC-* | low-code | web-admin | `/low-code` | lowcode.* | compliance test |
| i18n RU/EN/ZH | FP-I18N-* | localization, apps | all | — | core.translations | PLANNED |
| Golden E2E | FP-E2E-GOLDEN-001 | all | multi | chain | all schemas | SKELETON |
| Security abuse | FP-SEC-* | api-gateway RBAC | — | mutations | — | PLANNED |
| Performance | FP-PERF-* | all | — | k6 scripts | — | DESIGN ONLY |
| DR / rebuild | FP-DR-* | CT, shipment | — | rebuild protocol | snapshots | staging blocked |

## Legacy ID mapping (do not rename)

| Legacy | Maps to |
|--------|---------|
| FC-A-DOM-* | FP-COST domain invariants |
| FC-A-SEC-* | FP-SEC / FP-E2E-SEC cost isolation |
| FC-A-E2E-* | FP-E2E-COST-001 |
| AUTH-STG-* | FP-AUTH staging manual (low-code pilot) |

## Coverage summary

| Metric | Count |
|--------|-------|
| DOMAINS_TOTAL | 19 |
| DOMAINS_WITH_UNIT_TESTS | 17 |
| DOMAINS_WITH_INTEGRATION | 14 |
| DOMAINS_WITH_PUBLIC_API_TESTS | 10 |
| DOMAINS_WITH_BROWSER_E2E | 2 (web-procurement FC intel, web-admin lint only) |
| DOMAINS_WITH_FULL_BUSINESS_E2E | 1 (partial smoke chain) |

| Classification | Count (catalog) |
|----------------|-----------------|
| AUTOMATED_COVERAGE | ~85 |
| DOCUMENTED_ONLY | ~60 |
| NOT_COVERED | ~15 |
