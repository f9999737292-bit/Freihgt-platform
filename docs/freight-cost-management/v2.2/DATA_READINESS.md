# FREIGHT COST INTELLIGENCE v2.2 — Data Readiness

**Status:** Evidence artifact (v2.2A)  
**Base SHA:** `e80af5238bb93d8f29432582beba50103e3b9367`  
**Date:** 2026-08-23

This document records **discovered** platform data — not aspirational fields. Every row cites an existing service, table, or API verified in the repository.

---

## 1. Executive Summary

| Area | Readiness | Class |
|------|-----------|-------|
| Cost ledger & per-order summary | Ready | A |
| Planned / actual / variance KPIs | Ready (tenant-scoped) | A |
| Lane analytics | Geography exists; aggregation not built | B |
| Carrier analytics | IDs ready; name enrichment needed | B |
| Accessorial analytics | Settlement lines + mapping exist | B |
| Order reference enrichment | Authoritative field exists; not wired to workspace | B |
| Weight / volume denominators | Cargo fields exist | B |
| Distance / cost-per-km | **Not found** | C |
| Pallet / loading meter | **Not found** | C |
| Budget vs actual | **Not found** | C |
| Forecasting runtime | Prerequisites incomplete | C |
| Cross-tenant benchmark | Prohibited | C (by policy) |

---

## 2. Source Classification Registry

### 2.1 Freight cost facts

| SOURCE | SERVICE | TABLE_OR_MODEL | FIELD | AUTHORITATIVE | DERIVED | TENANT_SCOPED | COMPANY_SCOPED | IMMUTABLE | VERSIONED | QUERYABLE_NOW | JOIN_KEY | QUALITY |
|--------|---------|----------------|-------|---------------|---------|---------------|----------------|-----------|-----------|---------------|----------|---------|
| Cost ledger entry | freight-cost-service | freight_cost.cost_entry | amount, entry_kind, currency_code | YES | NO | YES | YES (buyer/carrier cols) | YES (append-only) | via source_revision | YES | transport_order_id | COMPLETE |
| Cost summary | freight-cost-service | freight_cost.cost_summary_projection | planned/current/final amounts | NO | YES | YES | YES | NO | NO | YES | transport_order_id | COMPLETE |
| Charge code mapping | freight-cost-service | freight_cost.charge_code_mapping | normalized_category | YES (classification) | NO | YES (tenant override) | NO | NO | YES (mapping_version) | YES | charge_code | COMPLETE |
| Variance attribution | freight-cost-service | freight_cost.variance_attribution | reason_code, mapping_version | NO | YES | YES | NO | NO | YES | YES | transport_order_id | COMPLETE |
| Reconciliation finding | freight-cost-service | freight_cost.reconciliation_finding | finding_kind, status | NO | YES | YES | NO | NO | NO | YES | transport_order_id | COMPLETE |
| Planned cost snapshot | transport-order-service | transport.transport_order_rate_snapshots | via rebuild/ingest | YES | NO | YES | YES | YES | YES | YES (internal) | transport_order_id | COMPLETE |
| Settlement accrual/actual | billing-register-service | billing.freight_settlements | total_without_vat, status | YES | NO | YES | YES | NO | YES (version) | YES (internal) | transport_order_id | COMPLETE |
| Accessorial line | billing-register-service | billing.settlement_accessorials | charge_code, amount, status | YES | NO | YES | YES | NO | NO | YES (DB, no batch API) | settlement_id | PARTIAL |
| Outbox ingest | billing-register-service | billing.freight_cost_outbox | event_type, payload | YES (delivery) | NO | YES | NO | YES | YES (source_revision) | YES | transport_order_id | COMPLETE |

### 2.2 Transport / shipment dimensions

