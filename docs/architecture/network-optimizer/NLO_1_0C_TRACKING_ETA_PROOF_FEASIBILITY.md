# NLO-1.0C tracking and ETA proof feasibility

```text
AGENT=D
STAGE=NLO-TRACKING-ETA-PROOF-FEASIBILITY-1.0C
BASE_SHA=3f306fc407aeda4bdc652d825954292f4a54fad6
ORIGIN_MAIN=3f306fc407aeda4bdc652d825954292f4a54fad6
RELATED_PRS=340,351,358
REAL_HANDLER_PROOF_EXECUTED=NO
PRODUCT_SEAM_REQUIRED=NO
PRODUCT_BEHAVIOR_CHANGED=NO
MIGRATIONS_CREATED=NO
SCORING_CHANGED=NO
STAGING_STARTED=NO
FINAL_VERDICT=PROOF_PATH_DEFINED
```

This document defines the proof path. It does not run it. A green description is not a passing test.

## Blockers already recorded

PR #340, head `e8dd45364802ca5849d53fadaf3dfc50cb71d67c`, proves vehicle capability through `VehicleCapabilityHandler.Get` and `sourceclient.HTTP`. Its tracking section says `InternalHandler` holds `*service.TrackingQueryService`, and that service holds `*repository.TrackingRepository`. A synthetic source cannot sit under the real lookup without a database or a product change. The missing-item check in `TestNLO10A_ProtocolFailuresStayDistinct` uses a hand-written `{"items":{}}`. That is a client check, not a handler proof.

PR #351, head `14f9e53150ef724c756c0000baa35506af5b15e2`, repeats that blocker and proposes repository interfaces for `LookupTrackingStates`, `BatchActiveBindings`, and `LookupETAStates`. It also names the tables in migrations `000026` and `000027`, and records that `applyMigrations` in `location_dedup_integration_test.go` keeps only migration numbers `<= 14` or `26`. That filter loads tracking state and skips ETA.

PR #358, head `a7f97f6d69637b74d17e7dcd15b56034c2c4c8da`, is the MAX_REVENUE hidden-rate regression. It does not touch tracking. All three pull requests stay unchanged.

## Real sources

The NLO caller is `sourceclient.HTTP.TrackingState` and `sourceclient.HTTP.TrackingETA`. Both POST `{"shipmentIds":[...]}` to the tracking base URL with `X-Internal-Service-Token` and `X-Tenant-ID`. `currenttrip.Provider.Build` treats a tracking or ETA error as optional. A vehicle capability error still aborts the build.

The producer path, already exported, is:

| Step | Symbol |
| --- | --- |
| Route | `POST /internal/v1/tracking/states/lookup` and `POST /internal/v1/tracking/eta/lookup` |
| Handler | `InternalHandler.LookupStates`, `ETAInternalHandler.LookupDelivery` |
| Service | `TrackingQueryService.LookupTrackingStates`, `ETAQueryService.LookupDeliveryETA` |
| Evaluator | `StateEvaluator.RefreshComputedState`, `ETAStateEvaluator.RefreshComputedState` |
| JSON | `mapSummary`, `mapETATarget` |
| SQL | `TrackingRepository.LookupTrackingStates`, `TrackingRepository.BatchActiveBindings`, `ETARepository.LookupETAStates` |

`cmd/server/main.go` already builds this chain with `NewTrackingRepository`, `NewETARepository`, `NewStateEvaluator`, `NewETAStateEvaluator`, `NewTrackingQueryService`, `NewETAQueryService`, `NewInternalHandler`, and `NewETAInternalHandler`. A test harness can call those same constructors. No new interface is required.

`executioncontract.ApplyPayload` is a stop-event consumer used by the shipment contract test. The tracking server does not import it. `GET /internal/v1/tracking/execution-stops/{id}/eta` is a different route. Neither is the NLO lookup proof.

## Isolated database

Minimum database:

- PostgreSQL 16, matching `embeddedpostgres.V16` already required by `services/tracking-service/go.mod`.
- A new empty database on loopback. The existing helper creates `freight_track_loc_<random>` and drops it in cleanup. Do not point the test at a shared or staging database.
- Apply these existing files, in this order, and no others:
  1. `infrastructure/migrations/000026_add_shipment_telemetry_foundation_v0.7.0.up.sql`
  2. `infrastructure/migrations/000027_add_shipment_eta_intelligence_v0.7.1.up.sql`
- `000026` creates schema `tracking`. `000027` creates `tracking.shipment_eta_state` in that schema. Neither file has a foreign key to another schema. PostgreSQL 16 provides `gen_random_uuid()` without a new extension.
- Seed with `INSERT` into `tracking.shipment_tracking_binding`, `tracking.shipment_tracking_state`, and `tracking.shipment_eta_state`. Do not add columns and do not execute handwritten `CREATE TABLE`.

Tables and predicates the lookup actually uses:

| Table | Predicate |
| --- | --- |
| `tracking.shipment_tracking_binding` | `tenant_id = $1 AND shipment_id = ANY($2) AND status = 'active'` |
| `tracking.shipment_tracking_state` | `tenant_id = $1 AND shipment_id = ANY($2)` |
| `tracking.shipment_eta_state` | `tenant_id = $1 AND shipment_id = ANY($2) AND target_type = 'delivery'` |

Later migrations do not alter those three lookup tables. `000091` alters `tracking.eta_observation` for execution stops. The NLO delivery lookup does not read that table, so `000091` is outside this proof.

Migration head remains `000105_platform_postgres_role_foundation_v0_2_i1`. This path does not apply head and does not claim a full-platform schema proof. The existing `applyMigrations` filter must not be reused: it never loads `000027`.

