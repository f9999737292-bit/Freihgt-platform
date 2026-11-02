# TMS Control Tower shadow baseline 0.1

```text
STAGE=TMS-CONTROL-TOWER-SHADOW-BASELINE-0.1
BASE_SHA=2a99b615a237b6d1f6e6038ca86a7433c567dc78
KIND=DOCS_ONLY_ARCHITECTURE
PRODUCT_CODE_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
OPENAPI_AUTHORIZED=NO
GATEWAY_CHANGE_AUTHORIZED=NO
STAGING_MUTATION_AUTHORIZED=NO
PRIMARY_MODE_ACTIVATION_AUTHORIZED=NO
VENDOR_ADAPTER_IMPLEMENTATION_AUTHORIZED=NO
LIVE_PROVIDER_INGEST_AUTHORIZED=NO
PRODUCT_MIGRATION_HEAD=000105
```

This document records the Control Tower shadow read model and the tracking boundary on `origin/main` `2a99b615a237b6d1f6e6038ca86a7433c567dc78`. It does not change product code, migrations, OpenAPI, compose, or rollout policy. It does not enable `CONTROL_TOWER_READ_MODEL_MODE=primary` and it does not add a vendor adapter.

Inventory was read from this SHA. Runtime of the dedicated Control Tower staging VM was not observed in this stage.

## Status labels

| Label | Meaning in this document |
| --- | --- |
| `IMPLEMENTED` | Present in source on this SHA |
| `DEFERRED` | Named by an accepted contract and left for a later authorized stage |
| `BLOCKED` | A gate in source or an accepted ADR forbids the step now |
| `NOT_VERIFIED` | Not executed or not observed in this stage |

## Current pins

```text
READ_MODEL_SERVICE_PRESENT=IMPLEMENTED
GATEWAY_MODE_PARSER_PRESENT=IMPLEMENTED
MODES=disabled,shadow,primary
BASE_COMPOSE_GATEWAY_MODE=disabled
STAGING_SHADOW_COMPOSE_GATEWAY_MODE=shadow
STAGING_PREFLIGHT_REQUIRES_EFFECTIVE_MODE=shadow
STATUS_SUMMARY_PUBLIC_SOURCE_IN_SHADOW=LEGACY
STATUS_SUMMARY_COMPARISON_IN_SHADOW=IMPLEMENTED
PRIMARY_CODE_PATH=IMPLEMENTED
PRIMARY_ACTIVATION=BLOCKED
TRANSACTIONAL_SHIPMENT_SOURCE=shipment-service / transport.shipments
TRACKING_SERVICE_PRESENT=IMPLEMENTED
REAL_VENDOR_ADAPTER_PRESENT=NO
VENDOR_SELECTED=NO
PROVIDER_PUSH_ROUTES=INTERNAL_ONLY
GATEWAY_PROVIDER_INGEST_ROUTE=ABSENT
TRACKING_APPROACH_CONSUMER_DEFAULT=OFF_UNTIL_TOPIC_SET
DRIVER_EVENTS_CONSUMER_DEFAULT=false
STAGING_VM_EFFECTIVE_MODE=NOT_VERIFIED
COHORT_APPROVED_IN_THIS_STAGE=NOT_VERIFIED
```

## 1. Read-model mode

`services/api-gateway/internal/controltowerreadmodel/mode.go` accepts `disabled`, `shadow`, and `primary`. An empty value parses as `disabled`. Any other value fails configuration validation.

`Mode.Enabled()` is true for both `shadow` and `primary`. `disabled` does not construct the read-model HTTP client (`services/api-gateway/internal/controltower/service.go`).

| Surface | `disabled` | `shadow` | `primary` |
| --- | --- | --- | --- |
| Public `statusSummary` source | Legacy aggregate, else page-limited legacy | Same legacy source. Read model is fetched for comparison only | Read model, with legacy fallback on dependency or consumer-not-running |
| Workspace, risks, workflow, acknowledgement, automation | Read-model client absent; callers that require it fail or return empty | Read-model calls run. They are not a legacy comparison | Same read-model calls as shadow |

The statement "shadow does not change the user-facing response" in [CONTROL_TOWER_FULL_STATUS_BASELINE.md](../../CONTROL_TOWER_FULL_STATUS_BASELINE.md) applies to the status summary merge. It does not describe workspace, risk, workflow, acknowledgement, or automation. Those paths key off `Mode.Enabled()`, so they run in `shadow`.

