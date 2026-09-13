# RFx v3.0E7 — Buyer XLSX Import P4 Commit Discovery

**Status:** `ARCHITECTURE_FROZEN_ACCEPTED`

**Base:** `origin/main` @ `06615a92530c4812a36c3a13c4e5af7cbd242ac4`
**Discovery branch:** `discovery/rfx-buyer-xlsx-import-p4-commit-v3.0e7-phase2`

| Marker | Value |
|---|---|
| `STATUS` | `ARCHITECTURE_FROZEN_ACCEPTED` |
| `CONTROLLER_VERDICT` | `GO` |
| `CONTROLLER_DECISION_REQUIRED` | `NO` |
| `P4_IMPLEMENTATION_AUTHORIZED` | `YES` |
| `P4_IMPLEMENTATION_STARTED` | `NO` |
| `P4_PRODUCT_CODE_CHANGED` | `NO` |
| `P4_TEST_CODE_CHANGED` | `NO` |
| `P4_OPENAPI_CHANGED` | `NO` |
| `P4_CI_CHANGED` | `NO` |
| `MIGRATION_000074_CREATED` | `NO` |
| `MAX_MIGRATION_CONTRACT` | `000073` |
| `MIGRATION_000073_SUFFICIENT` | `YES` |
| `MIGRATION_000074_REQUIRED` | `NO` |
| `NEXT_ACTION` | `PUBLISH_P4_ARCHITECTURE_DRAFT_PR` |

---

## 1. Status and scope

P4 Commit applies a **single-use, atomic** mutation of an existing active buyer DRAFT using a persisted Preview analysis row created by P3.

| In scope (P4 v1) | Out of scope |
|---|---|
| `POST …/xlsx-import/commit` with `{ "analysis_id": "<uuid>" }` | Create from XLSX (new event) |
| UPDATE_EXISTING_DRAFT only | Carrier XLSX |
| Server-loaded stored proposal + hash re-verify | ERP/SAP/1C |
| Idempotency-Key required | Frontend / training / browser |
| Actor-only binding (Preview creator) | Shared approval workflow |
| Lots + questionnaire graph replace on active draft | Migration 000074 |

Frozen product rules:

```
UPDATE_EXISTING_DRAFT_ONLY=YES
CLIENT_SENDS_ANALYSIS_ID_ONLY=YES
IDEMPOTENCY_KEY_REQUIRED=YES
BINARY_XLSX_REUSED=NO
STORED_PROPOSAL_HASH_REVERIFIED=YES
SERVER_BASELINE_RECHECK_REQUIRED=YES
STALE_ANALYSIS_HTTP=409
EXPIRED_ANALYSIS_HTTP=409
CONSUMED_ANALYSIS_HTTP=409_OR_IDEMPOTENT_REPLAY
SUCCESS_HTTP=200

ATOMIC_GRAPH_LOTS_APPLY=REQUIRED
ANALYSIS_CONSUME_ATOMIC=REQUIRED
AUDIT_ATOMIC=REQUIRED
IDEMPOTENCY_ATOMIC=REQUIRED
PUBLISHED_VERSIONS_IMMUTABLE=REQUIRED
COMPETITOR_CONFIDENTIALITY=REQUIRED
```

---

## 2. Baseline evidence

| Item | Evidence |
|---|---|
| Preview P2/P2.1/P3 merged | PR #125 → `b2df9ac20ae9bf9dbbb48f5e78199766643ad2ef` (head `db5bb9ef`, CI `34719259509`) |
| Docs closeout merged | PR #127 → `06615a92530c4812a36c3a13c4e5af7cbd242ac4` (head `8acdd6b0`, CI `34752643889`) |
| Migration 000073 | Present — `infrastructure/migrations/000073_rfx_excel_erp_exchange_v3_0e7_phase2.{up,down}.sql` |
| Migration 000074 | Absent / not authorized |
| P4 status in roadmap/docs | `NOT_STARTED` |
| Preview route | `POST /v1/rfx-events/{id}/xlsx-import/preview` — `packages/shared-go/rfx/e7_excel_exchange_routes.go` |
| Commit route | **Not implemented** |

---

## 3. Existing P3 architecture (summary)

P3 Preview (`services/rfx-service/internal/service/excel_exchange_preview.go`):

1. Authorize BuyerManage + load active draft baseline (event row version, draft row version, questionnaire tree, lots).
2. Parse XLSX via `xlsxexchange.ParseBuyerImportPreview`.
3. On `ready_to_commit=true`, persist immutable analysis via `ImportAnalysisRepository.CreatePreview` inside `txRunner.Run`.
4. Canonical payload built by `CanonicalImportPayloadJSON` / `buildCanonicalImportPayload` — includes **full proposed lots + questionnaire** and baseline bindings.

