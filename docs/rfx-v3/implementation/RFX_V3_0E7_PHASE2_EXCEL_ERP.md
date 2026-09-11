# RFx v3.0E7 Phase 2 — Excel Import/Export + ERP Integration

**Status:** `IMPLEMENTATION_IN_PROGRESS`
**Base:** `origin/main` @ `a6b66bea51579d689e98188e31446e436524927d`

| Marker | Value |
|---|---|
| `PHASE2_STATUS` | `IMPLEMENTATION_IN_PROGRESS` |
| `BUYER_XLSX_EXPORT_V1_STATUS` | `IMPLEMENTED_ACCEPTED` |
| `CONTROLLER_ACCEPTANCE` | YES |
| `PR123_MERGED` | YES |
| `PR123_HEAD` | `9a7502902d4ddbdedbb4e54817c183b8a00a7140` |
| `PR123_MERGE_SHA` | `a6b66bea51579d689e98188e31446e436524927d` |
| `PR123_MERGED_AT` | `2026-09-11T19:50:20Z` |
| `PR123_CI_RUN_ID` | `34639373871` |
| `PR123_CI_CONCLUSION` | `success` |

**Next stage:** Buyer XLSX Import Preview/Commit — **NOT STARTED**

---

## 1. Controller decisions (2026-09-11)

| Decision | Value |
|---|---|
| `MIGRATION_000073_AUTHORIZED` | YES |
| `MIGRATION_000074_ALLOWED` | NO |
| `XLSX_MULTIPART_DIRECT_TO_RFX` | YES |
| `XLSX_BINARY_PERSISTENCE` | NO |
| `NORMALIZED_PREVIEW_PERSISTENCE` | YES |
| `ERP_GENERIC_CONTRACT_AUTHORIZED` | YES |
| `SAP_ADAPTER_AUTHORIZED` | NO |
| `ONE_C_ADAPTER_AUTHORIZED` | NO |
| `PUBLIC_X_INTERNAL_TOKEN_ALLOWED` | NO |
| `E7_PHASE_3_STARTED` | NO |
| `E7_PHASE_4_STARTED` | NO |

---

## 2. Go toolchain + Excelize gate

