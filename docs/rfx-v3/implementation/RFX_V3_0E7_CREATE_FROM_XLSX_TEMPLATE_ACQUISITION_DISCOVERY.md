# RFx v3.0E7 F5 XLSX Template Acquisition Discovery

Discovery-only architecture decision for how a buyer obtains a CREATE-compatible XLSX on `/tenders/new-from-xlsx`.

```
DISCOVERY_ONLY=YES
IMPLEMENTATION_AUTHORIZED=NO
PRODUCT_CODE_CHANGES=NO
MIGRATIONS_AUTHORIZED=NO
F5_TEMPLATE_ACQUISITION_STATUS=DISCOVERY_ACCEPTED
CONTROLLER_VERDICT=ACCEPT_F5_TEMPLATE_ACQUISITION_SCOPE
F5_TEMPLATE_ACQUISITION_IMPLEMENTATION_AUTHORIZED=NO
F5_OVERALL_STATUS=IMPLEMENTATION_IN_PROGRESS
FRONTEND_PHASE2_STATUS=IMPLEMENTATION_IN_PROGRESS
TRAINING_STATUS=NOT_STARTED
ERP_BROWSER_CLIENT_STATUS=NOT_STARTED
TMS_STATUS=OUT_OF_SCOPE
AWARD_TO_TRANSPORT_ORDER_IN_E7_BROWSER_GATE=NO
RECOMMENDED_VARIANT=A_BACKEND_GENERATED_BLANK_WORKBOOK
F5_TA_1=FIXED_ACCEPTED
F5_TA_2=NOTE_CLOSED
ACCEPTED_DISCOVERY_HEAD=c0e39487482b9ffeca3706c089b15adc762007c6
ACCEPTED_CI_RUN=35745948306
ACCEPTED_CI_ATTEMPT=2
OPENAPI_SOURCE_OF_TRUTH=scripts/openapi/generate_openapi.py
ACCEPTED_ENDPOINT=GET /api/v1/rfx-events/xlsx-create/template
NEXT_ACTION=AUTHORIZE_F5_TEMPLATE_ACQUISITION_W1
```

Controller verdict `ACCEPT_F5_TEMPLATE_ACQUISITION_SCOPE` accepts **Variant A** only: read-only `GET /api/v1/rfx-events/xlsx-create/template`, blank BUYER XLSX V1, OpenAPI source-of-truth `scripts/openapi/generate_openapi.py`. This document does **not** authorize W1–W4, product code, OpenAPI/generator edits, migrations, frontend product changes, training, ERP browser, TMS, or Award→TO. F5 overall and Frontend Phase 2 remain `IMPLEMENTATION_IN_PROGRESS`.

---

## 1. Recovery identity

| Item | Value |
| --- | --- |
| Worktree | `D:\Projects\freight-platform-wt\rfx-f5-template-acquisition-discovery-v3.0e7` |
| Branch | `discovery/rfx-f5-template-acquisition-v3.0e7` |
| START_HEAD / expected base | `717f2d54f9d8987d3a62f56540752fe1e3aba9d4` |
| `origin/main` at discovery start | `717f2d54f9d8987d3a62f56540752fe1e3aba9d4` |
| merge-base `HEAD` `origin/main` | `717f2d54f9d8987d3a62f56540752fe1e3aba9d4` |
| Working tree at start | clean; no merge / rebase / cherry-pick / revert |
| Neighbor worktrees | not modified (accepted F5 frontend, old F5 discovery, F5 backend, E7, stashes) |

Identity check commands (discovery start):

```powershell
git rev-parse --show-toplevel
git branch --show-current
git rev-parse HEAD
git rev-parse origin/main
git merge-base HEAD origin/main
git status --short
git worktree list
```

Result: identity match. Discovery proceeded.

---

## 2. Current-state inventory

### 2.1 What exists today

| Surface | Status | Evidence |
| --- | --- | --- |
| F5 CREATE preview/commit backend | `IMPLEMENTED_ACCEPTED` | `docs/rfx-v3/implementation/RFX_V3_0E7_CREATE_FROM_XLSX_BACKEND_W1.md` |
| F5 CREATE frontend upload-only | `IMPLEMENTED_ACCEPTED` | `docs/rfx-v3/implementation/RFX_V3_0E7_CREATE_FROM_XLSX_FRONTEND_UPLOAD_ONLY.md` |
| F1 buyer UPDATE export/import | `IMPLEMENTED_ACCEPTED` | `GET /api/v1/rfx-events/{id}/xlsx-export`; `/tenders/:id` panel |
| F2 carrier XLSX | `IMPLEMENTED_ACCEPTED` | different schema `BINTRANS_RFX_CARRIER_XLSX_V1` |
| Blank CREATE template download | **does not exist** | no route, no static `.xlsx` asset, no UI button |
| Official way to obtain a CREATE-compatible workbook without an existing DRAFT | **none** | see §4 |

### 2.2 Frozen F5 CREATE routes (do not change in this discovery)

| Method | Public path | Downstream | operationId | Policy |
| --- | --- | --- | --- | --- |
| POST | `/api/v1/rfx-events/xlsx-create/preview` | `/v1/rfx-events/xlsx-create/preview` | `post_preview_buyer_new_rfx_event_xlsx_create` | `PolicyBuyerManage` |
| POST | `/api/v1/rfx-events/xlsx-create/commit` | `/v1/rfx-events/xlsx-create/commit` | `post_commit_buyer_new_rfx_event_xlsx_create` | `PolicyBuyerManage` |

