# API contracts

Skeleton only. Production spec `packages/openapi` is unchanged. The draft file is [contracts/network-optimizer-v0.1.openapi.yaml](contracts/network-optimizer-v0.1.openapi.yaml).

Routes follow the repository prefix `/api/v1/...` rather than a bare `/network` root. Gateway remains the public entrance.

## Operations

| Method and path | Intent |
|-----------------|--------|
| `POST /api/v1/network/capacities` | Create capacity |
| `GET /api/v1/network/capacities/{id}` | Read capacity if in scope |
| `POST /api/v1/network/load-opportunities` | Publish or draft an opportunity |
| `GET /api/v1/network/load-opportunities/{id}` | Read if in scope |
| `POST /api/v1/network/matches/search` | Run feasibility and deterministic ranking for the caller |
| `POST /api/v1/network/route-plans/generate` | Build a plan from a feasible set |
| `POST /api/v1/network/consolidation/analyze` | Hard-constraint consolidation |
| `POST /api/v1/network/offers` | Create an offer from a match |
| `POST /api/v1/network/offers/{id}/accept` | Accept |
| `POST /api/v1/network/offers/{id}/counter` | Counter |
| `POST /api/v1/network/offers/{id}/reject` | Reject |

Search and generate are commands that create an `OptimizationJob`. They are not anonymous queries.

## Classification

| Operation | public carrier API | public shipper API | internal service API | platform-only | driver app | web-admin |
|-----------|--------------------|--------------------|----------------------|---------------|------------|-----------|
| Create own capacity | yes | no | yes, same rules | audit | no in v0.1 | support view |
| Read capacity | own or published scope | only if scope includes them | yes | audit | own assignment later | audit |
| Create load opportunity | no | yes, own orders | yes | no | no | no |
| Read load opportunity | in scope | own | yes | audit | no | audit |
| Match search | own capacity | own loads | yes | no | no | no |
| Generate route plan | own | own fleet or own loads | yes | no | no | no |
| Analyze consolidation | own | own | yes | no | no | no |
| Create / counter / reject offer | yes if recipient or owner | owner of the load | yes | no | no | no |
| Accept offer | recipient carrier | no | yes | no | no | no |
| Read score internals and foreign snapshots | no | no | yes | audit | no | audit |
| City rule authoring | no | no | no | yes | no | later admin |
| Prediction provider ingest | no | no | yes | yes | no | no |

Optimizer internals (full decision snapshot, other tenant keys, raw GPS history) are not on the public carrier or shipper APIs. The carrier explanation is a redacted `score_explanation`.

## Errors

| Situation | Behavior |
|-----------|----------|
| Out of scope id | 404 without revealing that the row exists |
| Version conflict | 409 with current version |
| Hard reject | 200 search result or 422 on accept, with reason codes |
| Unpriced revenue objective | 422 naming the missing commercial input |
| Stale city rules on urban generate | 422 `CITY_RULES_STALE` |

## Examples

- [examples/load-opportunity.json](examples/load-opportunity.json)
- [examples/predicted-capacity.json](examples/predicted-capacity.json)
- [examples/match-explanation.json](examples/match-explanation.json)
- [examples/city-rule.example.json](examples/city-rule.example.json) — `EXAMPLE_NOT_NORMATIVE`

## Slot, driver, and shipment calls

These stay on existing owners. The draft does not add booking, driver, or shipment mutation routes.
