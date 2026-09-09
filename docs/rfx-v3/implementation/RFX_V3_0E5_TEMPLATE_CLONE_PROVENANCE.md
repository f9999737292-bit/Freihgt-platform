# RFx v3.0E5 — Template Clone + Provenance

**Status:** IMPLEMENTED_PENDING_CONTROLLER_ACCEPTANCE

**Branch:** `feat/rfx-template-clone-provenance-v3.0e5`  
**Base:** `a53f5a5fa5556d61ce09e1b7fb2d306afef8bc60` (E4 docs closeout on `main`)  
**Migration:** `000071_rfx_template_clone_provenance_v3_0e5`  
**Scope:** E5 only (no E6/E7, no v3.0F, no staging/pilot)

---

## 1. Summary

Buyer-managed clone of a published (or explicitly selected superseded) RFx template version into a new RFx event with immutable template provenance, initial event DRAFT questionnaire version, deep graph materialization (sections/questions/options/rules with remapped rule targets), mandatory idempotency, and audit `rfx.event.created_from_template.v1`.

Scoring, responses, invitations, offers, and evaluation results are **not** copied or created.

---

## 2. API

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

## 3. Source version rules

| Source status | Result |
|---------------|--------|
| `PUBLISHED` | **201** clone |
| `SUPERSEDED` (explicit version id) | **201** clone + `source_version_warning=true` |
| `DRAFT` | **409** conflict |
| Template aggregate `ARCHIVED` | **409** conflict |
| Cross-tenant template version | **404** |

---

## 4. Provenance (DB)

Migration `000071`:

- Column `rfx.rfx_events.source_template_version_id` (nullable; NULL for manual create)
- Composite FK `(tenant_id, source_template_version_id)` → `rfx.rfx_template_versions(tenant_id, id)`
- Trigger `trg_rfx_events_provenance_immutable` — UPDATE of provenance forbidden
- Supporting unique index on `rfx_template_versions(tenant_id, id)`

Manual event creation leaves provenance NULL.

---

## 5. Transaction (atomic)

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

## 6. Idempotency

- Operation: `CLONE_EVENT_FROM_TEMPLATE`
- Scope: `tenant_id` + `actor_id` + operation + `template_version_id`
- Header: `Idempotency-Key` (required, max 128)
- Same key + same body → replay same event (**201** semantics via stored payload)
- Same key + different body → **409**
- Expired key reuse → new event

---

## 7. Not copied

| Artifact | Copied |
|----------|--------|
| Score models | NO |
| Responses | NO |
| Invitations / participants | NO |
| Offers / bids | NO |
| Evaluation results | NO |

---

## 8. Integration tests

Package: `services/rfx-service/internal/integration/templatelibrary/`  
Cases: **E5-INT-01..32** (clone, provenance, isolation, graph, idempotency, migration up/down, gateway alignment, feature flag fail-closed).

CI: existing `rfx-version-lifecycle-v3-integration` job runs `./internal/integration/templatelibrary/...` with `REQUIRE_TEST_DATABASE=1`.

---

## 9. Out of scope (E5)

- E6 Studio frontend
- E7 browser acceptance
- v3.0F qualification pool
- Staging / pilot deployment changes
- Migration `000072`

---

## 10. Controller gate

**E6/E7:** NOT_STARTED — separate authorization required.  
**Merge:** DO NOT MERGE until controller acceptance.
