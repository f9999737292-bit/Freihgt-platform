# External tracking provider contract v0.2A

STAGE=TRACKING-EXTERNAL-PROVIDER-CONTRACT-0.2A
BASE_SHA=cbbba8e413c8ef1c716b242533d7f79aaaafe47a
VENDOR_SELECTED=NO
REAL_VENDOR_ADAPTER_PRESENT=NO
VENDOR_ADAPTER_IMPLEMENTATION_AUTHORIZED=NO
STAGING_PROVIDER_ENABLEMENT_AUTHORIZED=NO
PROVIDER_CREDENTIAL_CREATION_AUTHORIZED=NO
REAL_PROVIDER_LIVE_INGEST_AUTHORIZED=NO

Status: accepted provider-neutral contract freeze. This document records current `tracking-service` behavior and the boundary a later vendor adapter must respect. It does not authorize adapter implementation, credential creation, schema change, gateway exposure, staging change, or live ingest.

Decision record: [ADR-TRACK-001](../adr/ADR-TRACK-001-external-provider-ingest-contract.md)

## 1. Current source truth

TRACKING_SERVICE_IMPLEMENTED=YES

| Port | Interface | Registered adapters |
|------|-----------|---------------------|
| Telemetry | `TelemetryProviderAdapter` | `generic`, `driver_mobile` |
| ETA | `ETAProviderAdapter` | `generic` |
| Slots | `SlotProviderAdapter` | `generic` |

`cmd/server/main.go` builds those three registries separately. No other adapter is registered.

Provider push routes:

- `POST /internal/v1/tracking/providers/{provider}/locations`
- `POST /internal/v1/tracking/providers/{provider}/eta`
- `POST /internal/v1/tracking/providers/{provider}/slots`

Driver locations stay on a separate route:

- `POST /internal/v1/tracking/driver/shipments/{shipmentId}/locations`

Provider ingress authentication is the header `X-Provider-Secret`. The expected value is loaded from `TRACKING_PROVIDER_SECRETS`. A single value with no colon and no comma is stored under the `generic` key; `TRACKING_GENERIC_PROVIDER_SECRET` is the fallback environment key for that single-secret form. This document records no secret value.

Internal service authentication is the header `X-Internal-Service-Token`.

Each ingest handler reads at most 1 MiB (`1 << 20`).

Already implemented platform capabilities are telemetry and location, current tracking state, location history, ETA, ETA history, slots, slot history, quality and freshness, deduplication, the tracking-loss detector, shipper reads, carrier reads, and the metrics in section 20.

## 2. Capability model

CAPABILITY_REGISTRATION_INDEPENDENT=YES

The capabilities are `TELEMETRY`, `ETA`, and `SLOTS`. A future provider may implement any explicitly declared subset: telemetry only, telemetry plus ETA, all three, or another declared subset. A vendor is not required to implement every port.

After the secret check succeeds, each route calls only its own registry:

- locations call `Registry.Get`
- ETA calls `ETARegistry.Get`
- slots call `SlotRegistry.Get`

A miss returns HTTP 400 `VALIDATION_ERROR` with message `unsupported provider`. The request is not forwarded to another provider code and is not silently handled by the `generic` adapter.

Current evidence: `driver_mobile` is registered for telemetry only. An ETA or slot request for that code is unsupported even though `DriverMobileAdapter.Normalize` delegates to `GenericAdapter`.

## 3. Payload and domain boundary

PROVIDER_PAYLOAD_DOMAIN_OPAQUE=YES
DOMAIN_DEPENDS_ON_VENDOR_SCHEMA=NO

```
vendor payload
    -> provider adapter
    -> normalized model
    -> tracking service / domain
    -> persistence and customer reads
```

`ProviderPayload` is an opaque byte slice. Domain and repository code consume `NormalizedLocationInput`, `NormalizedETAInput`, and `NormalizedSlotInput` only. Vendor-specific fields stay inside the adapter. This stage adds no vendor-specific column.

## 4. Provider code

PROVIDER_CODE_CLIENT_AUTHORITY=NO
PROVIDER_CODE_RUNTIME_CONFIGURED=YES

`providerCode` is the `{provider}` path segment. The handler uses it as an exact key in the secret map and passes the same string to the capability registry. `ProviderCode()` is stored without case folding. `parseProviderSecrets` trims whitespace and does not lowercase keys.