P3 does **not** mutate event/version/graph/lots, audit, or idempotency records.

---

## 4. Migration 000073 sufficiency

### 4.1 Schema summary (`rfx.rfx_import_analyses`)

| Column | Role |
|---|---|
| `id`, `tenant_id`, `actor_id`, `actor_company_id` | Identity binding |
| `workbook_type`, `schema_version` | Immutable workbook contract |
| `target_type`, `target_id`, `target_version` | DRAFT binding |
| `canonical_payload_json`, `canonical_hash` | Immutable proposal + SHA-256 |
| `status` | `PREVIEWED` \| `CONSUMED` \| `EXPIRED` |
| `validation_summary` | Preview counts (non-authoritative for commit) |
| `expires_at`, `created_at` | TTL boundary |
| `consumed_at`, `result_reference_type`, `result_reference_id` | Single-use consume proof |

### 4.2 DB guards (evidence: migration lines 67–98)

| Guard | Mechanism |
|---|---|
| Payload/hash immutability | Trigger `prevent_rfx_import_analysis_payload_mutation` rejects changes to payload, hash, bindings, tenant/actor |
| Single-use consume | Trigger rejects `CONSUMED → non-CONSUMED`; `MarkConsumed` requires `status = PREVIEWED` |
| Consumed invariants | CHECK: `CONSUMED` ⇔ `consumed_at` + result reference present |

### 4.3 Discovery answers

| # | Question | Answer | Evidence |
|---|---|---|---|
| 1 | Atomic lock via `SELECT … FOR UPDATE`? | **YES** (P4 must add repo method) | Standard PostgreSQL row lock on PK; pattern exists in `LockEventVersionState`, `LockVersionByID` |
| 2 | Distinguish READY / CONSUMED / EXPIRED / INVALID? | **YES** | `status` + `expires_at` + `consumed_at`; INVALID = domain revalidation failure (422, no consume) |
| 3 | Double consume under concurrency? | **NO** (with FOR UPDATE + MarkConsumed guard) | Second updater gets `pgx.ErrNoRows` → Conflict |
| 4 | Consume in same tx as graph writes? | **YES** | `ImportAnalysisRepository.WithTx(tx)` already supported |
| 5 | DB guards on payload/tenant/re-consume? | **YES** | Trigger + MarkConsumed WHERE clause |
| 6 | Migration 000074 required? | **NO** | See §4.4 |

### 4.4 Why 000074 is not required

P4 needs **application-layer** additions only:

- `LockImportAnalysisForUpdate(id, tenant_id)` — `SELECT … FOR UPDATE`
- Optional lazy `UPDATE status='EXPIRED'` (not required v1 — app may treat `now >= expires_at` as effective EXPIRED while row remains `PREVIEWED`)
- Lot update/soft-delete repository methods (table `rfx_lots` already has `deleted_at`; no schema change)
- New commit orchestration service (no new tables)

**Conclusion:** `MIGRATION_000073_SUFFICIENT=YES`, `MIGRATION_000074_REQUIRED=NO`.

---

## 5. Stored proposal / hash contract

### 5.1 Payload shape (`canonicalImportPayload` in `buyer_import_hash.go`)

Persisted JSON contains:

| Field | Commit relevance |
|---|---|
| `target_event_id`, `target_draft_version_id`, `target_version_number` | Stale baseline binding |
| `event_row_version`, `draft_row_version` | Optimistic stale detection |
| `lots[]` | **Full proposed lot set** (stable key: `lot_number`) |
| `questionnaire.{sections,questions,options,rules}` | **Full proposed graph** (stable codes only) |
| `questionnaire_diff_hash`, `lots_diff_hash` | Integrity cross-check (informational) |
| `commit_affecting_warnings` | Re-evaluated at commit; stale metadata warnings already surfaced at Preview |

### 5.2 Proposal completeness

| Question | Answer |
|---|---|
| Full proposed state or diff-only? | **Full proposed state** — P4 applies payload directly without re-parsing XLSX |
| Re-parse XLSX at commit? | **NO** — `BINARY_XLSX_REQUIRED_FOR_COMMIT=NO` |
| Client-supplied proposal/hash? | **FORBIDDEN** — only `analysis_id` + `Idempotency-Key` |
| Hash re-verifiable after JSONB round-trip? | **YES** — `StableStoredPayload` + `VerifyStoredCanonicalPayloadHash` (P3 proven) |
| Trust XLSX metadata tenant/event? | **NO** — server bindings in payload compared to locked server state |

Commit-relevant fields: all `lots[]` and `questionnaire.*` stable-code records.  
Verify-only at Preview (not re-trusted at commit): workbook metadata row versions (already bound into payload baseline fields).

---

## 6. Graph and lot mutation inventory

### 6.1 Questionnaire entities (active DRAFT)