| SOURCE | SERVICE | TABLE_OR_MODEL | FIELD | AUTHORITATIVE | DERIVED | TENANT_SCOPED | QUERYABLE_NOW | JOIN_KEY | QUALITY | CLASS |
|--------|---------|----------------|-------|---------------|---------|---------------|---------------|----------|---------|-------|
| Transport order ID | transport-order-service | transport.transport_orders | id | YES | NO | YES | YES | id | COMPLETE | A |
| Order number | transport-order-service | transport.transport_orders | order_number | YES | NO | YES | YES | id | COMPLETE | B |
| Shipper | transport-order-service | transport.transport_orders | shipper_company_id | YES | NO | YES | YES | id | COMPLETE | A |
| Consignee | transport-order-service | transport.transport_orders | consignee_company_id | YES | NO | YES | YES | id | COMPLETE | A |
| Origin location | transport-order-service | transport.transport_orders | origin_location_id | YES | NO | YES | YES | locations.id | COMPLETE | A |
| Destination location | transport-order-service | transport.transport_orders | destination_location_id | YES | NO | YES | YES | locations.id | COMPLETE | A |
| Country | transport-order-service | transport.locations | country_code | YES | NO | YES | YES | id | COMPLETE | A |
| Region | transport-order-service | transport.locations | region | YES | NO | YES | YES | id | PARTIAL (nullable) | A |
| City | transport-order-service | transport.locations | city | YES | NO | YES | YES | id | PARTIAL (nullable) | A |
| Postal code | transport-order-service | transport.locations | postal_code | YES | NO | YES | YES | id | PARTIAL | A |
| Coordinates | transport-order-service | transport.locations | lat, lon | YES | NO | YES | YES | id | PARTIAL | A |
| Timezone | transport-order-service | transport.locations | timezone | YES | NO | YES | YES | id | COMPLETE | A |
| Transport mode | transport-order-service | transport.transport_orders | transport_mode | YES | NO | YES | YES | id | COMPLETE | A |
| Equipment type | transport-order-service | transport.transport_orders | equipment_type | YES | NO | YES | YES | id | PARTIAL (nullable) | A |
| Planned pickup (TO) | transport-order-service | transport.transport_orders | requested_pickup_date | YES | NO | YES | YES | id | PARTIAL | A |
| Shipment ID | shipment-service | transport.shipments | id | YES | NO | YES | YES | transport_order_id | COMPLETE | A |
| Shipment number | shipment-service | transport.shipments | shipment_number | YES | NO | YES | YES | id | COMPLETE | A |
| Carrier (shipment) | shipment-service | transport.shipments | carrier_company_id | YES | NO | YES | YES | id | COMPLETE | A |
| Actual pickup/delivery | shipment-service | transport.shipments | actual_pickup_at, actual_delivery_at | YES | NO | YES | YES | id | PARTIAL (null until executed) | A |
| Gross weight | transport-order-service | transport.cargoes | gross_weight | YES | NO | YES | YES | cargo_id | PARTIAL (nullable) | B |
| Volume | transport-order-service | transport.cargoes | volume | YES | NO | YES | YES | cargo_id | PARTIAL (nullable) | B |
| Distance (route/planned) | — | — | — | NO | — | — | NO | — | NOT_AVAILABLE | C |
| Pallet count | — | — | — | NO | — | — | NO | — | NOT_AVAILABLE | C |
| Loading meters | — | — | — | NO | — | — | NO | — | NOT_AVAILABLE | C |

### 2.3 Company / carrier enrichment

| SOURCE | SERVICE | TABLE_OR_MODEL | FIELD | AUTHORITATIVE | JOIN_KEY | BATCH_LOOKUP | CLASS |
|--------|---------|----------------|-------|---------------|----------|--------------|-------|
| Carrier company ID | freight-cost-service | cost_summary_projection | carrier_company_id | YES | company id | N/A | A |
| Carrier legal name | company-service | core.companies | legal_name | YES | id | NO (single GET only) | B |
| Carrier short name | company-service | core.companies | short_name | YES | id | NO | B |

**Evidence gap:** `workspace_service.go` sets `CarrierName` from UUID string; v2.1E added `SanitizeDisplayLabel()` but no authoritative name lookup.

### 2.4 Budget

| SOURCE | SERVICE | TABLE | FIELD | CLASS |
|--------|---------|-------|-------|-------|
| Budget entity | — | — | — | C |

`git grep budget` across Go/SQL found **no** freight cost budget model. `planned_cost` is a rate snapshot — **not** a budget.

---

## 3. Transport / Order Data Inventory