The code selects the adapter, selects the secret, and is an input to every current dedup key. It is not caller authority over tenant or shipment. No vendor code is assigned in this contract. The only codes registered in source are `generic` and `driver_mobile`.

## 5. Authentication

Three mechanisms stay distinct, and driver authentication is a fourth:

| Boundary | Credential | What it authorizes |
|----------|------------|--------------------|
| Provider ingress | `X-Provider-Secret` | Submission of a payload for one configured provider code |
| Internal service | `X-Internal-Service-Token` | Service-to-service calls, including driver location ingest and internal lookups |
| Driver path | trusted internal context plus `X-Driver-ID` | A driver location for a server-supplied tenant, shipment, and driver |
| Human | gateway JWT | A person using a product API |

Provider ingress authentication is not internal service authentication, is not driver authentication, and is not human JWT authentication.

The current provider check is the same on locations, ETA, and slots. The provider code must exist in the loaded secret map, the configured value must be non-empty, the header must be non-empty, and the header must equal the configured value. Failure is HTTP 401 `UNAUTHORIZED`, message `provider authentication failed`, with empty details.

PROVIDER_SECRET_REQUIRED=YES
EMPTY_PROVIDER_SECRET_ALLOWED=NO
UNKNOWN_PROVIDER_ALLOWED=NO

A provider code absent from the secret map fails this 401 check and does not reach adapter lookup. HTTP 400 `unsupported provider` occurs only after the secret check passes and that capability registry has no adapter.

The secret is protected environment configuration. It is not committed. The auth check does not log it. Each provider code has its own map entry, so secrets are rotatable independently.

DYNAMIC_PROVIDER_SECRET_RELOAD_PRESENT=NO

`config.Load` reads the map once at process start. There is no reload, watch, or signal handler for `ProviderSecrets`. Rotation requires a controlled runtime restart or recreate. This stage does not add reload.

## 6. Tenant, shipment, and device authority

EXTERNAL_PROVIDER_MAY_SET_TENANT_AUTHORITY=NO
EXTERNAL_PROVIDER_MAY_SET_SHIPMENT_AUTHORITY=NO
DEVICE_BINDING_SERVER_OWNED=YES
TENANT_SERVER_DERIVED=YES
SHIPMENT_SERVER_DERIVED=YES
AUTO_BIND_UNKNOWN_PROVIDER_DEVICE=FORBIDDEN

The normalized telemetry, ETA, and slot structs have no tenant or shipment field. Ordinary ingest sets `TenantID` and `ShipmentID` from `FindActiveBindingByDeviceAnyTenant(providerCode, providerDeviceId)`:

```
provider_code = $1 AND provider_device_id = $2 AND status = 'active'
ORDER BY active_from DESC
LIMIT 1
```

A tenant or shipment identifier that happens to appear in a vendor payload is a non-authoritative claim. Facility, location, and execution-stop identifiers are data claims and do not grant access.

`ProviderDeviceID` is required. An empty value increments `rejected` and is not stored. `pgx.ErrNoRows` from the binding lookup does the same. Provider ingest does not insert into `tracking.shipment_tracking_binding`.

The active-device unique index is `(tenant_id, provider_code, provider_device_id) WHERE status = 'active'`. It does not prevent the same provider device from being active in two tenants. The any-tenant lookup then returns the newest `active_from`. Future onboarding must refuse a second active binding for the same provider code and device id. That onboarding API is separate scope and is not created here.

The persisted location event uses `binding.VehicleID` and `binding.DriverID`. Payload vehicle and driver values are not the authority for those columns.

## 7. Normalized telemetry

`NormalizedLocationInput`:

| Field | Current ingest rule |
|-------|---------------------|
| ProviderDeviceID | required; empty is an item rejection |
| ProviderEventID | optional; stored when supplied |
| Latitude | required; finite and within -90..90 |
| Longitude | required; finite and within -180..180 |
| RecordedAt | required |
| SpeedKph | optional |
| HeadingDegrees | optional |
| AccuracyMeters | optional |
| AltitudeMeters | optional |
| SourceType | stored; the current generic adapter fills `vehicle_telematics` only when the field is empty |
| VehicleID | optional on the normalized struct; the persisted event uses the binding |
| DriverID | optional on the normalized struct; the persisted event uses the binding |

WIRE_TIMESTAMP_FORMAT=RFC3339
NORMALIZED_TIMEZONE=UTC

