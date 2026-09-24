# Current state inventory

Discovery date: 2026-09-24. Worktree: `D:\Projects\freight-platform-wt\transport-network-optimizer-v0.1`. HEAD: `2c0ad2bd573ca78788d09d13cfe629b4023b88d9`.

Status is taken from services, migrations, and OpenAPI. Documents alone are `DOCS_ONLY`.

| Capability | Existing owner/service | Status | Reusable | Gap | Notes |
| ---------- | ---------------------- | ------ | -------- | --- | ----- |
| Transport Order | `transport-order-service` | IMPLEMENTED | PARTIAL | No stops or legs. Mode validation is ROAD-only (`internal/domain/transport_order.go`). | O–D, cargo, requested pickup/delivery dates, company parties, version. OpenAPI `packages/openapi/transport-order-service.yaml`. |
| Shipment | `shipment-service` | IMPLEMENTED | PARTIAL | One shipment is one origin and one destination. | Assigns `driver_id` and `vehicle_id`. Status history in `transport.shipment_status_history`. |
| Shipment Legs | — | NOT_FOUND | NO | No `shipment_leg`, stop sequence, or shared movement table in services or migrations. | Future `RouteLeg` must be additive. See ADR-NET-005. |
| Vehicle | `shipment-service` (`transport.vehicles`) | IMPLEMENTED | PARTIAL | Weight and volume only. No body dimensions, axle, ADR, temperature zones, or availability calendar. | `CapacityWeight`, `CapacityVolume`, `equipment_type`, `carrier_company_id`. |
| Trailer | — | NOT_FOUND | NO | No trailer table or Go type. | Domain model reserves `trailer_id`. Do not invent a trailer master in this freeze. |
| Driver | `shipment-service` | IMPLEMENTED | PARTIAL | No shift, home base, or hours model. | License and carrier company. Mobile tasks exist. `apps/driver-mobile` has no GPS upload; location ingest is `tracking-service`. |
| Carrier | `company-service` | IMPLEMENTED | PARTIAL | Legal/company master, not a capacity network. | `company_type` includes `CARRIER`, `FORWARDER`, `LSP`, `WAREHOUSE`, `TERMINAL`. |
| Location | `transport-order-service` (`transport.locations`) | IMPLEMENTED | YES for points | Coordinates optional. No geocoder. No road network. | `lat`/`lon` numeric. Types include `WAREHOUSE`, `DISTRIBUTION_CENTER`, `TERMINAL`, `CUSTOMER_SITE`. Default timezone `Europe/Moscow` is a location default, not a city-rule engine. |
| Facility | — | PARTIAL | NO as a master | `facility_id` UUID on tracking slot rows only. No facility table. | Hub/depot/cross-dock are architectural roles mapped later onto locations. |
| Slot | `tracking-service` | PARTIAL | PARTIAL | Ingest and query, not a booking owner API with hold/confirm. Not in `packages/openapi`. | `tracking.shipment_slot_*` (`000028`). Gateway read routes exist. Shipment status `PICKUP_SLOT_BOOKED` exists; delivery slot status is stored but is not a full booking workflow. |
| Tracking | `tracking-service` | IMPLEMENTED | PARTIAL | Positions for monitoring, not planning. | `tracking.location_event`, Haversine in `internal/domain/quality.go`. No PostGIS, no geohash. |
| ETA | `tracking-service` | IMPLEMENTED | PARTIAL | Stored and queried. No Kafka event `shipment.eta.updated`. | `tracking.shipment_eta_state` (`000027`). Prediction provider must read this, not assume a bus event. |
| Rates | `contract-rate-service` | IMPLEMENTED | YES as read-only SSOT | Optimizer must not copy rate tables. | Contracts, cards, lines, resolve. Gateway `/api/v1/transport-contracts*`, `/api/v1/rates/resolve`. |
| Freight Cost | `freight-cost-service` | IMPLEMENTED | YES as ledger owner | README still says stateless; code and migrations are a DB ledger. | Planned/accrual/billed/paid snapshots, variance. Optimizer must not insert `CostEntry`. |
| RFx | `rfx-service` | IMPLEMENTED | YES as tender owner | No anonymous load board. | Event types include `RFQ`, `SPOT_RFQ`, `MINI_TENDER`, award, responses. |
| Award | `rfx-service` | IMPLEMENTED | YES | Award converts toward transport order. | `internal/domain/award_conversion.go`. Optimizer must not award. |
| Carrier Offer | `rfx-service` offer lines and freight-request bids | PARTIAL | NO as marketplace offer | `RfxResponseOfferLine` is an RFx response line, not a network offer. | New `CarrierOffer` is a network aggregate that may hand off to RFx. |
| Documents | `document-service` | IMPLEMENTED | PARTIAL | Registry and mock signing, not legal EDO. | Tenant-scoped. Types include labels such as ETRN/POD. |
| EDO | `document-service` plus ADR pack | PARTIAL | NO for operator exchange | EDO 0.3 and `transport-edo-service` are not in code. | ADR-EDO-001 … 009 accepted as docs. Billing `mark-sent-to-edo` is a mock step. |
| Settlement | `billing-register-service` | IMPLEMENTED | YES as settlement owner | Requires a transport-order rate snapshot to create freight settlement. | `billing.freight_settlement*`. Payment obligations live in `payment-service`. |
| Control Tower | `control-tower-read-model-service` plus gateway BFF | IMPLEMENTED | PARTIAL | KPIs are shipment/risk/case/automation, not network utilization. | Consumes `shipment.status.v1` and `driver.events.v1`. |
| Event Bus | Kafka for shipment and driver; HTTP outbox for billing and payment | PARTIAL | YES as pattern | No platform-wide bus. RFx has no Kafka outbox. | Topic defaults `shipment.status.v1`, `driver.events.v1`. Outbox worker can be disabled. |
| Outbox | `shipment-service` `transport.shipment_event_outbox` | IMPLEMENTED | YES | Envelope is camelCase JSON, not the snake_case ADR field names literally. | `000014`. Tracking can insert the same outbox. `driver.task_*` rows are not all Kafka-published. |
| Tenant Context | `api-gateway` JWT auth | IMPLEMENTED | YES | Downstream trusts gateway headers only. | `auth.go` strips client `X-Tenant-ID` / `X-User-ID` and sets them from JWT. `packages/shared-go/lowcode/headers.go`. |
| Geo model | locations + tracking points + Haversine | PARTIAL | PARTIAL | No distance matrix, zones, corridors, or routing vendor. | Candidate search and road distance must stay separate. See ADR-NET-010. |
| Marketplace load board | — | NOT_FOUND | NO | Freight request is tenant-scoped sourcing (`SPOT`, `MINI_TENDER`, …), visible to carriers only in `PUBLISHED` / `RESPONSES_OPEN`. | Publication into `LoadOpportunity` is new and explicit. |
| Cargo physical model | `transport.cargoes` / `transport.cargo_items` | PARTIAL | PARTIAL | Has weight, volume, temperature, dangerous flag, item hazard class. No pallets, linear metres, dimensions, stackability, or co-load rules. | `services/transport-order-service/internal/domain/cargo.go`. |
| Rate snapshot | `transport-order-service` | IMPLEMENTED | YES as frozen commercial input | Immutable plan price, not an optimizer output. | `transport.transport_order_rate_snapshots` (`000051`). |
| Identity / RBAC | `identity-service`, `company-service`, gateway RBAC | IMPLEMENTED | YES | No separate ABAC engine. | Shipper vs carrier is role maps plus `X-Company-ID` / `X-Actor-Kind`. |
| OpenAPI generation | `scripts/openapi/generate_openapi.py` | IMPLEMENTED | YES for later | This freeze does not edit `packages/openapi`. | Skeleton lives under `contracts/`. |
| Driver mobile | `apps/driver-mobile` | PARTIAL | PARTIAL | Delay, problem, POD. No in-app GPS publisher found. | Capacity source `DRIVER_APP` is a future port, not current behavior. |
| Prediction / ML | — | NOT_FOUND | NO | No capacity prediction model. | Interface only. First provider is rule-based. |
| Solver / VRP | — | NOT_FOUND | NO | No optimization engine. | Staged heuristics. See ADR-NET-011. |

