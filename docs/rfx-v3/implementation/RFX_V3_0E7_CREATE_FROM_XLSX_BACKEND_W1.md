# RFx v3.0E7 F5 Create-from-XLSX Backend W1

Accepted human-JWT `CREATE_NEW_DRAFT` backend wave. Frontend, blank-template download, participants import, scoring import, training, ERP browser client, TMS, and Award→Transport Order remain unauthorized.

## Markers

```
F5_BACKEND_W1_STATUS=IMPLEMENTED_ACCEPTED
F5_CREATE_FROM_XLSX_BACKEND_W1_STATUS=IMPLEMENTED_ACCEPTED
CONTROLLER_VERDICT=ACCEPT_F5_BACKEND_W1
F5_CREATE_FROM_XLSX_CONTRACT_VERDICT=ACCEPT_F5_BACKEND_W1
F5_BACKEND_W1_PRODUCT_HEAD=1f5deadaa4e81e352c041a5b97318b1a640ac48f
F5_BACKEND_W1_CI_RUN=35695381395
MIGRATION_REQUIRED=NO
F5_OVERALL_STATUS=IMPLEMENTATION_IN_PROGRESS
F5_FRONTEND_STATUS=NOT_STARTED
F5_FRONTEND_IMPLEMENTATION_AUTHORIZED=NO
F5_TEMPLATE_ACQUISITION_DECISION_REQUIRED=YES
TRAINING_STATUS=NOT_STARTED
ERP_BROWSER_CLIENT_STATUS=NOT_STARTED
TMS_STATUS=OUT_OF_SCOPE
AWARD_TO_TRANSPORT_ORDER_IN_E7_BROWSER_GATE=NO
FOLLOW_UP_FINDINGS_BLOCK_F5_W1_MERGE=NO
NEXT_ACTION=PLAN_F5_FRONTEND
```

## Acceptance

| Item | Value |
| --- | --- |
| Controller verdict | `ACCEPT_F5_BACKEND_W1` |
| Accepted product HEAD | `1f5deadaa4e81e352c041a5b97318b1a640ac48f` |
| Accepted product CI | `35695381395` |
| Mode | `CREATE_NEW_DRAFT` |
| First commit | `201` |
| Idempotent replay | `201` same `event_id` |
| Created event | `status=DRAFT`, `creation_channel=EXCEL` |
| Questionnaire | graph imported, `questionnaire_enabled=false` |
| Participants | not created |
| BLOCKER / HIGH / MEDIUM | none |

## Frozen routes

| Method | Path | operationId | Success |
| --- | --- | --- | --- |
| POST | `/api/v1/rfx-events/xlsx-create/preview` | `post_preview_buyer_new_rfx_event_xlsx_create` | 200 / 422 |
| POST | `/api/v1/rfx-events/xlsx-create/commit` | `post_commit_buyer_new_rfx_event_xlsx_create` | 201 / 201 replay |

Both routes: human JWT, `PolicyBuyerManage`, `RFX_EXCEL_EXCHANGE_ENABLED=false` → 404, shared gateway rate limit → 429. Not registered on `/integrations/erp/*`. Blank-template download is not added in W1.

## NOTE follow-ups

| ID | Note |
| --- | --- |
| F5-N1 | `ValidateCreateRfxEventInput` still uses wall-clock; CREATE commit additionally revalidates deadline through injected `nowFn`. |
| F5-N2 | Cross-tenant fixture is not a seeded buyer-manage actor in the foreign tenant; fail-closed 404 still comes from tenant-scoped analysis lookup. |
| F5-N3 | CREATE commit shares the buyer-manage RBAC and excel-exchange flag middleware group with preview. |

## Follow-up

`F5_TEMPLATE_ACQUISITION_DECISION_REQUIRED=YES` — do not add a silent template download route. F5 overall remains `IMPLEMENTATION_IN_PROGRESS`.