| FIELD | SOURCE | AUTHORITATIVE | QUALITY | JOIN_KEY | CLASS |
|-------|--------|---------------|---------|----------|-------|
| transport_order_id | transport.transport_orders.id | YES | COMPLETE | PK | A |
| shipment_id | transport.shipments.id | YES | COMPLETE | transport_order_id | A |
| transport order number | transport.transport_orders.order_number | YES | COMPLETE | id | B |
| shipper | transport.transport_orders.shipper_company_id | YES | COMPLETE | company id | A |
| carrier | cost_summary_projection.carrier_company_id / shipments.carrier_company_id | YES | COMPLETE | company id | A |
| forwarder | transport.shipments.forwarder_company_id | YES | PARTIAL | company id | A |
| origin | transport.locations via origin_location_id | YES | PARTIAL | location id | A |
| destination | transport.locations via destination_location_id | YES | PARTIAL | location id | A |
| origin facility | transport.locations (location_type, name) | YES | PARTIAL | location id | A |
| destination facility | transport.locations | YES | PARTIAL | location id | A |
| country | transport.locations.country_code | YES | COMPLETE | location id | A |
| region/state | transport.locations.region | YES | PARTIAL | location id | A |
| city | transport.locations.city | YES | PARTIAL | location id | A |
| postal code | transport.locations.postal_code | YES | PARTIAL | location id | A |
| coordinates | transport.locations.lat/lon | YES | PARTIAL | location id | A |
| timezone | transport.locations.timezone | YES | COMPLETE | location id | A |
| transport mode | transport.transport_orders.transport_mode | YES | COMPLETE | id | A |
| equipment type | transport.transport_orders.equipment_type | YES | PARTIAL | id | A |
| vehicle type | transport.vehicles.vehicle_type | YES | PARTIAL | shipment.vehicle_id | B |
| cargo type | transport.cargoes.cargo_type | YES | COMPLETE | cargo_id | A |
| weight | transport.cargoes.gross_weight | YES | PARTIAL | cargo_id | B |
| volume | transport.cargoes.volume | YES | PARTIAL | cargo_id | B |
| pallet count | NOT_FOUND | NO | NOT_AVAILABLE | — | C |
| loading meters | NOT_FOUND | NO | NOT_AVAILABLE | — | C |
| planned distance | NOT_FOUND | NO | NOT_AVAILABLE | — | C |
| actual distance | NOT_FOUND (tracking Haversine ≠ route) | NO | NOT_AVAILABLE | — | C |
| planned pickup | transport_orders.requested_pickup_date / shipments.planned_pickup_at | YES | PARTIAL | id | A |
| actual pickup | transport.shipments.actual_pickup_at | YES | PARTIAL | shipment id | A |
| planned delivery | requested_delivery_date / planned_delivery_at | YES | PARTIAL | id | A |
| actual delivery | transport.shipments.actual_delivery_at | YES | PARTIAL | shipment id | A |

---

## 4. Freight Cost Fact Inventory

| Concept | Source | Table / kind | Authoritative | Notes |
|---------|--------|--------------|---------------|-------|
| Cost entry | freight-cost-service | freight_cost.cost_entry | YES | Append-only |
| Planned cost | PLANNED_COST_SNAPSHOT | cost_entry | YES (derived from TO snapshot) | Immutable snapshot |
| Actual cost (current) | CURRENT_ACTUAL_COST_SNAPSHOT | cost_entry / projection | YES | Requires approved settlement, no disputes |
| Actual cost (final) | FINAL_ACTUAL_COST_SNAPSHOT | cost_entry / projection | YES | READY_FOR_PAYMENT |
| Accrual | ACCRUAL_COST_SNAPSHOT | cost_entry | YES | Includes pre-approval state |
| Adjustment | New cost_entry with supersedes_entry_id | cost_entry | YES | Does not UPDATE prior row |
| Billing amount | BILLED_COST_SNAPSHOT | cost_entry | YES | From billing link |
| Invoice amount | Indirect via billing register | billing registers | YES (billing domain) | Not duplicated in cost ledger as invoice doc |
| Settled / paid | PAID_AMOUNT_SNAPSHOT | cost_entry | YES | From payment obligation |
| Variance | Derived | cost_summary_projection | NO | planned vs current/final |
| Cost category | charge_code_mapping.normalized_category | freight_cost.charge_code_mapping | YES (classification) | Versioned |
| Charge code | settlement_accessorials.charge_code | billing | YES | Raw code |
| Accessorial | settlement_accessorials | billing | YES | Status model: PROPOSED/APPROVED/REJECTED/DISPUTED |
| Currency | All money fields | currency_code CHAR(3) | YES | Per order/settlement |
| Mapping version | variance_attribution.mapping_version | freight_cost | YES (pin) | Historical semantics |
| Reclassification | variance_attribution is_current flip | freight_cost | NO (derived) | Does not mutate ledger |
| Reconciliation finding | reconciliation_finding | freight_cost | NO (signal) | Does not change cost |
| created_at / recorded_at | cost_entry.recorded_at | freight_cost | YES | Ingest time |
| effective date | source_occurred_at | cost_entry | YES | From source event |
| posted/finalized | financial_finality on projection | cost_summary_projection | NO (derived) | FINAL_ACTUAL = finalized for analytics |

