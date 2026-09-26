# RFx v3.0E7 F5 training T1 RU

Russian buyer guide and Russian trainer script for the accepted F5 Create-from-XLSX flow. This wave does not authorize EN, ZH, a UI help link, or product changes.

```
F5_OVERALL_STATUS=IMPLEMENTED_ACCEPTED
TRAINING_DISCOVERY_STATUS=DISCOVERY_ACCEPTED
TRAINING_IMPLEMENTATION_AUTHORIZED=T1_RU_ONLY
TRAINING_T1_RU_STATUS=TEXT_COMPLETE_SCREENSHOTS_BLOCKED
T1_RU_SCREENSHOTS_STATUS=BLOCKED_LIVE_CAPTURE
TRAINING_STATUS=IMPLEMENTATION_IN_PROGRESS
TRAINING_EN_STATUS=NOT_STARTED
TRAINING_ZH_STATUS=NOT_STARTED
TRAINING_T4_UI_STATUS=NOT_STARTED
FRONTEND_PHASE2_STATUS=IMPLEMENTATION_IN_PROGRESS
NEXT_ACTION=CAPTURE_F5_TRAINING_T1_RU_SCREENSHOTS
```

Base is `origin/main` `c38f2d13f08fffe2018d631881ff93b3ad2479f2`, merge of PR #160. Accepted discovery stays in [training discovery](./RFX_V3_0E7_CREATE_FROM_XLSX_TRAINING_DISCOVERY.md). Accepted product evidence stays in [overall final acceptance](./RFX_V3_0E7_CREATE_FROM_XLSX_OVERALL_FINAL_ACCEPTANCE.md).

T1 is not awaiting controller acceptance of a complete wave. Screenshots are a blocker.

## Scope

Allowed text:

- [training index](../training/README.md)
- [Russian user guide](../training/F5_CREATE_FROM_XLSX_RU_USER_GUIDE.md)
- [Russian trainer script](../training/F5_CREATE_FROM_XLSX_RU_TRAINER_SCRIPT.md)

No product frontend, backend, OpenAPI, migrations, CI workflow, browser tests, i18n, or W1/W2/W3/F5 contract edits.

## Source facts used

Screen copy is quoted from `apps/web-procurement/i18n/ru-RU/tenders.json` keys `createFromExcel`, `creationChannel`, and `buyerXlsxCreate`, plus `common.status`, `common.yes`, and `common.no`. Buyer speech uses **тендер**. Codes `SPOT_RFQ`, `FREIGHT`, and `DRAFT` stay codes with a short gloss. Excel channel copy is **«Создан из Excel»** under **«Канал создания»**.

The guide states that Metadata already contains `schema_name` and `schema_version`, and that header-only sheets are Lots, Sections, Questions, Options, and Rules. Support text does not teach the localStorage feature flag. Flag-off remains an unavailable page and HTTP 404 on the backend.

## Deliverables

| Artifact | State |
| --- | --- |
| RU user guide | Written. Usable without pictures. Covers access, template, seven sheets, metadata, preview, double-click, DRAFT, channel, no publish, no participants, no ERP, retry table, CREATE versus F1 |
| RU trainer script | Written. Synthetic company Demo Buyer, number `RFX-DEMO-001`, title «Демонстрационный тендер из Excel». Forbids publish, participants, ERP, real companies, identifier readout, the `Code:` line, and demonstrating F1 as CREATE |
| Screenshot assets | Not created |

## Screenshots

`T1_RU_SCREENSHOTS_STATUS=BLOCKED_LIVE_CAPTURE`

Planned names, not present in git:

- `docs/rfx-v3/training/assets/ru-01-list-button.png`
- `docs/rfx-v3/training/assets/ru-02-create-page.png`
- `docs/rfx-v3/training/assets/ru-03-template-download.png`
- `docs/rfx-v3/training/assets/ru-04-metadata-filled.png`
- `docs/rfx-v3/training/assets/ru-05-preview-ready.png`
- `docs/rfx-v3/training/assets/ru-06-created-draft.png`

Capture was not run. `TEST_DATABASE_URL` was `MISSING` in the agent environment, and web-procurement port 3005 was not listening. Starting a stack or drawing a fake screen is outside this wave. The guide and the index do not link to these files.

## Checks

| Check | Result |
| --- | --- |
| Quoted RU strings against `ru-RU/tenders.json` and `common.json` | Recorded in the T1 commit notes after the local string scan |
| Markdown links | Relative links in this file and the training index point at files that exist |
| PNG files | None. No broken image links |
| PNG signature and EXIF | NOT_RUN. No images |
| Visual screenshot review | NOT_RUN |
| Secrets, UUID samples, tokens | Must be absent from the new docs. Confirmed by the local scan before commit |
| Product, test, workflow diff | None in this wave |
| Guide without images | Required and written that way |

## Exclusions

EN, ZH, T2 terminology review as a separate wave, T4 UI link, T5, ERP, TMS, Award to transport order, and marking Frontend Phase 2 accepted. `TRAINING_STATUS` is not `IMPLEMENTED_ACCEPTED`.
