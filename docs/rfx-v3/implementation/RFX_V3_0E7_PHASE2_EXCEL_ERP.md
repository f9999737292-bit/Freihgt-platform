# RFx v3.0E7 Phase 2 — Excel Import/Export + ERP Integration

**Status:** `IMPLEMENTATION_IN_PROGRESS`
**Branch:** `feat/rfx-excel-erp-v3.0e7-phase2`
**Base:** `origin/main` @ `a56db57e4666057c326c5a0a2017bad3e81f890d`

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
| `services/rfx-service/go.mod` | **1.22** |
| CI (`.github/workflows/ci.yml`) | **1.22** |
| `services/rfx-service/Dockerfile` | **golang:1.22-alpine** |
| Local toolchain (informational) | 1.26.4 |

`github.com/xuri/excelize/v2@v2.11.0` declares **Go 1.25.0** (`go list -m -json`).

**BLOCKER:** `EXCELIZE_V2_11_REQUIRES_GO_1_25`

Phase 2 does **not** authorize Go toolchain bump. Workbook parse/generate via Excelize is blocked until controller approves either:

1. rfx-service Go 1.25 alignment (go.mod + CI + Dockerfile), or
2. an alternate Excel library compatible with Go 1.22.

**Allowed without Excelize:** migration 000073, normalized preview persistence, ZIP security inspection (stdlib), domain/repository skeleton.

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

---

## 4. Workbook schema versions

| Workbook | `workbook_type` | `schema_version` |
|---|---|---|
| Buyer tender | `BUYER_TENDER` | `BINTRANS_RFX_BUYER_XLSX_V1` |
| Carrier offer | `CARRIER_OFFER` | `BINTRANS_RFX_CARRIER_XLSX_V1` |

Sheet layout contract: **pending full task spec sections 4+** (truncated in controller packet).

---

## 5. Implementation progress

| Area | Status |
|---|---|
| Migration 000073 | IMPLEMENTED (pending DB integration run) |
| ZIP security inspection | IMPLEMENTED (`internal/xlsxsecurity`) |
| Import analysis repository | IMPLEMENTED |
| External object link repository | IMPLEMENTED |
| Excelize dependency | **BLOCKED** |
| Multipart HTTP handlers | NOT STARTED |
| Workbook parse/generate | NOT STARTED |
| ERP generic JSON contract API | NOT STARTED |
| OpenAPI | NOT STARTED |
| E7P2-INT integration matrix | PARTIAL (migration tests E7P2-INT-01..05) |

---

## 6. Preserved channels (unchanged)

1. Buyer manual create
2. Buyer template create (E5/E6)
3. Carrier direct submit (+ E7 late submission)
