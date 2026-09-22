# RFx v3.0E7 F5 Frontend Upload-Only

Human-JWT `CREATE_NEW_DRAFT` frontend first wave. Blank-template download, F1 `UPDATE_EXISTING_DRAFT` changes, ERP browser client, training, TMS, and Award→Transport Order remain out of scope.

## Markers

```
F5_FRONTEND_UPLOAD_ONLY_STATUS=IMPLEMENTED_ACCEPTED
CONTROLLER_VERDICT=ACCEPT_F5_FRONTEND_UPLOAD_ONLY
ACCEPTED_PRODUCT_HEAD=2acb7c0e27381c69707bde409447452a6e54d2f7
ACCEPTED_CI_RUN=35731621656
F5_TEMPLATE_ACQUISITION_DECISION=A_UPLOAD_ONLY_FIRST_WAVE
F5_OVERALL_STATUS=IMPLEMENTATION_IN_PROGRESS
FRONTEND_PHASE2_STATUS=IMPLEMENTATION_IN_PROGRESS
TRAINING_STATUS=NOT_STARTED
```

Do not mark F5 overall or Frontend Phase 2 as `IMPLEMENTED_ACCEPTED` from this wave. Blank-template acquisition/generation stays out of this wave.

## Accepted contract

- Entry: `/tenders` button `Create from Excel` for BuyerManage + Excel flag
- Separate page: `/tenders/new-from-xlsx`
- Upload-only preview/commit: `POST /api/v1/rfx-events/xlsx-create/preview` then `POST /api/v1/rfx-events/xlsx-create/commit`
- Stable Idempotency-Key: `buyer-xlsx-create-commit:<analysis_id>`
- HTTP 201 → `/tenders/{event_id}`
- Created event stays `DRAFT` with `creation_channel=EXCEL`
- Zero participants
- No publish, submit, or ERP browser/`/integrations/erp/*` calls
- Role, flag, tenant, and company isolation stay fail-closed
- Browser gate `rfx-buyer-xlsx-create-browser-e2e`: 5 tests, 14 contract proofs, 0 skipped, retries 0

## Non-blocking follow-ups

Do not treat these as merge blockers for this wave:

- `F5-CR-1`: machine code must not be the primary user-facing string
- `F5-CR-2`: optional Vue mount coverage
- `F5-CR-3`: optional live invalidation for remaining metadata fields
- `F5-CR-4`: optional live retry with the same Idempotency-Key
- `F5-CR-5`: optional localization of CREATE-specific codes