### Authoritative actual freight cost

**WHAT_IS_AUTHORITATIVE_ACTUAL_FREIGHT_COST=**  
For analytics default: `cost_summary_projection.final_actual_amount` when `financial_finality=FINAL_ACTUAL`; else `current_actual_amount` when `CURRENT_ACTUAL`; accrual reported separately.

**WHEN_COST_BECOMES_ANALYTICS_ELIGIBLE=**  
When active cost summary projection row exists for `(tenant_id, transport_order_id)` with at least `PLANNED_COST_SNAPSHOT` ingested.

---

## 5. Cost Lifecycle (Discovered)

```
COST_LIFECYCLE=
  PLANNED → ACCRUAL → CURRENT_ACTUAL → FINAL_ACTUAL → BILLED → PAYABLE → PAID

ACTIVE_ENTRY_RULE=
  Latest cost_entry per source dimension via source_cursor; superseded chain excluded from sums

SUPERSEDED_ENTRY_RULE=
  Rows with supersedes_entry_id set remain in ledger; analytics uses projection active amounts only

FINALITY_RULE=
  FINAL_ACTUAL when settlement status READY_FOR_PAYMENT and open_dispute_count=0

ANALYTICS_ELIGIBILITY_RULE=
  Include all transport orders with cost_summary_projection; label data_stage; exclude superseded ledger rows from manual sums
```

---

## 6. Carrier Enrichment

```
CARRIER_ID_SOURCE=cost_summary_projection.carrier_company_id
CARRIER_NAME_SOURCE=core.companies.legal_name (fallback short_name)
CARRIER_COMPANY_SOURCE=company-service
JOIN_KEY=carrier_company_id → core.companies.id
BATCH_LOOKUP_AVAILABLE=NO (only GET /v1/companies/{id} per company today)
INTERNAL_ENDPOINT_AVAILABLE=NO dedicated internal batch
EVENT_AVAILABLE=NO
```

**Preferred enrichment:** **D. Hybrid** — dimension snapshot in analytics projection + batch read at build time (new internal batch endpoint in v2.2D).

---

## 7. Order Reference Enrichment

```
TRANSPORT_ORDER_ID_SOURCE=transport.transport_orders.id
ORDER_REFERENCE_SOURCE=transport.transport_orders.order_number
AUTHORITATIVE_SERVICE=transport-order-service
JOIN_KEY=transport_order_id
BATCH_LOOKUP_AVAILABLE=NO (internal API only GET rate-snapshot today)
EVENT_AVAILABLE=NO
```

---

## 8. Canonical Lane Discovery

```
CANONICAL_LANE_EXISTS=PARTIAL
LANE_GRAIN=city→city (within country) + transport_mode + equipment_type
LANE_KEY_COMPONENTS=origin_country, origin_city, dest_country, dest_city, transport_mode, equipment_type
LANE_DIRECTIONAL=YES
NORMALIZATION_RULE=TRIM + uppercase city; require country_code; missing city → lane NOT_AVAILABLE for that order
CARDINALITY_RISK=MEDIUM
```

| Option | Availability | Stability | Cardinality | Benchmark sample | Verdict |
|--------|--------------|-----------|-------------|------------------|---------|
| A city→city | HIGH | MEDIUM | MEDIUM | GOOD | **Selected** |
| B region→region | MEDIUM | HIGH | LOW | GOOD | Fallback label only |
| C facility→facility | HIGH | LOW | HIGH | POOR | Drill-down only |
| D hybrid | HIGH | MEDIUM | MEDIUM | GOOD | **Selected** (city primary, facility in drill-down) |