## Reuse rules

The optimizer may read tenant-scoped orders, cargo, locations, vehicles, shipments, tracking ETA, slots, and rate snapshots through trusted APIs. It must not scan `transport.shipments` across tenants. It must not write freight-cost ledger rows, RFx awards, billing settlements, or slot bookings.

## Closeout invariants

These labels are the publication contract. They restate the inventory above.

```text
TRAILER_CURRENT_STATE=NOT_FOUND
FACILITY_MASTER_CURRENT_STATE=NOT_FOUND
ETA_KAFKA_EVENT_CURRENT_STATE=NOT_FOUND
DRIVER_APP_GPS_PUBLICATION_CURRENT_STATE=NOT_FOUND
```

`FACILITY_MASTER_CURRENT_STATE` is `NOT_FOUND` even though slot rows carry an unbound `facility_id`. That UUID is not a facility master.

`ETA_KAFKA_EVENT_CURRENT_STATE` is `NOT_FOUND`: ETA rows exist in tracking storage. There is no Kafka event to consume.

`DRIVER_APP_GPS_PUBLICATION_CURRENT_STATE` is `NOT_FOUND`: `apps/driver-mobile` does not publish GPS. Tracking ingest elsewhere is not that app.

```text
HAVERSINE_POLICY
```

Haversine, or any straight-line distance, is a candidate pre-filter only. It is not canonical road distance, not canonical travel time, and not canonical freight cost distance.

```text
UNKNOWN_CAPACITY_POLICY
```

Missing pallet counts and linear metres stay `UNKNOWN`. Never coerce an unknown value to zero.

```text
MULTISTOP_EXECUTION_POLICY
```

Planning contracts for stops and legs may exist. Production multi-stop execution stays gated until the execution domain supports stops and legs.

```text
EDO_RUNTIME_POLICY
```

The document registry exists. The EDO operator runtime is not an implementation dependency for NLO-0.1 or NLO-0.2.

## Evidence anchors

- Cargo: `services/transport-order-service/internal/domain/cargo.go`
- Vehicle: `services/shipment-service/internal/domain/vehicle.go`
- Location: `services/transport-order-service/internal/domain/location.go`
- Shipment outbox names: `services/shipment-service/internal/domain/outbox.go`
- Driver events: `services/shipment-service/internal/domain/driver_events.go`
- Event naming: `docs/adr/ADR-EDO-006-event-naming-versioning.md`
- Freight request: `services/rfx-service/internal/domain/freight_request.go`
- Gateway trust: `services/api-gateway/internal/http/middleware/auth.go`
