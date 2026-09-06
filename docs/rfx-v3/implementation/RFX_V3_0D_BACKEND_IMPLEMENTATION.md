# RFx v3.0D — Backend Implementation Report

**Status:** `IMPLEMENTED_BACKEND_PENDING_CONTROLLER_ACCEPTANCE`  
**Branch:** `feat/bintrans-enterprise-rfx-scoring-knockout-v3.0d`  
**Frontend:** `NOT_STARTED`

---

## Scope delivered

- Migration `000067_rfx_scoring_v3_0d` — score models, criteria, bindings, answer scores, qualification results
- Automatic scoring engine (NUMBER linear, YES_NO, SINGLE_SELECT, MULTI_SELECT)
- Knockout on persisted valid answers (post-validation business outcome)
- Explainability JSON per answer score
- Buyer score-model API (GET/PUT/validate/publish)
- Buyer response score read API (score + explanation)
- Post-submit scoring trigger (separate transaction; submit never rolled back)
- Idempotent score persistence `(response, answer, criterion, model_version)`
- Gateway RBAC parity (`PolicyBuyerManage` / `PolicyBuyerRead`)
- OpenAPI path stubs for v3.0D routes
- CI job `rfx-scoring-v3-integration` (PostgreSQL 16, fail-closed)

## Explicitly deferred

- MANUAL / HYBRID / SYSTEM_DERIVED v3 scoring
- Knockout override / Senior Buyer role
- Qualification pools / RFI→RFQ handoff
- `rfx_answers.score_model_version` column (**DEFERRED** — authoritative history in `rfx_answer_scores` / `rfx_qualification_results`)
- Full transactional outbox (`EVENT_TRANSPORT=DEFERRED_V3_0J`; audit records `rfx.score.calculated.v1` / `rfx.knockout.triggered.v1`)
- Frontend (web-admin Studio scoring, web-procurement evaluation extension)

## Legacy compatibility

- v1 `commercial_score` / `technical_score` / `total_score` / `evaluation_rank` **unchanged**
- v3 scoring activates only when pinned `rfx_version_id` has a **PUBLISHED** score model
- Award semantics unchanged in v3.0D

## Deterministic fixture (integration)

| Carrier | ADR | Fleet | Total | Knockout | Status |
|---|---|---|---|---|---|
| A | true | 50 | 70 | NO | QUALIFIED |
| B | false | 100 | 60 | YES | REJECTED |

Carrier B `ADR=false` answer remains persisted after submit.

## Key files

| Area | Path |
|---|---|
| Migration | `infrastructure/migrations/000067_rfx_scoring_v3_0d.up.sql` |
| Domain engine | `services/rfx-service/internal/domain/scoring_engine.go` |
| Repository | `services/rfx-service/internal/repository/score_repository.go` |
| Services | `score_model_service.go`, `scoring_service.go` |
| Submit hook | `carrier_response_service.go` |
| Handlers | `http/handlers/score_handler.go` |
| Integration | `internal/integration/scoringv3/` |

## Validation

| Check | Status |
|---|---|
| `go test ./...` (rfx-service) | PASS (unit) |
| `go build ./...` (rfx-service, api-gateway) | PASS |
| `rfx-scoring-v3-integration` | PENDING CI PostgreSQL gate |

---

**NEXT_ACTION:** Controller authorize v3.0D frontend after backend CI PASS.