---

## 9. Distance

```
DISTANCE_SOURCE=NOT_AVAILABLE
DISTANCE_AUTHORITATIVE=NO
DISTANCE_UNIT=N/A
DISTANCE_QUALITY=NOT_AVAILABLE
COST_PER_KM_READY=NO
```

**Note:** `tracking-service` computes Haversine between GPS points for speed quality — not authoritative route distance for cost analytics.

---

## 10. Cargo Denominators

```
WEIGHT_SOURCE=transport.cargoes.gross_weight
WEIGHT_UNIT=implicit kg (schema NUMERIC, no unit column — platform convention)
WEIGHT_QUALITY=PARTIAL

VOLUME_SOURCE=transport.cargoes.volume
VOLUME_UNIT=implicit m³ (no unit column)
VOLUME_QUALITY=PARTIAL

PALLET_SOURCE=NOT_AVAILABLE
PALLET_QUALITY=NOT_AVAILABLE

LOADING_METERS_SOURCE=NOT_AVAILABLE
LOADING_METERS_UNIT=N/A
LOADING_METERS_QUALITY=NOT_AVAILABLE
```

| Metric | Ready |
|--------|-------|
| COST_PER_KG | PARTIAL (CLASS B — needs cargo join, null guard) |
| COST_PER_TONNE | PARTIAL (derive from kg if present) |
| COST_PER_M3 | PARTIAL (CLASS B) |
| COST_PER_PALLET | NO (CLASS C) |
| COST_PER_LOADING_METER | NO (CLASS C) |

---

## 11. Accessorial Discovery

```
ACCESSORIAL_ENTITY=billing.settlement_accessorials
ACCESSORIAL_SOURCE=billing-register-service
CATEGORY_SOURCE=freight_cost.charge_code_mapping.normalized_category
CHARGE_CODE_SOURCE=settlement_accessorials.charge_code
MAPPING_SOURCE=freight_cost.charge_code_mapping (PLATFORM + TENANT, mapping_version)
AMOUNT_SOURCE=settlement_accessorials.amount (APPROVED lines for analytics default)
CURRENCY_SOURCE=settlement_accessorials.currency_code
ORDER_LINK=freight_settlements.transport_order_id
CARRIER_LINK=freight_settlements.carrier_company_id
STATUS_MODEL=PROPOSED | APPROVED | REJECTED | DISPUTED
CAN_SEPARATE_BASE_FREIGHT_FROM_ACCESSORIAL=YES (base_freight_amount vs approved_accessorial_total)
```

| KPI | CLASS | Notes |
|-----|-------|-------|
| ACCESSORIAL_TOTAL | B | Sum APPROVED accessorials + settlement approved_accessorial_total |
| ACCESSORIAL_RATE | B | accessorial / (base + accessorial) |
| ACCESSORIAL_PER_ORDER | B | Per transport_order_id |
| ACCESSORIAL_BY_CATEGORY | B | Requires mapping pin at projection time |
| ACCESSORIAL_BY_CARRIER | B | Join via settlement |
| ACCESSORIAL_BY_LANE | B | Requires lane enrichment |

---

## 12. Budget

```
REAL_BUDGET_SOURCE=NOT_AVAILABLE
BUDGET_MODEL_EXISTS=NO
BUDGET_VS_ACTUAL_READY=CLASS_C
```

| Concept | Distinction |
|---------|-------------|
| RATE_ESTIMATE | TO rate snapshot — exists |
| PLANNED_COST | PLANNED_COST_SNAPSHOT — exists |
| ACCRUAL | ACCRUAL_COST_SNAPSHOT — exists |
| BUDGET | **NOT_FOUND** |
| ACTUAL_COST | Settlement-backed — exists |
| INVOICE | Billing register — exists (adjacent domain) |
| SETTLEMENT | freight_settlements — exists |

---

## 13. KPI Readiness Matrix