| Source | Go version |
|---|---|
| `go.work` | 1.25.0 |
| `services/rfx-service/go.mod` | **1.25.0** (aligned via PR #121) |
| CI (`.github/workflows/ci.yml`) | **1.25** |
| `services/rfx-service/Dockerfile` | **golang:1.25-alpine** |
| Local toolchain (informational) | 1.26.4 |

`github.com/xuri/excelize/v2@v2.11.0` is a direct dependency (commit `9491027`). Buyer export uses Excelize in-process for workbook generation only; import remains NOT STARTED.

---

## 3. Migration 000073

Files:

- `infrastructure/migrations/000073_rfx_excel_erp_exchange_v3_0e7_phase2.up.sql`
- `infrastructure/migrations/000073_rfx_excel_erp_exchange_v3_0e7_phase2.down.sql`

Objects:

| Object | Purpose |
|---|---|
| `rfx.rfx_import_analyses` | Normalized preview records (no binary XLSX) |
| `rfx.rfx_external_object_links` | ERP external identity → `rfx_event_id` mapping |
| `rfx.rfx_events.creation_channel` | `MANUAL` / `TEMPLATE` / `EXCEL` / `ERP` provenance |

Backfill:

- `source_template_version_id IS NOT NULL` → `TEMPLATE`
- all other existing events → `MANUAL`

Migration **000074** is **not** authorized and **not** present on `main`.

---

## 4. Workbook schema versions

| Workbook | `workbook_type` | `schema_version` |
|---|---|---|
| Buyer tender | `BUYER_TENDER` | `BINTRANS_RFX_BUYER_XLSX_V1` |
| Carrier offer | `CARRIER_OFFER` | `BINTRANS_RFX_CARRIER_XLSX_V1` |

### 4.1 Buyer export workbook contract (`BINTRANS_RFX_BUYER_XLSX_V1`)

Sheet order (fixed):

1. `Instructions` — tri-lingual static guidance (RU / EN / ZH)
2. `Metadata` — key/value export snapshot (`schema_name`, `schema_version`, `exported_at_utc`, tenant/event/version ids, row versions, creation channel)
3. `Lots` — buyer lot rows (`lot_number`, `name`, `description`, `category`, `estimated_value`, `currency_code`, `status`)
4. `Sections` — `section_code`, `title_*`, `description_*`, `sort_order`
5. `Questions` — `section_code`, `question_code`, `question_type`, titles/descriptions, `required`, `sort_order`, `validation_json`
6. `Options` — `question_code`, `option_code`, labels, `sort_order`
7. `Rules` — `rule_code`, `source_question_code`, `condition`, `target_question_code`, `action`, `sort_order`

**LOTS model:** export includes all non-deleted lots for the event via `ListLotsByEvent`. Lots are sorted deterministically by `lot_number`, then `name`. No separate questionnaire lot graph exists.

**I18N_MONOLINGUAL mapping:** event questionnaire strings (`Title`, `Label`, `HelpText`, `Description`) are duplicated into `_ru`, `_en`, and `_zh` columns with identical values until dedicated locale fields exist on event graphs.

**Competitor exclusion:** no participant, carrier response, bid, scoring, invitation, or award data is written to any sheet.

**Security:** generated workbooks pass `xlsxsecurity.InspectUpload`; formula-like cell prefixes are sanitized; no formula XML elements, macros, OLE, or external links.

---

## 5. Buyer XLSX Export V1 — accepted (PR #123)

| Item | Value |
|---|---|
| Route (gateway) | `GET /api/v1/rfx-events/{id}/xlsx-export` |
| Route (service) | `GET /v1/rfx-events/{id}/xlsx-export` |
| Feature flag | `RFX_EXCEL_EXCHANGE_ENABLED` (default **false** → HTTP **404**) |
| RBAC | **BuyerManage** (`PolicyBuyerManage`) |
| Response | `200` `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` attachment |
| Preconditions | Active **DRAFT** questionnaire version required → **409** if missing |
| Tenant isolation | Cross-tenant → **404**; cross-company buyer → **404** (fail closed) |
| Audit policy | **None** — read-only export; no audit events, idempotency records, or import-analysis rows written |

Implementation:

- `services/rfx-service/internal/service/excel_exchange_service.go`
- `services/rfx-service/internal/http/handlers/excel_exchange_handler.go`
- `services/rfx-service/internal/xlsxexchange/buyer_workbook.go`
- `packages/shared-go/rfx/e7_excel_exchange_routes.go` (route parity anchor)

OpenAPI profile: `xlsx_export_buyer_draft` (binary 200 response); operationId `get_export_buyer_draft_rfx_event_as_xlsx_workbook`.

### 5.1 Acceptance record (Buyer XLSX Export V1)

| Acceptance criterion | Evidence |
|---|---|
| Schema | `BINTRANS_RFX_BUYER_XLSX_V1` |
| Workbook sheets | 7 fixed sheets (Instructions, Metadata, Lots, Sections, Questions, Options, Rules) |
| Export precondition | Active **DRAFT** questionnaire version only |
| RBAC | **BuyerManage** required; BuyerRead → HTTP **403** |
| Feature flag | `RFX_EXCEL_EXCHANGE_ENABLED` default **false** → HTTP **404** |
| Tenant / company isolation | Cross-tenant and cross-company buyer → HTTP **404** (fail closed) |
| Formula hardening | Leading-whitespace formula injection blocked; original values preserved |
| Competitor confidentiality | No participant, response, bid, scoring, invitation, or award data |
| Semantic determinism | Repeated export yields canonical snapshot (E7P2-INT-14) |
| Export side effects | No writes — event/version/graph, import-analysis, idempotency unchanged |
| Migration | **000073** on `main` |
| Excelize | `github.com/xuri/excelize/v2@v2.11.0` |
| Integration tests | E7P2-INT-06..20 |
| OpenAPI | Binary `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` response |

### 5.2 Controller review findings — closed

| Finding | Status |
|---|---|
| OpenAPI operationId mismatch | **CLOSED** — `get_export_buyer_draft_rfx_event_as_xlsx_workbook` |
| Leading-whitespace formula injection | **CLOSED** — `SetCellStr` + text style; original values preserved |
| Competitor scan evidence (E7P2-INT-18) | **CLOSED** — DB sentinels + cell/ZIP/XML scan |
| OpenAPI parity | **CLOSED** — route parity test strengthened |
| BuyerRead HTTP 403 (E7P2-INT-20) | **CLOSED** |

Historical CI attempts prior to remediation are retained for audit only; they are not current evidence.

### 5.3 Known limitations (honest scope)

- Event questionnaire i18n remains **monolingual** — export duplicates the source value into RU/EN/ZH columns.
- **No audit trail** for export — consistent with read-only GET pattern; no audit events written.
- Scoring, participants, responses, offers, and awards are **not exported**.
- Binary XLSX is **not stored** in the database.

---

## 6. Implementation progress

| Area | Status |
|---|---|
| Migration 000073 | **IMPLEMENTED_ACCEPTED** |
| ZIP security inspection | **IMPLEMENTED** (`internal/xlsxsecurity`) |
| Import analysis repository | **IMPLEMENTED** (foundation) |
| External object link repository | **IMPLEMENTED** (foundation) |
| Buyer draft XLSX export | **IMPLEMENTED_ACCEPTED** (PR #123) |
| Buyer workbook generate (Excelize) | **IMPLEMENTED_ACCEPTED** |
| OpenAPI (`xlsx_export_buyer_draft`) | **IMPLEMENTED_ACCEPTED** |
| Gateway RBAC + identity spoof test | **IMPLEMENTED_ACCEPTED** |
| CI job `rfx-excel-exchange-v3-integration` | **IMPLEMENTED_ACCEPTED** |
| E7P2-INT integration matrix | **IMPLEMENTED_ACCEPTED** (E7P2-INT-01..20) |
| Buyer XLSX import (preview/commit) | **NOT_STARTED** |
| Multipart import HTTP handlers | **NOT_STARTED** |
| ERP generic JSON contract API | **NOT_STARTED** |
| Carrier offer export/import | **NOT_STARTED** |
| Phase 2 frontend | **NOT_STARTED** |
| Phase 2 training (RU/EN/ZH) | **NOT_STARTED** |
| Phase 2 browser acceptance | **NOT_STARTED** |

| Future capability marker | Status |
|---|---|
| `BUYER_XLSX_IMPORT` | `NOT_STARTED` |
| `CARRIER_XLSX` | `NOT_STARTED` |
| `ERP_API` | `NOT_STARTED` |
| `FRONTEND_PHASE2` | `NOT_STARTED` |
| `TRAINING` | `NOT_STARTED` |
| `BROWSER_ACCEPTANCE` | `NOT_STARTED` |

---

## 7. Test evidence

| Test ID | Scope | Expected |
|---|---|---|
| E7P2-INT-01..05 | Migration 000073 | Up/down/up, backfill, import-analysis immutability |
| E7P2-INT-06 | Rich draft export | HTTP 200, Excelize reopen, graph parity |
| E7P2-INT-07 | No active draft | HTTP 409 |
| E7P2-INT-08 | Cross-tenant | HTTP 404 |
| E7P2-INT-09 | Cross-company buyer | HTTP 404 |
| E7P2-INT-10 | Carrier actor | HTTP 403 |
| E7P2-INT-11 | Unauthenticated | HTTP 401 |
| E7P2-INT-12 | Feature disabled | HTTP 404, no writes |
| E7P2-INT-13 | Unknown event | HTTP 404 |
| E7P2-INT-14 | Repeated export | Deterministic canonical snapshot |
| E7P2-INT-15 | Read-only side effect | Event/version/graph unchanged |
| E7P2-INT-16 | Import analysis | Count unchanged |
| E7P2-INT-17 | Idempotency | Count unchanged |
| E7P2-INT-18 | Competitor exclusion | No carrier/participant sheets or IDs |
| E7P2-INT-19 | Route parity smoke | Shared route anchor + HTTP 200 |
| E7P2-INT-20 | BuyerRead actor | HTTP 403 |

Gateway: `TestE7GatewayIdentityHeaderSpoofXlsxExport` — spoof headers stripped, verified JWT identity forwarded.

Run locally (requires PostgreSQL):

```bash
cd services/rfx-service
REQUIRE_TEST_DATABASE=1 TEST_DATABASE_URL='postgres://...' \
  go test -tags=integration ./internal/integration/excelexchange/... -count=1 -v
```

---

## 8. Preserved channels (unchanged)

1. Buyer manual create
2. Buyer template create (E5/E6)
3. Carrier direct submit (+ E7 late submission)
