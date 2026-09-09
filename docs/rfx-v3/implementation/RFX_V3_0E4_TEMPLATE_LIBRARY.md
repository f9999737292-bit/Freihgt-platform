# RFx v3.0E4 — Template Library Backend

**Status:** `IMPLEMENTED_PENDING_CONTROLLER_ACCEPTANCE`  
**Migration:** `000070_rfx_template_library_v3_0e4`  
**Scope:** E4 only (no clone-to-event / E5)

## Summary

Tenant-scoped RFx template library with aggregate lifecycle (`ACTIVE` / `ARCHIVED`), version lifecycle (`DRAFT` / `PUBLISHED` / `SUPERSEDED`), separate normalized questionnaire graph, buyer authorization, idempotent publish/fork, and audit events.

## API (rfx-service `/v1`, gateway `/api/v1`)

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

## Authorization

- Carrier (without buyer role): **403** on all template endpoints
- Cross-tenant resource: **404**
- Company-owned template: buyer must belong to `owner_company_id`; otherwise **403**
- Tenant-wide template (`owner_company_id` NULL): any buyer in tenant

## Idempotency

Operations: `PUBLISH_TEMPLATE_VERSION`, `FORK_TEMPLATE_DRAFT`  
Scope: `tenant_id` + `actor_id` + `operation` + `template_id`

## Audit events

- `rfx.template.created.v1`
- `rfx.template.updated.v1`
- `rfx.template.archived.v1`
- `rfx.template.deleted.v1`
- `rfx.template.version.published.v1`
- `rfx.template.version.forked.v1`
- Graph mutation events (`rfx.template.section.*`, `rfx.template.question.*`, etc.)

## Integration tests

Package: `services/rfx-service/internal/integration/templatelibrary/`  
CI: extended `rfx-version-lifecycle-v3-integration` job with `REQUIRE_TEST_DATABASE=1`

## Out of scope (E5+)

- `POST /rfx-events/from-template`
- `source_template_version_id`
- Frontend template library UI