The current generic adapter parses `recordedAt` with `time.RFC3339` and stores `UTC()`. A parse failure fails the whole payload. The adapter does not invent coordinates or timestamps. The service rejects an item whose `RecordedAt` is after `now + 5 minutes`.

## 8. Normalized ETA

`NormalizedETAInput`:

| Field | Current rule |
|-------|----------------|
| ProviderDeviceID | required |
| ProviderEventID | optional; included in the dedup key when supplied |
| TargetType | `pickup`, `delivery`, or `execution_stop`; the current generic adapter fills `delivery` only when empty |
| TargetReference | optional |
| ExecutionStopID | optional UUID; required before an `execution_stop` item can be stored |
| EstimatedArrivalAt | required RFC3339, stored UTC; a future arrival time is not rejected |
| SourceObservedAt | required RFC3339, stored UTC; rejected when after `now + 5 minutes` |
| SourceType | must be `provider_eta`, `carrier_eta`, `driver_eta`, or `manual_operator`; the current generic adapter fills `provider_eta` only when empty |
| ProviderConfidence | optional |

EXECUTION_STOP_PROVIDER_ASSERTION_AUTHORITATIVE=NO

An `execution_stop` item does not take shipment identity from the payload. The service loads active bindings for the provider device, collects their driver and vehicle ids, and loads active execution targets for those actors. The item is rejected unless exactly one execution is present, the locked execution `LiveETAStopID` equals the supplied stop id, the locked tenant matches, and the locked driver or vehicle matches a binding actor. The existing server lock remains authoritative.

## 9. Normalized slots

`NormalizedSlotInput`:

| Field | Current rule |
|-------|----------------|
| ProviderDeviceID | required |
| ProviderSlotID | optional; part of the dedup key when supplied |
| ProviderVersion | optional; part of the dedup key when supplied |
| SlotType | `pickup` or `delivery`; the current generic adapter fills `delivery` only when empty |
| FacilityID | optional; a valid UUID is stored and does not select a tenant |
| LocationID | optional; a valid UUID is stored and does not select a tenant |
| WindowStart | required RFC3339, stored UTC |
| WindowEnd | required RFC3339, stored UTC |
| Timezone | optional |
| SlotStatus | stored; the current generic adapter fills `booked` only when empty |
| SourceObservedAt | required RFC3339; rejected when after `now + 5 minutes` |
| SourceType | must be an enabled slot source; the current generic adapter fills `warehouse_api` only when empty |
| BookedAt | optional RFC3339 |
| ConfirmedAt | optional RFC3339 |
| CancelledAt | optional RFC3339 |

WINDOW_START_BEFORE_WINDOW_END_REQUIRED=YES

The service rejects an item unless `WindowStart.Before(WindowEnd)`. The current generic adapter drops an unordered window during normalization, so that item never appears in `received`. A future vendor adapter must not emit an unordered window as a valid observation. Invalid facility or location strings are ignored as identifiers.

Enabled slot sources are `internal_booking`, `warehouse_api`, `carrier_api`, `shipper_api`, `manual_operator`, and `system_import`.

## 10. Delivery model

PUSH_INGEST_SUPPORTED=YES
PULL_POLLING_PROVIDER_IMPLEMENTED=NO

The current runtime is the three internal POST routes. `tracking-service` does not contain a polling scheduler, webhook subscription manager, websocket worker, or scheduled fetcher. A provider that can only be polled or subscribed to needs a separate stage. This contract does not add that worker.

## 11. Replay and deduplication

EXACTLY_ONCE_DELIVERY_CLAIM=NO
AT_LEAST_ONCE_PROVIDER_DELIVERY_ALLOWED=YES
SERVER_DEDUP_REQUIRED=YES

Inserts use `ON CONFLICT DO NOTHING` on the unique key `(tenant_id, dedup_key)` for `location_event`, `eta_observation`, and `shipment_slot_revision`. A conflict returns `inserted=false` and increments `deduplicated`. A provider may retry. An unchanged dedup key does not require a new event id.

Current key inputs are hashed with SHA-256 and hex-encoded:

| Family | Inputs |
|--------|--------|
| Location | provider code, provider device id, `recordedAt` in UTC RFC3339Nano, latitude and longitude to 7 decimal places. `providerEventID` is stored and is not part of this key |
| Shipment ETA | provider code, target type, server-derived shipment id, estimated arrival and source observed time in UTC RFC3339Nano, provider event id or empty |
| Execution-stop ETA | provider code, target type, execution stop id, estimated arrival and source observed time in UTC RFC3339Nano, provider event id or empty |
| Slot | provider code, slot type, server-derived shipment id, window start, window end, source observed time in UTC RFC3339Nano, provider slot id or empty, provider version or empty |