| Entity | Service methods | Repository methods | Stable key | UUID policy | Delete |
|---|---|---|---|---|---|
| Section | `CreateSection`, `UpdateSection`, `DeleteSection`, `ReorderSections` | Same in `questionnaire_repository.go` | `section_code` | Server `gen_random_uuid()` | Soft delete + OCC `version` |
| Question | `CreateQuestion`, `UpdateQuestion`, `DeleteQuestion`, … | Same | `question_code` | Server UUID | Soft delete + OCC |
| Option | `CreateOption`, `UpdateOption`, `DeleteOption` | Same | `option_code` | Server UUID | Soft delete + OCC |
| Rule | `CreateRule`, `UpdateRule`, `DeleteRule` | Same; resolves `target_question_code` → UUID | `rule_code` | Server UUID | Soft delete + OCC |
| Draft touch | `SaveDraft` | `TouchDraftVersion` | N/A | N/A | Bumps `rfx_versions.version` |

**Gap:** `QuestionnaireService` methods are **autocommit** — each call is separate tx + post-commit audit. P4 cannot call HTTP or service layer directly for atomic apply.

**Reference pattern:** `TemplateQuestionnaireService.runGraphMutation` + `VersionLifecycleService` tx/idempotency/audit — all repos support `WithTx(tx)`.

### 6.2 Lots (event-scoped)

| Entity | Service | Repository | Stable key | UUID policy | Delete |
|---|---|---|---|---|---|
| Lot | `RfxService.CreateLot` only | `RfxRepository.CreateLot`, `ListLotsByEvent` | `lot_number` | Server UUID | **No public delete/update API** |

**Implementation gap (P4 product code, not migration):** need `UpdateLotByNumber`, `SoftDeleteLotByNumber` (or equivalent) using existing `rfx_lots.deleted_at`.

### 6.3 Bulk replace

No existing `ReplaceDraftGraphFromProposal` operation. P4 algorithm must:

1. Build stable-code → existing UUID map from locked draft state.
2. Upsert/update/create entities present in proposal.
3. Soft-delete entities absent from proposal (safe order: rules → options → questions → sections; lots separately).
4. Re-resolve rule targets after question UUID mapping.

---

## 7. UUID / stable-code semantics (frozen proposal)

| Rule | Semantics |
|---|---|
| Existing stable code in DRAFT | Preserve existing UUID |
| New stable code in proposal | Server-generated UUID on insert |
| Stable code absent from proposal | Soft-delete from DRAFT |
| Client/XLSX UUID columns | Ignored (workbook has no UUID columns) |
| Rule `target_question_code` | Resolved to event-local UUID after mapping |
| Cross-event UUID reuse | Forbidden by stable-code scoping |
| Published/superseded versions | Immutable — commit refuses non-DRAFT targets |

Edge cases:

| Case | P4 v1 behavior |
|---|---|
| Stable code rename | **Delete old + create new** (no rename-in-place) |
| Question type change | Allowed if domain validation passes on revalidation |
| Delete question with options/rules | Cascade via soft-delete order |
| `lot_number` change | Treat as delete old lot + create new lot |
| Sort order only change | Update in place |
| Empty optional fields | Normalize to NULL/empty per existing repo conventions |

---

## 8. Stale baseline protection

### 8.1 Fields compared at commit (inside tx, after locks)

| Field | Source at commit | Source in analysis payload |
|---|---|---|
| `tenant_id` | Actor + analysis row | `tenant_id` column |
| `event_id` | URL + analysis | `target_event_id` |
| `draft_version_id` | `LockEventVersionState.draft_version_id` | `target_draft_version_id` |
| `event_row_version` | `LockEventVersionState.event_version` | `event_row_version` |
| `draft_row_version` | `LockVersionByID.version` | `draft_row_version` |
| `target_version_number` | Draft `version_number` | `target_version_number` |
| Active draft status | `domain.EnsureDraftVersionMutable` | N/A |

### 8.2 Stale condition

**Any mismatch → HTTP 409 `stale_target`**, no consume, no graph/lot writes, no audit.

### 8.3 ABA analysis

| Scenario | Detected? |
|---|---|
| Row versions advanced and restored to same integers | **Partial risk** — if versions literally match, stale check passes; mitigated by draft UUID binding + entity-level OCC on updates/deletes |
| Active DRAFT replaced (new draft_version_id) | **YES** — `target_draft_version_id` mismatch |
| Order-only graph edits | **YES** — `draft_row_version` bump via `TouchDraftVersion` / entity updates |
| Lot-only edits without draft bump | **Gap** — lots are event-scoped; lot mutations do not bump draft row version today |

**Mitigation for lot-only stale gap (P4 v1):** persist and compare **`baseline_lots_fingerprint`** (server lot set at Preview time) against the current DB lot fingerprint inside commit tx before mutation. Mismatch → 409 `stale_target`.

