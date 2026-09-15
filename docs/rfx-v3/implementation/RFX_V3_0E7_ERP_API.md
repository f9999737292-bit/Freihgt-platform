# RFx v3.0E7 Phase 2 — Generic ERP JSON API Architecture

**Status:** `ARCHITECTURE_FROZEN_PENDING_REVIEW`  
**Base:** `origin/main` @ `72a555637b3137ff5ea11e5b79c928f2b405bb3f`  
**Mode:** `DOCS_ONLY` — no product implementation in this stream

| Marker | Value |
|---|---|
| `ERP_API_DISCOVERY_STATUS` | `COMPLETE` |
| `ERP_API_ARCHITECTURE_STATUS` | `FROZEN_PENDING_REVIEW` |
| `ERP_API_IMPLEMENTATION_STATUS` | `NOT_STARTED` |
| `ERP_API_IMPLEMENTATION_AUTHORIZED` | `NO` |
| `ERP_API_IMPLEMENTATION_STARTED` | `NO` |
| `BUYER_RFQ_ERP_INTEGRATION` | `ARCHITECTURE_FROZEN_PENDING_REVIEW` |
| `CARRIER_ERP_INTEGRATION` | `NOT_REQUIRED_CURRENT_SCOPE` |
| `CREATE_FROM_XLSX_STATUS` | `NOT_STARTED` |
| `MIGRATION_000074_CREATED` | `NO` |
| `MIGRATION_REQUIRED_PROPOSED` | `YES` |

**Normative companions:**

- [ADR-RFX-012: Canonical ERP JSON Contract](../adr/ADR-RFX-012-ERP-CANONICAL-JSON-CONTRACT.md)
- [ADR-RFX-013: ERP Machine Authentication](../adr/ADR-RFX-013-ERP-MACHINE-AUTHENTICATION.md)
- [ADR-RFX-014: ERP Preview/Commit and Idempotency](../adr/ADR-RFX-014-ERP-PREVIEW-COMMIT-IDEMPOTENCY.md)
- [ADR-RFX-015: External IDs and Reference Mapping](../adr/ADR-RFX-015-ERP-EXTERNAL-IDS-REFERENCE-MAPPING.md)
- [Acceptance test matrix](./RFX_V3_0E7_ERP_API_ACCEPTANCE_MATRIX.md)
- [Sequence diagrams](./RFX_V3_0E7_ERP_API_DIAGRAMS.md)

---

## 1. Purpose and scope

Design a **vendor-neutral JSON API** for Buyer ERP / SAP / 1C / TMS systems to create and update **RFx DRAFT** events without Excel, without auto-publish, and without carrier-side mutation.

### 1.1 In scope (v1)

| Operation | Description |
|---|---|
| Create DRAFT Preview | Validate ERP JSON; persist analysis when `ready_to_commit=true` |
| Create DRAFT Commit | Apply stored analysis; allocate internal event; link external ID |
| Update DRAFT Preview | Validate against existing DRAFT baseline |
| Update DRAFT Commit | Apply stored analysis to existing DRAFT |
| Get RFx by internal ID | Read DRAFT metadata + readiness summary |
| Get RFx by external system + external ID | Resolve via `rfx_external_object_links` |
| Get analysis / operation status | Poll preview/commit outcome |
| Get capabilities / schema | Machine-readable contract version and limits |

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

---

## 2. Repository capability inventory

Evidence-backed assessment of reusable platform mechanisms.