### 1.1 Status summary merge

`Merge` in `services/api-gateway/internal/controltowerreadmodel/merge.go`:

- `shadow` sets `statusSummary.source` to legacy when a usable legacy input exists.
- Comparison results are `MATCH`, `TOTAL_MISMATCH`, `STATUS_COUNT_MISMATCH`, `LEGACY_LIMITED_DATASET`, `LEGACY_FULL_AGGREGATE_UNAVAILABLE`, `LEGACY_FULL_AGGREGATE_INCOMPLETE`, `LEGACY_UNAVAILABLE`, `READ_MODEL_UNAVAILABLE`, and `READ_MODEL_NOT_RUNNING` (`compare.go`).
- `applyStatusSummaryMerge` copies the summary, freshness, and warnings. It does not copy the comparison result into the public summary. The gateway records the comparison in metrics and the shadow log (`read_model_merge.go`).
- A read-model error in `shadow` does not replace the legacy summary.
- `primary` serves the read model when the fetch succeeds and the consumer is running when required. Failure sets `fallbackUsed` and returns the legacy summary. This code path is `IMPLEMENTED`. Turning the process to `primary` is `BLOCKED`.

Legacy input order is unchanged from the full-status baseline: complete `GET /internal/v1/shipments/status-summary` from shipment-service, then page-limited counts from the shipment list. Exact comparison runs only when the full aggregate is available and not limited.

Default timeouts in gateway config are `800ms` for the read model and, in the full-status baseline, `800ms` for the legacy aggregate. Neither dependency blocks gateway `/ready` (`services/api-gateway/README.md`).

### 1.2 Where the mode is set

| Location | Effective gateway mode | Status |
| --- | --- | --- |
| `infrastructure/docker-compose/docker-compose.yml` | `disabled` | `IMPLEMENTED` repository default |
| `infrastructure/docker-compose/docker-compose.staging-shadow.yml` | `shadow`, with `CONTROL_TOWER_CONSUMER_ENABLED=true` and `SHIPMENT_OUTBOX_ENABLED=true` | `IMPLEMENTED` override. The file comment forbids treating this as a secret store |
| `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_preflight.sh` | Fails unless the rendered api-gateway mode is `shadow` | `IMPLEMENTED` gate. `primary` and `disabled` both fail that check |
| Dedicated staging VM | Unknown in this stage | `NOT_VERIFIED`. No SSH and no live compose render were run |

`CONTROL_TOWER_READ_MODEL_MODE=primary` remains unauthorized:

- [BINTRANS_PRODUCT_ROADMAP_2026_2027_NEXT.md](../BINTRANS_PRODUCT_ROADMAP_2026_2027_NEXT.md) sets `CONTROL_TOWER_READ_MODEL_MODE_PRIMARY_AUTHORIZED=NO`.
- [ADR-PLAT-002](../../adr/ADR-PLAT-002-agent-ready-action-model.md) repeats that pin and does not authorize domain writes from the read model.
- [BINTRANS_STAGING_SHADOW_SMOKE.md](../../BINTRANS_STAGING_SHADOW_SMOKE.md) states that primary stays disabled on BINTRANS staging.

This stage does not approve a cohort and does not run Day 0 observation.

## 2. What the read model is

`control-tower-read-model-service` is a projection. `transport.shipments` in shipment-service remains the transactional shipment status source ([CONTROL_TOWER_FULL_STATUS_BASELINE.md](../../CONTROL_TOWER_FULL_STATUS_BASELINE.md)).

Consumers in `services/control-tower-read-model-service/cmd/server/main.go`:

| Consumer | Starts when | Staging-shadow compose on this SHA |
| --- | --- | --- |
| Shipment status | `CONTROL_TOWER_CONSUMER_ENABLED=true` (default `false`) | `true`. Topic default `shipment.status.v1` |
| Driver events | `CONTROL_TOWER_DRIVER_EVENTS_ENABLED=true` (default `false`) | Default `false` via `${CONTROL_TOWER_DRIVER_EVENTS_ENABLED:-false}` |
| Tracking approach | Status consumer enabled and `CONTROL_TOWER_TRACKING_KAFKA_TOPIC` is non-empty | Topic variable is absent from the shadow override, so this consumer stays off |

The shipment-status consumer classifies `tracking.stop.approaching` as `RouteApproach` and can apply it when that payload arrives on the topic it reads (`event_route.go`, `consumer.go`). The dedicated tracking consumer uses a separate topic and group. Its default group id, if a topic is set, is `control-tower-tracking-approach-v1`, and that group must not equal the shipment-status group.

