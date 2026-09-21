# RFx v3.0E7 F5 Create-from-XLSX Backend W1

Status markers for independent controller review. This wave implements human JWT `CREATE_NEW_DRAFT` only. Frontend, blank-template download, participants import, scoring import, training, ERP browser client, TMS, and Award→Transport Order remain unauthorized.

## Markers

```
F5_CREATE_FROM_XLSX_BACKEND_W1_STATUS=IMPLEMENTED_AWAITING_REVIEW
F5_CREATE_FROM_XLSX_CONTRACT_VERDICT=ACCEPT_WITH_CHANGES
MIGRATION_REQUIRED=NO
F5_FRONTEND_IMPLEMENTATION_AUTHORIZED=NO
F5_TEMPLATE_ACQUISITION_DECISION_REQUIRED=YES
NEXT_ACTION=INDEPENDENT_CONTROLLER_REVIEW_F5_BACKEND_W1
```

## Frozen routes

| Method | Path | operationId | Success |
| --- | --- | --- | --- |
| POST | `/api/v1/rfx-events/xlsx-create/preview` | `post_preview_buyer_new_rfx_event_xlsx_create` | 200 / 422 |
| POST | `/api/v1/rfx-events/xlsx-create/commit` | `post_commit_buyer_new_rfx_event_xlsx_create` | 201 / 201 replay |

Both routes: human JWT, `PolicyBuyerManage`, `RFX_EXCEL_EXCHANGE_ENABLED=false` → 404, shared gateway rate limit → 429. Not registered on `/integrations/erp/*`. Blank-template download is not added in W1.

## Follow-up

`F5_TEMPLATE_ACQUISITION_DECISION_REQUIRED=YES` — do not add a silent template download route.