| Capability | Existing | Reusable | Gap | Evidence path |
|---|---|---|---|---|
| Buyer DRAFT create (manual) | Yes | Yes | ERP uses dedicated ingress | `services/rfx-service/internal/service/rfx_service.go`; `POST /v1/rfx-events/` |
| Buyer DRAFT update (metadata) | Yes | Yes | — | `PATCH /v1/rfx-events/{id}` |
| Questionnaire graph CRUD | Yes | Yes | ERP applies via commit reconcile | `questionnaire_service.go`; routes in `router.go:102-116` |
| Template clone → DRAFT | Yes | Partial | ERP may reference template in JSON | `template_clone_service.go`; `POST /v1/rfx-events/from-template` |
| Publish readiness gate | Yes | Yes | ERP must not bypass | `ValidatePublish`; `POST /v1/rfx-events/{id}/validate-publish` |
| Buyer XLSX Preview/Commit | Yes | **Pattern reuse** | ERP uses JSON not XLSX | `excel_exchange_preview.go`, `excel_exchange_commit.go` |
| `rfx_import_analyses` store | Yes | Yes | Extend `schema_version` / `workbook_type` (migration 000074) | `000073_rfx_excel_erp_exchange_v3_0e7_phase2.up.sql` |
| Canonical payload + SHA-256 | Yes | Yes | ERP canonical differs from XLSX | `buyer_import_hash.go`; `domain/excel_exchange.go` |
| Analysis TTL 24h | Yes | Yes | Same policy for ERP | `importAnalysisTTL = 24 * time.Hour` in `excel_exchange_preview.go` |
| PREVIEWED → CONSUMED | Yes | Yes | — | `ImportAnalysisRepository.MarkConsumed`; DB trigger |
| Idempotency-Key + body hash | Yes | Yes | New operation scopes | `idempotency_repository.go`; migration `000068` |
| Optimistic concurrency | Yes | Yes | Body tokens, not HTTP ETag | `expected_version`, `save_version`; `questionnaire_handler.go` |
| External object links | Partial | Yes | No HTTP lookup yet | `external_object_link_repository.go`; migration `000073` |
| Tenant / company isolation | Yes | Yes | Machine auth must bind same | `excel_exchange_service.go:authorizeBuyerRead` |
| Gateway RBAC | Yes | Yes | Extend for integration scopes | `api-gateway/internal/rfxrbac/guard.go` |
| JWT user auth | Yes | Not for ERP M2M | Human login only today | `identity-service/auth_service.go` |
| OAuth / client credentials | No | — | **New** — ADR-013 | — |
| ERP HTTP routes | No | — | **New** | `ERP_API_STATUS=NOT_STARTED` |
| RFx outbox / webhooks | No | Pattern from shipment | Deferred v1 | `RFX_V3_EVENTS.md` |
| HTTP ETag / If-Match | No | — | Use body version tokens | No ETag in rfx-service |

---

## 3. ERP API v1 operations

All routes are **proposed** (not implemented). Gateway prefix: `/api/v1`. Service prefix: `/v1`.

Feature flag (proposed): `RFX_ERP_INTEGRATION_ENABLED` (default **false** → HTTP 404), distinct from `RFX_EXCEL_EXCHANGE_ENABLED`.

### 3.1 Operation matrix

| # | Method / path (gateway) | operationId | Auth scope | Request | Response | Idempotency | Primary errors |
|---|---|---|---|---|---|---|---|
| 1 | `POST /api/v1/integrations/erp/rfx/drafts/preview` | `postErpRfxDraftCreatePreview` | `rfx:draft:preview`, `rfx:draft:create` | `BINTRANS_RFX_ERP_JSON_V1` body | Preview envelope + optional `analysis_id` | Optional on preview | 400, 401, 403, 413, 422, 429 |
| 2 | `POST /api/v1/integrations/erp/rfx/drafts/commit` | `postErpRfxDraftCreateCommit` | `rfx:draft:commit`, `rfx:draft:create` | `{ "analysis_id": "uuid" }` | `{ "rfx_event_id", "external_link_id", "creation_channel": "ERP" }` | **Required** `Idempotency-Key` | 401, 403, 404, 409, 422, 429 |
| 3 | `POST /api/v1/rfx-events/{id}/erp-import/preview` | `postErpRfxDraftUpdatePreview` | `rfx:draft:preview`, `rfx:draft:read` | JSON body + optional baseline tokens | Preview envelope + optional `analysis_id` | Optional | 400, 401, 403, 404, 409, 413, 422 |
| 4 | `POST /api/v1/rfx-events/{id}/erp-import/commit` | `postErpRfxDraftUpdateCommit` | `rfx:draft:commit`, `rfx:draft:read` | `{ "analysis_id": "uuid" }` | `{ "rfx_event_id", "applied_at" }` | **Required** | 401, 403, 404, 409, 422 |
| 5 | `GET /api/v1/rfx-events/{id}` | `getRfxEventById` | `rfx:draft:read` | — | Existing event DTO (DRAFT-focused subset) | — | 401, 403, 404 |
| 6 | `GET /api/v1/integrations/erp/rfx/by-external-id` | `getErpRfxByExternalId` | `rfx:draft:read` | Query: `external_system`, `external_object_id`, optional `external_version` | Event summary + link metadata | — | 401, 403, 404 |
| 7 | `GET /api/v1/integrations/erp/analyses/{analysis_id}` | `getErpImportAnalysisStatus` | `rfx:status:read` | — | Analysis status, expiry, validation summary | — | 401, 403, 404 |
| 8 | `GET /api/v1/integrations/erp/capabilities` | `getErpIntegrationCapabilities` | `rfx:status:read` (or unscoped read) | — | Schema version, limits, supported mappings | — | 401, 429 |

