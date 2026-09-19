# RFx v3.0E7 Phase 2 — Generic ERP JSON API Architecture

**Status:** `FROZEN_ACCEPTED`
**Base:** `origin/main` @ `72a555637b3137ff5ea11e5b79c928f2b405bb3f`
**Mode:** `DOCS_ONLY` — no product implementation in this stream

| Marker | Value |
|---|---|
| `ERP_API_DISCOVERY_STATUS` | `COMPLETE` |
| `ERP_API_ARCHITECTURE_STATUS` | `FROZEN_ACCEPTED` |
| `ERP_API_IMPLEMENTATION_STATUS` | `NOT_STARTED` |
| `ERP_API_IMPLEMENTATION_AUTHORIZED` | `NO` |
| `ERP_API_IMPLEMENTATION_STARTED` | `NO` |
| `BUYER_RFQ_ERP_INTEGRATION` | `ARCHITECTURE_FROZEN_ACCEPTED` |
| `CARRIER_ERP_INTEGRATION` | `NOT_REQUIRED_CURRENT_SCOPE` |
| `CREATE_FROM_XLSX_STATUS` | `NOT_STARTED` |
| `MIGRATION_000074_CREATED` | `YES` |
| `MIGRATION_REQUIRED_PROPOSED` | `YES` |
| `CONTROLLER_PREVIOUS_VERDICT` | `CHANGES_REQUIRED` |
| `CONTROLLER_VERDICT` | `ACCEPT_ERP_API_ARCHITECTURE` |
| `MERMAID_VALIDATION` | `MANUAL_ONLY` |

### Controller acceptance evidence

| Field | Value |
|---|---|
| Reviewed head | `c538bcf073f901f2e80a26b77fb14a6f4a9491c6` |
| Controller verdict | `ACCEPT_ERP_API_ARCHITECTURE` |
| HIGH findings open | 0 |
| MEDIUM findings open | 0 |
| Residual LOW findings | Fixed in acceptance-alignment commit |
| Implementation authorization | NO |

**Normative companions:**

- [ADR-RFX-012: Canonical ERP JSON Contract](../adr/ADR-RFX-012-ERP-CANONICAL-JSON-CONTRACT.md)
- [ADR-RFX-013: ERP Machine Authentication](../adr/ADR-RFX-013-ERP-MACHINE-AUTHENTICATION.md)
- [ADR-RFX-014: ERP Preview/Commit and Idempotency](../adr/ADR-RFX-014-ERP-PREVIEW-COMMIT-IDEMPOTENCY.md)
- [ADR-RFX-015: External IDs and Reference Mapping](../adr/ADR-RFX-015-ERP-EXTERNAL-IDS-REFERENCE-MAPPING.md)
- [Acceptance test matrix](./RFX_V3_0E7_ERP_API_ACCEPTANCE_MATRIX.md)
- [Implementation plan](./RFX_V3_0E7_ERP_API_IMPLEMENTATION_PLAN.md) (`FROZEN_ACCEPTED`)
- [Implementation waves and test ownership](./RFX_V3_0E7_ERP_API_IMPLEMENTATION_WAVES.md)
- [Sequence diagrams](./RFX_V3_0E7_ERP_API_DIAGRAMS.md)

---

## 1. Purpose and scope

Design a **vendor-neutral JSON API** for Buyer ERP / SAP / 1C / TMS systems to create and update **RFx DRAFT** events without Excel, without auto-publish, and without carrier-side mutation.

### 1.1 In scope (v1)

| Operation | Description |
|---|---|
| Create DRAFT Preview | Validate ERP JSON; persist analysis when `ready_to_commit=true` |
| Create DRAFT Commit | Apply stored analysis; allocate internal event; link stable external identity |
| Update DRAFT Preview | Validate against existing DRAFT baseline |
| Update DRAFT Commit | Apply stored analysis to existing DRAFT |
| Get RFx by internal ID | ERP allowlisted DTO only |
| Get RFx by stable external identity | Resolve via `rfx_external_object_links` |
| Get analysis status | Read persisted analysis row (sync v1 — not async job polling) |
| Get capabilities / schema | Authenticated, principal-scoped limits |