**Assessment:** Event/draft row versions are **insufficient** for lot-only mutation detection. **`baseline_lots_fingerprint` is mandatory** (P4-IMPL-04 / `P4_ENTRY_GATE_BASELINE_LOT_FINGERPRINT`). Not a migration blocker.

### 8.4 Lot fingerprint semantics (frozen)

| Term | Meaning |
|---|---|
| `proposed_lots_hash` | Canonical hash of the **imported lot set** (`lots[]` in stored proposal) — the state Commit will apply |
| `baseline_lots_fingerprint` | Canonical fingerprint of the **server lot set at Preview time** — bound into the analysis row for stale detection |
| Commit stale check | Current DB lot fingerprint must equal stored `baseline_lots_fingerprint`; event/draft row versions alone do not cover lot-only edits |

P4 implementation must persist `baseline_lots_fingerprint` when creating the Preview analysis (or equivalent immutable binding) and re-verify it after `LockImportAnalysisForUpdate`.

### 8.5 ABA policy (frozen v1)

```
ABA_POLICY=CONTENT_EQUIVALENT_STATE_ACCEPTED_V1
STRICT_ABA_REJECTION=DEFERRED
CONTENT_EQUIVALENT_REPLAY_ACCEPTED=YES
```

| Rule | v1 behavior |
|---|---|
| Content-equivalent restore | If data changed and returned to a **fully identical canonical state**, Commit **may proceed** when all binding/fingerprint checks pass |
| Strict historical ABA | Rejected-then-restored version **numbers** without content change are **not** auto-blocked — monotonic revision model deferred |
| Draft replacement | **Detected** — `target_draft_version_id` mismatch → 409 |
| Non-equivalent edits | **Detected** — event/draft row version mismatch → 409 |
| Lot-only non-equivalent edits | **Detected** — `baseline_lots_fingerprint` mismatch → 409 |

---

## 9. Authorization and binding

| Requirement | Policy |
|---|---|
| Feature flag | Default off (same as Preview) |
| RBAC | BuyerManage only; BuyerRead/Carrier → 403 |
| Tenant isolation | Cross-tenant → 404 |
| Company ownership | Existing `authorizeBuyerManage` event owner check |
| Gateway identity | Trusted headers only (gateway middleware) |
| Analysis actor binding | **`analysis.actor_id` must equal commit actor** (P4-ADR-05) |
| Shared BuyerManage commit | **Deferred** — not P4 v1 |

---

## 10. Expiry, consumption, concurrency

### Expired

Expiry is evaluated **after** `LockImportAnalysisForUpdate` and **before** any graph/lot mutation, audit, consume, or idempotency store.

If `now() >= analysis.expires_at` (UTC):

- HTTP **409** `analysis_expired`
- Optional: lazy `UPDATE status='EXPIRED'` (same tx, before consume attempt)
- No graph writes, no consume, no audit, no idempotency success row
- Row may remain physically `PREVIEWED` in DB — app treats `expires_at` as authoritative (`EXPIRY_AFTER_LOCK=REQUIRED`)
- `MarkConsumed` must not run on expired analysis; commit path fail-closed on status + expiry in the same transaction

### Consumed

| Case | Behavior |
|---|---|
| Same `Idempotency-Key` + same fingerprint | **200 replay** stored response |
| Different key, analysis already consumed | **409** `analysis_already_consumed` |
| MarkConsumed | Sets `status=CONSUMED`, `consumed_at`, `result_reference_type=DRAFT_EVENT`, `result_reference_id=<draft_version_id>` |

### Concurrent commits (two different keys)

1. Both pass pre-checks until analysis lock.
2. First acquires `FOR UPDATE`, consumes, commits.
3. Second sees `status != PREVIEWED` → 409 (or idempotent replay if same key).

### Canonical lock order (P4-ADR-08)

1. **Idempotency record** — `IdempotencyRepository.Get` (no lock; insert at end)
2. **Event** — `LockEventVersionState` (`rfx_events FOR UPDATE`)
3. **Draft version** — `LockVersionByID` (`rfx_versions FOR UPDATE`)
4. **Import analysis** — `LockImportAnalysisForUpdate` (new)
5. **Graph/lot rows** — mutated via repos bound to same `tx`

Matches ascending FK ownership: event → version → analysis(target) → questionnaire entities.

---

## 11. Idempotency contract

Pattern: `VersionLifecycleService` + `IdempotencyRepository` (`rfx_idempotency_records`).

| Field | P4 value |
|---|---|
| `operation` | `BUYER_XLSX_IMPORT_COMMIT` (new constant) |
| `aggregate_scope` | `event_id` |
| `idempotency_key` | Required header |
| `request_body_hash` | SHA-256 of canonical JSON: `{ "analysis_id": "<uuid>" }` scoped to event |

