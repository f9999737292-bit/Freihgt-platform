# RFx v3.0E5 — Template Clone + Provenance

**Status:** IMPLEMENTED_ACCEPTED

**Merged to main:** PR #115 via merge commit `81ff86b0f54b1053491d20dc06f51c4dfef537fd`
**Migration:** `000071_rfx_template_clone_provenance_v3_0e5`  
**Scope:** E5 only (no E6/E7, no v3.0F, no staging/pilot)

---

## 1. Controller acceptance record

| Field | Value |
|---|---|
| `STATUS` | IMPLEMENTED_ACCEPTED |
| `CONTROLLER_ACCEPTANCE` | YES |
| `PR115_MERGED` | YES |
| `PR115_HEAD` | `5c26d34e3e623ac1d1be414455c8a28b18ae1c62` |
| `PR115_MERGE_SHA` | `81ff86b0f54b1053491d20dc06f51c4dfef537fd` |
| `CI_RUN_ID` | 34401634396 |
| `CI_EXACT_HEAD` | YES |
| `CI_CONCLUSION` | success |
| `E5_001_E5_005_CLOSED` | YES |
| `MIGRATION_000071_ON_MAIN` | YES |
| `MIGRATION_000072_CREATED` | NO |

---

## 2. Summary

Buyer-managed clone of a published (or explicitly selected superseded) RFx template version into a new RFx event with immutable template provenance, initial event DRAFT questionnaire version, deep graph materialization (sections/questions/options/rules with remapped rule targets), mandatory idempotency, and audit `rfx.event.created_from_template.v1`.

Scoring, responses, invitations, offers, and evaluation results are **not** copied or created.

E5 delivers **only** the buyer RFQ creation channel **from published template** (`POST /rfx-events/from-template`). Manual event creation, Excel import/export, SAP/1C/ERP/TMS integration, late submission workflows, and carrier Excel import/export are **not** implemented in E5.

---

## 3. API

| Method | Gateway path | Service path | Auth | Notes |
|--------|--------------|--------------|------|-------|
| POST | `/api/v1/rfx-events/from-template` | `/v1/rfx-events/from-template` | BuyerManage | **201**, **Idempotency-Key** required |

Gated by `RFX_VERSIONING_V3_ENABLED` in rfx-service (same as E1–E4 versioning).

### Request body

- `template_version_id` (required UUID)
- Event create fields: `rfx_number`, `rfx_type`, `category`, `title`, `owner_company_id`, optional `description`, `currency_code`, `valid_from`, `valid_to`, `response_deadline`

### Response (201)

Event fields plus:

- `draft_version_id`
- `source_template_id`
- `source_template_version_id`
- `source_version_number`
- `source_version_status` (`PUBLISHED` or `SUPERSEDED`)
- `source_version_warning` (true when cloning explicit `SUPERSEDED` source)

---

## 4. Source version rules

| Source status | Result |
|---------------|--------|
| `PUBLISHED` | **201** clone |
| `SUPERSEDED` (explicit version id) | **201** clone + `source_version_warning=true` |
| `DRAFT` | **409** conflict |
| Template aggregate `ARCHIVED` | **409** conflict |
| Cross-tenant template version | **404** |

---

## 5. Provenance (DB)

Migration `000071`:

- Column `rfx.rfx_events.source_template_version_id` (nullable; NULL for manual create)
- Composite FK `(tenant_id, source_template_version_id)` → `rfx.rfx_template_versions(tenant_id, id)`
- Trigger `trg_rfx_events_provenance_immutable` — UPDATE of provenance forbidden
- Supporting unique index on `rfx_template_versions(tenant_id, id)`

Manual event creation leaves provenance NULL.

---

## 6. Transaction (atomic)

Single transaction:

1. Lock template aggregate
2. Validate tenant/company access and ACTIVE aggregate
3. Lock selected template version; validate PUBLISHED or SUPERSEDED
4. Create event with provenance
5. Create initial DRAFT event version
6. Deep-copy questionnaire graph (new UUIDs; rule targets remapped)
7. Record audit `rfx.event.created_from_template.v1`
8. Store idempotency replay payload
9. Commit

