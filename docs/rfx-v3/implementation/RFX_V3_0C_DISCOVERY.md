# RFx v3.0C — Discovery Report

**Status:** `DISCOVERY_COMPLETE`  
**Base:** `origin/main` @ `9daee4c` (v3.0B merged)  
**Branch:** `feat/bintrans-enterprise-rfx-carrier-response-v3.0c`

---

## CURRENT_RESPONSE_MODEL

| Artifact | State |
|----------|--------|
| **`rfx.rfx_responses`** | One row per `(rfx_event_id, participant_company_id)`; status `DRAFT`/`SUBMITTED` (+ legacy `WITHDRAWN`/`REJECTED`/`ACCEPTED` in DB) |
| **`rfx.rfx_response_offer_lines`** | Commercial pricing lines (000038) |
| **`rfx.rfx_participants`** | Invitation/participation gate |
| **Answer persistence** | **None** — no `rfx_answers` table |

Go domain uses only `DRAFT` and `SUBMITTED`. Architecture maps `IN_PROGRESS` → DB `DRAFT`.

---

## CURRENT_CARRIER_PORTAL

| App | Routes | Role |
|-----|--------|------|
| **`web-procurement`** | `/carrier/tenders`, `/carrier/tenders/[id]` | Primary carrier RFx UI |
| **`web-admin`** | `/rfx/[id]/studio/*` | Buyer questionnaire builder only |

API client: `apps/web-procurement/composables/useCarrierRfxApi.ts`

Existing flow: list tenders → create DRAFT response → patch commercial offer → submit.

**No questionnaire answer APIs consumed today.**

---

## CURRENT_RESPONSE_STATUS_MODEL

| Product (derived) | DB / implementation |
|-------------------|---------------------|
| `NOT_STARTED` | No `rfx_responses` row |
| `IN_PROGRESS` | `DRAFT` |
| `SUBMITTED` | `SUBMITTED` |

Submit updates participant to `RESPONSE_SUBMITTED`. Event must be `PUBLISHED` or `RESPONSES_OPEN` with open deadline.

---

## REUSE_PLAN

| Area | Action |
|------|--------|
| `rfx_responses` aggregate | **Extend** with `rfx_version_id`, `save_version`, `last_saved_at`, `completion_percent` |
| `CreateResponse` / `SubmitResponse` | **Keep** commercial path; extend start to pin published version |
| Carrier auth | **Reuse** `carrier_authorization.go`, participant checks |
| Questionnaire definition | **Reuse** `QuestionnaireRepository`, `rule_engine.go`, question types |
| Answer validation | **New** L1–L4 layer on top of definition + rules |
| Gateway | **Extend** `rfxrbac` carrier routes + new carrier-response paths |
| Frontend | **Extend** `web-procurement` tender detail with questionnaire workspace tab |
| OpenAPI | **Extend** `rfx-service.yaml` (currently missing carrier response paths) |
| Browser E2E | **Extend** pattern from v3.0B studio browser gate (production api-gateway) |

---

## MIGRATION_REQUIRED

**Migration `000066_rfx_carrier_response_v3_0c`:**

1. **ALTER `rfx.rfx_responses`**
   - `rfx_version_id UUID REFERENCES rfx.rfx_versions(id)`
   - `save_version BIGINT NOT NULL DEFAULT 0`
   - `last_saved_at TIMESTAMPTZ`
   - `last_saved_by UUID`
   - `completion_percent NUMERIC(5,2) NOT NULL DEFAULT 0`

2. **CREATE `rfx.rfx_answers`**
   - Unique `(rfx_response_id, question_id)`
   - Index `(tenant_id, rfx_response_id)`
   - Composite tenant-safe FKs

**Not in v3.0C:** scoring tables, preview session table (preview uses client sandbox initially).

---

## API MATRIX (v3.0C target)

| Method | Path | Gateway policy | rfx-service |
|--------|------|----------------|-------------|
| GET | `/api/v1/rfx-events/{id}/carrier-response` | CarrierRead | New |
| POST | `/api/v1/rfx-events/{id}/carrier-response/start` | CarrierRespond | New (idempotent) |
| PATCH | `/api/v1/rfx-events/{id}/carrier-response/answers` | CarrierRespond | New (atomic batch) |
| POST | `/api/v1/rfx-events/{id}/carrier-response/validate` | CarrierRespond | New |
| POST | `/api/v1/rfx-events/{id}/carrier-response/submit` | CarrierRespond | New (questionnaire gate) |
| GET | `/api/v1/rfx-events/{id}/carrier-response/summary` | CarrierRead | New |
| POST | `/api/v1/rfx-events/{id}/responses` | CarrierRespond | **Legacy kept** |
| PATCH | `/api/v1/rfx-responses/{id}` | CarrierRespond | **Legacy commercial** |
| POST | `/api/v1/rfx-responses/{id}/submit` | CarrierRespond | **Legacy kept** |

---

## ResponseVersion mapping

| Domain concept | Implementation |
|----------------|----------------|
| `ResponseVersion` | `rfx_responses.save_version` (BIGINT optimistic concurrency for answer batches) |
| Row lock | Existing `rfx_responses.version` (INT) for submit/status transitions |
| Per-answer lock | `rfx_answers.version` |

---

## HIDDEN_ANSWER_POLICY

**`IGNORE_ON_SAVE`:** Questions hidden by evaluated SHOW/HIDE rules are excluded from validation, completion denominator, and persistence. Any previously stored answer for a newly hidden question is deleted on the next successful save batch.

---

## STOP conditions

None triggered — legacy `rfx_responses` extend safely; v3.0B buyer 400 contract preserved separately from carrier 422 contract.