### 1.2 Explicitly out of scope (v1)

| Exclusion | Marker |
|---|---|
| Publish / auto-publish | `AUTO_PUBLISH_ALLOWED=NO` |
| Modify PUBLISHED RFx | Forbidden |
| Carrier response mutation | Forbidden |
| Competitor / bid / scoring / award visibility | Forbidden |
| Transport order creation | Forbidden |
| SAP / 1C / Oracle adapters | `SAP_ADAPTER_AUTHORIZED=NO`, `ONE_C_ADAPTER_AUTHORIZED=NO` |
| Create from XLSX (`NEW_EVENT` via Excel) | `CREATE_FROM_XLSX_STATUS=NOT_STARTED` |
| Carrier ERP integration | `CARRIER_ERP_INTEGRATION=NOT_REQUIRED_CURRENT_SCOPE` |
| Async webhooks (v1) | Deferred — see §13 |
| Migration 000074 execution | `MIGRATION_000074_CREATED=NO` |
| Lanes/routes graph via ERP commit | `DEFERRED_DOMAIN_GAP` — see §6.3 |

---

## 2. Repository capability inventory

| Capability | Existing | Reusable | Gap | Evidence path |
|---|---|---|---|---|
| Buyer XLSX Preview/Commit | Yes | Pattern reuse | ERP JSON ingress new | `excel_exchange_preview.go`, `excel_exchange_commit.go` |
| `rfx_import_analyses` | Yes | Extend via 000074 | No `integration_principal_id`; ERP schema blocked by CHECK | `000073_rfx_excel_erp_exchange_v3_0e7_phase2.up.sql` |
| Canonical SHA-256 | Yes | ERP canonical builder new | XLSX hash path only today | `buyer_import_hash.go` |
| Idempotency store | Yes | New operation constants | — | `idempotency_repository.go`, `000068` |
| External object links | Partial | Stable-key redesign in 000074 | Current unique index includes revision | `external_object_link_repository.go` |
| Tenant/company isolation | Yes | Same fail-closed | M2M binding new | `excel_exchange_service.go` |
| OAuth / client credentials | No | — | New identity dependency | ADR-013 |
| `rfx_lanes` domain | Yes (DB) | Not in XLSX/ERP v1 commit | DEFERRED_DOMAIN_GAP | `000004_create_rfx_tables.up.sql` |

---

## 3. ERP API v1 operations (complete matrix)

Feature flags: `RFX_ERP_INTEGRATION_ENABLED` (default **false** → 404). Gateway prefix `/api/v1`; service prefix `/v1`.

### 3.1 Operation constants (frozen — M-01)

| Constant | HTTP | Purpose |
|---|---|---|
| `ERP_BUYER_CREATE_PREVIEW` | POST preview (CREATE) | Idempotency scope label; audit |
| `ERP_BUYER_CREATE_COMMIT` | POST commit (CREATE) | Idempotency scope label; audit |
| `ERP_BUYER_UPDATE_PREVIEW` | POST preview (UPDATE) | Idempotency scope label; audit |
| `ERP_BUYER_UPDATE_COMMIT` | POST commit (UPDATE) | Idempotency scope label; audit |

Legacy name `ERP_BUYER_IMPORT_COMMIT` is **retired** — do not use.

### 3.2 Full operation specification (M-04 / M-05)

#### Op 1 — Create DRAFT Preview