Whether a live broker is publishing `tracking.stop.approaching`, and whether any staging process has set `CONTROL_TOWER_TRACKING_KAFKA_TOPIC`, is `NOT_VERIFIED`.

## 3. Tracking boundary

Accepted contract: [ADR-TRACK-001](../../adr/ADR-TRACK-001-external-provider-ingest-contract.md) and [tracking-external-provider-contract-v0.2a.md](../tracking-external-provider-contract-v0.2a.md).

```text
VENDOR_SELECTED=NO
VENDOR_ADAPTER_IMPLEMENTATION_AUTHORIZED=NO
STAGING_PROVIDER_ENABLEMENT_AUTHORIZED=NO
PROVIDER_CREDENTIAL_CREATION_AUTHORIZED=NO
LIVE_INGEST_AUTHORIZED=NO
REAL_VENDOR_ADAPTER_PRESENT=NO
```

`services/tracking-service/cmd/server/main.go` registers:

| Port | Adapters |
| --- | --- |
| Telemetry | `generic`, `driver_mobile` |
| ETA | `generic` |
| Slots | `generic` |

No other `ProviderCode()` exists under `services/tracking-service`. `driver_mobile` is telemetry-only. ETA and slot registries do not include it. `DriverMobileAdapter.Normalize` delegates to `GenericAdapter`; that is not a vendor adapter.

Provider push routes on tracking-service:

- `POST /internal/v1/tracking/providers/{provider}/locations`
- `POST /internal/v1/tracking/providers/{provider}/eta`
- `POST /internal/v1/tracking/providers/{provider}/slots`

Authentication is `X-Provider-Secret`. The driver path is separate: `POST /internal/v1/tracking/driver/shipments/{shipmentId}/locations`, and it does not use the provider secret.

API Gateway on this SHA exposes read routes for operator, shipper, and carrier tracking, ETA, and slots, plus `POST /api/v1/driver/me/shipments/{shipmentId}/locations`. `services/api-gateway/internal/http/router.go` has no provider ingest route. Direct public exposure of provider routes stays unauthorized by ADR-TRACK-001.

Customer reads stay vendor-neutral. Provider payload remains opaque to domain and repository code. An unknown provider code is rejected. Provider ingest does not create a shipment binding. Polling, websocket, and stream subscription are not the current runtime; ADR-TRACK-001 records inbound push only.

Control Tower does not own provider secrets, provider device binding, or telemetry storage. It may project an approach event after tracking-service has published one. Alert thresholds stay outside the provider adapter. The loss detector remains a tracking-service process.

A missing per-provider rate limit remains the gap already named by ADR-TRACK-001. This document does not add one.

## 4. Explicitly closed

| Item | Status | Why |
| --- | --- | --- |
| `CONTROL_TOWER_READ_MODEL_MODE=primary` | `BLOCKED` | Roadmap, ADR-PLAT-002, and staging preflight. Code path exists and is not an activation permit |
| Real vendor adapter | `BLOCKED` | ADR-TRACK-001. Registries contain `generic` and telemetry `driver_mobile` only |
| Live provider ingest and credential creation | `BLOCKED` | Same ADR |
| Public provider route on the gateway | `BLOCKED` | Route absent; ADR forbids direct public exposure |
| New migration `000106` | `BLOCKED` for this stage | Product head on this SHA is `000105` (`000105_platform_postgres_role_foundation_v0_2_i1`). No `000106` file is present |
| Resume `feat/control-tower-guarded-actions-v0.8.1` | `DEFERRED` / out of scope | This stage starts from `origin/main` only. Commit distance of that branch was not remeasured here (`NOT_VERIFIED`) |
| Unblock `ExecuteAwardOrder` | `DEFERRED` | Caller-identity contract belongs to Agent A (`PLATFORM-AWARD-LOCK-CALLER-IDENTITY-0.1`). This document does not change shipment-service |

`services/shipment-service/internal/integration/transportexecution/role_foundation_login_integration_test.go` expects `rfx.lock_award_transport_order_for_execution` to be absent and expects award-row update under the shipment login to return SQLSTATE `42501`. That test was not executed in this stage (`NOT_RUN`). The expectation is source evidence only.

## 5. Dependencies