| KPI | NUMERATOR | DENOMINATOR / DIMENSION | SOURCE | CLASS | QUALITY | BLOCKER |
|-----|-----------|-------------------------|--------|-------|---------|---------|
| Total Freight Cost | final/current actual | tenant, period, currency | cost_summary_projection | A | COMPLETE | — |
| Planned Cost | planned_amount | tenant, period, currency | cost_summary_projection | A | COMPLETE | — |
| Actual Cost | current/final actual | tenant, period, currency | cost_summary_projection | A | COMPLETE | — |
| Variance | planned − actual | per order | cost_summary_projection | A | COMPLETE | — |
| Variance % | variance / planned | per order | derived | A | PARTIAL | zero planned guard |
| Cost per Order | actual | order count=1 | cost_summary_projection | A | COMPLETE | — |
| Cost per Shipment | actual | shipment_id | cost_summary + shipments | B | PARTIAL | 1:1 mostly |
| Cost per km | actual | distance | — | C | NOT_AVAILABLE | no distance |
| Cost per kg | actual | gross_weight | cargoes | B | PARTIAL | null weight |
| Cost per tonne | actual | weight/1000 | cargoes | B | PARTIAL | null weight |
| Cost per m³ | actual | volume | cargoes | B | PARTIAL | null volume |
| Cost per pallet | — | — | — | C | NOT_AVAILABLE | no pallet field |
| Cost per loading meter | — | — | — | C | NOT_AVAILABLE | no ldm field |
| Cost by Carrier | actual | carrier_company_id | cost_summary_projection | B | PARTIAL | name enrichment |
| Cost by Lane | actual | lane_key | TO + locations | B | PARTIAL | projection not built |
| Cost by Cost Category | attribution | normalized_category | variance + mapping | B | PARTIAL | mapping pin |
| Accessorial Cost | approved accessorial | — | settlement_accessorials | B | PARTIAL | batch read |
| Accessorial Rate | accessorial / total | — | settlement | B | PARTIAL | — |
| Reconciliation Finding Rate | open findings | order count | reconciliation_finding | A | COMPLETE | — |
| Budget vs Actual | — | — | — | C | NOT_AVAILABLE | no budget |
| Forecast | — | — | — | C | NOT_AVAILABLE | v2.2A assessment only |

---

## 14. CLASS Summary

### CLASS_A (analytics-ready now, from existing projection)

- Total / planned / accrued / current / final actual costs (single currency filter)
- Variance amount and count
- Reconciliation finding count / rate
- Cost per transport order (implicit)
- Order/carrier/company IDs for grouping keys
- Data stage and financial finality filters

### CLASS_B (exists, needs enrichment and/or v2.2 projection)

- Lane spend and benchmarks
- Carrier spend with display names
- Accessorial totals by category/carrier/lane
- Cost per kg / m³ / tonne
- Cost per shipment (explicit shipment grain)
- Order reference display
- Tenant historical benchmark percentiles
- Rule-based savings opportunities

### CLASS_C (absent or prohibited)

- Cost per km (no distance)
- Cost per pallet / loading meter
- Budget vs actual
- Forecast runtime
- Cross-tenant market benchmark
- FX-normalized cross-currency totals

---

## 15. Gaps & Planned Resolution

| GAP | PLANNED_RESOLUTION | Target |
|-----|-------------------|--------|
| No analytics projection tables | v2.2B migration + rebuild job | v2.2B |
| Carrier/order UUID labels | Batch enrichment + snapshot columns | v2.2D |
| No lane aggregation API | lane_period_projection | v2.2C |
| Accessorial workspace NOT_AVAILABLE | accessorial_period_projection + API | v2.2D |
| No batch company/TO internal API | Add internal batch read endpoints | v2.2D |
| No distance | Future route engine integration | post-v2.2 |
| No budget entity | Future FP&A module | post-v2.2 |
| No pallet/ldm | Extend cargo model or TMS integration | post-v2.2 |

---

## 16. Evidence Index

| Claim | Evidence path |
|-------|---------------|
| Append-only ledger | `000054_freight_cost_ledger_v2.1B.up.sql` triggers |
| Outbox event types | `freight_cost_outbox.go` constants |
| Locations schema | `000003_create_transport_tables.up.sql` |
| Accessorials schema | `000042_freight_settlement_v1.7.up.sql` |
| Charge mappings seed | `000058_freight_cost_variance_explainability_v2.1C.up.sql` |
| Workspace NOT_AVAILABLE lanes | `dto/workspace.go` DataCapabilityNotAvailable |
| RBAC buyer vs carrier | `freightcostrbac/policies.go` |
| Company names | `000002_create_core_tables.up.sql` core.companies |