| Field | Value |
|---|---|
| Gateway | `POST /api/v1/integrations/erp/rfx/drafts/preview` |
| Service | `POST /v1/integrations/erp/rfx/drafts/preview` |
| operationId | `postErpRfxDraftCreatePreview` |
| Auth | OAuth client_credentials or API-key principal (typed) |
| Scopes | `rfx:draft:preview`, `rfx:draft:create` |
| Request | `BINTRANS_RFX_ERP_JSON_V1` with `requested_operation=CREATE_DRAFT` |
| Response | `ErpImportPreviewResponse` (`ready_to_commit`, `analysis_id?`, `errors[]`, `warnings[]`) |
| Idempotency | Optional |
| Concurrency | N/A (no target event yet) |
| State | No RFx row created |
| Audit | `rfx.erp.preview.created.v1` when analysis persisted |
| Errors | 400, 401, 403, 413, 422, 429 |
| Flag | `RFX_ERP_INTEGRATION_ENABLED` |

#### Op 2 — Create DRAFT Commit

| Field | Value |
|---|---|
| Gateway | `POST /api/v1/integrations/erp/rfx/drafts/commit` |
| Service | `POST /v1/integrations/erp/rfx/drafts/commit` |
| operationId | `postErpRfxDraftCreateCommit` |
| Scopes | `rfx:draft:commit`, `rfx:draft:create` |
| Request | `{ "analysis_id": "uuid" }` |
| Response | `201 ErpCreateCommitResponse` (`rfx_event_id`, `external_link_id`, `creation_channel=ERP`, `external_revision`) |
| Idempotency | **Required** `Idempotency-Key`; op `ERP_BUYER_CREATE_COMMIT` |
| Concurrency | Atomic stable external identity claim |
| State | DRAFT only; no publish |
| Audit | `rfx.erp.draft.created.v1` |
| Errors | 401, 403, 404, 409 (`external_id_conflict`, `idempotency_conflict`, `stale_mapping_context`), 422, 429 |

`stale_mapping_context` applies to all ERP Commit operations (CREATE and UPDATE) when the mapping set was RETIRED after Preview — Commit applies pinned resolved values only, no re-mapping (ADR-015).

#### Op 3 — Update DRAFT Preview

| Field | Value |
|---|---|
| Gateway | `POST /api/v1/rfx-events/{id}/erp-import/preview` |
| Service | `POST /v1/rfx-events/{id}/erp-import/preview` |
| operationId | `postErpRfxDraftUpdatePreview` |
| Scopes | `rfx:draft:preview`, `rfx:draft:read` |
| Request | `BINTRANS_RFX_ERP_JSON_V1` with `requested_operation=UPDATE_DRAFT` |
| Response | `ErpImportPreviewResponse` |
| Idempotency | Optional |
| Concurrency | Captures `event_row_version`, `draft_row_version`, `baseline_lots_fingerprint`, and when a link exists `baseline_external_link_id` + `baseline_external_revision` |
| State | Target must be DRAFT |
| External (existing link) | Requires `external` with same normalized `system`/`object_id` and a new unused `revision`. Missing/repeated revision → **422 `VALIDATION_ERROR`** `details.field=external.revision`. Identity change rejected before analysis persist. JSON Schema cannot express this state-dependent rule. |
| External (no link) | `external` remains optional. First bind is allowed. |
| Audit | `rfx.erp.preview.created.v1` when persisted |
| Errors | 400, 401, 403, 404, 409, 413, 422, 429 |
| Flag | `RFX_ERP_INTEGRATION_ENABLED` |

#### Op 4 — Update DRAFT Commit

| Field | Value |
|---|---|
| Gateway | `POST /api/v1/rfx-events/{id}/erp-import/commit` |
| Service | `POST /v1/rfx-events/{id}/erp-import/commit` |
| operationId | `postErpRfxDraftUpdateCommit` |
| Scopes | `rfx:draft:commit`, `rfx:draft:read` |
| Request | `{ "analysis_id": "uuid" }` |
| Response | `200 ErpUpdateCommitResponse` (`rfx_event_id`, `applied_at`) |
| Idempotency | **Required**; op `ERP_BUYER_UPDATE_COMMIT` |
| Concurrency | Revalidates baseline tokens from stored analysis, including external-link identity and current revision |
| External | Existing link: backfill previous revision if missing from history, append new revision, update link metadata in place (`rfx_event_id` never rebound). No link + `external`: insert link + initial history row. No link + no `external`: no link writes. Drift → **409 `stale_target`**. Same-key replay → stored 200, no extra history. |
| Audit | `rfx.erp.draft.updated.v1` |
| Errors | 401, 403, 404, 409, 422, 429 |