Registry: `packages/shared-go/rfx/e7_excel_exchange_routes.go` lines 101–126.  
Service Chi: `services/rfx-service/internal/http/router.go` lines 70–73 (static `/xlsx-create/*` **before** `/{id}`).  
Gateway: `services/api-gateway/internal/http/router.go` lines 342–351, flag middleware 502–511.

### 2.3 F1 buyer UPDATE export (cannot be the CREATE acquisition path)

| Item | Fact | Evidence |
| --- | --- | --- |
| Endpoint | `GET /api/v1/rfx-events/{id}/xlsx-export` | `e7_excel_exchange_routes.go` 23–35 |
| Requires existing event | yes; unknown event → 404 | `TestE7P2INT13UnknownEvent404` |
| Requires active DRAFT questionnaire | yes; no draft → **409** | `GetActiveDraftVersion` `questionnaire_repository.go` 40–63; `TestE7P2INT07NoActiveDraft409` |
| Event-specific identity in workbook | yes: `tenant_id`, `rfx_event_id`, `rfx_version_id`, version/row versions, `creation_channel`, template source | `buyer_workbook.go` 168–183; `excel_exchange_service.go` 213–226; `TestE7P2INT06` metadata assert 309–317 |
| Filename | `bintrans-rfx-{eventID}-draft-v{version}.xlsx` | `excel_exchange_service.go` 153 |
| Content-Type | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` | `BuyerDraftXLSXContentType` |
| Content-Disposition / cache | `attachment; filename="..."`; `Cache-Control: no-store`; `X-Content-Type-Options: nosniff` | `respond/binary.go` 8–12 |
| UI | `/tenders/:id` only; `data-testid="buyer-xlsx-export"` | `BuyerXlsxExchangePanel.vue` 198–206 |
| Visible when | Excel flag + BuyerManage + event `DRAFT` | `buyerXlsxAccess.ts` 12–20 |

Creating a temporary RFx solely to download this export is **rejected** as an acquisition strategy.

### 2.4 F5 frontend upload-only (no download)

Page `/tenders/new-from-xlsx` (`new-from-xlsx.vue`) hosts `BuyerXlsxCreatePanel.vue`.

Present:

- List entry `data-testid="buyer-xlsx-create-entry"` on `/tenders` (`index.vue` 131–138)
- Multipart upload, preview, commit
- Metadata fields (owner company, number, title, type, category, description, deadline, currency)
- Flag/role gate `data-testid="buyer-xlsx-create-gate"` / `buyer-xlsx-create-unavailable`

Absent:

- No download/template button
- No `xlsx-create/template` client path (`buyerXlsxCreateApiRoutes.ts` has preview/commit only)
- Browser live specs upload `BROWSER_E2E_WORKBOOK_PATH`, they do not download a template (`buyer-xlsx-create-live.spec.ts` 44–47; `helpers.ts` 17)

### 2.5 Feature flag, RBAC, rate limit

| Control | Behavior | Evidence |
| --- | --- | --- |
| `RFX_EXCEL_EXCHANGE_ENABLED` default `false` | flag-off → **404** | gateway `config.go` 213; `excelExchangeFlagMiddleware` 502–511 |
| Frontend `NUXT_PUBLIC_RFX_EXCEL_EXCHANGE_ENABLED` | hides entry/panel | `useRfxExcelExchangeFeature.ts` |
| Human JWT | default class for non-integration routes | `e7_route_classifier.go` `RequiresHumanAuth` 86–93 |
| BuyerManage | `PLATFORM_ADMIN`, `PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, `FORWARDER_MANAGER` | gateway `policies.go` 5–10; domain `HasBuyerManageRole` `authorization.go` 50–57; frontend `BUYER_XLSX_MANAGE_ROLES` |
| Denied | `SHIPPER_LOGIST`, carrier roles, no BuyerManage | `HasBuyerRole` includes logist; `HasBuyerManageRole` does not |
| Shared gateway rate limit | IP token bucket, default 50 RPS / burst 100; **429** | `ratelimit.go`; `config.go` 126–147 |
| `tenant_id` query | rejected **403** via `requireActor` | `handlers/context.go` 29–54 |

### 2.6 CREATE multipart vs workbook contents

Event identity for CREATE is **not** taken from the workbook. It is multipart:

Required: `file`, `owner_company_id`, `rfx_number`, `title`, `rfx_type`, `category`.  
Optional: `description`, `response_deadline`, `currency_code`.  
Forbidden: `tenant_id`, `auto_publish`, `publish`, `participants`, `erp`, hash fields (`buyerXlsxCreateApiRoutes.ts` 6–31; `buyer_xlsx_create_multipart.go` 19–38).

Workbook IDs are not authority (generated `packages/openapi/rfx-service.yaml` currently documents this at line 5194; runtime: `createUntrustedMetadataKeys` in `buyer_create_parser.go` 41–53). That YAML file is a generated artifact, not an editable OpenAPI source — see §7.7.

### 2.7 Tests and docs already on `main`

- Unit: `buyer_create_parser_test.go`, `buyer_workbook_test.go`, `formula_injection_test.go`, frontend `buyerXlsxCreate.test.ts`
- Integration: `buyer_xlsx_create_integration_test.go`, `buyer_xlsx_export_integration_test.go`
- Browser gate: `rfx-buyer-xlsx-create-browser-e2e` (`.github/workflows/ci.yml` 1098)
- Docs: F5 Backend W1, F5 Frontend Upload-Only, F1 preview/commit, roadmap markers `F5_TEMPLATE_ACQUISITION_DECISION=A_UPLOAD_ONLY_FIRST_WAVE` and `F5_TEMPLATE_ACQUISITION_DECISION_REQUIRED=YES`

No product tests or docs currently describe an official CREATE template download.

---