Any failure rolls back all mutations.

---

## 7. Idempotency

- Operation: `CLONE_EVENT_FROM_TEMPLATE`
- Scope: `tenant_id` + `actor_id` + operation + `template_version_id`
- Header: `Idempotency-Key` (required, max 128)
- Same key + same body → replay same event (**201** semantics via stored payload)
- Same key + different body → **409**
- Expired key reuse → new event

---

## 8. Not copied

| Artifact | Copied |
|----------|--------|
| Score models | NO |
| Responses | NO |
| Invitations / participants | NO |
| Offers / bids | NO |
| Evaluation results | NO |

---

## 9. Controller remediation (E5-001..E5-005)

| Item | Scope | Outcome |
|---|---|---|
| **E5-001** | PR body restoration with full summary, scope, tests, CI evidence | Closed |
| **E5-002** | Migration 000071 down cleanup — schema-qualified index drops; REM-004 validates full object cleanup + legacy data + up restore | Closed |
| **E5-003** | Feature-flag HTTP tests — REM-005/INT-32 real **404** when disabled; REM-006 HTTP **201** when enabled | Closed |
| **E5-004** | Atomic rollback coverage — REM-001 graph copy inject at option phase; REM-002 audit; REM-003 idempotency | Closed |
| **E5-005** | Acceptance evidence hardening — canonical graph equality, full UUID isolation, real template/event draft mutations, discovered zero business artifacts, E5-REM-007..012 | Closed at `5c26d34` |

---

## 10. Integration tests

Package: `services/rfx-service/internal/integration/templatelibrary/`  
CI job: `rfx-version-lifecycle-v3-integration` with `REQUIRE_TEST_DATABASE=1`

| Suite | Coverage |
|---|---|
| E5-INT-01..32 | Clone, provenance, isolation, canonical graph equality, UUID disjointness, rule target remap, template/event mutation independence, zero business artifacts, idempotency, migration up/down, gateway alignment, feature flag fail-closed HTTP |
| E5-REM-001..012 | Graph/audit/idempotency rollback, down migration cleanup, feature-flag HTTP, canonical equality, UUID isolation, mutation isolation, zero artifacts, rule targets event-local |
| E1–E4 regressions | Template library and version lifecycle continuity on shared CI job |

All suites passed on PR #115 head `5c26d34e3e623ac1d1be414455c8a28b18ae1c62` (CI run `34401634396`, 48/48 checks).

---

## 11. Post-merge closeout

- E5 clone-from-template is **accepted and merged to `main`** at `81ff86b0f54b1053491d20dc06f51c4dfef537fd` (PR #115 head `5c26d34e3e623ac1d1be414455c8a28b18ae1c62`, CI `34401634396`).
- Migration **000071** is on `main`; migration **000072** was not created.
- `POST /rfx-events/from-template` is on `main` with immutable `source_template_version_id` and trigger `trg_rfx_events_provenance_immutable`.
- Questionnaire graph is materialized independently with new UUIDs; rule targets are remapped to event-local questions.
- Scoring models and downstream business artifacts (responses, participants, offer lines, evaluation/qualification results) are **not** copied on clone.
- E5-INT-01..32 and E5-REM-001..012 passed on exact HEAD.
- Staging and pilot were **not** changed as part of E5.
- **E6** (Studio frontend) is accepted and merged to `main` at `176461729dc2200d458eefad70ccdc126a2041b3` (PR #117).
- **E7** (browser acceptance) has **not** started; separate controller authorization is required before E7.
- Complete v3.0E remains **IMPLEMENTATION_IN_PROGRESS** until E7 is accepted.

---

## 12. Out of scope (E5 / E6+)

- Browser acceptance gate (E7)
- v3.0F qualification pool
- Manual buyer RFQ creation UX beyond existing APIs
- Excel import/export
- SAP/1C/ERP/TMS integration
- Late submission & deadline exceptions
- Carrier Excel import/export
- Staging/pilot rollout

**E6:** IMPLEMENTED_ACCEPTED (see `RFX_V3_0E6_STUDIO_FRONTEND.md`)
**E7:** NOT_STARTED