#### Op 5 — Get RFx by internal ID

| Field | Value |
|---|---|
| Gateway | `GET /api/v1/integrations/erp/rfx-events/{id}` |
| Service | `GET /v1/integrations/erp/rfx-events/{id}` |
| operationId | `getErpRfxEventById` |
| Scopes | `rfx:draft:read` |
| Response | **`ErpRfxDraftSummary`** (allowlisted — §3.3) — not human UI DTO |
| Idempotency | N/A |
| State | DRAFT only → 409 if PUBLISHED |
| Audit | None (read) |
| Errors | 401, 403, 404, 409 |

#### Op 6 — Get RFx by stable external identity

| Field | Value |
|---|---|
| Gateway | `GET /api/v1/integrations/erp/rfx/by-external-id` |
| Service | `GET /v1/integrations/erp/rfx/by-external-id` |
| operationId | `getErpRfxByExternalId` |
| Scopes | `rfx:draft:read` |
| Query | `external_system`, `external_object_id` (required); `external_revision` (optional — history metadata only) |
| Response | `ErpRfxDraftSummary` + `external_link` metadata |
| Lookup | Stable key without revision → exactly one RFx |
| Errors | 401, 403, 404 |

#### Op 7 — Get analysis status

| Field | Value |
|---|---|
| Gateway | `GET /api/v1/integrations/erp/analyses/{analysis_id}` |
| Service | `GET /v1/integrations/erp/analyses/{analysis_id}` |
| operationId | `getErpImportAnalysisStatus` |
| Scopes | `rfx:status:read` |
| Response | `ErpAnalysisStatus` (`status`, `expires_at`, `consumed_at?`, `validation_summary`, `ready_to_commit` snapshot) |
| Semantics | **Sync v1** — reads persisted analysis row; does not imply background worker |
| Errors | 401, 403, 404 |

#### Op 8 — Get capabilities

| Field | Value |
|---|---|
| Gateway | `GET /api/v1/integrations/erp/capabilities` |
| Service | `GET /v1/integrations/erp/capabilities` |
| operationId | `getErpIntegrationCapabilities` |
| Scopes | **`rfx:status:read` only** (M-03 — no unauthenticated read) |
| Response | Principal-scoped: `schema_version`, `limits`, `supported_mapping_types`, `deferred_fields[]` |
| Errors | 401, 403, 429 |

### 3.3 ERP GET DTO allowlist (`ErpRfxDraftSummary`) — M-05

| Field | Included |
|---|---|
| `rfx_event_id` | Yes |
| `status` | Yes (must be `DRAFT` for v1 GET) |
| `rfx_type`, `title`, `description` | Yes |
| `currency_code`, `timezone`, `response_deadline` | Yes |
| `creation_channel` | Yes |
| `external_link` (stable identity + current revision) | Yes |
| `publish_readiness_summary` | Yes (counts only) |
| `lot_count`, `questionnaire_counts` | Yes |
| `event_row_version`, `draft_row_version` | Yes (for UPDATE baseline) |
| Participants, carriers, bids, scores, awards | **Forbidden** |
| User identities, audit internals, secrets | **Forbidden** |

---

## 4. M2M actor / principal model (H-02)

Human and ERP paths **must not** silently alias integration principals to human `actor_id`.

### 4.1 Analysis row actor semantics (proposed migration 000074)