## 3. Actual workbook contract (shared BUYER XLSX V1)

Single schema name for buyer workbooks:

| Marker | Value | Source |
| --- | --- | --- |
| `schema_name` | `BINTRANS_RFX_BUYER_XLSX_V1` | `domain.SchemaVersionBuyerXLSXV1` |
| `schema_version` | `1` | `schemaVersionNumber` in `snapshot.go` 12 |

Mode is **not** a workbook cell. It is assigned by the parser entry point:

- `ParseBuyerImportPreview` → `UPDATE_DRAFT` (`buyer_import_parser.go` 106)
- `ParseBuyerCreatePreview` → `CREATE_NEW_DRAFT` (`buyer_create_parser.go` 69)

The same binary can therefore be parsed as UPDATE or CREATE. `TestParseBuyerCreatePreviewDoesNotCallUpdateEntryPoint` (`buyer_create_parser_test.go` 77–92) asserts the two entry points do not share mode.

### 3.1 Sheets (order is contract)

`buyer_workbook.go` 21–39; `validateWorkbookStructure` `buyer_import_parser.go` 167–205:

1. `Instructions`
2. `Metadata`
3. `Lots`
4. `Sections`
5. `Questions`
6. `Options`
7. `Rules`

Missing sheet → `missing_sheet`. Extra sheet → `unexpected_sheet`. Wrong order → `sheet_order_mismatch`. Hidden / very-hidden → `hidden_sheet_denied`. Merged cells → `merged_cell_denied`.

### 3.2 Headers (stable English machine names; not localized)

| Sheet | Headers |
| --- | --- |
| Lots | `lot_number`, `name`, `description`, `category`, `estimated_value`, `currency_code`, `status` |
| Sections | `section_code`, `title_ru`, `title_en`, `title_zh`, `description_ru`, `description_en`, `description_zh`, `sort_order` |
| Questions | `section_code`, `question_code`, `question_type`, `title_ru`, `title_en`, `title_zh`, `description_ru`, `description_en`, `description_zh`, `required`, `sort_order`, `validation_json` |
| Options | `question_code`, `option_code`, `label_ru`, `label_en`, `label_zh`, `sort_order` |
| Rules | `rule_code`, `source_question_code`, `condition`, `target_question_code`, `action`, `sort_order` |

Source: `buyer_import_types.go` 70–80; writer `buyer_workbook.go` 197–333.  
Instruction rows are RU/EN/ZH prose; headers stay English.

### 3.3 Required vs optional cells

- Header-only Lots / empty questionnaire is **CREATE-valid** (`TestParseBuyerCreatePreviewHeaderOnlyLotsReady`, `TestParseBuyerCreatePreviewEmptyQuestionnaireReady`).
- If a Lots data row exists: `lot_number` and `name` required (`buyer_import_parser.go` 672–697). Other lot fields optional.
- Questionnaire rows require their stable codes when present; empty sheets are allowed for CREATE.
- Metadata required keys for both parsers: `schema_name`, `schema_version`.

### 3.4 Metadata identity

Allowed keys (`buyer_import_types.go` 90–96): `schema_name`, `schema_version`, `exported_at_utc`, `tenant_id`, `rfx_event_id`, `rfx_version_id`, `version_number`, `version_status`, `event_row_version`, `version_row_version`, `creation_channel`, `source_template_version_id`, `source_template_version_number`.

F1 generator writes **all** of them (`writeMetadataSheet`).

CREATE parser (`parseCreateMetadataSheet`):

- Requires matching schema name/version
- **Does not reject** identity keys
- Emits `metadata_mismatch` **warnings** and ignores them (`buyer_create_parser.go` 233–243)
- Does not bind `rfx_event_id` into the new event

UPDATE parser additionally warns on event/version ID mismatch vs the target DRAFT (`buyer_import_parser.go` 579–598) and errors if `version_status` is present and not `DRAFT`.

### 3.5 Canonical hash

CREATE hash is of the **analysis payload** (multipart event shell + parsed graph + `mode=CREATE_NEW_DRAFT`), not of workbook identity (`buyer_create_hash.go` 64–118; `TestCanonicalCreatePayloadOmitsTrustedWorkbookIDs`; `TestCanonicalCreatePayloadHashIgnoresWorkbookIdentity`).

Download of a blank template must not persist a hash or analysis.

### 3.6 Formula-safety

- Parser: any non-empty Excel formula → `formula_denied` structural 400 (`buyer_import_parser.go` 470–486; `TestParseBuyerCreatePreviewFormulaDenied`).
- Generator: formula-like text (`=`, `+`, `-`, `@`) is stored as text + text number format (`buyer_workbook.go` 140–148, 390–401; `formula_injection_test.go`).
- ZIP/XLSX inspect via `xlsxsecurity.InspectUpload`. Unsafe package → `unsafe_package`.
- Competitor columns (`carrier_id`, `offer_rate`, …) denied.

### 3.7 Localization

Sheet names and headers are **stable English**. Cell values may be RU/EN/ZH. Instructions are trilingual. UI chrome is localized separately (`tenders.json` RU/EN/ZH).

### 3.8 Backward compatibility

Old F1 exports remain CREATE-parseable (warnings only). Introducing a **required** new metadata key (for example `mode`) would break existing compatible files and the accepted upload-only wave. Do not add a required envelope field in the template-acquisition implementation.

---

## 4. Gap analysis

Proven answers to the discovery questions:

| Question | Proven answer |
| --- | --- |
| Where does the user get the source file today? | There is **no** official CREATE source. Browser tests inject a fixture via `BROWSER_E2E_WORKBOOK_PATH`. A practitioner can only (a) hand-build a V1 workbook, or (b) export an existing F1 DRAFT. |
| Blank / example / DRAFT export? | No blank or example generator exists. Only F1 DRAFT snapshot export exists. |
| Who generates the file? | F1: `GenerateBuyerDraftWorkbook` from a live DRAFT snapshot. CREATE: nobody. Zero `.xlsx` assets in the repo. |
| Compatible with F5 CREATE parser? | Shared sheet/header schema yes. F1 files parse as CREATE with identity **warnings**. Header-only graph is CREATE-ready. |
| New backend endpoint needed? | **Yes**, if the product must give users a CREATE-compatible file without a pre-existing DRAFT. |
| Download button location? | Missing. Must be added on `/tenders/new-from-xlsx`. `/tenders` already has the create-from-Excel **entry**, not a download. |
| Roles / flags? | Same as F5 CREATE: human JWT + BuyerManage + `RFX_EXCEL_EXCHANGE_ENABLED`. |
| Workbook versioning? | `BINTRANS_RFX_BUYER_XLSX_V1` / `schema_version=1`. No CREATE-specific version exists. |
| `download → fill → preview → commit` tested? | **No**. Current browser gate starts at **upload**. |

F1 export as CREATE basis:

- Requires an existing event and active DRAFT (409 otherwise).
- Contains tenant/event/version identity.
- CREATE parser does **not** reject an `UPDATE_DRAFT` “envelope” because the file has no mode field; it **accepts** F1 bytes and warns on identity keys.
- Safe enough as “user’s own compatible XLSX”, **not** safe as the official blank-template path (mixes F1 UPDATE identity with F5 CREATE, requires a live DRAFT, leaks source event IDs into the file).
- There is **no** current way to obtain a CREATE-compatible workbook without manual preparation **or** an existing DRAFT.

Hidden “create a throwaway DRAFT, export, delete” is out of scope and must not be recommended.

---

## 5. Variant comparison (A–D)

Scoring: **Fit** = supports Upload-Only UX + CREATE parser + no throwaway RFx. **Risk** = schema drift, identity mix, security, ops.

### A. Backend-generated blank workbook (new read-only endpoint)

Backend generates a header-only V1 workbook from the same writer the CREATE parser already accepts.

| Criterion | Assessment |
| --- | --- |
| Upload-Only UX | Fits: download is additive; upload of own file stays |
| CREATE parser | Direct: header-only + schema metadata is already ReadyToCommit |
| Schema drift | Lowest if generator shares `buyerSheetOrder` / header slices / `Generate*` helpers + parity test |
| Workbook identity | Omit event/tenant keys → no CREATE warnings |
| Localization | Instructions RU/EN/ZH; headers stay English |
| Security / formulas | Reuse generator text-cell + parser deny-formulas |
| RBAC / tenant / company | Same BuyerManage + flag; no tenant/company data in file |
| Feature flag / rate limit / caching | Same excel group; `no-store`; shared 429 |
| Versioning / maintenance | One Go generator; bump `schema_version` with parser |
| Browser testability | Real GET download, then fill/upload |
| Accessibility | Native button + `aria-live` status, same panel |
| Backend / frontend / OpenAPI | All three required (small) |
| Mix with F1 | Separate path `/xlsx-create/template` vs `/{id}/xlsx-export` |
| Own compatible XLSX | Still allowed |

### B. Versioned static XLSX asset (frontend/CDN)

| Criterion | Assessment |
| --- | --- |
| Upload-Only UX | Button can serve a file, but file is not generated from parser schema |
| CREATE parser | Only if asset is rebuilt whenever parser/headers change |
| Schema drift | **High**: binary can rot; repo currently has **zero** xlsx assets |
| Identity | Can be blank, but no server enforcement |
| Localization | Must store one trilingual file or three assets |
| Security | CDN/public leak of schema; weaker RBAC (static often unauthenticated) |
| Flag / rate limit | Frontend hide ≠ 404; CDN caching fights schema bumps |
| Versioning | Manual pin; easy to ship stale `schema_version` |
| Browser tests | Possible, but CI must vendor the binary |
| Product changes | Frontend + asset pipeline; OpenAPI optional and then drift is worse |
| Mix with F1 | Lower path collision, higher contract collision |
| Own XLSX | Allowed |

Rejected: no mechanical generator↔parser lock; contradicts “do not rely on documentation alone”.

### C. Existing DRAFT export (F1)

| Criterion | Assessment |
| --- | --- |
| Upload-Only UX | Forces a prior manual/template DRAFT; not a create-from-xlsx first step |
| CREATE parser | Accepts with identity warnings; hashes ignore workbook IDs |
| Schema drift | Low (same generator), but **wrong mode purpose** |
| Identity | Event/tenant/version IDs always present |
| Security | Cross-event confusion; users may think they are updating the source DRAFT |
| Tenant isolation | Export is tenant-scoped; using it to CREATE clones another event’s graph |
| Mix with F1 | **Direct mix** of UPDATE export and CREATE commit |
| Own XLSX | This *is* a compatible file, not a template |
| Hidden temp RFx | The only way to get a “blank-ish” F1 file without prior work — **forbidden** |

Rejected as the **supported** acquisition method.

### D. Backend-generated example workbook (demo rows)

| Criterion | Assessment |
| --- | --- |
| Upload-Only UX | Same as A plus demo lots/questions |
| CREATE parser | Demo rows would commit if the user forgets to replace them |
| Schema drift | Same as A if generated |
| Identity | Can be blank of IDs, but not blank of business data |
| Safety | Accidental `creation_channel=EXCEL` DRAFT with demo content |
| Testability | Harder: tests must distinguish template vs leftover examples |
| Mix with F1 | Lower than C, higher than A (looks like a real tender) |