| Dependency | Owner | Effect on this baseline |
| --- | --- | --- |
| Award-lock caller identity | Agent A | Future award cutover. Not required to record shadow or tracking facts |
| Document requirement rebaseline | Agent B | Future POD completeness. Control Tower must not treat a document row as `POD_REQUIRED` ([TMS_POD_REQUIREMENT_SOURCE_0_4C.md](TMS_POD_REQUIREMENT_SOURCE_0_4C.md)) |
| NLO public next-load fields | Agent D | Future Control Tower display of next-load. No Control Tower file in that slice, and this document does not add the fields |
| Carrier-score policy | Agent E | Not a shadow-mode input |
| Customer portal baseline | Agent F | Portals read tracking through gateway routes already listed. They do not select the Control Tower mode |
| Operator read-only staging check | Operator | Required before any claim that the VM is in `shadow`. Not run here |
| PR #322 | Autopilot queue | Files under `tools/bintrans_autopilot` and `docs/engineering/autopilot`. Not modified |

## 6. Open questions

1. Which Control Tower HTTP routes are intentionally projection-backed in `shadow`, and which must stay legacy until a primary gate? Source shows the split in section 1. A product decision is not in this SHA (`NOT_VERIFIED` as policy).
2. Is `CONTROL_TOWER_TRACKING_KAFKA_TOPIC` set on the dedicated staging VM? Repository shadow compose does not set it. Live value is `NOT_VERIFIED`.
3. Is `COHORT_APPROVED` yes or no on that VM? This stage did not read `/protected/**`.
4. Does any unpublished branch still carry guarded-action behavior that `main` lacks? Not reviewed. That branch is not the base of this document.

## 7. Next stage

```text
NEXT_RECOMMENDED_STAGE=TMS-CONTROL-TOWER-SHADOW-SURFACE-INVENTORY-0.2
NEXT_STAGE_KIND=DOCS_ONLY
PRIMARY_MODE_IN_NEXT_STAGE=NO
VENDOR_ADAPTER_IN_NEXT_STAGE=NO
```

The next document should list each gateway Control Tower route and mark it `LEGACY_PUBLIC`, `READ_MODEL_PUBLIC_IN_SHADOW`, or `COMPARISON_ONLY`, using this SHA's split as the starting inventory. It should not enable primary mode, add an adapter, open a provider route, or change migration `000105`.

A live shadow observation remains a separate operator-authorized read-only step. It is not the next document, and it is not authorized here.

## 8. Sources

| Source | What it fixes |
| --- | --- |
| `services/api-gateway/internal/controltowerreadmodel/mode.go` | Mode enum |
| `services/api-gateway/internal/controltowerreadmodel/merge.go` | Shadow keeps legacy summary; primary code path |
| `services/api-gateway/internal/controltowerreadmodel/compare.go` | Comparison codes |
| `services/api-gateway/internal/controltower/read_model_merge.go` | Fetch plus metric/log |
| `services/api-gateway/internal/controltower/service.go` | Client only when mode is enabled |
| `services/api-gateway/internal/controltower/workspace.go`, `risk.go`, `automation.go` | Read-model use under `Mode.Enabled()` |
| `infrastructure/docker-compose/docker-compose.yml` | Default `disabled` |
| `infrastructure/docker-compose/docker-compose.staging-shadow.yml` | Shadow override and consumer flags |
| `scripts/ops/bintrans_ct_staging/bintrans_ct_staging_preflight.sh` | Effective mode must be `shadow` |
| `services/control-tower-read-model-service/cmd/server/main.go` | Three consumers |
| `services/control-tower-read-model-service/internal/config/config.go` | Tracking consumer enable rule; driver default |
| `services/tracking-service/cmd/server/main.go` | Adapter registries |
| `services/tracking-service/internal/http/router.go` | Internal provider routes |
| `services/api-gateway/internal/http/router.go` | Public tracking reads; no provider ingest |
| `docs/adr/ADR-TRACK-001-external-provider-ingest-contract.md` | Vendor boundary closed |
| `docs/adr/ADR-PLAT-002-agent-ready-action-model.md` | Primary and autonomous writes closed |
| `docs/CONTROL_TOWER_FULL_STATUS_BASELINE.md` | Legacy aggregate and shadow comparison rules |
| `infrastructure/migrations/000105_platform_postgres_role_foundation_v0_2_i1.up.sql` | Product migration head file on this SHA |

```text
DOCS_ONLY=YES
PRODUCT_CODE_CHANGED=NO
MIGRATIONS_CHANGED=NO
STAGING_CHANGED=NO
```