### 3.2 ERP forbidden actions

| Action | Enforcement |
|---|---|
| Publish | No route; commit never calls `PublishEvent` |
| Modify PUBLISHED RFx | State gate → 409 `stale_target` |
| Carrier responses | No carrier-scoped ERP routes in v1 |
| Scoring / award | No ERP routes to evaluation endpoints |
| Transport order | No ERP award conversion |

---

## 4. Preview → Commit pattern

Reuse the **Buyer XLSX P3/P4** pipeline with JSON ingress instead of multipart XLSX.

### 4.1 Flow separation

| Mode | `target_type` | Preview route | Commit route |
|---|---|---|---|
| `CREATE_DRAFT` | `NEW_EVENT` | Op #1 | Op #2 |
| `UPDATE_EXISTING_DRAFT` | `DRAFT_EVENT` | Op #3 | Op #4 |

**Do not** mix ERP Create with Create-from-XLSX or template clone HTTP routes. ERP Create uses external ID allocation + `creation_channel=ERP`.

### 4.2 Reuse mapping

| XLSX mechanism | ERP equivalent |
|---|---|
| `ParseBuyerImportPreview` | `ParseErpJsonPreview` (new) |
| `CanonicalImportPayloadJSON` | `CanonicalErpPayloadJSON` (new, same hash pipeline) |
| `ImportAnalysisRepository.CreatePreview` | Same repository |
| `CommitBuyerImportAnalysis` | `CommitErpImportAnalysis` (new, shared reconcile) |
| `reconcileBuyerDraftGraph` / `reconcileBuyerLots` | Reuse unchanged |
| `BUYER_XLSX_IMPORT_COMMIT` idempotency op | `ERP_BUYER_IMPORT_COMMIT` (new constant) |

### 4.3 Analysis persistence (OPTION A)

Mirror buyer XLSX: persist analysis row **only when** `ready_to_commit=true` (zero blocking errors).

| Field | ERP value |
|---|---|
| `workbook_type` | `BUYER_TENDER` (proposed; or `ERP_BUYER_TENDER` after migration 000074) |
| `schema_version` | `BINTRANS_RFX_ERP_JSON_V1` |
| `target_type` | `NEW_EVENT` or `DRAFT_EVENT` |
| `canonical_payload_json` | Server-normalized proposal + baseline tokens |
| `canonical_hash` | SHA-256 of JSONB-stable bytes |
| `expires_at` | `created_at + 24h` |
| `status` | `PREVIEWED` → `CONSUMED` |

### 4.4 Commit contract

Commit request body contains **only** `analysis_id`. Server applies stored canonical payload — client cannot post alternate graph at commit time.

Commit revalidates:

- Hash integrity (`canonical_hash_mismatch`)
- Actor / integration principal binding (`actor_binding_denied`)
- Expiry (`analysis_expired`)
- Single-use (`analysis_already_consumed`)
- Stale baseline (`stale_target`, `proposal_revalidation_failed`)

---

## 5. Machine authentication (summary)

Full decision: [ADR-RFX-013](../adr/ADR-RFX-013-ERP-MACHINE-AUTHENTICATION.md).