| Case | Result |
|---|---|
| Same key + same request fingerprint | **200 replay** — return stored response body from `rfx_idempotency_records` |
| Same key + different request fingerprint | **409** `idempotency_conflict` |
| Different key + analysis already consumed | **409** `analysis_already_consumed` |
| Tx rollback | No durable idempotency success row |
| Store timing | After successful graph/lot mutation + audit + `MarkConsumed`, before `COMMIT` |

**Frozen contract (`IDEMPOTENCY_REPLAY_CONTRACT=FROZEN`):**

- Response body must be persisted in the existing idempotency record **or** deterministically reconstructable from stored fields on replay.
- Graph/lots apply, analysis consume, audit, and idempotency completion occur in **one transaction** (`ATOMIC_GRAPH_LOTS_CONSUME_AUDIT_IDEMPOTENCY=REQUIRED`).

---

## 12. Atomic transaction algorithm

```
CommitBuyerXlsxImport(eventID, analysisID, idempotencyKey):
  1. AuthN/RBAC/feature flag (outside or start of tx — fail fast)
  2. Validate UUIDs + non-empty idempotency key → 400
  3. BEGIN
  4. idem = IdempotencyRepo.Get(scope, key)
     if idem && hash match → return replay 200
     if idem && hash mismatch → 409
  5. eventState = LockEventVersionState(eventID)
  6. draft = LockVersionByID(eventState.draft_version_id)
  7. EnsureDraftVersionMutable(draft)
  8. analysis = LockImportAnalysisForUpdate(analysisID)
  9. Verify tenant/event/draft/actor bindings
 10. if now >= analysis.expires_at → 409 EXPIRED
 11. if analysis.status == CONSUMED → 409 or replay path
 12. VerifyStoredCanonicalPayloadHash(payload, hash) → else 422/409
 13. Compare stale baseline fields + lot fingerprint → 409 if stale
 14. Re-run domain validation on decoded proposal (parser validate helpers)
 15. Build stableCode→UUID map from current draft + lots
 16. Apply lots: create/update/soft-delete by lot_number
 17. Apply graph: sections → questions → options → rules (delete pass reverse order)
 18. TouchDraftVersion (bump draft row version once)
 19. recordAudit("rfx.buyer_xlsx_import.committed.v1", safe payload)
 20. MarkConsumed(analysis, result=draft_version_id)
 21. IdempotencyRepo.Store(200 response body)
 22. COMMIT → return 200
```

Rollback at any step → full rollback; analysis remains PREVIEWED.

---

## 13. Audit contract

| Item | Value |
|---|---|
| Event type | `rfx.buyer_xlsx_import.committed.v1` |
| Safe payload fields | `event_id`, `analysis_id`, `actor_id`, `draft_version_id`, old/new event version, old/new draft version, change counts, `canonical_hash`, `committed_at` |
| Excluded | Binary XLSX, competitor data, full questionnaire payload, free-text answers |
| Atomicity | Same transaction as graph mutation + consume + idempotency |

---

## 14. HTTP / OpenAPI contract (proposal)

| Layer | Route |
|---|---|
| Service | `POST /v1/rfx-events/{id}/xlsx-import/commit` |
| Gateway | `POST /api/v1/rfx-events/{id}/xlsx-import/commit` |

Request:

```json
{ "analysis_id": "<uuid>" }
```

Header: `Idempotency-Key: <required>`

Response 200:

```json
{
  "event_id": "...",
  "draft_version_id": "...",
  "analysis_id": "...",
  "event_version": 0,
  "draft_version": 0,
  "committed_at": "...",
  "changes": {
    "lots": {"added": 0, "updated": 0, "deleted": 0},
    "sections": {"added": 0, "updated": 0, "deleted": 0},
    "questions": {"added": 0, "updated": 0, "deleted": 0},
    "options": {"added": 0, "updated": 0, "deleted": 0},
    "rules": {"added": 0, "updated": 0, "deleted": 0}
  }
}
```

HTTP semantics:

| Code | Condition |
|---|---|
| 200 | Applied or idempotent replay |
| 400 | Malformed body / missing idempotency key |
| 401 | Unauthenticated |
| 403 | BuyerRead / Carrier |
| 404 | Feature disabled / cross-tenant / hidden resource |
| 409 | Stale baseline, expired, consumed (other key), idempotency conflict, no active draft |
| 422 | Stored proposal fails domain revalidation |
| 500 | Unexpected tx/repository failure (full rollback) |

**201 not used** — mutates existing DRAFT in place.

Route manifest entry (future): mirror Preview entry in `e7_excel_exchange_routes.go` with `IdempotencyRequired: true`.

---