Rejected as the default download. Example rows may be a later optional query (`?example=1`) only after a controller decision; not this discovery’s recommendation.

### Comparison matrix (summary)

| | A blank backend | B static asset | C F1 export | D example backend |
| --- | --- | --- | --- | --- |
| Official CREATE source without DRAFT | yes | yes (stale risk) | no | yes |
| Parser lock | strongest | weak | strong / wrong purpose | strong / unsafe content |
| Identity leakage | none | none | yes | none (IDs) / yes (demo data) |
| F1 mix | isolated path | isolated file | mixed | content confusion |
| New endpoint | yes | no | no | yes |
| Recommended | **YES** | no | no | no |

---

## 6. Selected solution

**Variant A — backend-generated blank workbook.**

The supported file is a **blank template**, not an example and not an F1 DRAFT export.

- Generated at request time from the canonical CREATE sheet/header contract.
- Contains `schema_name` + `schema_version` only in Metadata.
- Data sheets are headers only (zero lots/sections/questions/options/rules).
- Instructions describe CREATE / new DRAFT, not “export of an existing DRAFT”.
- Users may still upload any other V1-compatible XLSX (including a cleaned F1 export).
- F1 `/{id}/xlsx-export` and F1 UI stay unchanged.

---

## 7. API contract (new endpoint required)

### 7.1 Paths and registry

| Field | Value |
| --- | --- |
| HTTP method | `GET` |
| Public / OpenAPI / gateway path | `/api/v1/rfx-events/xlsx-create/template` |
| Downstream / service path | `/v1/rfx-events/xlsx-create/template` |
| Service Chi path | `/xlsx-create/template` |
| Registry name | `export_buyer_create_xlsx_template` |
| operationId | `get_buyer_new_rfx_event_xlsx_create_template` |
| Success status | `200` |
| Idempotency | not required |
| FeatureFlagProtected | `true` |
| RBACPolicy | `PolicyBuyerManage` |
| Route class | human JWT (`RequiresHumanAuth` default) |
| Integration / ERP | **not** registered on `/integrations/erp/*` |

Registration rules:

- Service: same excel-flag group as preview/commit, **before** `/{id}` (`router.go` 70–73 pattern).
- Gateway: same `excelExchangeFlagMiddleware` group as other excel routes (`router.go` 342–351).
- Extend `E7ExcelExchangeRoutes()` and existing route-parity tests (`e7_excel_exchange_route_parity_test.go`, rfx-service `excel_exchange_route_parity_test.go`).

### 7.2 Request

- No body.
- No path parameters.
- No query parameters as authority.
- `tenant_id` query → **403** (`rejectClientTenantQuery` via `requireActor`).
- `owner_company_id` query/body → **not accepted** (blank file has no company data; company is selected later on preview multipart).
- Headers: standard `Authorization` human Bearer; gateway-established `X-Tenant-ID` / `X-User-ID` only after JWT.

### 7.3 Response