| Column | Human (XLSX/UI) | ERP M2M |
|---|---|---|
| `actor_id` | Required UUID (human user) | **NULL** |
| `integration_principal_id` | **NULL** | Required UUID |
| `actor_company_id` | Required | Required (from credential binding) |

**CHECK constraint (proposed):**

```sql
(actor_id IS NOT NULL AND integration_principal_id IS NULL)
OR (actor_id IS NULL AND integration_principal_id IS NOT NULL)
```

- Commit ownership: ERP compares `analysis.integration_principal_id` to authenticated principal.
- XLSX continues `analysis.actor_id == actor.UserID` (`excel_exchange_commit.go`).
- Gateway injects `X-Integration-Principal-ID` for M2M; strips client-supplied values.
- Audit records `actor_kind=INTEGRATION` + `integration_principal_id` for ERP; never logs secrets.

See §16 for full migration 000074 proposal.

---

## 5. Stable external identity (H-03)

Four concepts are **separate**:

| Concept | Purpose |
|---|---|
| Stable external object identity | One RFx per ERP business object |
| External payload revision | Metadata history (`external_revision`) |
| BINTRANS save version | `event_row_version` / `draft_row_version` |
| Idempotent request identity | `Idempotency-Key` HTTP retry |

### 5.1 Stable uniqueness key (frozen)

```
tenant_id
+ integration_principal_id
+ external_system          -- normalized uppercase [A-Z0-9_], max 64
+ external_object_type     -- "RFX_EVENT"
+ normalized_external_object_id  -- NFC trim, max 256
```

**`external_revision` is NOT part of stable uniqueness.** Multiple revisions update the same link row's revision metadata and `payload_hash`; they do not create a second RFx.

**UPDATE of an existing link (controller policy B):** every accepted UPDATE must supply a new `external.revision` and persist a history row. Unique `(link_id, external_revision)` from migration `000074` forbids replaying a revision string; omitted/repeated revision is **422 `VALIDATION_ERROR`** at Preview, not a unique-index 500. If E4 CREATE stored `"1"` only on the link, the first accepted UPDATE backfills that row before appending the new revision. Changing `system`/`object_id` is forbidden (stable identity immutable). `000075` is not authorized.

### 5.2 Normalization

| Rule | Value |
|---|---|
| Unicode | NFC normalization |
| Case | `external_system` uppercase; `external_object_id` case-sensitive preserved after trim |
| Empty | Rejected 422 |
| Rebind | Forbidden — stable identity immutable after CREATE commit |
| Disabled principal | 403 |
| Duplicate CREATE | 409 `external_id_conflict` on stable key |
| GET without revision | Returns the single RFx for stable key |
| GET with `external_revision` query | Returns same RFx + requested revision metadata if known; 404 if revision never recorded |

---

## 6. Canonical JSON and freight domain (H-05)

Schema: **`BINTRANS_RFX_ERP_JSON_V1`**. Full field table: ADR-012.

### 6.0 Parser ingress limits (frozen — E3)

| Limit | Value | HTTP | Machine code |
|---|---|---|---|
| Max body size | 2 MiB | 413 | — |
| Max JSON nesting depth | 32 (`ERP_JSON_MAX_DEPTH`) | 400/422 | `json_depth_exceeded` |
| Duplicate JSON keys | Reject fail-closed | 422 | `duplicate_field` |
| Unknown top-level field | Reject | 422 | `unknown_field` |
| Deferred top-level (`lanes[]`, etc.) | Reject | 422 | `unsupported_field_v1` |

Depth and duplicate-key checks run during streaming ingest before large allocations.

### 6.1 Freight field disposition