## 15. Error semantics (machine codes)

| Machine code | HTTP | Consume? |
|---|---|---|
| `analysis_not_found` | 404 | NO |
| `analysis_expired` | 409 | NO |
| `analysis_already_consumed` | 409 | NO |
| `stale_target` | 409 | NO |
| `canonical_hash_mismatch` | 422 | NO |
| `proposal_revalidation_failed` | 422 | NO |
| `idempotency_conflict` | 409 | NO |
| `feature_disabled` | 404 | NO |
| `actor_binding_denied` | 403 | NO |

---

## 16. Test matrix E7P2-INT-43..70

| ID | Scope | Layer | Expected |
|---|---|---|---|
| INT-43 | Successful commit | PG+HTTP | 200, consumed, graph matches proposal |
| INT-44 | Graph/lot parity with stored proposal | PG | Exact match post-commit |
| INT-45 | Existing UUIDs preserved | PG | Stable codes map to same UUIDs |
| INT-46 | New stable codes → new UUIDs | PG | New rows, new IDs |
| INT-47 | Removed entities deleted | PG | Soft-deleted absent codes |
| INT-48 | Rule targets resolved locally | PG | No dangling target_question_id |
| INT-49 | Published/superseded unchanged | PG | Non-draft versions untouched |
| INT-50 | Stale event row version | HTTP | 409, no writes |
| INT-51 | Stale draft row version | HTTP | 409, no writes |
| INT-52 | Replaced active draft ID | HTTP | 409, no writes |
| INT-53 | Expired analysis | HTTP | 409, no consume |
| INT-54 | Consumed + new idempotency key | HTTP | 409 |
| INT-55 | Same-key replay | HTTP | 200, no duplicate writes |
| INT-56 | Same key, different analysis_id | HTTP | 409 |
| INT-57 | Concurrent commits | PG | One mutation only |
| INT-58 | Audit atomic with mutation | PG | Audit iff commit succeeds |
| INT-59 | Idempotency atomic | PG | No success row on rollback |
| INT-60 | Analysis consume atomic | PG | No CONSUMED on failure |
| INT-61 | Mid-graph failure rollback | PG | No partial graph |
| INT-62 | Lot failure rollback | PG | No partial lots |
| INT-63 | Hash mismatch denied | HTTP | 422, no consume |
| INT-64 | Actor binding denied | HTTP | 403 |
| INT-65 | BuyerRead 403 | HTTP | 403 |
| INT-66 | Carrier 403 | HTTP | 403 |
| INT-67 | Cross-tenant 404 | HTTP | 404 |
| INT-68 | Identity spoof denied | HTTP | 403/404 per gateway tests |
| INT-69 | Feature disabled 404/no writes | HTTP | 404 |
| INT-70 | Service/gateway/OpenAPI parity | Static | Route manifest parity |

Guard: `TestE7P2CommitMatrixIDsCompleteAndUnique` — IDs 43..70 inclusive, no gaps/duplicates.

### Extra tests (no matrix ID)

| Test | Purpose |
|---|---|
| `TestCommitLotFingerprintStaleDetectionExtra` | Lot-only ABA/stale protection |
| `TestCommitTransactionLeakExtra` | No open tx after failure |
| `TestCommitDeterministicReplayExtra` | Byte-stable idempotent response |
| `TestCommitCompetitorConfidentialityExtra` | No competitor fields in audit/response |
| `TestMigration073RegressionExtra` | Up/down unchanged |
| `TestP3PreviewRegressionExtra` | Preview matrix unaffected |

---

## 17. ADR decisions

| ADR | Decision | Status | Evidence |
|---|---|---|---|
| P4-ADR-01 | Migration 000073 sufficient for consume/TTL/immutability | **APPROVED_PROPOSAL** | `000073_*.up.sql`, `import_analysis_repository.go` |
| P4-ADR-02 | Apply **full stored proposal**, not diff-only | **APPROVED_PROPOSAL** | `buildCanonicalImportPayload`, `canonicalImportPayload.lots/questionnaire` |
| P4-ADR-03 | UUID preservation by stable code | **APPROVED_PROPOSAL** | Questionnaire unique constraints; `copyQuestionnaireGraph` ID map precedent |
| P4-ADR-04 | Stale fields: event/draft row versions + draft UUID + lot fingerprint | **APPROVED_PROPOSAL** | Payload bindings + lot-scope gap mitigation |
| P4-ADR-05 | Actor-only binding (Preview creator commits) | **APPROVED_PROPOSAL** | `actor_id` column + security model |
| P4-ADR-06 | TTL boundary `now >= expires_at` → 409 | **APPROVED_PROPOSAL** | P3 24h TTL, domain status constants |
| P4-ADR-07 | Consumed replay vs 409 semantics | **APPROVED_PROPOSAL** | Idempotency pattern from publish/fork |
| P4-ADR-08 | Lock order: idempotency check → event → draft → analysis → graph | **APPROVED_PROPOSAL** | `LockEventVersionState`, `LockVersionByID` |
| P4-ADR-09 | Graph replacement via ordered CRUD in one tx | **APPROVED_PROPOSAL** | Repo `WithTx`; no bulk op exists |
| P4-ADR-10 | Lot replacement via create/update/soft-delete | **APPROVED_PROPOSAL** | `CreateLot` exists; update/delete to be added |
| P4-ADR-11 | Audit event `rfx.buyer_xlsx_import.committed.v1` | **APPROVED_PROPOSAL** | Audit repository pattern |
| P4-ADR-12 | HTTP/OpenAPI contract §14 | **APPROVED_PROPOSAL** | Preview route precedent |
| P4-ADR-13 | Error mapping §15 | **APPROVED_PROPOSAL** | `apperrors.Conflict/Validation/NotFound` |
| P4-ADR-14 | Test range INT-43..70 | **APPROVED_PROPOSAL** | Continuation after INT-42 |

