# RFx v3.0E7 F5 Frontend Upload-Only

Human-JWT `CREATE_NEW_DRAFT` frontend first wave. Blank-template download, F1 `UPDATE_EXISTING_DRAFT` changes, ERP browser client, training, TMS, and Award→Transport Order remain out of scope.

## Markers

```
F5_FRONTEND_UPLOAD_ONLY_STATUS=IMPLEMENTATION_IN_PROGRESS
F5_TEMPLATE_ACQUISITION_DECISION=A_UPLOAD_ONLY_FIRST_WAVE
F5_OVERALL_STATUS=IMPLEMENTATION_IN_PROGRESS
```

Do not mark F5 or Frontend Phase 2 as `IMPLEMENTED_ACCEPTED` from this wave.

## Product surface

- Entry: `/tenders` button `Create from Excel` for BuyerManage + Excel flag
- Page: `/tenders/new-from-xlsx`
- Preview: `POST /api/v1/rfx-events/xlsx-create/preview`
- Commit: `POST /api/v1/rfx-events/xlsx-create/commit` with `Idempotency-Key=buyer-xlsx-create-commit:<analysis_id>`
- Success: HTTP 201 → `/tenders/{event_id}`, status `DRAFT`, `creation_channel=EXCEL`, zero participants
