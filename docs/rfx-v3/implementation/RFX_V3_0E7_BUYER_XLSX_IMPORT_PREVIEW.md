# RFx v3.0E7 — Buyer XLSX Import Preview Discovery

**Status:** `IMPLEMENTATION_IN_PROGRESS`
**IMPLEMENTATION_STARTED:** `YES`
**Base:** `origin/main` @ `e8a7fb582328d452d1f800915b0015fa0d31d6d7`
**Discovery branch:** `discovery/rfx-buyer-xlsx-import-preview-v3.0e7-phase2`
**Implementation branch:** `feat/rfx-buyer-xlsx-import-preview-parser-v3.0e7-phase2`
**Architecture review HEAD:** `5d51af525cd9ee3f55fb8c58ab5f11be584ade5e`

| Marker | Value |
|---|---|
| `BUYER_XLSX_EXPORT_V1_STATUS` | `IMPLEMENTED_ACCEPTED` (PR #123) |
| `BUYER_XLSX_IMPORT_PREVIEW_STATUS` | `P2_1_HARDENING_IMPLEMENTED` |
| `P2_PARSER_VALIDATOR` | `IMPLEMENTED_ACCEPTED` |
| `P2_1_HARDENING` | `IMPLEMENTED_PENDING_CONTROLLER_REVIEW` |
| `P3_PREVIEW_SERVICE_HTTP` | `NOT_STARTED` |
| `MAX_PREVIEW_ISSUES` | `2000` |
| `P4_COMMIT` | `NOT_STARTED` |
| `WORKBOOK_SCHEMA` | `BINTRANS_RFX_BUYER_XLSX_V1` |
| `MAX_MIGRATION` | `000073` |
| `MIGRATION_000074_ALLOWED` | `NO` |

---

## 1. Accepted baseline

| Item | Evidence |
|---|---|
| Export V1 merged | PR #123 → `a6b66bea51579d689e98188e31446e436524927d` |
| Docs closeout merged | PR #124 → `e8a7fb582328d452d1f800915b0015fa0d31d6d7` |
| Migration 000073 | Present on `main` |
| Migration 000074 | Absent / not authorized |
| Buyer XLSX import routes/handlers | **Not implemented** on `main` |
| `rfx_import_analyses` repository | Foundation only — **no service callers** |
| Controller decision | `NORMALIZED_PREVIEW_PERSISTENCE=YES` (Phase 2 doc §1) |
| Controller decision | `XLSX_BINARY_PERSISTENCE=NO` |
| Controller decision | `XLSX_MULTIPART_DIRECT_TO_RFX=YES` (authorized, not implemented) |

---

## 2. Existing code inventory

| Area | Path / artifact | State on main |
|---|---|---|
| Buyer workbook generator | `services/rfx-service/internal/xlsxexchange/buyer_workbook.go` | **Implemented** (export) |
| Canonical snapshot | `services/rfx-service/internal/xlsxexchange/canonical.go` | **Implemented** (export determinism) |
| Formula hardening | `buyer_workbook.go` (`SetCellStr`, text style NumFmt 49) | **Implemented** |
| ZIP security | `services/rfx-service/internal/xlsxsecurity/inspect.go` | **Implemented** |
| Export service | `services/rfx-service/internal/service/excel_exchange_service.go` | **Implemented** (read-only GET) |
| Export handler | `services/rfx-service/internal/http/handlers/excel_exchange_handler.go` | **Implemented** |
| Route manifest | `packages/shared-go/rfx/e7_excel_exchange_routes.go` | Export route only |
| Import analysis domain | `services/rfx-service/internal/domain/excel_exchange.go` | **Foundation** |
| Import analysis repo | `services/rfx-service/internal/repository/import_analysis_repository.go` | `CreatePreview`, `GetByID`, `MarkConsumed` |
| External object links | `services/rfx-service/internal/repository/external_object_link_repository.go` | Foundation (ERP, not import) |
| E2 version compare | `services/rfx-service/internal/domain/version_compare.go` | Questionnaire graph only — **no Lots** |
| Graph mutation APIs | `RfxService` section/question/option/rule CRUD + reorder | **Implemented** |
| Create event | `RfxService.CreateEvent`, `POST /from-template` | **Implemented** |
| Multipart upload in rfx-service | — | **None** |
| Gateway body limit middleware | `services/api-gateway/internal/http/middleware/bodylimit.go` | Pattern exists (`MaxBytesReader`) |
| Low-code import preview (reference) | `services/low-code-service/.../admin_form_template_service.go` | Separate domain; records preview audit |
| Integration tests | E7P2-INT-01..20 | Export + migration foundation |

---

## 3. Export workbook schema inventory

Sheet order (fixed, from `buyer_workbook.go`):

`Instructions` → `Metadata` → `Lots` → `Sections` → `Questions` → `Options` → `Rules`

All data cells are written as strings via `SetCellStr`. Numeric/bool fields are stringified at export.

### 3.1 Instructions

No header row. Six static rows × three locale columns (A=RU, B=EN, C=ZH). Not tabular import data.

| SHEET | COLUMN | TYPE | REQUIRED | STABLE KEY | SOURCE DOMAIN FIELD | IMPORTABLE | NOTES |
|---|---|---|---|---|---|---|---|
| Instructions | A (RU) | string | — | — | static | NO | Row 1 embeds schema name |
| Instructions | B (EN) | string | — | — | static | NO | Row 2 embeds schema version `1` |
| Instructions | C (ZH) | string | — | — | static | NO | Guidance only |

### 3.2 Metadata (key/value rows; col A = key, col B = value; no header)

| SHEET | COLUMN | TYPE | REQUIRED | STABLE KEY | SOURCE DOMAIN FIELD | IMPORTABLE | NOTES |
|---|---|---|---|---|---|---|---|
| Metadata | schema_name | string | yes (always written) | schema_name | constant `BINTRANS_RFX_BUYER_XLSX_V1` | VERIFY_ONLY | Must match; reject mismatch |
| Metadata | schema_version | string | yes | schema_version | constant `"1"` | VERIFY_ONLY | Must be `1` |
| Metadata | exported_at_utc | string | yes | — | `BuyerDraftMetadata.ExportedAtUTC` | NO | RFC3339; excluded from canonical compare |
| Metadata | tenant_id | string | yes | — | `BuyerDraftMetadata.TenantID` | NO | **Never trust for auth** |
| Metadata | rfx_event_id | string | yes | — | `BuyerDraftMetadata.RfxEventID` | VERIFY_ONLY | Compare to URL target; warning if mismatch |
| Metadata | rfx_version_id | string | yes | — | `BuyerDraftMetadata.RfxVersionID` | VERIFY_ONLY | Compare to server active draft |
| Metadata | version_number | string | yes | — | `BuyerDraftMetadata.VersionNumber` | VERIFY_ONLY | Informational |
| Metadata | version_status | string | yes | — | `BuyerDraftMetadata.VersionStatus` | VERIFY_ONLY | Must be `DRAFT` for UPDATE mode |
| Metadata | event_row_version | string | yes | — | `BuyerDraftMetadata.EventRowVersion` | BASELINE | Optimistic-lock baseline |
| Metadata | version_row_version | string | yes | — | `BuyerDraftMetadata.VersionRowVersion` | BASELINE | Optimistic-lock baseline |
| Metadata | creation_channel | string | yes | — | `CreationChannel` | NO | Immutable on event |
| Metadata | source_template_version_id | string | no | — | optional UUID | NO | Informational |
| Metadata | source_template_version_number | string | no | — | optional int | NO | Informational |

### 3.3 Lots (header row 1)

Headers (exact): `lot_number`, `name`, `description`, `category`, `estimated_value`, `currency_code`, `status`

| SHEET | COLUMN | TYPE | REQUIRED | STABLE KEY | SOURCE DOMAIN FIELD | IMPORTABLE | NOTES |
|---|---|---|---|---|---|---|---|
| Lots | lot_number | string | yes | **lot_number** | `RfxLot.LotNumber` | YES | Unique per event |
| Lots | name | string | yes | — | `RfxLot.Name` | YES | |
| Lots | description | string | no | — | `RfxLot.Description` | YES | |
| Lots | category | string | no | — | `RfxLot.Category` | YES | |
| Lots | estimated_value | string (decimal) | no | — | `RfxLot.EstimatedValue` | YES | Parse as float64 |
| Lots | currency_code | string | no | — | `RfxLot.CurrencyCode` | YES | |
| Lots | status | string | yes (export always writes) | — | `RfxLot.Status` | YES | Validate against allowed lot statuses |

Sorted at export by `lot_number`, then `name`.

### 3.4 Sections (header row 1)

Headers: `section_code`, `title_ru`, `title_en`, `title_zh`, `description_ru`, `description_en`, `description_zh`, `sort_order`

| SHEET | COLUMN | TYPE | REQUIRED | STABLE KEY | SOURCE DOMAIN FIELD | IMPORTABLE | NOTES |
|---|---|---|---|---|---|---|---|
| Sections | section_code | string | yes | **section_code** | `Section.SectionCode` | YES | `ValidateSectionCode` |
| Sections | title_ru/en/zh | string | yes* | — | `Section.Title` | YES | **I18N monolingual**: import uses `_ru` as authoritative; `_en/_zh` must match or warn |
| Sections | description_ru/en/zh | string | no | — | `Section.Description` | YES | Same triplicate rule |
| Sections | sort_order | string (int) | yes | — | `Section.SortOrder` | YES | |

### 3.5 Questions (header row 1)

Headers: `section_code`, `question_code`, `question_type`, `title_ru`, `title_en`, `title_zh`, `description_ru`, `description_en`, `description_zh`, `required`, `sort_order`, `validation_json`

| SHEET | COLUMN | TYPE | REQUIRED | STABLE KEY | SOURCE DOMAIN FIELD | IMPORTABLE | NOTES |
|---|---|---|---|---|---|---|---|
| Questions | section_code | string | yes | FK | parent section | YES | Must reference existing section_code |
| Questions | question_code | string | yes | **question_code** | `Question.QuestionCode` | YES | Unique globally in workbook |
| Questions | question_type | string | yes | — | `Question.QuestionType` | YES | Must be in allowed set (TEXT, NUMBER, …) |
| Questions | title_ru/en/zh | string | yes | — | `Question.Label` | YES | Monolingual triplicate |
| Questions | description_ru/en/zh | string | no | — | `Question.HelpText` | YES | Monolingual triplicate |
| Questions | required | string (bool) | yes | — | `Question.Required` | YES | `"true"` / `"false"` |
| Questions | sort_order | string (int) | yes | — | `Question.SortOrder` | YES | |
| Questions | validation_json | string (JSON) | no | — | `Question.ValidationRuleJSON` | YES | Canonical JSON; empty allowed |

### 3.6 Options (header row 1)

Headers: `question_code`, `option_code`, `label_ru`, `label_en`, `label_zh`, `sort_order`

| SHEET | COLUMN | TYPE | REQUIRED | STABLE KEY | SOURCE DOMAIN FIELD | IMPORTABLE | NOTES |
|---|---|---|---|---|---|---|---|
| Options | question_code | string | yes | FK | parent question | YES | |
| Options | option_code | string | yes | **option_code** | `QuestionOption.OptionCode` | YES | Unique per question_code |
| Options | label_ru/en/zh | string | yes | — | `QuestionOption.Label` | YES | Monolingual triplicate |
| Options | sort_order | string (int) | yes | — | `QuestionOption.SortOrder` | YES | |

### 3.7 Rules (header row 1)

Headers: `rule_code`, `source_question_code`, `condition`, `target_question_code`, `action`, `sort_order`

| SHEET | COLUMN | TYPE | REQUIRED | STABLE KEY | SOURCE DOMAIN FIELD | IMPORTABLE | NOTES |
|---|---|---|---|---|---|---|---|
| Rules | rule_code | string | yes | **rule_code** | `QuestionRule.RuleCode` | YES | Unique |
| Rules | source_question_code | string | no | — | derived from `ConditionJSON` | VERIFY | Must match first code in condition |
| Rules | condition | string (JSON) | yes | — | `QuestionRule.ConditionJSON` | YES | Canonical JSON |
| Rules | target_question_code | string | no | — | resolved from `TargetQuestionID` | YES | Empty if no target |
| Rules | action | string | yes | — | `QuestionRule.Action` | YES | `SHOW` / `HIDE` / `REQUIRE` |
| Rules | sort_order | string (int) | yes | — | `QuestionRule.SortOrder` | YES | |

**Not exported (must reject if present as extra sheets/columns with competitor data):** participants, carrier responses, bids, scoring, invitations, awards, lanes.

---

## 4. Gap matrix

| Capability | Export V1 | Import Preview (target) | Gap |
|---|---|---|---|
| Route / OpenAPI | GET xlsx-export | POST multipart preview | **Not started** |
| Workbook parser (7 sheets) | Generator only | Reader + header validation | **Not started** |
| Schema version gate | Writes metadata | Must verify `BINTRANS_RFX_BUYER_XLSX_V1` / `1` | **Not started** |
| Security pipeline | Post-generate inspect | Pre-Excelize inspect | Reuse `xlsxsecurity` |
| Normalized payload | N/A | Build canonical import proposal JSON | **Not started** |
| Domain validation | N/A | Reuse/create validators for lots + graph | Partial reuse |
| Diff vs current DRAFT | N/A | E2 compare for graph; separate Lots diff | **Partial** (E2 lacks Lots) |
| Persist preview | N/A | `rfx_import_analyses` | Repo exists, no caller |
| Commit apply | N/A | Future increment | Out of scope |
| Multipart handling | N/A | Direct to rfx-service | No pattern in rfx-service yet |
| Feature flag | `RFX_EXCEL_EXCHANGE_ENABLED` | Same flag (recommended) | Exists |
| RBAC | BuyerManage | BuyerManage | Pattern exists |
| Integration tests | E7P2-INT-06..20 | E7P2-INT-21+ | **Not started** |

---

## 5. Preview purpose and non-goals

### 5.1 Preview MUST

1. Accept uploaded XLSX (multipart).
2. Enforce transport/file security (`xlsxsecurity` before Excelize).
3. Verify schema name/version.
4. Parse all seven allowed sheets with exact headers.
5. Validate types and required fields.
6. Normalize values (trim, bool/int/float/JSON parsing).
7. Build canonical import proposal (stable-code graph + lots).
8. Run domain validation (codes, references, cycles, question types).
9. Compute diff vs current server DRAFT (UPDATE mode).
10. Return row/field-level errors and warnings.
11. **Not apply** any change to event/version/graph/lots.
12. **Not publish** or submit carrier data.
13. **Not store binary XLSX** (DB or filesystem).

### 5.2 Preview MUST NOT

- Target PUBLISHED or SUPERSEDED versions as mutable.
- Trust workbook `tenant_id`, actor, or company for authorization.
- Use workbook UUIDs for authorization decisions.
- Write to `rfx_events`, `rfx_versions`, questionnaire tables, lots (except analysis table per chosen option), audit, or idempotency (Preview itself).
- Expose competitor/participant/bid/scoring data in response.

### 5.3 Out of scope (this discovery)

- Commit implementation
- CREATE_DRAFT FROM XLSX (deferred increment)
- Carrier XLSX, ERP, frontend, training, browser acceptance
- Migration 000074
- OpenAPI publication (design only)

---

## 6. Preview persistence decision

### 6.1 `rfx_import_analyses` design (migration 000073)

| Property | Design |
|---|---|
| Purpose | Immutable normalized preview record for later Commit |
| Binary XLSX | **Not stored** — only `canonical_payload_json` + `canonical_hash` |
| Status lifecycle | `PREVIEWED` → `CONSUMED` \| `EXPIRED` |
| Payload immutability | DB trigger blocks mutation of payload/target/tenant/actor fields |
| Single-use | Trigger + `MarkConsumed` only from `PREVIEWED` |
| TTL | `expires_at` required on insert; **no default TTL constant in Go yet** |
| Expiry enforcement | Status `EXPIRED` defined; **no repository method or worker yet** |
| Target binding | `target_type` (`NEW_EVENT` \| `DRAFT_EVENT` \| `CARRIER_RESPONSE`), optional `target_id`, `target_version` |
| Service usage today | **Zero callers** — export does not write (E7P2-INT-16) |

### 6.2 Requirement distinction

| Requirement | Meaning |
|---|---|
| `PREVIEW_NO_EVENT_WRITES` | No mutation of event graph, lots, audit, idempotency |
| `PREVIEW_NO_DB_WRITES` | No rows inserted/updated anywhere, including `rfx_import_analyses` |

These are **not equivalent**. Migration 000073 and controller decision `NORMALIZED_PREVIEW_PERSISTENCE=YES` anticipate **normalized preview persistence**.

### 6.3 Options

#### OPTION A — Persisted analysis (recommended)

Preview persists one immutable row in `rfx_import_analyses` when validation succeeds (or optionally when structurally parsed with summary — controller choice).

| Pros | Cons |
|---|---|
| Commit references `analysis_id` | Preview performs DB INSERT |
| TTL + single-use via existing schema | Requires expiry job/policy |
| Audit trail of normalized payload | Preview failure after parse needs clear atomicity |
| No re-upload for Commit | Must bind analysis to tenant/actor/event |
| Aligns with `NORMALIZED_PREVIEW_PERSISTENCE=YES` | |

**Expected counts:** `PREVIEW_ANALYSIS_WRITE=YES` (one row), `PREVIEW_EVENT_GRAPH_WRITES=NO`.

#### OPTION B — Fully stateless preview

Preview returns canonical payload + hash only; zero DB writes.

| Pros | Cons |
|---|---|
| Strictest read-only semantics | Commit must re-parse/re-upload XLSX |
| | Payload tampering risk without signed token |
| | Race between preview and commit |
| | Single-use without storage impossible |
| | **Leaves migration 000073 unused** — conflicts with `NORMALIZED_PREVIEW_PERSISTENCE=YES` |

**Expected counts:** `PREVIEW_ANALYSIS_WRITE=NO`.

#### OPTION C — Two-level preview

1. **Dry-run** — stateless validate + diff; no DB write.
2. **Prepare-for-commit** — persists analysis when `ready_to_commit=true`.

| Pros | Cons |
|---|---|
| UX: fast iterative dry-run | Two endpoints or mode flag |
| Commit path still uses persisted analysis | More API surface |
| | Higher implementation complexity |

### 6.4 Recommendation

**Recommend OPTION A** for first implementation increment, with optional future **dry-run query param** (`?persist=false`) as a non-persisting diagnostic mode (subset of OPTION C) only if controller authorizes.

Rationale:

- Controller already authorized `NORMALIZED_PREVIEW_PERSISTENCE=YES`.
- Migration 000073, repository, and immutability triggers are built for this path.
- Commit will need single-use, TTL, and tamper-evident hash — hard without persistence.
- OPTION B contradicts frozen Phase 2 design.

**Controller decision required:** confirm OPTION A (or C with explicit dry-run scope).

---

## 7. Recommended architecture

### 7.1 Initial implementation scope

| Mode | v1 Preview | v1 Commit (future) |
|---|---|---|
| **UPDATE EXISTING DRAFT** | **YES — primary** | YES |
| **CREATE NEW EVENT FROM XLSX** | **NO — defer** | Defer to increment 2 |

Rationale: CREATE requires `CreateEvent` + initial draft graph + lots in one transaction; higher risk. UPDATE reuses existing event shell, active DRAFT gate, and graph mutation patterns proven in Studio.

### 7.2 UPDATE_DRAFT preview flow

```
POST /v1/rfx-events/{id}/xlsx-import/preview
  → multipart field `file`
  → RBAC BuyerManage + tenant/company isolation
  → load event + active DRAFT version (409 if missing)
  → security pipeline → parse → validate → canonical payload
  → diff vs server DRAFT
  → if OPTION A: INSERT rfx_import_analyses (PREVIEWED, expires_at)
  → JSON response (issues + summary + optional analysis_id)
```

### 7.3 CREATE_DRAFT (deferred)

- Workbook metadata UUIDs are **informational only**.
- Server assigns tenant/company/actor on Commit only.
- `target_type=NEW_EVENT`, `target_id=NULL` in analysis row when implemented later.

### 7.4 Stale baseline handling

- Response includes `target_event_row_version`, `target_version_row_version` from **server** at preview time.
- Workbook metadata row versions compared → **warning** if mismatch (file exported from older draft).
- Commit (future) must reject if server row versions advanced (`409 stale target`).

---

## 8. Upload and security pipeline (frozen proposal)

### 8.1 Multipart contract (proposed)

| Parameter | Value | Source / rationale |
|---|---|---|
| HTTP method | `POST` | New route (not on main) |
| Multipart field name | `file` | Conventional; align with future OpenAPI |
| Max request body | **5 MiB + multipart overhead (~64 KiB)** → **5.0625 MiB gateway cap** | Match `xlsxsecurity.DefaultMaxUploadBytes` |
| Max compressed XLSX | **5 MiB** | `DefaultMaxUploadBytes` |
| Max expanded ZIP | **50 MiB** | `DefaultMaxExpandedBytes` |
| Max compression ratio | **100:1** | `DefaultMaxZipRatio` |
| Max ZIP entries | **256** | `DefaultMaxZipEntries` |
| Allowed Content-Type (file part) | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`, `application/octet-stream`, `application/zip`, empty | `xlsxsecurity` allowlist |
| Filename | Ignored for trust | Security decision |
| Binary persistence | **Forbidden** | Controller + Phase 2 doc |
| Streaming | Read full body into memory bounded by MaxBytesReader; acceptable at 5 MiB | Matches export post-check |

### 8.2 Proposed row/cell limits (new constants — not on main)

| Limit | Proposed value | Rationale |
|---|---|---|
| Max rows per data sheet | **10 000** | DoS guard beyond export sizes |
| Max total cells (all sheets) | **500 000** | Memory bound |
| Max string cell length | **8 192** | UTF-8 runes |
| Max lots | **500** | Align with enterprise tender |
| Max sections / questions / options / rules | **500 / 5 000 / 20 000 / 5 000** | DoS guard |

### 8.3 Mandatory pipeline order

1. HTTP body size limit (gateway + service `MaxBytesReader`).
2. Multipart boundary / single `file` part validation.
3. Filename ignored for trust.
4. MIME allowlist on file part.
5. Read bytes (bounded).
6. **`xlsxsecurity.InspectUpload`** — ZIP signature, traversal, ratio, expanded size, macro/OLE/external links, formula XML rejection.
7. **Excelize open** (only after step 6).
8. Sheet presence + exact header row validation (seven sheets).
9. Reject merged cells, hidden sheets (strict), formula cells (type check + XML already rejected).
10. Row parse + normalization.
11. Schema name/version from Metadata sheet.
12. Domain validation + graph integrity.
13. Canonical JSON + SHA-256 hash.
14. Optional: persist analysis (OPTION A).
15. Build response.

**Excelize MUST NOT run before `xlsxsecurity.InspectUpload`.**

---

## 9. Workbook trust model (frozen proposal)

| Rule | Policy |
|---|---|
| Tenant authority | Verified JWT / gateway headers only |
| Company authority | Server-side membership check |
| Event ID authority | URL path `{id}` — not workbook metadata |
| Draft version authority | Server `GetActiveDraftVersion` |
| Workbook UUID fields | Informational / mismatch warnings only |
| Unknown sheets | **ERROR** (`unexpected_sheet`) |
| Unknown columns in known sheet | **ERROR** (`invalid_header`) |
| Missing required columns | **ERROR** |
| Missing optional columns | Allowed if header set matches export template |
| Duplicate stable codes | **ERROR** |
| Dangling cross-sheet references | **ERROR** |
| Cyclic rules | **ERROR** |
| Self-target rule | **ERROR** |
| Unsupported question type | **ERROR** |
| Invalid JSON in condition/validation | **ERROR** |
| Formula cells | **ERROR** (XML blocked earlier; cell value `=` prefix also rejected) |
| Empty data rows | Skip if all cells empty (deterministic) |
| Merged cells | **ERROR** |
| Hidden sheets | **ERROR** (strict) |
| Hidden rows/columns | **WARNING** first implementation; upgrade to ERROR if abused |
| External links / names | **ERROR** (ZIP inspect) |
| I18N columns | `_ru` authoritative; `_en/_zh` mismatch → **WARNING** (not translation) |
| Competitor extra columns | **ERROR** (not silent ignore) |
| Date/locale numeric ambiguity | Parse decimals with invariant format; reject locale-specific dates in v1 |

---

## 10. Preview response contract (draft — no OpenAPI change yet)

### 10.1 Top-level fields

```json
{
  "schema_name": "BINTRANS_RFX_BUYER_XLSX_V1",
  "schema_version": "1",
  "mode": "UPDATE_DRAFT",
  "target_event_id": "uuid",
  "target_draft_version_id": "uuid",
  "target_event_row_version": 3,
  "target_version_row_version": 7,
  "canonical_payload_hash": "sha256-hex",
  "ready_to_commit": false,
  "analysis_id": "uuid-or-null",
  "summary": {
    "errors": 2,
    "warnings": 1,
    "sections_added": 0,
    "sections_changed": 1,
    "questions_added": 0,
    "lots_added": 0,
    "lots_changed": 0
  },
  "errors": [],
  "warnings": []
}
```

- `analysis_id`: present only when OPTION A persist succeeds and `ready_to_commit=true` (controller may choose persist-on-any-parse — **decision required**).
- Never return full workbook, stack traces, SQL, or internal paths.

### 10.2 Issue object

| Field | Type | Notes |
|---|---|---|
| `severity` | `error` \| `warning` | |
| `machine_code` | string | Stable enum (§11) |
| `sheet` | string | e.g. `Questions` |
| `row` | int | 1-based; 0 for sheet-level |
| `column` | string | Header name or empty |
| `stable_code` | string | e.g. `Q_LOGISTICS` |
| `message_key` | string | i18n key for UI |
| `params` | object | Safe string/number params only |

### 10.3 HTTP validation semantics (recommended)

| Condition | HTTP | `ready_to_commit` |
|---|---|---|
| Malformed multipart / oversize / unsafe ZIP | 400 / 413 | n/a |
| Unauthenticated | 401 | n/a |
| Wrong role (BuyerRead, carrier) | 403 | n/a |
| Feature disabled / cross-tenant event | 404 | n/a |
| No active DRAFT / stale target (pre-check) | 409 | n/a |
| Structurally parsed workbook with domain errors | **422** | `false` |
| Fully valid workbook | **200** | `true` |

**Recommendation:** use **422** (not 200) when the workbook is readable but invalid — consistent with rfx-service validation patterns and prevents clients treating HTTP success as commit-ready.

Dry-run diagnostic mode (if authorized later) may return 200 with `ready_to_commit=false` for UX; default production preview uses 422.

---

## 11. Error code freeze (proposed)

| machine_code | HTTP | Description |
|---|---|---|
| `invalid_multipart` | 400 | Missing `file`, multiple files, bad boundary |
| `file_too_large` | 413 | Compressed upload exceeds limit |
| `invalid_xlsx_signature` | 400 | Not a ZIP/XLSX |
| `unsafe_package` | 400 | Macro, OLE, external link, traversal, ratio |
| `unsupported_schema` | 400 | schema_name/version mismatch |
| `missing_sheet` | 422 | Required sheet absent |
| `unexpected_sheet` | 422 | Extra sheet present |
| `invalid_header` | 422 | Column header mismatch |
| `missing_required_value` | 422 | Required cell empty |
| `invalid_type` | 422 | Bool/int/float/JSON parse failure |
| `duplicate_stable_code` | 422 | lot/section/question/option/rule code collision |
| `dangling_reference` | 422 | FK by stable code missing |
| `cyclic_rule` | 422 | Rule graph cycle |
| `self_target_rule` | 422 | Rule targets itself |
| `formula_denied` | 422 | Formula-like cell content |
| `merged_cell_denied` | 422 | Merged cells detected |
| `hidden_sheet_denied` | 422 | Hidden sheet |
| `competitor_column_denied` | 422 | Unknown/forbidden column with sensitive name pattern |
| `active_draft_missing` | 409 | No mutable DRAFT |
| `forbidden` | 403 | Insufficient role |
| `cross_tenant_not_found` | 404 | Fail closed |
| `feature_disabled` | 404 | Flag off |
| `stale_target` | 409 | Row version baseline stale (commit phase; preview warning) |
| `too_many_rows` | 422 | Sheet row cap |
| `too_many_cells` | 422 | Total cell cap |
| `preview_not_ready` | 422 | Generic aggregate when errors present |
| `internal_failure` | 500 | Unexpected |

---

## 12. Canonical diff and hash

### 12.1 Workbook canonical hash

Reuse/adapt `CanonicalWorkbookSnapshot` (`canonical.go`):

- Parse seven sheets to `map[string][][]string`.
- Exclude Metadata row `exported_at_utc`.
- Normalize row widths.
- SHA-256 over canonical JSON representation.

Used for: repeated preview determinism tests; optional dedup (not required v1).

### 12.2 Import proposal canonical payload

Separate structure for `canonical_payload_json` in analysis row:

- Normalized lots array (by `lot_number`).
- Normalized sections/questions/options/rules (stable codes only — **no DB UUIDs**).
- Server target binding: `event_id`, `draft_version_id`, row versions, mode.
- Excludes competitor data.

Hash: SHA-256 hex of canonical JSON (sorted keys) — matches `canonical_hash` column constraint.

### 12.3 Diff model

| Domain | Reuse E2 compare? | Approach |
|---|---|---|
| Sections/Questions/Options/Rules | **Partial** | Map workbook proposal → `VersionCompareSnapshot`; call `CompareVersionSnapshots(serverDraft, proposal)` |
| Lots | **No** | **Separate Lots diff** — E2 compare has no lot support |
| Scoring | N/A | Not in workbook |

Diff classes (align with E2 where possible): `added`, `removed`, `changed`, `reordered`.

I18N triplication: compare logical field from `_ru` only; ignore `_en/_zh` unless mismatch → warning.

---

## 13. RBAC, isolation, confidentiality

| Control | Policy |
|---|---|
| Required role | **BuyerManage** (`PolicyBuyerManage`) |
| BuyerRead | **403** (mirror E7P2-INT-20) |
| Carrier roles | **403** |
| Tenant | From verified JWT; cross-tenant → **404** |
| Company | Buyer company membership; cross-company → **404** |
| Identity spoof | Gateway strips spoofed headers (existing test pattern) |
| Feature flag | `RFX_EXCEL_EXCHANGE_ENABLED` default **false** → **404**, no writes |
| Response leakage | No participants, bids, carrier rates, submission times |
| Upload content | Reject competitor-named columns/sheets explicitly |

---

## 14. Idempotency

| Operation | Idempotency-Key | Rationale |
|---|---|---|
| Preview (OPTION A) | **Optional** — not required v1 | Each upload creates new analysis; safe to retry with new key |
| Preview dry-run | Not required | Read-only |
| Commit (future) | **Required** | Mutating; reuse `rfx_idempotency_records` pattern from `VersionLifecycleService` |
| Analysis reuse | Bind `analysis_id` to tenant + actor + target event + hash | Prevent cross-event replay |
| Single-use | **`MarkConsumed` on Commit only** | Preview read of analysis allowed until expiry |
| Expired analysis | Commit rejects with `409` | Requires expiry worker or lazy check on `expires_at` |
| Body hash scope (Commit) | Include `analysis_id` + expected row versions | Not XLSX re-upload when OPTION A |

---

## 15. No-write matrix

### 15.1 Always unchanged by Preview

| Store | Preview impact |
|---|---|
| `rfx_events` | NO WRITE |
| `rfx_versions` | NO WRITE |
| `rfx_lots` | NO WRITE |
| sections / questions / options / rules | NO WRITE |
| scoring / participants / responses / offers / awards | NO WRITE |
| audit events | NO WRITE |
| `rfx_idempotency_records` | NO WRITE (Preview v1) |

### 15.2 Analysis table by option

| Option | `rfx_import_analyses` |
|---|---|
| OPTION A (recommended) | **Exactly 1 INSERT** when persist policy satisfied |
| OPTION B | **0 writes** |
| OPTION C dry-run | **0 writes** |
| OPTION C prepare | **1 INSERT** |

### 15.3 Explicit markers (recommended OPTION A)

```
PREVIEW_EVENT_GRAPH_WRITES=NO
PREVIEW_DATABASE_WRITES=YES
PREVIEW_ANALYSIS_WRITE=YES
RFX_IMPORT_ANALYSES_USAGE=PREVIEW_PERSISTENCE_FOR_COMMIT
```

If controller chooses OPTION B:

```
PREVIEW_EVENT_GRAPH_WRITES=NO
PREVIEW_DATABASE_WRITES=NO
PREVIEW_ANALYSIS_WRITE=NO
RFX_IMPORT_ANALYSES_USAGE=DEFERRED_UNUSED
```

---

## 16. Test strategy (draft — not implemented)

### 16.1 Unit tests

- Security pipeline order enforcement
- Header parsing per sheet
- Row normalization and empty-row skip
- Stable-code FK validation
- Rule cycle / self-target detection
- Formula / merged / hidden detection
- Canonical JSON + hash determinism
- Issue sort order (sheet, row, column, code)

### 16.2 Integration tests (continue from E7P2-INT-20)

| ID | Scope |
|---|---|
| E7P2-INT-21 | Rich workbook preview success (UPDATE_DRAFT) |
| E7P2-INT-22 | No event/graph/lot writes |
| E7P2-INT-23 | Analysis row persisted (OPTION A) / absent (OPTION B) |
| E7P2-INT-24 | Schema mismatch |
| E7P2-INT-25 | Missing / unexpected sheet |
| E7P2-INT-26 | Invalid headers |
| E7P2-INT-27 | Duplicate stable codes |
| E7P2-INT-28 | Dangling references |
| E7P2-INT-29 | Rule cycle / self-target |
| E7P2-INT-30 | Malformed JSON cells |
| E7P2-INT-31 | Formula / macro / OLE / external link rejection |
| E7P2-INT-32 | Oversized upload |
| E7P2-INT-33 | BuyerRead 403 |
| E7P2-INT-34 | Carrier 403 |
| E7P2-INT-35 | Cross-tenant 404 |
| E7P2-INT-36 | Cross-company fail closed |
| E7P2-INT-37 | Feature disabled 404 + no writes |
| E7P2-INT-38 | Gateway identity spoof |
| E7P2-INT-39 | Stale metadata row version warning |
| E7P2-INT-40 | Deterministic repeated preview |
| E7P2-INT-41 | Competitor column rejected |
| E7P2-INT-42 | Route/gateway/OpenAPI parity |

---

## 17. Implementation increments (proposal)

| Increment | Scope | Depends on |
|---|---|---|
| **P1** | Controller decision on persistence option + UPDATE-only scope | This discovery |
| **P2** | Parser + validator library (no HTTP) | P1 |
| **P3** | Preview HTTP handler + multipart + OPTION A persist | P2 |
| **P4** | OpenAPI + gateway route + E7P2-INT-21..42 | P3 |
| **P5** | Commit increment (separate contract) | P4 |
| **P6** | CREATE_DRAFT FROM XLSX (optional) | P5 |

---

## 18. Controller decisions required

| ID | Decision | Discovery recommendation |
|---|---|---|
| CD-01 | Preview persistence option (A / B / C) | **OPTION A** |
| CD-02 | Persist analysis only when `ready_to_commit=true` vs always on parse | **Only when ready_to_commit=true** |
| CD-03 | Initial mode: UPDATE_DRAFT only vs include CREATE | **UPDATE_DRAFT only** |
| CD-04 | HTTP semantics for validation failures | **422** with structured issues |
| CD-05 | Default analysis TTL | **24 hours** (proposed — not in code) |
| CD-06 | Expiry enforcement mechanism | Lazy check on read + future worker |
| CD-07 | I18N triplicate mismatch | **WARNING** (not error) |
| CD-08 | Hidden rows/columns | **WARNING** v1 |
| CD-09 | Dry-run param without persist | Defer unless UX requires |
| CD-10 | Multipart field name `file` | **Approve** |
| CD-11 | Row/cell limits (§8.2) | **Approve or adjust** |
| CD-12 | Proceed to P2 implementation | Pending CD-01..11 |

---

## 19. Scope guards

| Guard | Discovery phase |
|---|---|
| Product code changed | **NO** |
| Tests changed | **NO** |
| OpenAPI changed | **NO** |
| CI changed | **NO** |
| Dependencies changed | **NO** |
| Migration 000074 | **NO** |
| Push / PR / merge | **NO** |
| Buyer Import implementation | **NO** |
| Staging / pilot | **NO** |

---

## 20. Summary markers

```
DISCOVERY_COMPLETED=YES
EXPORT_SCHEMA_INVENTORIED=YES
SEVEN_SHEETS_INVENTORIED=YES
IMPORT_PREVIEW_IMPLEMENTATION_STARTED=NO

PREVIEW_PERSISTENCE_OPTIONS_ANALYZED=YES
RECOMMENDED_OPTION=A
PREVIEW_EVENT_GRAPH_WRITES=NO
PREVIEW_DATABASE_WRITES=YES
PREVIEW_ANALYSIS_WRITE=YES
RFX_IMPORT_ANALYSES_USAGE=PREVIEW_PERSISTENCE_FOR_COMMIT
CONTROLLER_DECISION_REQUIRED=YES

RECOMMENDED_INITIAL_MODE=UPDATE_DRAFT
CREATE_DRAFT_PREVIEW=DEFERRED
UPDATE_DRAFT_PREVIEW=YES

SECURITY_PIPELINE_FROZEN=YES
TRUST_MODEL_FROZEN=YES
UPLOAD_LIMITS_FROZEN=YES
FORMULA_POLICY_FROZEN=YES
HIDDEN_CONTENT_POLICY_FROZEN=YES

PREVIEW_RESPONSE_DRAFTED=YES
ERROR_CODES_DRAFTED=YES
HTTP_VALIDATION_SEMANTICS=422_ON_DOMAIN_ERRORS
CANONICAL_DIFF_REUSE=E2_PARTIAL_PLUS_LOTS_DIFF
LOTS_DIFF_MODEL=SEPARATE_STABLE_CODE_DIFF

BUYER_MANAGE_REQUIRED=YES
BUYER_READ_DENIED=YES
CARRIER_DENIED=YES
TENANT_ISOLATION=YES
COMPANY_ISOLATION=YES
IDENTITY_SPOOF_DENIED=YES
COMPETITOR_COLUMNS_REJECTED=YES
FEATURE_FLAG_DEFAULT_OFF=YES

TEST_MATRIX_DRAFTED=YES
NEXT_TEST_ID=E7P2-INT-21

CONTROLLER_VERDICT=APPROVE_ARCHITECTURE
NEXT_ACTION=CONTROLLER_REVIEW_BUYER_XLSX_IMPORT_P2
```

---

## 21. Controller architecture approval (P2)

| Item | Decision |
|---|---|
| `ARCHITECTURE_REVIEW_HEAD` | `5d51af525cd9ee3f55fb8c58ab5f11be584ade5e` |
| `CONTROLLER_VERDICT` | `APPROVE_ARCHITECTURE` |
| CD-01 | `OPTION_A` — analysis persistence deferred to P3 |
| CD-02 | Analysis insert **only** when `ready_to_commit=true` && `errors==0` (P3 service) |
| CD-03 | `UPDATE_EXISTING_DRAFT_ONLY` |
| CD-04 | Future HTTP **422** structured preview body (direct JSON, not `respond.Error`) |
| CD-05 | TTL **24h** (P3) |
| CD-06 | Lazy expiry check on read |
| CD-07 | I18N mismatch → **WARNING** |
| CD-08 | Hidden rows/columns → **WARNING** v1 |
| CD-09 | Stateless dry-run **DEFERRED** |
| CD-10 | Multipart field `file` (P3) |
| CD-11 | Limits from §8.2 |
| CD-12 | P2 implementation **AUTHORIZED** |

### P2 clarifications (frozen)

- **Analysis guard (CD-02):** P2 parser never writes `rfx_import_analyses`. Analysis row creation is P3-only when `ReadyToCommit=true`.
- **Structured 422 (CD-04):** P2 returns `BuyerImportPreview` domain DTO only. HTTP status/body mapping is P3; future handler emits structured JSON 422 directly.
- **`target_version` semantics:** DB `target_version` = draft `version_number`. Optimistic `event_row_version` and `draft_row_version` are separate fields in canonical payload — never mixed with business version number.
- **Issue ordering:** ERROR before WARNING → workbook sheet order → row → column → `machine_code` → `stable_code`. Package-level issues (empty `sheet`) sort before sheet-scoped issues.

### P2 controller review (accepted)

| Marker | Value |
|---|---|
| `CONTROLLER_VERDICT` | `ACCEPT_P2` |
| `NON_BLOCKING_FINDINGS` | `M-P2-01..06` (P2.1 hardening scope) |
| `M-P2-06` | Early stop after structural errors — **documented fail-fast gate**, not a defect |

### P2.1 hardening (implemented)

| Marker | Value |
|---|---|
| `P2_1_HARDENING` | `IMPLEMENTED_PENDING_CONTROLLER_REVIEW` |
| `MAX_PREVIEW_ISSUES` | `2000` |
| `TRUNCATION_ISSUE_CODE` | `ISSUE_LIMIT_REACHED` |
| `TRUNCATION_SEMANTICS` | Up to 1999 regular issues; slot 2000 is deterministic synthetic ERROR with `limit=2000` |
| `PRODUCTION_LIMITS` | Fixed — caller cannot override security/import limits or `OpenWorkbook` |
| `OPEN_WORKBOOK_HOOK` | Package-internal only (`parseBuyerImportPreview` + tests) |
| `CONTEXT_CANCELLATION` | Checked before inspection, after open, per sheet, every 128 rows / 256 cells, before diff/hash |
| `EXPORT_IMPORT_ROUNDTRIP` | Full semantic Export V1 → Preview parser proof (lots/sections/questions/options/rules/validation/sort) |
| `MULTI_NODE_CYCLE` | A→B→C→A deterministic `cyclic_rule`, no hash, no panic |
| `HASH_SENSITIVITY` | Table-driven commit-relevant field matrix + JSON key-order independence |
| `NAMED_RANGES_POLICY` | Fail closed at ZIP inspection: workbook `definedName` with formula (`=…`) or external book brackets (`[Book]…`) rejected |
| `PIVOT_DRAWING_COMMENT_POLICY` | Not consumed by parser; export generator does not emit pivot/drawing/comment parts; external relationships remain forbidden |

Production limits (immutable via public API):

| Limit | Value |
|---|---|
| Compressed upload | 5 MiB |
| Expanded ZIP | 50 MiB |
| Compression ratio | 100:1 |
| ZIP entries | 256 |
| Sheets | 7 (fixed order) |
| Rows / sheet | 10 000 |
| Total cells | 500 000 |
| String / JSON field | 8 192 |
| Preview issues | 2 000 |

### P2 implementation status

| Marker | Value |
|---|---|
| `STATUS` | `IMPLEMENTATION_IN_PROGRESS` |
| `P2_PARSER_VALIDATOR` | `IMPLEMENTED_ACCEPTED` |
| `P2_1_HARDENING` | `IMPLEMENTED_PENDING_CONTROLLER_REVIEW` |
| `P2_DATABASE_WRITES` | `NO` |
| `P2_EVENT_GRAPH_WRITES` | `NO` |
| `P2_ANALYSIS_WRITES` | `NO` |
| `P2_FILESYSTEM_WRITES` | `NO` |
| `SECURITY_BEFORE_EXCELIZE` | `YES` |
| `READY_TO_COMMIT_RULE` | `ERRORS_ZERO` |
| Parser package | `services/rfx-service/internal/xlsxexchange/buyer_import_*.go` |
| Unit tests | `buyer_import_parser_test.go`, `buyer_import_p21_test.go` + export/security tests |

### P2 / P2.1 parser test evidence (local)

```
go test ./internal/xlsxexchange/...  → PASS
go test ./internal/xlsxsecurity/...  → PASS
go vet ./internal/xlsxexchange/... → PASS
go build ./...                       → PASS
go test -race ./internal/xlsxexchange/... → NOT_RUN (CGO_ENABLED=0)
```

Coverage highlights: deterministic issue cap (`ISSUE_LIMIT_REACHED`), fixed production limits, internal-only workbook opener, context cancellation in long loops, full Export→Parse semantic round-trip, multi-node rule cycle, hash sensitivity matrix, security-before-Excelize, seven-sheet contract, metadata trust, I18N mismatch warning, E2 questionnaire diff reuse, separate lots diff, canonical hash determinism, forbidden-import source scan, external defined-name rejection.

### P3+ remains not started

| Item | Status |
|---|---|
| HTTP handler / multipart | `NOT_STARTED` |
| Repository persistence | `NOT_STARTED` |
| OpenAPI / gateway | `NOT_STARTED` |
| Commit / CREATE FROM XLSX | `NOT_STARTED` |
