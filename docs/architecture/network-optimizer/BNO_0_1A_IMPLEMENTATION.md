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

`SOURCE_VERIFY_SERVICE_AUTH=INTERNAL_SERVICE_TOKEN`

`SOURCE_VERIFY_SERVICE_IDENTITY=PLATFORM_INTERNAL_SERVICE`

`SOURCE_VERIFY_TENANT_CONTEXT=ACTOR_TENANT_HEADER`

`SOURCE_VERIFY_SCOPE=INTERNAL_OWNERSHIP_PROBE`

`RAW_TENANT_HEADER_ALONE_SUFFICIENT=NO`

`PUBLIC_ROUTE_REUSED=NO`

`DEDICATED_INTERNAL_PROBE=YES`

Publication does not call the public shipment or transport-order reads. Those reads are not service-authenticated, and the transport-order public read also applies company visibility. The ownership probes are:

- `GET /internal/v1/transport-orders/{id}/ownership`
- `GET /internal/v1/shipments/{id}/ownership`

Both sit behind `packages/shared-go/internalauth`. The caller must present `X-Internal-Service-Token`, the same shared platform credential already used by internal routes such as transport-order rate snapshot. An empty or wrong token is denied. The token is read from `INTERNAL_SERVICE_TOKEN`. It is not hardcoded.

`integrationauth` is the external ERP principal mechanism. Its scopes are `rfx:*`, its tenant comes from the principal, and the gateway injects those headers only after verifying an integration JWT. That is not the platform service-to-service boundary, so this probe does not add integration scopes or principals.

After the token check, the probe still loads the row with `id` and the actor tenant. A valid service credential cannot select by id alone. The response is only `id` and `tenant_id`. A foreign tenant and a missing id return the same not-found body.

Network-optimizer builds a new request. It sets the configured service token and `X-Tenant-ID` from the actor already taken from the gateway JWT. It does not copy client `Authorization`, integration principal, scope, auth-scheme, actor-kind, or internal-token headers. The gateway strips those headers before this service sees a public request. A missing token, a downstream 401, a tenant or id mismatch, a missing body field, a redirect, or an unexpected status fails closed and does not publish.

## Indexes

Owner lists use `(owner_tenant_id, status, created_at)`. Marketplace reads use visibility and status, plus the availability or pickup window. Active publication of the same source is unique per owner. Invited-carrier and shipper-network audience arrays use GIN indexes. These indexes serve tenant-scoped reads. They are not a global matching index.