| Layer | Decision |
|---|---|
| Primary | OAuth 2.0 Client Credentials (new token endpoint) |
| Secondary | Hashed API key (`Authorization: Bearer bt_…`) as controlled fallback |
| Optional hardening | mTLS at ingress (infrastructure gate, not app code v1 blocker) |
| Forbidden | Public `X-Internal-Service-Token` (`PUBLIC_X_INTERNAL_TOKEN_ALLOWED=NO`) |
| Tenant binding | From credential record — **never** from client body/headers |
| Company binding | From credential record + membership validation |
| Scopes | `rfx:draft:create`, `rfx:draft:read`, `rfx:draft:preview`, `rfx:draft:commit`, `rfx:status:read` |

Gateway continues to strip spoofed `X-Tenant-ID`, `X-User-ID`, `X-Company-ID` per `auth_context.go`.

---

## 6. Canonical JSON contract (summary)

Full schema: [ADR-RFX-012](../adr/ADR-RFX-012-ERP-CANONICAL-JSON-CONTRACT.md).

**Schema name (frozen pending review):** `BINTRANS_RFX_ERP_JSON_V1`

Naming audit: follows existing `BINTRANS_RFX_BUYER_XLSX_V1` / `BINTRANS_RFX_CARRIER_XLSX_V1` pattern. Requires migration 000074 to extend DB `chk_rfx_import_analysis_schema_version`.

Top-level sections:

- `schema_version`, `requested_operation` (`CREATE_DRAFT` | `UPDATE_DRAFT`)
- `external` (`system`, `object_id`, `object_version`)
- `event` (type, title, description, currency, timezone, deadline)
- `lots[]`, `questionnaire` (sections, questions, options, rules)
- `template_reference` (optional)
- `invited_carriers[]` (optional, policy-gated)
- `extensions` (namespaced, allowlisted keys only)

**No arbitrary mass assignment** — unknown top-level keys → 422 `unknown_field`.

---

## 7. External IDs and reference mappings (summary)

Full policy: [ADR-RFX-015](../adr/ADR-RFX-015-ERP-EXTERNAL-IDS-REFERENCE-MAPPING.md).

**Uniqueness key:** `tenant_id + integration_principal_id + external_system + external_object_type + external_object_id + external_version`

Existing index: `uq_rfx_external_object_identity` in migration 000073.

---

## 8. Idempotency and concurrency (summary)

Full policy: [ADR-RFX-014](../adr/ADR-RFX-014-ERP-PREVIEW-COMMIT-IDEMPOTENCY.md).

| Concern | Policy |
|---|---|
| Idempotency-Key | Required on all Commit operations; max 128 chars |
| Scope | `{tenant, integration_principal_id, operation, route_target}` |
| Same key + same body | Replay stored response |
| Same key + different body | 409 `idempotency_conflict` |
| External ID | Does **not** replace Idempotency-Key |
| Concurrency | `event_row_version`, `draft_row_version`, `baseline_lots_fingerprint` in canonical payload |
| HTTP ETag | Not used in v1 — body tokens only |

---

## 9. Error contract

### 9.1 Envelope

```json
{
  "error": {
    "code": "mapping_validation_failed",
    "message": "Human-readable summary",
    "retryable": false,
    "correlation_id": "req-uuid",
    "issues": [
      {
        "severity": "ERROR",
        "code": "unknown_currency_code",
        "json_pointer": "/event/currency",
        "external_source": "SAP:WAERS",
        "message": "Currency ZZZ is not mapped",
        "retryable": false
      }
    ],
    "issues_truncated": false
  }
}
```

| Field | Rule |
|---|---|
| `issues` cap | Max 50; set `issues_truncated=true` if more |
| Secrets | Never in envelope |
| Correlation | Propagate `X-Request-ID` |

### 9.2 HTTP matrix

| HTTP | Use |
|---|---|
| 400 | Malformed JSON / multipart |
| 401 | Missing or invalid credential |
| 403 | Scope / company / actor denied |
| 404 | Hidden or not found (fail-closed cross-tenant) |
| 409 | State / version / idempotency / external ID conflict |
| 413 | Payload too large |
| 422 | Domain / mapping validation |
| 429 | Rate limit |
| 500 | Internal |
| 503 | Retryable dependency failure |

