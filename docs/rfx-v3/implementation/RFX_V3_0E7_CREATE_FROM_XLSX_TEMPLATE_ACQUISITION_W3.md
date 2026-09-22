# RFx v3.0E7 F5 XLSX Template Acquisition W3

Live browser acceptance for the blank CREATE XLSX download. This wave does not change the W1 endpoint or the W2 frontend.

```
F5_TEMPLATE_ACQUISITION_W1_STATUS=IMPLEMENTED_ACCEPTED
F5_TEMPLATE_ACQUISITION_W2_STATUS=IMPLEMENTED_ACCEPTED
F5_TEMPLATE_ACQUISITION_W3_STATUS=IMPLEMENTATION_IN_PROGRESS
F5_OVERALL_STATUS=IMPLEMENTATION_IN_PROGRESS
FRONTEND_PHASE2_STATUS=IMPLEMENTATION_IN_PROGRESS
```

No controller verdict is recorded here. W3 does not accept F5 overall or Frontend Phase 2.

## Gate

The scenario runs inside the existing job `rfx-buyer-xlsx-create-browser-e2e`. The stack is production api-gateway, rfx-service, Postgres, and web-procurement. Identity and company HTTP stubs stay the existing harness stubs. Specs do not intercept product endpoints. Playwright retries stay 0 and `forbidOnly` stays true. Failure artifacts upload only when the job fails.

The Go harness runs one Playwright test, `blank template download writes nothing`, and then compares SQL counts for `rfx_events`, `rfx_import_analyses`, `rfx_participants`, `rfx_responses`, `rfx_awards`, `transport_orders`, and `rfx_idempotency_records`. That isolated download must leave those counts unchanged. The full suite then runs, including download followed by preview. The analysis count is expected to rise only after that preview. Commit of the blank template is not part of the W3 scenario.

## Live checks

On `/tenders/new-from-xlsx`, with BuyerManage and the Excel flag on, the button is visible before a file is chosen. A double click sends one `GET /api/v1/rfx-events/xlsx-create/template` with no body, query, event id, `tenant_id`, `owner_company_id`, or `Idempotency-Key`. The response is HTTP 200, XLSX MIME, `Content-Disposition` attachment, `Cache-Control: no-store`, and `X-Content-Type-Options: nosniff`. Playwright records a download named `bintrans-rfx-buyer-xlsx-v1-create-template.xlsx`. The saved bytes are non-empty and start with the XLSX ZIP local header.

The same saved file is uploaded through the CREATE form. Preview is multipart HTTP 200, `CREATE_NEW_DRAFT`, ready to commit, with empty lots and questionnaire and no event binding. A separate case previews an existing workbook, downloads the blank template, and checks that the selected file, metadata, findings, `analysis_id`, and commit control stay in place. No second preview or commit is sent. The commit Idempotency-Key is not rendered; it stays tied to the unchanged `analysis_id`, and no commit request is issued.

`SHIPPER_LOGIST` and the carrier role do not see the button, and a raw GET returns 403. Flag-off hides the button and a raw GET returns 404. An unauthenticated raw GET returns 401. RU, EN, and ZH show the localized button name and no raw i18n key. The button is native, `type=button`, focusable, and exposes `aria-busy` while the request is in flight. Success uses the polite live region.

`/tenders` keeps only Create from Excel. `/tenders/new` still opens the manual wizard. `/tenders/:id` still shows the F1 export control and does not show the blank-template button.

A live download error is not produced here. Forcing 401, 403, 404, or 429 would require intercepting the product endpoint or hiding the button. Those states remain covered by the W2 component tests.

## Out of scope

W1 download handler, W2 frontend behavior, OpenAPI, RBAC, feature-flag middleware, route registry, parser and commit semantics, migrations, ERP, training, TMS, and Award→Transport Order.
