# BNO-0.1A implementation

Foundation only. This is not an optimizer release.

## IMPLEMENTED

- Load opportunity as an explicit marketplace projection owned by `network-optimizer-service`. It is not a cross-tenant shipment read.
- Capacity as a time-bounded aggregate. The only creatable source is `MANUAL`.
- Explicit publication from a tenant-owned transport order or shipment. The service checks ownership through a single-id read and copies the publication fields only.
- Visibility scopes for loads: `PRIVATE`, `INVITED_CARRIERS`, `MARKETPLACE`, `ANONYMIZED_MARKETPLACE`, `NETWORK_OPTIMIZATION_ONLY`.
- Visibility scopes for capacity: `PRIVATE`, `SHIPPER_NETWORK`, `MARKETPLACE`, `ANONYMIZED`.
- `SHIPPER_NETWORK` is visible only to `audience_tenant_ids`. `INVITED_CARRIERS` is visible only to the verified carrier company.
- Owner reads, marketplace reads, withdraw, and versioned update.
- State transitions that this release can actually perform: load `DRAFT -> PUBLISHED -> WITHDRAWN` and capacity `AVAILABLE -> WITHDRAWN`. Invalid transitions fail closed.
- Optimistic version checks and `Idempotency-Key` replay.
- Migration `000075` schema `network_optimizer`, with audit and outbox tables.
- Gateway routes under `/api/v1/network/...` with JWT authentication, membership-checked company context, and role checks.
- Events: `network.load_opportunity.published`, `network.load_opportunity.updated`, `network.load_opportunity.withdrawn`, `network.capacity.published`, `network.capacity.updated`, `network.capacity.withdrawn`.
- The task names `capacity.available` is represented by `network.capacity.published` with status `AVAILABLE`, matching ADR-NET-008 past-tense names.

## NOT_IMPLEMENTED

- Predictive capacity
- Next-load matching and ranking
- Backhaul and roundtrip
- Consolidation solver
- Multi-stop execution
- Regional optimizer
- Urban optimizer, including Moscow and Saint Petersburg runtime rules
- Machine learning
- Match, chain, route-plan, and prediction events
- Pallet count, linear metres, and trailer capability. Those attributes are not columns. Unknown stays omitted and is never stored as zero.
- Haversine or any commercial distance
- Gateway readiness dependency on this service. The route is registered, and `/ready` does not require the new process, so existing gateway readiness does not fail before a separate deploy task.

## Security

Tenant and user identity on the public API come from the verified gateway JWT. `X-Tenant-ID`, `X-User-ID`, `X-User-Email`, and `X-Platform-Admin` supplied by the client are stripped before the JWT values are set. Company membership is checked before `X-Company-ID` is forwarded. A caller who is not allowed to see a record receives the same not-found response as a missing id.

### Anonymized geography

`Place` and capacity location fields are an optional location id, a free-text label, and exact latitude/longitude. There is no validated city or zone field in this release. `ANONYMIZED_MARKETPLACE` loads and `ANONYMIZED` capacities therefore omit exact coordinates and raw facility, warehouse, or yard labels. Time windows, equipment, and other non-identifying published facts remain. Owner reads and non-anonymized marketplace scopes still return the stored location fields. This is not reverse geocoding and it does not treat a coordinate as a city.

### Source verification

`SOURCE_VERIFY_SERVICE_AUTH=NONE`

`SOURCE_VERIFY_TENANT_CONTEXT=ACTOR_TENANT_HEADER`

`RAW_CLIENT_HEADER_TRUSTED=NO`

Publication checks ownership with `GET /v1/transport-orders/{id}` or `GET /v1/shipments/{id}`. Those handlers resolve the tenant only from `X-Tenant-ID` and load the row with that tenant predicate. They do not accept `X-Internal-Service-Token` as authorization. The transport-order `/internal/v1` routes do require that token, and they serve rate snapshots and award creation, so they are not the ownership probe.

The verifier builds a new request and sets `X-Tenant-ID` from the actor tenant already taken from the gateway JWT. It does not copy `Authorization`, user, company, platform-admin, or internal-service headers from the caller. A matching `tenant_id` in the downstream body is required. A mismatch, a missing `tenant_id`, a not-found, or a redirect fails closed. A public caller cannot move that check onto another tenant by spoofing identity headers: the gateway replaces those headers before this service sees the request.

## Indexes

Owner lists use `(owner_tenant_id, status, created_at)`. Marketplace reads use visibility and status, plus the availability or pickup window. Active publication of the same source is unique per owner. Invited-carrier and shipper-network audience arrays use GIN indexes. These indexes serve tenant-scoped reads. They are not a global matching index.