| Header / body | Value |
| --- | --- |
| Content-Type | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` |
| Content-Disposition | `attachment; filename="bintrans-rfx-buyer-xlsx-v1-create-template.xlsx"` |
| Cache-Control | `no-store` |
| X-Content-Type-Options | `nosniff` |
| Filename constant | `bintrans-rfx-buyer-xlsx-v1-create-template.xlsx` (ASCII, no user input) |
| Body | generated XLSX bytes |

Reuse `respond.BinaryAttachment` (`binary.go` 8–12).

### 7.4 Workbook contents

| Field | Value |
| --- | --- |
| Contract version | `BINTRANS_RFX_BUYER_XLSX_V1` / `1` |
| Conceptual mode | `CREATE_NEW_DRAFT` (parser-assigned; **do not** add a required `mode` metadata key) |
| Sheets / headers | identical to §3 |
| Metadata rows | `schema_name`, `schema_version` only |
| Data rows | none |
| Instructions | CREATE purpose, fill-then-upload, no formulas/macros; RU/EN/ZH |
| Hidden sheets | none |
| Formulas | none |

Recommended generator: `GenerateBuyerCreateBlankWorkbook()` in `xlsxexchange`, reusing `buyerSheetOrder`, header slices, and `cellWriter`. Do **not** call `ExportBuyerDraftWorkbook` or load an event.

### 7.5 Errors

| Status | When |
| --- | --- |
| 401 | missing/invalid human JWT |
| 403 | authenticated but not BuyerManage; or `tenant_id` query present |
| 404 | `RFX_EXCEL_EXCHANGE_ENABLED=false`; or route unknown |
| 429 | shared gateway rate limit |
| 500 | generator failure |

No 409 (no DRAFT precondition). No 422 (no preview). No 201.

### 7.6 Rate limit and cache

- Same shared gateway limiter as other `/api/v1/*` excel routes (default 50 RPS / burst 100).
- `Cache-Control: no-store` so a schema bump cannot be served from a stale browser cache.
- No CDN/public cache.

### 7.7 OpenAPI

**Authoritative source:** `scripts/openapi/generate_openapi.py` (Python).
`packages/openapi/rfx-service.yaml`, `packages/openapi/openapi.yaml`, and `packages/openapi/openapi.json` are **generated artifacts**. Do not hand-edit them as source-of-truth. `scripts/openapi/cmd/generate/main.go` is a Makefile fallback only and is **not** the source-of-truth for this endpoint. Future W1 uses the Python generator that current CI/`make openapi-generate` already run.

W1 (when separately authorized) must update the Python generator, not YAML:

| Generator structure | Change |
| --- | --- |
| `ENDPOINTS` | add `GET /api/v1/rfx-events/xlsx-create/template` |
| `E7_EXCEL_EXCHANGE_ENDPOINT_PROFILES` | include the new profile |
| `BINARY_RESPONSE_PROFILES` | include the new profile (today this set is only F1/F2 export) |
| Endpoint description / responses | human JWT, `PolicyBuyerManage`, Excel-flag semantics aligned with existing excel routes |

Generated contract for `GET /api/v1/rfx-events/xlsx-create/template`:

- `200` binary XLSX
- MIME `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`
- `403` non-BuyerManage / forbidden `tenant_id` query
- flag-off `404`
- shared gateway rate-limit `429`
- document filename, no DB writes, no event/analysis, not an ERP route

After the generator change, W1 must:

1. run generation;
2. re-run generation and prove idempotency;
3. run `make openapi-check`;
4. prove generator ↔ committed artifact parity;
5. leave no manual YAML drift.

This discovery does **not** change the generator or any generated spec.

### 7.8 Side-effect ban (download)

The handler must perform:

- **no** database writes
- **no** event creation
- **no** preview analysis creation
- **no** participants
- **no** publish / submit
- **no** ERP metadata
- **no** `/integrations/erp/*`

Tenant from verified auth is used only to authenticate the caller, not to stamp the workbook.

### 7.9 Formula-safety

Generated cells are strings via the existing writer. Implementation tests must reopen the bytes and assert `GetCellFormula` is empty on every sheet (mirror `formula_injection_test.go`).

---

## 8. UX decision

### 8.1 Placement

| Place | Role |
| --- | --- |
| `/tenders/new-from-xlsx` | **Primary** download button, above the file input |
| `/tenders` | Existing `Create from Excel` entry only (navigates to the page). **No** second download |
| `/tenders/:id` F1 panel | **Unchanged**. No CREATE template button |

One main entry: list → create-from-xlsx page → download template → fill → upload → preview → commit.

### 8.2 Copy

| Key | EN | RU | ZH |
| --- | --- | --- | --- |
| `tenders.buyerXlsxCreate.downloadTemplate` | Download Excel template | Скачать шаблон Excel | 下载 Excel 模板 |
| `tenders.buyerXlsxCreate.downloadHint` | Download a blank buyer template, fill the sheets, then upload it here. You may also upload your own compatible XLSX. | Скачайте пустой шаблон покупателя, заполните листы и загрузите файл здесь. Можно загрузить и свой совместимый XLSX. | 下载空白买方模板，填写工作表后在此上传。也可以上传自己的兼容 XLSX。 |
| `tenders.buyerXlsxCreate.status.downloading` | Downloading template | Скачивание шаблона | 正在下载模板 |
| `tenders.buyerXlsxCreate.status.downloaded` | Template downloaded | Шаблон скачан | 模板已下载 |
| `tenders.buyerXlsxCreate.errors.downloadFailed` | The Excel template could not be downloaded. | Не удалось скачать шаблон Excel. | 无法下载 Excel 模板。 |

Keep existing title/hint. Extend hint so download is visible but upload-of-own-file remains first-class.

Filename shown to the browser: `bintrans-rfx-buyer-xlsx-v1-create-template.xlsx`.

### 8.3 Sequence

1. BuyerManage + flag-on user opens `/tenders` → `Create from Excel`.
2. On `/tenders/new-from-xlsx` clicks **Download Excel template**.
3. Fills Lots/questionnaire offline (or skips to header-only).
4. Fills event metadata on the page (multipart, not workbook).
5. Uploads the filled file **or** any other compatible XLSX.
6. Preview → Commit 201 → `/tenders/{event_id}`.
7. Event stays `DRAFT`, `creation_channel=EXCEL`, zero participants.

### 8.4 Flag / roles

| Actor | Download button |
| --- | --- |
| BuyerManage + flag on | visible, enabled |
| BuyerManage + flag off | hidden; page shows `buyer-xlsx-create-unavailable` (same as today) |
| `SHIPPER_LOGIST` / carrier / no BuyerManage | hidden / unavailable |
| API flag-off | 404 |
| API unauthorized role | 403 |

### 8.5 Accessibility and states

- `data-testid="buyer-xlsx-create-download-template"`
- Loading uses existing Button `loading` + `aria-live` status region `buyer-xlsx-create-status`
- Error uses `buyer-xlsx-create-error` / `role="alert"`
- Keyboard-focusable button; do not replace the file input
- Do not reuse `buyer-xlsx-export` (F1)

### 8.6 Own-file upload

The file input stays. Download is optional. A user with a compatible V1 file never has to click download.

---

## 9. Roles, flags, security

| Rule | Contract |
| --- | --- |
| Allowed | `PLATFORM_ADMIN`, `PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, `FORWARDER_MANAGER` |
| Denied | `SHIPPER_LOGIST`; `CARRIER_ADMIN` / `CARRIER_DISPATCHER`; roles without BuyerManage |
| Auth | human JWT only |
| Capability | BuyerManage (`CanBuyerManage` / `HasBuyerManageRole`) |
| Flag | `RFX_EXCEL_EXCHANGE_ENABLED`; flag-off → **404** |
| Tenant | verified auth context only |
| Company | not a download parameter; no company rows in the file |
| Query `tenant_id` | forbidden 403 |
| Cross-tenant leakage | none in file; 403/401 before bytes |
| Filename / headers | fixed constants; no user interpolation |
| Fail-closed | same as F5 preview/commit |

---

## 10. Compatibility and schema-drift control

**Source of truth:** Go package `services/rfx-service/internal/xlsxexchange` — shared sheet names, header slices, schema constants, writer, CREATE parser.

Mechanical lock (implementation must add; discovery only specifies):

1. `GenerateBuyerCreateBlankWorkbook()` uses the same `buyerSheetOrder` and `lotsHeaders` / `sectionsHeaders` / … as `GenerateBuyerDraftWorkbook` and `dataSheetHeaders`.
2. Unit: generated blank → `ParseBuyerCreatePreview` → `Mode=CREATE_NEW_DRAFT`, `ReadyToCommit=true`, zero errors, **zero** identity warnings.
3. Unit: generated blank must **not** be required to pass UPDATE preview against a real baseline (UPDATE may be ready with empty diffs; do not treat UPDATE success as the CREATE contract).
4. Unit: every sheet formula-empty; no hidden sheets; metadata keys exactly `{schema_name, schema_version}`.
5. Integration: HTTP 200 bytes → CREATE preview 200 ready → commit 201 DRAFT/EXCEL.
6. Route/OpenAPI parity tests include the new GET. OpenAPI parity is against Python-generated artifacts from `scripts/openapi/generate_openapi.py`, not a hand-edited `rfx-service.yaml`.
7. When headers/schema change, blank generator and both parsers change in the **same** commit; CI fails if generator sheets ≠ parser `buyerSheetOrder`.

Do **not** rely on markdown as the only lock.

Old F1 files remain uploadable (warnings). After a parser-breaking schema bump, `schema_version` must increment and old blanks fail `unsupported_schema` (already structural 400).

---

## 11. Test matrix

Exact browser/CI gate to extend: **`rfx-buyer-xlsx-create-browser-e2e`**.

CI dependencies already on that job (`.github/workflows/ci.yml` 1098+): Postgres 16, `RFX_EXCEL_EXCHANGE_ENABLED=true`, `BROWSER_E2E=1`, production `api-gateway` build, pnpm, Playwright Chromium, ban on `test.skip` / `test.fixme` / `test.only` / `page.route`.

W1/W2 also need existing unit/integration jobs that already compile `xlsxexchange`, gateway parity, and `apps/web-procurement` vitest. No new workflow file in discovery.

### 11.1 Unit (W1 / W2)

| ID | Assert |
| --- | --- |
| U1 | `GenerateBuyerCreateBlankWorkbook` sheet order + headers |
| U2 | Metadata only schema fields; filename constant |
| U3 | Content-Disposition / Content-Type / Cache-Control helpers |
| U4 | Mode/version: parse → `CREATE_NEW_DRAFT` + V1 / `1` |
| U5 | Formula-safety: no formulas; formula-like text is text |
| U6 | Route classification: GET template is human + BuyerManage + flag-protected; not integration |
| U7 | Frontend: testid, i18n RU/EN/ZH parity, no F1 export path, no ERP/publish/participants |

### 11.2 Integration (W1)

| ID | Assert |
| --- | --- |
| I1 | GET template 200 + valid XLSX signature |
| I2 | Downloaded bytes pass CREATE preview (ready) |
| I3 | Filled (or header-only) workbook passes commit 201 |
| I4 | Event `status=DRAFT`, `creation_channel=EXCEL` |
| I5 | Zero participants; no publish/submit/ERP |
| I6 | Flag-off 404; no writes |
| I7 | Unauthorized 401; non-manage 403; logist/carrier 403 |
| I8 | `tenant_id` query 403; company query ignored/rejected |
| I9 | Shared rate-limit 429 |
| I10 | Download: no DB writes, no analysis row, no event |
| I11 | Python generator + generated OpenAPI artifacts / `E7ExcelExchangeRoutes` / Chi / gateway parity |
| I12 | Generator ↔ CREATE parser parity |

### 11.3 Browser (W3, same gate)

| ID | Assert |
| --- | --- |
| B1 | Download visible for allowed buyer |
| B2 | Hidden for logist / carrier |
| B3 | Hidden/unavailable when flag off |
| B4 | Click downloads real bytes |
| B5 | Content-Type + filename correct (page.request or download event; **no** `page.route`) |
| B6 | Downloaded file uploads in the same page |
| B7 | Preview success |
| B8 | Commit 201 |
| B9 | Redirect `/tenders/{event_id}` |
| B10 | Human GET event: DRAFT + EXCEL |
| B11 | Zero participants; no publish/submit/ERP |
| B12 | 0 skipped; retries 0 |

Existing upload-only cases stay. Add download→fill→commit; keep fixture-upload as “own compatible XLSX”.

---

## 12. Implementation waves

Discovery **does not** authorize any wave.

### W1 — backend generator / endpoint / OpenAPI / parity

- **Scope:** `GenerateBuyerCreateBlankWorkbook`; GET template handler; Chi + gateway + `E7ExcelExchangeRoutes`; Python OpenAPI generator (`scripts/openapi/generate_openapi.py`: `ENDPOINTS`, `E7_EXCEL_EXCHANGE_ENDPOINT_PROFILES`, `BINARY_RESPONSE_PROFILES`, description/responses); regenerate artifacts; `make openapi-check`; unit/integration from §11.1–11.2.
- **Prerequisites:** controller authorization of this discovery; accepted F5 backend W1 (already on `main`).
- **Tests:** xlsxexchange unit; excel exchange integration; route parity; generator idempotency and generator ↔ committed OpenAPI artifact parity.
- **Stop:** 200 blank file parses CREATE-ready; flag/RBAC/no-write proven; Python-generated OpenAPI artifacts match `make openapi-check` with no hand-edited YAML.
- **Forbidden:** frontend product UX; migrations; F1 rewrite; ERP; publish/participants; changing preview/commit contracts.

### W2 — frontend download UX / i18n

- **Scope:** button + copy on `/tenders/new-from-xlsx` only; client GET; loading/error; RU/EN/ZH; vitest source safety.
- **Prerequisites:** W1 contract frozen.
- **Tests:** `buyerXlsxCreate.test.ts` extensions.
- **Stop:** button isolated from F1; own-file upload remains.
- **Forbidden:** `/tenders/:id` changes; ERP browser; training.

### W3 — live browser acceptance

- **Scope:** extend `rfx-buyer-xlsx-create-browser-e2e`; download→fill→preview→commit; flag-off/role hide; 0 skipped, retries 0, no `page.route`.
- **Prerequisites:** W1+W2 on the same reviewable branch or already merged by controller order.
- **Stop:** all §11.3 rows green on exact HEAD.
- **Forbidden:** closing F5 overall or Frontend Phase 2 without W4; training.

### W4 — controller acceptance / alignment

- **Scope:** docs/roadmap markers only after evidence; no extra product scope.
- **Prerequisites:** W1–W3 PASS on one product HEAD.
- **Stop:** controller verdict recorded; still no training/ERP/TMS/Award→TO.
- **Forbidden:** silent `IMPLEMENTED_ACCEPTED` without controller; merge of discovery itself as product.

---

## 13. Risks

| Risk | Mitigation |
| --- | --- |
| Users keep using F1 export as a “template” | Copy + isolated button; F1 panel unchanged |
| Adding `mode` metadata breaks upload-only files | Do not add a required envelope key |
| Example rows committed by mistake | Variant D not selected |
| Schema drift if someone later copies a static file | Parity test on generated bytes |
| Chi `/{id}` swallows `xlsx-create` | Register static GET beside existing preview/commit |
| Filename injection | Fixed constant only |
| Caching stale schema | `no-store` |
| Treating download as analysis/event | Explicit no-write tests |
| Closing F5 from discovery | Status is review-only |

---

## 14. Open questions (non-blocking for the recommendation)

1. Optional later `?example=1` demo rows — **not** in W1–W4 unless a new controller task.
2. Whether Instruction prose should mention F1 export as an advanced “own file” path (copy only; no product coupling).
3. Whether W3 must fill at least one lot in the downloaded blank (parser allows zero; product UX may still want one lot before publish — publish is out of scope).
4. Dedicated job vs extending `rfx-buyer-xlsx-create-browser-e2e` — this discovery chooses **extend the existing gate**.

---

## 15. Explicit out of scope

- Implementation in this worktree
- Migrations
- Changes to accepted Upload-Only preview/commit behavior
- F1 rewrite or sharing the F1 export button
- ERP browser client / `/integrations/erp/*`
- Training
- TMS
- Award → Transport Order
- Auto-publish / auto-submit
- Auto-adding participants
- Closing F5 overall or Frontend Phase 2
- Merging existing feature branches
- Creating a temporary RFx to harvest an export
- Static checked-in XLSX as source of truth

---

## 16. Controller acceptance

**Variant A accepted** as the only supported acquisition method:

1. New read-only `GET /api/v1/rfx-events/xlsx-create/template`.
2. Backend-generated **blank** BUYER XLSX V1 workbook (headers + schema metadata only).
3. Primary button on `/tenders/new-from-xlsx`; list page stays an entry only.
4. Same human JWT / BuyerManage / excel flag / fail-closed tenant rules as F5 CREATE.
5. Own compatible XLSX remains valid.
6. OpenAPI source-of-truth is `scripts/openapi/generate_openapi.py` (`F5-TA-1=FIXED_ACCEPTED`). Go `cmd/generate` remains a Makefile fallback only (`F5-TA-2=NOTE_CLOSED`).
7. W1→W4 stay a **separate** implementation stream. Implementation has not started.

```
F5_TEMPLATE_ACQUISITION_STATUS=DISCOVERY_ACCEPTED
CONTROLLER_VERDICT=ACCEPT_F5_TEMPLATE_ACQUISITION_SCOPE
ACCEPTED_DISCOVERY_HEAD=c0e39487482b9ffeca3706c089b15adc762007c6
ACCEPTED_CI_RUN=35745948306
ACCEPTED_CI_ATTEMPT=2
F5_TA_1=FIXED_ACCEPTED
F5_TA_2=NOTE_CLOSED
F5_TEMPLATE_ACQUISITION_IMPLEMENTATION_AUTHORIZED=NO
F5_OVERALL_STATUS=IMPLEMENTATION_IN_PROGRESS
FRONTEND_PHASE2_STATUS=IMPLEMENTATION_IN_PROGRESS
NEXT_ACTION=AUTHORIZE_F5_TEMPLATE_ACQUISITION_W1
```

---

## 17. W1 implementation pointer

Authorized W1 backend evidence is recorded in `RFX_V3_0E7_CREATE_FROM_XLSX_TEMPLATE_ACQUISITION_W1.md`. This discovery document stays accepted and does not close F5 overall, Frontend Phase 2, W2, or W3.

```
F5_TEMPLATE_ACQUISITION_W1_STATUS=IMPLEMENTED_ACCEPTED
CONTROLLER_VERDICT=ACCEPT_F5_TEMPLATE_ACQUISITION_W1
F5_TEMPLATE_ACQUISITION_W1_PRODUCT_HEAD=c835d19ee5d22bacd852852c7d4a3c723f5d4f5c
F5_TEMPLATE_ACQUISITION_W1_CI_RUN=35760651845
F5_TEMPLATE_ACQUISITION_W2_STATUS=NOT_STARTED
F5_TEMPLATE_ACQUISITION_W3_STATUS=NOT_STARTED
```