---

## 18. Blockers

| ID | Severity | Description | Blocks architecture? |
|---|---|---|---|
| P4-IMPL-01 | Implementation | Missing lot update/soft-delete repository methods | NO — add in P4 code without migration |
| P4-IMPL-02 | Implementation | Missing `LockImportAnalysisForUpdate` repository method | NO — SQL only |
| P4-IMPL-03 | Implementation | Missing atomic commit orchestration service | NO — new service layer |
| P4-IMPL-04 | Implementation | Lot-only stale detection needs fingerprint compare | NO — app logic |

**No migration schema blocker identified.** Architecture freeze may proceed pending controller decision.

`IMPLEMENTATION_BLOCKERS=P4-IMPL-01,P4-IMPL-02,P4-IMPL-03,P4-IMPL-04`

---

## 19. Implementation sequence (post-approval)

1. Repository: `LockImportAnalysisForUpdate`, lot update/delete by `lot_number`.
2. Domain: commit input validation, operation constants, error codes.
3. Service: `CommitBuyerXlsxImport` orchestration (tx + locks + apply + audit + consume + idempotency).
4. HTTP handler + router + gateway route manifest entry.
5. OpenAPI contract + generator.
6. Integration tests INT-43..70 + extras.
7. CI job extension (`rfx-excel-exchange-v3-integration`).

P3 Preview code remains unchanged.

---

## 20. P4 implementation entry gates

Mandatory implementation conditions (not migration blockers):

```
P4_IMPL_01_LOT_RECONCILIATION=REQUIRED
P4_IMPL_02_ANALYSIS_FOR_UPDATE_LOCK=REQUIRED
P4_IMPL_03_ATOMIC_ORCHESTRATION=REQUIRED
P4_IMPL_04_BASELINE_LOT_FINGERPRINT=REQUIRED

P4_ENTRY_GATE_BASELINE_LOT_FINGERPRINT=MANDATORY
P4_ENTRY_GATE_HASH_REVERIFICATION=MANDATORY
P4_ENTRY_GATE_SERVER_BASELINE_RECHECK=MANDATORY
P4_ENTRY_GATE_ACTOR_BINDING=MANDATORY
P4_ENTRY_GATE_IDEMPOTENCY_REPLAY=MANDATORY
P4_ENTRY_GATE_EXPIRY_AFTER_LOCK=MANDATORY
P4_ENTRY_GATE_SINGLE_TRANSACTION=MANDATORY
```

| Gate | Requirement |
|---|---|
| Baseline lot fingerprint | Persist `baseline_lots_fingerprint` at Preview; re-check after analysis lock at Commit |
| Hash re-verification | `VerifyStoredCanonicalPayloadHash` before mutation |
| Server baseline recheck | Event/draft row versions, draft UUID, actor/tenant/event bindings |
| Actor binding | `analysis.actor_id` equals commit actor |
| Idempotency replay | Same key + same fingerprint → 200 without duplicate writes |
| Expiry after lock | `now >= expires_at` → 409 before mutation/consume |
| Single transaction | Graph, lots, audit, consume, idempotency store — one tx |

`P4_IMPLEMENTATION_AUTHORIZED=YES` grants **technical authorization only** after formal docs PR closeout. Product implementation has **not** started.

---

## 21. Controller Architecture Review Record

Independent architecture review completed on discovery commit `adf895395bb9e7782fb63fddb6749807486b8e10`.