Reuse existing machine codes from `domain/excel_exchange.go` where applicable.

---

## 10. Audit and observability

| Event | Emitted |
|---|---|
| `rfx.erp.client.authenticated.v1` | On successful M2M auth |
| `rfx.erp.access.denied.v1` | Scope / company denial |
| `rfx.erp.preview.created.v1` | Analysis persisted |
| `rfx.erp.draft.created.v1` | CREATE commit success |
| `rfx.erp.draft.updated.v1` | UPDATE commit success |
| `rfx.erp.commit.replayed.v1` | Idempotent replay |
| `rfx.erp.validation.failed.v1` | Preview blocking errors |
| `rfx.erp.credential.rotated.v1` | Admin rotation |
| `rfx.erp.credential.revoked.v1` | Revocation |

**Never log:** secrets, tokens, full ERP payload, competitor data.

**Metrics (proposed):** preview latency, commit latency, analysis expiry rate, idempotency replay rate, mapping failure rate, 429 count.

**SLO (proposed):** p99 commit < 2s excluding mapping; 99.9% availability for sync v1.

---

## 11. Async and webhooks

**Decision:** Synchronous v1 is sufficient.

| Rationale | Detail |
|---|---|
| Preview/Commit already bounded | 24h analysis TTL; commit is atomic |
| No long-running ERP batch | v1 caps payload size |
| Outbox not on RFx | `RFX_V3_EVENTS.md` — RFx outbox not migrated |
| Webhook infra absent | No signed callback delivery in repo |

**Future scope:** async operation status + signed webhooks when RFx outbox lands (v3.0J).

---

## 12. Data model (proposed entities)

| Entity | Owner | Tenant scope | Unique constraints | Lifecycle | Retention | Reuse/new |
|---|---|---|---|---|---|---|
| `rfx_integration_principals` | identity/admin | tenant | `(tenant_id, client_id)` | active → revoked | indefinite | **New** |
| `rfx_integration_credentials` | identity/admin | tenant | `(principal_id, credential_version)` | issued → rotated → revoked | hash only after display | **New** |
| `rfx_integration_scopes` | admin | principal | `(principal_id, scope)` | static enum | — | **New** |
| `rfx_external_object_links` | rfx-service | tenant | `uq_rfx_external_object_identity` | create → update hash | indefinite | **Reuse** (000073) |
| `rfx_import_analyses` | rfx-service | tenant | PK `id` | PREVIEWED → CONSUMED/EXPIRED | 24h+ audit | **Reuse** (extend schema) |
| `rfx_idempotency_records` | rfx-service | tenant | scope + key | 24h TTL | 24h | **Reuse** |
| `rfx_reference_mapping_sets` | admin/tenant | tenant | `(tenant_id, mapping_type, version)` | draft → active → retired | versioned | **New** |
| `rfx_reference_mapping_entries` | admin/tenant | tenant | `(set_id, external_code)` | effective-dated | with set | **New** |

`MIGRATION_REQUIRED_PROPOSED=YES` — migration **000074** (not authorized, not created).

---

## 13. Threat model (summary)

| Threat | Entry point | Control | Required test | Residual risk |
|---|---|---|---|---|
| Tenant spoofing | Headers/body | Gateway strip + credential binding | E7P2-INT-128 | Low |
| Company spoofing | Body company_id | Server-resolved company from credential | E7P2-INT-129 | Low |
| Credential theft | Network | TLS, hashed storage, rotation | E7P2-INT-120..124 | Medium without mTLS |
| Replay | Commit retry | Idempotency-Key + body hash | E7P2-INT-145..148 | Low |
| Cross-tenant access | IDOR | Tenant predicate on all reads | E7P2-INT-130 | Low |
| External ID collision | Duplicate Create | DB unique + 409 | E7P2-INT-138..140 | Low |
| Mass assignment | JSON body | Allowlist schema | E7P2-INT-155 | Low |
| Payload bomb | Large JSON | 413 size cap + depth limit | E7P2-INT-160 | Low |
| Stale overwrite | Concurrent edit | Baseline version tokens | E7P2-INT-149..152 | Low |
| Hash tampering | Commit | Server-side hash verify | E7P2-INT-153 | Low |
| Auto-publish | Malicious client | No publish in commit path | E7P2-INT-165 | Low |
| Competitor leak | Read APIs | Response field allowlist | E7P2-INT-166 | Low |

