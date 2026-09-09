# RFx v3.0E4 — Template Library Backend

**Status:** IMPLEMENTED_ACCEPTED

**Merged to main:** PR #113 via merge commit `5243bb5b8752e6d94bcdf7403697b64ff88def96`
**Migration:** `000070_rfx_template_library_v3_0e4`  
**Scope:** E4 only (no clone-to-event / E5)

---

## 1. Controller acceptance record

| Field | Value |
|---|---|
| `STATUS` | IMPLEMENTED_ACCEPTED |
| `CONTROLLER_ACCEPTANCE` | YES |
| `PR113_MERGED` | YES |
| `PR113_HEAD` | `c701ab32af38c0f0ef4db5bd51210634198d583a` |
| `PR113_MERGE_SHA` | `5243bb5b8752e6d94bcdf7403697b64ff88def96` |
| `CI_RUN_ID` | 34383868950 |
| `CI_EXACT_HEAD` | YES |
| `CI_CONCLUSION` | success |
| `E4_001_E4_006_CLOSED` | YES |
| `MIGRATION_000070_ON_MAIN` | YES |
| `MIGRATION_000071_CREATED` | NO |

---

## 2. Summary

Tenant-scoped RFx template library with aggregate lifecycle (`ACTIVE` / `ARCHIVED`), version lifecycle (`DRAFT` / `PUBLISHED` / `SUPERSEDED`), separate normalized questionnaire graph, buyer authorization, idempotent publish/fork, and audit events.

---

## 3. API (rfx-service `/v1`, gateway `/api/v1`)

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| GET | `/rfx-templates` | Buyer read | Filters: `status`, `owner_company_id`, `rfx_type`, `search`; pagination `limit`/`offset` |
| POST | `/rfx-templates` | Buyer manage | **201** — creates ACTIVE template + DRAFT v1 |
| GET | `/rfx-templates/{id}` | Buyer read | Template + draft/published pointers + version summaries |
| PATCH | `/rfx-templates/{id}` | Buyer manage | Metadata; `expected_version` required |
| DELETE | `/rfx-templates/{id}` | Buyer manage | **204** — draft-only soft delete |
| POST | `/rfx-templates/{id}/archive` | Buyer manage | ACTIVE → ARCHIVED |
| POST | `/rfx-templates/{id}/versions/publish` | Buyer manage | **Idempotency-Key** required |
| POST | `/rfx-templates/{id}/versions/fork-draft` | Buyer manage | **201**, **Idempotency-Key** required |
| GET | `/rfx-templates/{id}/questionnaire` | Buyer read | DRAFT graph |
| POST/PATCH/DELETE | `/rfx-templates/{id}/sections…` | Buyer manage | Mirrors event questionnaire |
| POST/PATCH/DELETE | `/rfx-templates/{id}/questions…` | Buyer manage | DRAFT only |
| POST/PATCH/DELETE | `/rfx-templates/{id}/rules…` | Buyer manage | DRAFT only |

Routes are gated by `RFX_VERSIONING_V3_ENABLED` in rfx-service (same as E1–E3 versioning).

---

## 4. Authorization

- Carrier (without buyer role): **403** on all template endpoints
- Cross-tenant resource: **404**
- Company-owned template: buyer must belong to `owner_company_id`; otherwise **403**
- Tenant-wide template (`owner_company_id` NULL): any buyer in tenant

---

## 5. Idempotency

Operations: `PUBLISH_TEMPLATE_VERSION`, `FORK_TEMPLATE_DRAFT`  
Scope: `tenant_id` + `actor_id` + `operation` + `template_id`

---

## 6. Audit events

- `rfx.template.created.v1`
- `rfx.template.updated.v1`
- `rfx.template.archived.v1`
- `rfx.template.deleted.v1`
- `rfx.template.version.published.v1`
- `rfx.template.version.forked.v1`
- Graph mutation events (`rfx.template.section.*`, `rfx.template.question.*`, etc.)

---

## 7. Controller remediation (E4-001..E4-006)

| Item | Scope | Outcome |
|---|---|---|
| **E4-001..E4-004** | List isolation, transactional audit, composite FK integrity, fork provenance, duplicate question code safety | Closed at `dee8903` — E4-REM-001..028 |
| **E4-005** | Graph mutation publish/archive TOCTOU — lock template + DRAFT before graph mutation | Closed at `d827d98` — E4-REM-030..039 |
| **E4-006** | Atomic publish readiness — lock template + DRAFT before graph load and readiness evaluation | Closed at `c701ab3` — E4-REM-040..045 |

Graph mutations, audit writes, and publish readiness evaluation run under consistent row locks; publish cannot observe stale graph state between pre-check and mutation.

---

## 8. Integration tests

Package: `services/rfx-service/internal/integration/templatelibrary/`  
CI job: `rfx-version-lifecycle-v3-integration` with `REQUIRE_TEST_DATABASE=1`

| Suite | Coverage |
|---|---|
| E4-INT-01..42 | Template library lifecycle, authorization, idempotency, graph CRUD |
| E4-REM-001..045 | Controller remediation, concurrency, publish readiness races |
| E1–E3 regressions | Version lifecycle continuity on shared CI job |

All suites passed on PR #113 head `c701ab3` (CI run `34383868950`).

---

## 9. Post-merge closeout

- E4 template library backend is **accepted and merged to `main`** at `5243bb5b8752e6d94bcdf7403697b64ff88def96` (PR #113 head `c701ab32af38c0f0ef4db5bd51210634198d583a`, CI `34383868950`).
- Migration **000070** is on `main`; migration **000071** was not created.
- E4-INT-01..42 and E4-REM-001..045 passed on exact HEAD.
- Graph mutation atomicity, audit transactional integrity, and publish readiness locking are confirmed by integration coverage.
- Staging and pilot were **not** changed as part of E4.
- **E5** (clone-to-event / provenance) has **not** started; separate controller authorization is required before implementation.
- Complete v3.0E remains **IMPLEMENTATION_IN_PROGRESS** until E5–E7 are accepted.

---

## 10. Out of scope (E4 / E5+)

- `POST /rfx-events/from-template` (E5)
- `source_template_version_id` (E5)
- Event materialization from template (E5)
- Frontend template library UI (E6)
- Browser acceptance gate (E7)
- Late submission & deadline exceptions
- Excel import/export
- SAP/1C integration
- Staging/pilot rollout

**E5:** PENDING_CONTROLLER_AUTHORIZATION
**E6–E7:** NOT_STARTED