| Domain area | v1 status | Rationale |
|---|---|---|
| Event metadata (title, deadline, currency, timezone) | **SUPPORTED_V1** | Maps to `rfx_events` |
| Lots (number, name, value, category) | **SUPPORTED_V1** | Matches XLSX reconcile (`reconcileBuyerLots`) |
| Questionnaire graph | **SUPPORTED_V1** | Matches XLSX reconcile |
| Template reference | **SUPPORTED_V1** | Optional provenance |
| Lanes / routes (`rfx_lanes`) | **DEFERRED_DOMAIN_GAP** | DB exists; XLSX/ERP commit path does not reconcile lanes |
| Origin/destination locations | **DEFERRED_DOMAIN_GAP** | Lane-scoped; no lane ERP v1 |
| Cargo description (standalone) | **DEFERRED_DOMAIN_GAP** | Express via questionnaire questions |
| Weight / volume / packaging | **DEFERRED_DOMAIN_GAP** | Use questionnaire or lots `estimated_value` |
| Vehicle/body requirements | **DEFERRED_DOMAIN_GAP** | Questionnaire |
| Incoterms | **DEFERRED_DOMAIN_GAP** | Future mapping type; optional `extensions.freight.incoterm_code` warning-only if present |
| Temperature / hazardous | **DEFERRED_DOMAIN_GAP** | Questionnaire |
| Invited carriers | **OUT_OF_SCOPE** v1 | Policy gate deferred |

**ERP v1 parity:** aligned with Buyer XLSX UPDATE/CREATE commit surface (lots + questionnaire), not full freight lane graph.

Unknown deferred fields in payload → 422 `unsupported_field_v1`.

---

## 7. Mapping version pin (H-04)

Preview canonical payload **must include**:

```json
"mapping_context": {
  "mapping_set_id": "uuid",
  "mapping_set_version": 3,
  "mapping_types_applied": ["CURRENCY", "UNIT"]
}
```

Plus resolved canonical values after mapping. Included in SHA-256 hash.

**Commit policy (frozen):** Commit applies **pinned resolved values only** — no re-mapping. If mapping set was **RETIRED** after Preview, Commit returns **409 `stale_mapping_context`** (fail closed). Test: **E7P2-INT-193**.

---

## 8. Preview → Commit pattern

Reuse XLSX machinery with ERP extensions (ADR-014).

| Field | ERP value |
|---|---|
| `source_type` | `ERP_BUYER_JSON` (proposed 000074) |
| `schema_version` | `BINTRANS_RFX_ERP_JSON_V1` |
| `target_type` | `NEW_EVENT` or `DRAFT_EVENT` |
| Actor | `integration_principal_id` |

Commit body: `{ "analysis_id" }` only.

---

## 9. Concurrent CREATE policy (M-06)

| Rule | Value |
|---|---|
| Preview | Does not create RFx or reserve stable identity |
| Parallel previews | Allowed |
| Commit | Atomic transaction: create event + insert stable link + consume analysis + idempotency |
| Unique stable key | One winner |
| Loser | 409 `external_id_conflict` — no orphan RFx, no second link |
| Idempotent replay (winner) | Stored 201 response |
| Rollback | Full transaction rollback on any failure |

Test: **E7P2-INT-194**.

---

## 10. Authentication summary (M-02 / M-03)

Primary: **OAuth 2.0 client_credentials** (new — not existing infra).

Fallback: **API key** only for principals explicitly provisioned as `credential_type=API_KEY`.

| Policy | Value |
|---|---|
| OAuth-only principal | API key rejected → 401 `auth_scheme_denied` |
| API-key-only principal | OAuth token rejected → 401 `auth_scheme_denied` |
| No silent downgrade | OAuth failure does not fall back to API key |
| Gateway | Knows credential type at validation |
| Audit | Records `auth_scheme` |

Test: **E7P2-INT-190**.

Capabilities: **`rfx:status:read` required** — no anonymous access.

---

## 11. Idempotency and concurrency

See ADR-014. Commit requires `Idempotency-Key`. Body version tokens for UPDATE. No HTTP ETag v1.

---

## 12. Error contract