---

## 14. Vendor compatibility

| Capability | Canonical API | SAP adapter | 1C adapter | Oracle/Dynamics | TMS |
|---|---|---|---|---|---|
| Create DRAFT | JSON v1 | Map IDoc/BAPI → JSON | Map document → JSON | Map entity → JSON | Map shipment req → JSON |
| Update DRAFT | JSON v1 | Delta sync | Delta sync | Delta sync | Delta sync |
| External ID | Required | SAP doc number | 1C ref | External key | TMS load ID |
| Reference codes | Mapping table | SAP codes | 1C codes | ERP codes | TMS codes |
| Preview/Commit | Native | Pre-adapter transform | Pre-adapter transform | Pre-adapter transform | Pre-adapter transform |
| Auth | OAuth/API key | SAP OAuth config | 1C service user | Azure AD | TMS API key |

**Core API contains no vendor-specific fields.** Adapters are out of current implementation scope.

---

## 15. Block status

| Block | Status |
|---|---|
| Repository capability inventory | **COMPLETE** |
| RFx domain reuse | **COMPLETE** |
| Machine authentication inventory | **COMPLETE** |
| Integration infrastructure inventory | **COMPLETE** |
| ERP API scope | **COMPLETE** |
| Canonical JSON schema | **COMPLETE** (ADR-012) |
| Create Draft flow | **COMPLETE** |
| Update Draft flow | **COMPLETE** |
| Preview/Commit | **COMPLETE** (ADR-014) |
| External IDs | **COMPLETE** (ADR-015) |
| Reference mappings | **COMPLETE** (ADR-015) |
| Idempotency | **COMPLETE** (ADR-014) |
| Concurrency | **COMPLETE** |
| State safety | **COMPLETE** |
| Errors | **COMPLETE** |
| Audit | **COMPLETE** |
| Webhook/async decision | **COMPLETE** (sync v1) |
| Data model | **COMPLETE** (proposed) |
| Threat model | **COMPLETE** |
| Vendor compatibility | **COMPLETE** |
| Sequence diagrams | **COMPLETE** |
| State machines | **COMPLETE** |
| Acceptance test IDs | **COMPLETE** |
| ADRs | **COMPLETE** (012–015) |
| Roadmap markers | **COMPLETE** |

---

## 16. Implementation gate (not this stream)

```
ERP_API_IMPLEMENTATION_AUTHORIZED=NO
DO_NOT_MERGE_UNTIL_CONTROLLER_REVIEW=YES
NEXT_ACTION=INDEPENDENT_CONTROLLER_REVIEW_ERP_API_ARCHITECTURE
```

Controller must authorize before:

1. Migration 000074
2. Identity integration principal tables
3. Gateway ERP routes + scope middleware
4. rfx-service ERP preview/commit handlers
5. OpenAPI additions
6. Integration tests E7P2-INT-120..189

---

## 17. References

- [RFX_V3_0E7_PHASE2_EXCEL_ERP.md](./RFX_V3_0E7_PHASE2_EXCEL_ERP.md)
- [RFX_V3_0E7_BUYER_XLSX_IMPORT_PREVIEW.md](./RFX_V3_0E7_BUYER_XLSX_IMPORT_PREVIEW.md)
- [RFX_V3_0E7_BUYER_XLSX_IMPORT_P4_COMMIT.md](./RFX_V3_0E7_BUYER_XLSX_IMPORT_P4_COMMIT.md)
- [RFX_V3_SECURITY.md](../RFX_V3_SECURITY.md)
- [RFX_V3_ROADMAP.md](../RFX_V3_ROADMAP.md)
