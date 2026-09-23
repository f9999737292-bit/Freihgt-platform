# RFx v3.0E7 F5 Create-from-XLSX overall final acceptance

One live browser chain for the accepted F5 pieces. Controller verdict `ACCEPT_F5_OVERALL_FINAL_ACCEPTANCE` accepts F5 overall. Frontend Phase 2 stays in progress. Training stays not started.

```
CONTROLLER_VERDICT=ACCEPT_F5_OVERALL_FINAL_ACCEPTANCE
F5_TEMPLATE_ACQUISITION_W1_STATUS=IMPLEMENTED_ACCEPTED
F5_TEMPLATE_ACQUISITION_W2_STATUS=IMPLEMENTED_ACCEPTED
F5_TEMPLATE_ACQUISITION_W3_STATUS=IMPLEMENTED_ACCEPTED
F5_TEMPLATE_ACQUISITION_STATUS=IMPLEMENTED_ACCEPTED
F5_OVERALL_FINAL_ACCEPTANCE_STATUS=IMPLEMENTED_ACCEPTED
F5_OVERALL_STATUS=IMPLEMENTED_ACCEPTED
FRONTEND_PHASE2_STATUS=IMPLEMENTATION_IN_PROGRESS
TRAINING_STATUS=NOT_STARTED
NEXT_ACTION=PLAN_F5_TRAINING
```

## Lineage on this base

Base is `origin/main` `c725c2998b1aad1148e27cf24f51d705d691b7e8`, merge of PR #158.

| Block | Accepted product HEAD | Merge |
| --- | --- | --- |
| F5 backend CREATE W1 | `1f5deadaa4e81e352c041a5b97318b1a640ac48f` | `7c10126eed8897846b05f347d1270d13c3644a1f` |
| F5 frontend upload-only | `2acb7c0e27381c69707bde409447452a6e54d2f7` | `717f2d54f9d8987d3a62f56540752fe1e3aba9d4` |
| Template acquisition W1 | `c835d19ee5d22bacd852852c7d4a3c723f5d4f5c` | `40d56977be2f90408f63c77e94979c9d102ef67e` |
| Template acquisition W2 | `c51b39f5d7d2a683b4c1e3261cce93f8dee38231` | `ebd0f62657751615feb3f3672c4c4650fd58ed81` |
| Template acquisition W3 | `c7f5ae934eda50340836960ea04fc5f2b3c6a2e1` | `c725c2998b1aad1148e27cf24f51d705d691b7e8` |

Accepted write-ups stay in their own documents: [backend CREATE W1](./RFX_V3_0E7_CREATE_FROM_XLSX_BACKEND_W1.md), [frontend upload-only](./RFX_V3_0E7_CREATE_FROM_XLSX_FRONTEND_UPLOAD_ONLY.md), [template W1](./RFX_V3_0E7_CREATE_FROM_XLSX_TEMPLATE_ACQUISITION_W1.md), [template W2](./RFX_V3_0E7_CREATE_FROM_XLSX_TEMPLATE_ACQUISITION_W2.md), [template W3](./RFX_V3_0E7_CREATE_FROM_XLSX_TEMPLATE_ACQUISITION_W3.md).

## Chain

The spec `buyer-xlsx-create-overall-chain.spec.ts` runs inside the existing job `rfx-buyer-xlsx-create-browser-e2e`. It does not add a second browser stack or change the CI workflow. The spec does not call `page.route` and does not replace product responses.

On one BuyerManage session and one new RFx number the buyer opens `/tenders/new-from-xlsx`, downloads the blank template with one live click, receives a Playwright download, and uploads that saved file. Preview is one multipart POST, HTTP 200, `CREATE_NEW_DRAFT`, `ready_to_commit=true`, with the current `analysis_id`. A double click on commit sends one POST `/xlsx-create/commit` whose JSON body is only `{"analysis_id":"<current-analysis-id>"}` and whose `Idempotency-Key` is `buyer-xlsx-create-commit:<analysis_id>`. The response is HTTP 201 and the browser lands on `/tenders/{event_id}` with that same id. Human GET shows `DRAFT` and `creation_channel=EXCEL` with the entered metadata. Participants are `items=[]`. Questionnaire GET shows `questionnaire_enabled=false` and `version_status=DRAFT`.

The passive probe expects one template GET, one preview POST, and one commit POST. It expects no second preview after the commit click, and no publish, submit, participant mutation, award, Transport Order, or `/integrations/erp/` request.

The Go harness runs this spec alone before the full suite. The spec writes the created `rfx_number`, event id, analysis id, and Idempotency-Key to a temp evidence file. SQL then checks that one rfx number only: one `rfx_events` row, `DRAFT`, `EXCEL`, the same event id, the analysis `CONSUMED` with `NEW_EVENT` pointing at that event, one `BUYER_XLSX_CREATE_COMMIT` idempotency row with HTTP 201, and zero participants, responses, awards, award transport orders, ERP external links, and publish or submit audit rows. The draft questionnaire version stays `DRAFT`, disabled, and unpublished. Those identifiers are not printed to CI stdout.

## Acceptance

| Item | Value |
| --- | --- |
| Controller verdict | `ACCEPT_F5_OVERALL_FINAL_ACCEPTANCE` |
| PR | https://github.com/f9999737292-bit/Freihgt-platform/pull/159 |
| Accepted product HEAD | `3c3e37ce8b7d28b17799f20ade8c5c7246282986` |
| Accepted CI | `35886064236` |
| Isolated overall chain | `1 passed` |
| Scoped DB assertions | PASS |
| Full CREATE suite | `11 passed` |
| Skipped | 0 |
| Retries | 0 |

## Out of scope

Product changes in this alignment, ERP, training, TMS, and Award→Transport Order. Frontend Phase 2 stays `IMPLEMENTATION_IN_PROGRESS`. Training stays `NOT_STARTED`.