Preview validation uses `ErpImportPreviewResponse` with `errors[]` / `warnings[]` items (`machine_code`, JSON pointer `path`, optional `external_source`). HTTP mapping: structural ingress failures may return 400; domain validation returns **422** with zero analysis rows (OPTION A). Machine codes include `unknown_field`, `duplicate_field`, `unsupported_field_v1`, `json_depth_exceeded`, mapping errors, `external_id_conflict`, `stale_mapping_context`, `auth_scheme_denied`, `stale_target`.

---

## 13. Async and webhooks

**Sync v1.** Status endpoint reads analysis row only. Webhooks deferred to v3.0J.

---

## 14. Audit and observability

| Event | When |
|---|---|
| `rfx.erp.client.authenticated.v1` | M2M auth success (includes `auth_scheme`) |
| `rfx.erp.preview.created.v1` | Analysis persisted |
| `rfx.erp.draft.created.v1` | CREATE commit |
| `rfx.erp.draft.updated.v1` | UPDATE commit |
| `rfx.erp.commit.replayed.v1` | Idempotent replay |
| `rfx.erp.access.denied.v1` | Scope/company/scheme denial |

Never log: secrets, tokens, raw API keys, full payload, competitor data.

---

## 15. Threat model (H-01 — traceability corrected)

| Threat | Control | Exact test ID | Matrix title |
|---|---|---|---|
| Tenant spoofing | Gateway strip + credential tenant | E7P2-INT-128 | Spoofed X-Tenant-ID header |
| Company spoofing | Server-resolved company | E7P2-INT-129 | Spoofed X-Company-ID header |
| Credential theft | TLS, hash storage, rotation | E7P2-INT-120..124 | OAuth/API key auth tests |
| Auth scheme downgrade | Typed principals, no fallback | E7P2-INT-190 | OAuth-only cannot use API key |
| Replay | Idempotency-Key + body hash | E7P2-INT-140, 145, 157 | CREATE/UPDATE idempotent replay |
| Cross-tenant access | Tenant predicate | E7P2-INT-130 | Cross-tenant 404 |
| External ID collision | Stable unique + atomic commit | E7P2-INT-139, 194 | Duplicate/concurrent CREATE |
| Principal binding bypass | integration_principal_id on analysis | E7P2-INT-191 | Analysis binds to principal |
| Mass assignment | Allowlist schema | E7P2-INT-143 | CREATE mass assignment field |
| Payload bomb | Size/depth limits | E7P2-INT-147, 175 | Oversized / depth bomb |
| Stale overwrite | Baseline version tokens | E7P2-INT-151, 152 | Stale row versions |
| Hash tampering | Server hash verify | E7P2-INT-153 | Canonical hash tampered |
| Mapping drift | Pinned mapping_context | E7P2-INT-193 | Mapping version drift rejected |
| Auto-publish | No publish in commit | E7P2-INT-177 | Commit does not publish |
| Competitor leak | ERP GET DTO allowlist | E7P2-INT-188 | Competitor data absent in GET |
| Deferred freight fields | Allowlist rejects lanes[] | E7P2-INT-195 | Deferred lanes[] rejected |
| Stable identity ambiguity | GET without revision | E7P2-INT-192 | External lookup deterministic |

---

## 16. Migration 000074 (M-08 — implemented in E1)

| # | Change |
|---|---|
| 1 | `rfx.rfx_integration_principals` (tenant, client_id, credential_type, company_id, status, allowed_cidrs) |
| 2 | `rfx.rfx_integration_credentials` (hashed secrets, rotation, display-once) |
| 3 | `rfx.rfx_integration_scopes` |
| 4 | `rfx.rfx_import_analyses.integration_principal_id` nullable FK |
| 5 | Alter `actor_id` nullable; XOR CHECK actor vs principal |
| 6 | Extend `workbook_type` / `source_type` CHECK for `ERP_BUYER_JSON` |
| 7 | Extend `schema_version` CHECK for `BINTRANS_RFX_ERP_JSON_V1` |
| 8 | Replace `uq_rfx_external_object_identity` — **drop `external_version` from unique key** |
| 9 | Add `external_revision` column + revision history table (optional) on link |
| 10 | `rfx.rfx_reference_mapping_sets` + `entries` |
| 11 | Index: stable external lookup `(tenant_id, integration_principal_id, external_system, external_object_type, external_object_id)` |
| 12 | Repository: ERP branch in `CreatePreview` (ERP hash path, not XLSX-only) |
| 13 | Idempotency op constants: four ERP ops |
| 14 | Triggers: preserve payload immutability |
| 15 | Down migration: reversible; legacy XLSX rows keep `actor_id` populated |