An adapter must preserve a vendor's immutable event, slot, or version identifier when the vendor defines one. It must not mint a replacement identifier on retry. This contract does not define a new dedup algorithm. Location dedup still collapses the same device, timestamp, and coordinates when the event id differs, because that is the current key.

## 12. Ordering and late events

NETWORK_ARRIVAL_ORDER_AUTHORITATIVE=NO

Freshness and quality use event time. Receipt order is not business order.

Telemetry current state uses `UpsertTrackingStateIfNewer`. Replacement requires `EXCLUDED.last_recorded_at > shipment_tracking_state.last_recorded_at`. An older or equal `recordedAt` may be stored as history when its dedup key is new, and it does not replace current position, speed, heading, or provider code.

ETA replacement uses `ShouldReplaceETAObservation`. Higher source priority replaces lower priority. Lower priority does not replace. Equal priority replaces only when `sourceObservedAt` is later, or when it is equal and `receivedAt` is not earlier. The same function guards execution-stop ETA after the execution row is locked.

Slot replacement uses `ShouldReplaceSlotState`. An unavailable candidate does not replace an available window. An inactive slot status does not replace an active status (`proposed`, `booked`, `confirmed`). Otherwise higher slot-source priority wins, then a strictly later `sourceObservedAt`. Equal observation time does not replace.

An older provider observation that arrives later does not become authoritative state when these comparisons reject it.

## 13. Clock

WIRE_TIMESTAMP_FORMAT=RFC3339
NORMALIZED_TIMEZONE=UTC

| Family | Field checked | Current tolerance |
|--------|---------------|-------------------|
| Telemetry | `RecordedAt` | reject the item when after `now + 5 minutes` |
| ETA | `SourceObservedAt` | reject the item when after `now + 5 minutes` |
| Slots | `SourceObservedAt` | reject the item when after `now + 5 minutes` |

`EstimatedArrivalAt`, `WindowStart`, and `WindowEnd` are not subject to that future check. Telemetry ingest has no additional past-time hard reject. Older `recordedAt` affects freshness, quality, and whether current state moves.

Provider clock synchronization is operational. It is not authentication and not tenant authority.

## 14. Batch result

A successful authenticated call returns HTTP 200:

```json
{"received":0,"accepted":0,"deduplicated":0,"rejected":0}
```

PARTIAL_BATCH_ACCEPTANCE_SUPPORTED=YES

`received` is the number of observations returned by the adapter. Each observation is then accepted, deduplicated, or rejected independently. One invalid observation does not roll back observations already accepted in that request.

| Condition | HTTP | Code | Message |
|-----------|------|------|---------|
| Secret missing, empty, unknown code, or mismatch | 401 | UNAUTHORIZED | provider authentication failed |
| Capability registry miss | 400 | VALIDATION_ERROR | unsupported provider |
| Adapter normalization error | 400 | VALIDATION_ERROR | invalid provider payload, invalid provider ETA payload, or invalid provider slot payload |
| Body read error | 400 | VALIDATION_ERROR | invalid request body |

Malformed payload at adapter level is a request validation failure and returns no partial success body. An unbound device or an invalid normalized observation remains inside the HTTP 200 counts. No additional external status code is defined.

## 15. Size and abuse

The ingest reader limit is 1 MiB. Process timeouts in `main.go` are header 5 seconds, read 15 seconds, write 15 seconds, and idle 60 seconds. Those timeouts are not a provider quota.

PROVIDER_RATE_LIMIT_PRESENT=NO
PER_PROVIDER_QUOTA_PRESENT=NO

GAP_TRACK_PROVIDER_001=no explicit per-provider rate limit or quota. This stage does not add one.

## 16. Network exposure

DIRECT_PUBLIC_PROVIDER_EXPOSURE_AUTHORIZED=NO

The provider routes are on the tracking-service router under `/internal/v1/tracking`. They are not customer gateway routes. A later exposure review must cover TLS, reverse proxy or ingress placement, secret transport, rate limiting, request size, audit, and IP policy when one is appropriate. This stage changes no gateway route and no network exposure.

## 17. Driver boundary

DRIVER_MOBILE_USES_PROVIDER_SECRET=NO

