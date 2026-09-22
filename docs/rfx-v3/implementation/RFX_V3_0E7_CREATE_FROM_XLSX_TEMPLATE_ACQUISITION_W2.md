# RFx v3.0E7 F5 XLSX Template Acquisition W2

Frontend download UX for the accepted W1 blank CREATE template. W3 live browser acceptance, training, ERP, TMS, and Award→Transport Order are not started.

```
F5_TEMPLATE_ACQUISITION_W2_STATUS=IMPLEMENTED_AWAITING_CONTROLLER_REVIEW
F5_TEMPLATE_ACQUISITION_W3_STATUS=NOT_STARTED
```

F5 overall and Frontend Phase 2 stay **IMPLEMENTATION_IN_PROGRESS**. This document does not mark them accepted.

## Recovery identity

| Item | Value |
| --- | --- |
| Worktree | `D:\Projects\freight-platform-wt\rfx-f5-template-acquisition-w2-frontend-v3.0e7` |
| Branch | `feat/rfx-f5-template-acquisition-w2-frontend-v3.0e7` |
| Base / `origin/main` | `40d56977be2f90408f63c77e94979c9d102ef67e` |
| Base history | Merge pull request #156 |
| Post-merge CI | `35767256423` attempt 2 success |
| W1 product | unchanged |

## UX

Placement: `/tenders/new-from-xlsx`, beside the XLSX upload control, visible before a file is chosen.

`/tenders` keeps only the existing `Create from Excel` entry. The F1 UPDATE panel on `/tenders/:id` is unchanged.

Button copy:

| Locale | Label |
| --- | --- |
| ru-RU | Скачать пустой шаблон |
| en-US | Download blank template |
| zh-CN | 下载空白模板 |

The hint says the template is for a new tender, the filled file is uploaded on this page, downloading does not create a tender, and upload/create do not publish the tender or add participants.

The button uses the same frontend gate as Create from Excel: Excel exchange flag on and BuyerManage (`PLATFORM_ADMIN`, `PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, `FORWARDER_MANAGER`). `SHIPPER_LOGIST`, carrier roles, other non-manage roles, and flag-off do not see the button. That guard is UX only.

## Request

`GET /api/v1/rfx-events/xlsx-create/template` through the existing authenticated gateway client. No body, no query, no event id, and no `tenant_id` or company authority in the URL or body. The response stays binary. The saved name is the constant `bintrans-rfx-buyer-xlsx-v1-create-template.xlsx` unless `Content-Disposition` yields a safe `.xlsx` basename with no path or header-injection characters.

The browser download runs only after a click: object URL, temporary `<a download>`, remove the anchor, `URL.revokeObjectURL`. Server render does not touch `window`, `document`, or `URL.createObjectURL`. A second click while a download is in flight does not start another request. Download success and failure leave the selected file, metadata, preview, `analysis_id`, and commit Idempotency-Key in place. GET retries do not send an Idempotency-Key.

Localized errors cover 401, 403, 404, 429, network/5xx, and an empty or invalid workbook. The raw backend body is not the user-facing text.