`MIGRATION_000074_CREATED=YES` — implemented in Wave E1 (PR #139).

---

## 17. Block status (L-02)

| Block | Status |
|---|---|
| Repository capability inventory | **COMPLETE_REMEDIATED** |
| M2M actor/principal model | **COMPLETE_REMEDIATED** |
| Stable external identity | **COMPLETE_REMEDIATED** |
| Mapping version pin | **COMPLETE_REMEDIATED** |
| Freight schema coverage | **COMPLETE_REMEDIATED** |
| Operation matrix + ERP GET DTO | **COMPLETE_REMEDIATED** |
| Auth downgrade prevention | **COMPLETE_REMEDIATED** |
| Threat/test traceability | **COMPLETE_REMEDIATED** |
| Migration 000074 proposal | **COMPLETE_REMEDIATED** |
| Acceptance test matrix | **COMPLETE_REMEDIATED** |
| Diagrams | **COMPLETE_REMEDIATED** |
| ADRs 012–015 | **COMPLETE_REMEDIATED** |

---

## 18. Test IDs

| Marker | Value |
|---|---|
| `ERP_TEST_IDS` | `E7P2-INT-120..195` |
| `ERP_TEST_COUNT` | `76` |
| `NEXT_FREE_TEST_ID` | `E7P2-INT-196` |

---

## 19. Implementation gate

```
PUBLIC_ROUTE_CONTRACT_POLICY=INCREMENTAL_PER_WAVE
PUBLIC_ROUTE_OPENAPI_PER_WAVE=YES
ERP_API_IMPLEMENTATION_PLAN_STATUS=FROZEN_ACCEPTED
ERP_API_IMPLEMENTATION_AUTHORIZED=NO
ERP_API_IMPLEMENTATION_STARTED=NO
ERP_API_IMPLEMENTATION_STATUS=NOT_STARTED
ERP_API_E1_AUTHORIZED=NO
MIGRATION_000074_AUTHORIZED=NO
MIGRATION_000074_CREATED=NO
CONTROLLER_VERDICT=ACCEPT_ERP_API_IMPLEMENTATION_PLAN
NEXT_ACTION=E1_SCOPE_AUTHORIZATION_AND_EXECUTION_TASK
```

Each wave that adds a public route must publish OpenAPI + route manifest + parity in the **same PR** (§3.5 of implementation plan). E6 performs final exhaustive INT-185 consolidation; E6 is **not** the first publication wave for E2–E5 contracts.

See [RFX_V3_0E7_ERP_API_IMPLEMENTATION_PLAN.md](./RFX_V3_0E7_ERP_API_IMPLEMENTATION_PLAN.md).

---

## 20. References

- [RFX_V3_0E7_PHASE2_EXCEL_ERP.md](./RFX_V3_0E7_PHASE2_EXCEL_ERP.md)
- [RFX_V3_0E7_BUYER_XLSX_IMPORT_P4_COMMIT.md](./RFX_V3_0E7_BUYER_XLSX_IMPORT_P4_COMMIT.md)
- [RFX_V3_SECURITY.md](../RFX_V3_SECURITY.md)
- [RFX_V3_DATA_MODEL.md](../RFX_V3_DATA_MODEL.md)
- [RFX_V3_ROADMAP.md](../RFX_V3_ROADMAP.md)