```
ARCHITECTURE_REVIEW_RESULT=GO
ARCHITECTURE_FREEZE=ACCEPTED
DISCOVERY_DOC_ACCURACY=CONFIRMED
CONTROLLER_TECHNICAL_AUTHORIZATION=YES
P4_PRODUCT_IMPLEMENTATION_STARTED=NO
SEPARATE_IMPLEMENTATION_TASK_REQUIRED=YES

REVIEWED_HEAD=adf895395bb9e7782fb63fddb6749807486b8e10
REVIEWED_BASE=06615a92530c4812a36c3a13c4e5af7cbd242ac4
BLOCKER_FINDINGS=0
HIGH_FINDINGS=0
MEDIUM_FINDINGS=1
LOW_FINDINGS=2

MIGRATION_000073_SUFFICIENT=YES
MIGRATION_000074_REQUIRED=NO
MIGRATION_000074_CREATED=NO
STORED_PROPOSAL_COMPLETE=YES
STORED_HASH_REVERIFIABLE=YES
BINARY_XLSX_REQUIRED_FOR_COMMIT=NO
NEXT_TEST_RANGE=E7P2-INT-43..70
```

### Findings recorded

**MEDIUM (1):** Lot-only mutations do not increment draft row version. P4 must implement and test `baseline_lots_fingerprint` stale detection. Mandatory P4 implementation condition; **not** a migration blocker.

**LOW (1):** Row-version ABA — v1 accepts content-equivalent state replay (`ABA_POLICY=CONTENT_EQUIVALENT_STATE_ACCEPTED_V1`); strict historical ABA rejection deferred.

**LOW (2):** Physical `EXPIRED` status may remain `PREVIEWED` in DB; expiry enforced after lock and before mutation/consume via `expires_at`.

---

## 22. Final markers

```
STATUS=ARCHITECTURE_FROZEN_ACCEPTED
CONTROLLER_VERDICT=GO
CONTROLLER_DECISION_REQUIRED=NO
P4_IMPLEMENTATION_AUTHORIZED=YES
P4_IMPLEMENTATION_STARTED=NO
P4_PRODUCT_CODE_CHANGED=NO
P4_TEST_CODE_CHANGED=NO
P4_OPENAPI_CHANGED=NO
P4_CI_CHANGED=NO
MIGRATION_000074_CREATED=NO

UPDATE_EXISTING_DRAFT_ONLY=YES
CLIENT_SENDS_ANALYSIS_ID_ONLY=YES
IDEMPOTENCY_KEY_REQUIRED=YES
BINARY_XLSX_REUSED=NO
BINARY_XLSX_REQUIRED_FOR_COMMIT=NO
STORED_PROPOSAL_HASH_REVERIFIED=YES
STORED_PROPOSAL_COMPLETE=YES
STORED_HASH_REVERIFIABLE=YES
SERVER_BASELINE_RECHECK_REQUIRED=YES
STALE_ANALYSIS_HTTP=409
EXPIRED_ANALYSIS_HTTP=409
CONSUMED_ANALYSIS_HTTP=409_OR_IDEMPOTENT_REPLAY
SUCCESS_HTTP=200

ATOMIC_GRAPH_LOTS_APPLY=REQUIRED
ANALYSIS_CONSUME_ATOMIC=REQUIRED
AUDIT_ATOMIC=REQUIRED
IDEMPOTENCY_ATOMIC=REQUIRED
ATOMIC_GRAPH_LOTS_CONSUME_AUDIT_IDEMPOTENCY=REQUIRED
PUBLISHED_VERSIONS_IMMUTABLE=REQUIRED
COMPETITOR_CONFIDENTIALITY=REQUIRED

P4_IMPL_01_LOT_RECONCILIATION=REQUIRED
P4_IMPL_02_ANALYSIS_FOR_UPDATE_LOCK=REQUIRED
P4_IMPL_03_ATOMIC_ORCHESTRATION=REQUIRED
P4_IMPL_04_BASELINE_LOT_FINGERPRINT=REQUIRED
P4_ENTRY_GATE_BASELINE_LOT_FINGERPRINT=MANDATORY
BASELINE_LOT_FINGERPRINT_MANDATORY=YES
PROPOSED_LOTS_HASH_DISTINCT=YES

ABA_POLICY=CONTENT_EQUIVALENT_STATE_ACCEPTED_V1
STRICT_ABA_REJECTION=DEFERRED
CONTENT_EQUIVALENT_REPLAY_ACCEPTED=YES

IDEMPOTENCY_REPLAY_CONTRACT=FROZEN
EXPIRY_AFTER_LOCK=REQUIRED

MIGRATION_000073_SUFFICIENT=YES
MIGRATION_000074_REQUIRED=NO
IMPLEMENTATION_BLOCKERS=P4-IMPL-01,P4-IMPL-02,P4-IMPL-03,P4-IMPL-04
NEXT_TEST_RANGE=E7P2-INT-43..70
NEXT_ACTION=PUBLISH_P4_ARCHITECTURE_DRAFT_PR
```