`IngestDriverLocation` requires `X-Internal-Service-Token`. Tenant comes from the trusted tenant header. Shipment id comes from the path. Driver id comes from `X-Driver-ID`. An optional vehicle id comes from `X-Vehicle-ID`. The service may then ensure a `driver_mobile` binding for that trusted identity and only afterwards calls location ingest with provider code `driver_mobile`.

`DriverMobileAdapter.Normalize` reuses `GenericAdapter`. Shared normalization does not give the driver path external provider-secret semantics, and it does not give an external provider the driver binding behavior.

## 18. Customer DTO boundary

CUSTOMER_API_VENDOR_NEUTRAL=YES
RAW_PROVIDER_PAYLOAD_EXPOSED_TO_CUSTOMER=NO

Shipper and carrier mappers return canonical shipment, status, freshness, quality, position, ETA, and slot fields. They do not return `providerDeviceId`, `providerEventId`, `providerSlotId`, `providerConfidence`, or the raw provider payload. Operator shipment tracking is a separate response and may include provider provenance already mapped there. That operator response is not a raw vendor payload passthrough to shipper or carrier reads.

## 19. Errors

| Category | Current behavior |
|----------|------------------|
| AUTH_FAILURE | HTTP 401 before the body is parsed; details are empty |
| UNSUPPORTED_PROVIDER | HTTP 400 after authentication when that capability registry misses the code |
| MALFORMED_PAYLOAD | HTTP 400 when the adapter returns an error |
| UNBOUND_DEVICE | item `rejected` on HTTP 200; no binding insert |
| INVALID_OBSERVATION | item `rejected` on HTTP 200 for empty device, bad coordinates, future source time, disabled source, bad target or slot type, or an unordered window at the service |
| DUPLICATE | item `deduplicated` on HTTP 200 |
| STORAGE_FAILURE | item `rejected` when insert or the execution-stop transaction fails; the rest of the batch continues |
| INTERNAL_FAILURE | unexpected errors map to HTTP 500 `INTERNAL_ERROR` |

The service does not publish a provider retry schedule. Binding lookup failure is logged with provider code and device id. Error bodies and logs must not contain the provider secret or the raw provider payload.

PROVIDER_ERROR_DETAILS_MUST_NOT_LEAK_SECRETS=YES

## 20. Observability

Current series have no provider, tenant, or device labels.

| Area | Metric |
|------|--------|
| Locations accepted | `tracking_ingest_telemetry_events_received_total` |
| Locations rejected | `tracking_ingest_telemetry_events_rejected_total` |
| Locations deduplicated | `tracking_ingest_telemetry_events_deduplicated_total` |
| Location lag | `tracking_ingest_telemetry_ingestion_lag_seconds` |
| ETA accepted | `eta_ingest_eta_observations_received_total` |
| ETA rejected | `eta_ingest_eta_observations_rejected_total` |
| ETA deduplicated | `eta_ingest_eta_observations_deduplicated_total` |
| ETA lag | `eta_ingest_eta_ingestion_lag_seconds` |
| Slots accepted | `slot_ingest_slot_observations_received_total` |
| Slots rejected | `slot_ingest_slot_observations_rejected_total` |
| Slots deduplicated | `slot_ingest_slot_observations_deduplicated_total` |
| Slot replacing transition | `slot_ingest_slot_reschedule_total` |
| Tracking state | `tracking_state_tracking_active_shipments`, `tracking_state_tracking_stale_shipments`, `tracking_state_tracking_lost_shipments` |

`slot_ingest_slot_reschedule_total` increments whenever a slot state replacement is recorded, including a first available window and a cancellation. Provider secrets and raw payloads are forbidden in metrics and logs. Tenant ids and device ids must not be added as unbounded metric labels.

PROVIDER_OBSERVABILITY_CONTRACT_READY=YES

The current series cover received, rejected, deduplicated, ingestion lag, slot replacement, and active, stale, and lost state. The absent rate limit in section 15 remains a product gap, not a missing metric name.

## 21. Loss detector

LOSS_DETECTOR_IMPLEMENTED=YES
LOSS_DETECTOR_DEFAULT=ENABLED

`TRACKING_LOSS_DETECTOR_ENABLED` defaults to true in `config.Load`. The detector starts only when that flag is enabled. Its threshold fallback is `CONTROL_TOWER_DRIVER_TRACKING_LOST_AFTER_MINUTES`, otherwise the tracking stale-minutes value. An accepted staging environment may override and disable the detector. This stage does not change the source default and does not change staging.