The harness must pass freshness thresholds in a `config.Config` value: 10 and 30 minutes for tracking, 15 and 60 minutes for ETA. Those are the defaults in `config.Load`. The test must not call `config.Load`, because that reads `DATABASE_URL` and the development token.

## Three proof levels

| Level | What is real | What it proves | Status |
| --- | --- | --- | --- |
| Unit mock | Hand-written JSON or a fake lookup method | Client branches for a missing key, malformed JSON, timeout, and an empty base URL | Already used in PR #340 for protocol cases. Not the handler proof. |
| Handler integration | Real handlers, evaluators, repository SQL, migrations `000026` and `000027`, ephemeral Postgres, and `sourceclient.HTTP` | The contract NLO consumes, including tenant SQL and freshness recomputation | Defined here. Not executed. |
| Full E2E | NLO server, tracking server, and shipment server on migration head `000105` with deployment configuration | A running environment | Out of scope. Staging is not started. |

Go internal packages block a direct import in either direction. The handler integration therefore uses a test-only process inside `tracking-service`. It listens on `127.0.0.1:0`, prints the URL, and exits when stdin closes. The NLO test starts that process and calls the existing client. The process uses the exported constructors above. It is test infrastructure, not a product seam and not a second encoder.

`currenttrip.Provider.Build` also needs execution context and onboard cargo. Those handlers are already exercised by the shipment harness on PR #340. This feasibility branch does not copy that harness. The tracking proof can call `TrackingState` and `TrackingETA` directly. Linking them through `Provider.Build` waits until that harness is on the same branch.

## Scenario the future test must run

Use two tenants and non-default values. Latitude `0` is a real coordinate when the field is present. A nil timestamp is not an arrival and not a position.

Caller tenant `aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1`. Foreign tenant `bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1`.

Tracking rows:

- Caller shipment `...0001`: active binding, latitude `55.751244`, longitude `37.618423`, `last_recorded_at` two minutes before the call. Expect `lastKnownPosition` with those coordinates, freshness `fresh`, and the same values after `sourceclient.HTTP.TrackingState`.
- Caller shipment `...0002`: latitude `0`, longitude `37.618423`, and a timestamp. Expect the zero latitude to remain.
- Caller shipment `...0003`: state row with null latitude and longitude. Expect no `lastKnownPosition`. NLO latitude and longitude stay nil. Freshness `unknown` and age `0` do not become a position.
- Caller shipment `...0004`: no row. Expect an item with `trackingStatus=not_configured` and freshness `unknown`, not an omitted key and not coordinates `(0,0)`.
- Foreign tenant, same shipment id as `...0001`: latitude `59.939095`, longitude `30.315868`. A caller lookup must not return those coordinates.

ETA rows, all `target_type=delivery`:

- Caller shipment `...0001`: `estimated_arrival_at=2026-11-12T18:00:00Z`, `source_observed_at` two minutes before the call. Expect status `available`, freshness `fresh`, and that same arrival after `TrackingETA`.
- Caller shipment `...0005`: `estimated_arrival_at` null and `source_observed_at` null. Expect no arrival field. NLO `EstimatedArrivalAt` stays nil.
- Caller shipment `...0004`: no ETA row. Expect an item with status `unavailable`, freshness `unknown`, and no arrival. This is the real missing-row result. It is different from the hand-written missing key, which the client maps to status `UNKNOWN`.
- Foreign tenant ETA for `...0001` at `2026-11-12T23:00:00Z`. The caller result must keep `18:00:00Z`.

Protocol cases, still on the real handlers:

- Wrong or missing `X-Internal-Service-Token`: HTTP 401 `UNAUTHORIZED`.
- Missing `X-Tenant-ID`: HTTP 400 `VALIDATION_ERROR`.
- Empty NLO base URL: `currenttrip.ErrUnavailable` and zero requests to the harness.

An invalid shipment id is skipped by the handler and produces no item. Do not collapse 400 and 401 into one code. Do not treat age `0` as a coordinate or an arrival time.

## Seam

A product seam is not required for this proof. The repository is already the source behind the real handler, and the two existing migrations create the lookup tables on an empty PostgreSQL 16 database.

The interface in PR #351 remains a fallback only if an ephemeral database is later refused. Its contract, not implemented here, is:

- `TrackingQueryService.LookupTrackingStates` calls `LookupTrackingStates` and `BatchActiveBindings` through an interface that `*TrackingRepository` already implements.
- `ETAQueryService.LookupDeliveryETA` calls `LookupETAStates` through an interface that `*ETARepository` already implements.
- `NewTrackingQueryService` and `NewETAQueryService` keep their current concrete parameters. `cmd/server/main.go` stays on those constructors.
- The fallback does not prove the SQL `tenant_id` predicate. The database path does.

## Acceptance criteria for the later test

The implementation is acceptable only when all of these are true:

- The test process is built from the exported tracking constructors and the real `mapSummary` / `mapETATarget` functions.
- Schema changes are limited to executing the two existing migration files above.
- Seed values travel through SQL, the real repository, the real evaluator, the real handler, and `sourceclient.HTTP`.
- The positive cases use the coordinates, zero latitude, null arrival, and foreign-tenant values in the scenario.
- The test deletes only its ephemeral database and process.
- `go test` output shows the new test name, and a package line of `ok` without that name is not accepted.
- NLO scoring, publication, rates, activation, and migrations are untouched.
- PR #340, PR #351, and PR #358 are not updated by that change.

`REAL_HANDLER_PROOF_EXECUTED=NO` until that test has been run.