A provider adapter does not own Control Tower alert thresholds. The detector compares the last recorded location time with its configured threshold and emits transitions through the tracking outbox.

## 22. Schema and gateway impact

REAL_PROVIDER_NEEDS_NEW_SCHEMA=NO

Current tables already store provider code, provider device id, provider event id, provider slot id, and provider version on the normalized rows. A first adapter that maps into sections 7–9 does not need a migration. An identifier that cannot fit those columns would be a separate later decision. This stage creates no migration.

REAL_PROVIDER_NEEDS_GATEWAY_CHANGE=NO

Customer API gateway routes do not need to change for a provider that delivers to the existing internal ingest through later infrastructure routing. That routing is a future ingress decision, not a customer API change, and it is not authorized here.

TMS_CORE_CHANGE_REQUIRED=NO
TMS_MIGRATION_REQUIRED=NO

Existing execution-stop matching stays untouched.

## 23. Evidence required before vendor selection

This stage performs no vendor research and selects no vendor. A later selection must supply:

- provider delivery mechanism
- stable device identifier
- stable event identifier when the vendor has one
- timestamp semantics and clock source
- coordinate semantics and datum
- retry and replay behavior
- batch limits
- expected request rate
- authentication method
- telemetry capability
- ETA capability when claimed, including target types
- slot capability when claimed, including window and status vocabulary
- sandbox or test environment
- contractual SLA only when a contract evidences it
- data residency and compliance constraints where they apply

## 24. Future adapter acceptance

These tests are required before a real adapter is accepted. This stage does not implement them. Capability-specific cases run only for a capability the vendor declares.

VALID_PAYLOAD_NORMALIZES=PASS_REQUIRED
MALFORMED_PAYLOAD_REJECTED=PASS_REQUIRED
WRONG_SECRET_DENIED=PASS_REQUIRED
UNKNOWN_PROVIDER_DENIED=PASS_REQUIRED
UNKNOWN_DEVICE_NO_AUTOBIND=PASS_REQUIRED
DUPLICATE_REPLAY_DEDUPED=PASS_REQUIRED
CROSS_TENANT_SPOOF_BLOCKED=PASS_REQUIRED
TIMESTAMP_NORMALIZED=PASS_REQUIRED
OUT_OF_ORDER_STATE_SAFE=PASS_REQUIRED
CUSTOMER_DTO_VENDOR_NEUTRAL=PASS_REQUIRED
SECRET_NOT_LOGGED=PASS_REQUIRED

`CROSS_TENANT_SPOOF_BLOCKED` means a payload tenant or shipment claim cannot select a different tenant or shipment. The server binding remains the authority.

## 25. Future staging enablement

A first real provider is not enabled until all of the following are true:

PROVIDER_CODE_FROZEN_REQUIRED=YES
ADAPTER_TESTS_PASS_REQUIRED=YES
SECRET_CREATED_OUT_OF_REPO_REQUIRED=YES
NETWORK_INGRESS_REVIEW_REQUIRED=YES
RATE_LIMIT_DECISION_REQUIRED=YES
SANDBOX_ENDPOINT_VERIFIED_REQUIRED=YES
NO_PRODUCTION_CREDENTIALS_IN_STAGING_REQUIRED=YES

Required sequence:

1. this contract
2. vendor selection
3. adapter implementation
4. tests
5. Agent A infrastructure preflight
6. secret and network provisioning
7. controlled sandbox ingest
8. acceptance

Development tests must not use production provider data. Only a later stage may install staging secrets or network configuration.

REAL_PROVIDER_LIVE_INGEST_AUTHORIZED=NO

## 26. Gaps

GAP_TRACK_PROVIDER_001=no explicit per-provider rate limit or quota
GAP_TRACK_PROVIDER_002=no real vendor adapter is registered
GAP_TRACK_PROVIDER_003=a provider-neutral integration contract was not frozen before this 0.2A document
GAP_TRACK_PROVIDER_004=no live vendor sandbox acceptance has been run
GAP_TRACK_PROVIDER_005=active device uniqueness is per tenant; any-tenant lookup returns the newest `active_from` row

## 27. Scope protection

DRIVER_STAGING_AND_RELEASE_PATH_DEFERRED=YES

Driver release, CI, and staging remain a separate candidate stream. This contract does not modify TMS or driver-mobile.
