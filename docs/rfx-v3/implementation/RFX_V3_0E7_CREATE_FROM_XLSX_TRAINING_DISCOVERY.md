# RFx v3.0E7 F5 Create-from-XLSX training discovery

Discovery and wave plan for user training of the accepted F5 Create-from-XLSX flow. This document does not authorize a course, a help page, or product changes.

```
F5_OVERALL_STATUS=IMPLEMENTED_ACCEPTED
TRAINING_DISCOVERY_STATUS=DISCOVERY_COMPLETE_AWAITING_CONTROLLER_REVIEW
TRAINING_IMPLEMENTATION_AUTHORIZED=NO
TRAINING_STATUS=NOT_STARTED
FRONTEND_PHASE2_STATUS=IMPLEMENTATION_IN_PROGRESS
NEXT_ACTION=INDEPENDENT_CONTROLLER_REVIEW_F5_TRAINING_SCOPE
```

Base is `origin/main` `57bc501614f8aca981b7353182898861c4d8b6da`, merge of PR #159. Accepted product evidence stays in [overall final acceptance](./RFX_V3_0E7_CREATE_FROM_XLSX_OVERALL_FINAL_ACCEPTANCE.md).

## Source facts

Facts below are taken from the accepted UI, i18n, and CREATE contract on this base. Training copy must follow these strings. It must not invent a second vocabulary.

### Who sees Create from Excel

The tender list shows **Create from Excel** / **Создать из Excel** / **从 Excel 创建** only when the Excel feature flag is on and the session role is one of `PLATFORM_ADMIN`, `PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, or `FORWARDER_MANAGER`. That is the UI BuyerManage set in `canShowBuyerXlsxCreateEntry`. Other roles do not see the list button.

The page `/tenders/new-from-xlsx` still exists for a signed-in user. If the flag is off or the role is outside that set, the page shows `tenders.buyerXlsxCreate.unavailable` and **Back to manual creation** / **К ручному созданию** / **返回手动创建**, which goes to `/tenders/new`. The blank-template button is not rendered in that state.

Backend template download and CREATE preview/commit stay behind the accepted BuyerManage policy. Flag-off template download is HTTP 404. Training must say the feature is unavailable, not that the account is broken.

### Path and outcome

1. From the tender list, **Create from Excel** opens `/tenders/new-from-xlsx`.
2. The page title is **Create tender from Excel** / **Создать тендер из Excel** / **从 Excel 创建招标**.
3. The hint already says the flow previews findings and creates a new draft, and that it does not publish the tender or add participants.
4. **Download blank template** saves `bintrans-rfx-buyer-xlsx-v1-create-template.xlsx`. Download does not create a tender.
5. The buyer fills metadata on the page and uploads that file, or another compatible `.xlsx`.
6. **Preview** is one multipart POST. A ready preview enables **Create tender**.
7. Create sends one commit whose body is only the current preview id. A second click while create is in progress is ignored, and the button stays disabled after success.
8. Success redirects to `/tenders/{event_id}`. The event remains `DRAFT` with `creation_channel=EXCEL`.
9. The flow does not publish, does not add participants, and does not call ERP.

Manual creation remains `/tenders/new`. Updating an existing draft remains F1 on `/tenders/:id` (`UPDATE_EXISTING_DRAFT`). CREATE is `CREATE_NEW_DRAFT` on `/tenders/new-from-xlsx`. Training must keep those three paths separate.

### Workbook

The blank workbook has seven sheets, in this order: `Instructions`, `Metadata`, `Lots`, `Sections`, `Questions`, `Options`, `Rules`. Data sheets are headers only. Instructions describe CREATE upload. They do not describe automatic publication or participant creation. Empty lots, sections, and questions are valid for a new draft.

Only `.xlsx` is accepted. The maximum file size is 5 MiB. The RU size string uses **МиБ**. EN and ZH use **MiB**.

### Metadata

Required on the form and in the preview multipart body:

| Field | UI default on this base |
| --- | --- |
| `owner_company_id` | Current buyer company, labeled with the existing owner-company string |
| `rfx_number` | A generated `XLSX-` suffix the buyer can replace |
| `title` | Empty until the buyer types it |
| `rfx_type` | `SPOT_RFQ`; the select lists the product type codes |
| `category` | `FREIGHT`; the select lists the product category codes |

Optional, sent only when filled: `description`, `response_deadline`, `currency_code`.

The form does not send tenant id, publish, participants, or ERP fields.

Changing the file or any of those metadata fields clears the current preview and disables **Create tender** until Preview runs again. The status line is **Preview was cleared. Run preview again.** / **Предпросмотр сброшен. Запустите его снова.** / **预览已清除，请重新预览。**

### Findings and errors

Preview shows findings as translated sentences plus a sheet/row location when the product has one. Error findings also show a **Code:** line. Buyer and trainer materials must not copy that code, a hash, a formula, a token, a company id, or a request key. Teach the sentence above the code. Screenshots must crop or redact the code line.

`ready_to_commit` false disables **Create tender**. The status is **Preview found problems. Create tender is disabled.** Warnings can appear on a preview that is still ready to create. Training must say a warning is not the same as a blocked create.

Safe retry, matching the client:

| Situation | What the buyer sees | Safe repeat |
| --- | --- | --- |
| No file, wrong extension, missing metadata | Client message before any request | Fix the form, then Preview |
| File over 5 MiB, HTTP 413 | Workbook is larger than 5 MiB | Use a smaller `.xlsx` |
| Preview HTTP 422 or not ready | Workbook is readable but not ready to create a tender | Fix the workbook, Preview again |
| Preview expired or already used, HTTP 409 | Run preview again | Preview again; do not reuse the old result |
| Stored preview no longer matches or failed revalidation | Run preview again | Preview again |
| Create conflict with another request | This create request conflicts with another request | Do not hammer Create; Preview again if the draft was not created |
| HTTP 403 | No permission | Stop; ask an administrator |
| HTTP 404 on create | Create from Excel is unavailable | Stop; feature or route is off |
| HTTP 404 on template download | Blank template is unavailable, or this feature is turned off | Stop |
| HTTP 401 on template download | An active session is required | Sign in again, then download |
| HTTP 401 on preview or create | Generic temporary-unavailable sentence | Sign in again. The preview/create classifier does not use the template session sentence |
| HTTP 429 | Too many Excel requests | Wait, then retry. Create may be retried; Preview should be run again if it failed |
| HTTP 5xx or network loss | Temporarily unavailable | Create may be retried when a ready preview is still on screen. If preview failed, run Preview again |
| Empty or invalid template bytes | The template file was empty or invalid | Download again |

## Existing materials

| Location | What it is | Fit for F5 training |
| --- | --- | --- |
| `docs/LOW_CODE_PILOT_*_QUICK_GUIDE_V0.1.md` | Operator quick guides: audience, who can use, numbered steps, exclusions | Pattern only. Different product. Do not copy them into RFx or duplicate that tree |
| `docs/rfx-v3/implementation/` | Accepted contracts and this discovery | Source of facts, not a buyer course |
| `apps/web-procurement/i18n/{ru-RU,en-US,zh-CN}/tenders.json` `buyerXlsxCreate` | Live RU, EN, and ZH screen copy | Canonical UI wording. A course must quote it, not replace it |
| Create page hint and template hint | Already state draft-only, no publish, no participants | Reuse; do not contradict |
| In-app help route or contextual help panel | None for this flow | Gap |

There is no RFx training course, screenshot set, or trainer script on this base.

## Gap analysis

| Topic | Product today | Training gap |
| --- | --- | --- |
| Who sees the entry | Role set plus feature flag | No guide |
| Flag off and wrong role | Unavailable card and manual-wizard link | No guide |
| Where the page is | `/tenders/new-from-xlsx` from the tender list | No guide |
| Blank template | Download button and seven-sheet file | No sheet-by-sheet buyer explanation |
| Required and optional metadata | Form fields and defaults | No field guide that avoids internal ids in prose |
| `.xlsx` and 5 MiB | Client and server limit | Mentioned only as the size label |
| Preview and findings | Translated sentences, warnings versus blocking errors | No worked example |
| Preview invalidation | File or metadata change clears preview | Status string only |
| Double-click protection | Button disables while creating and after success | No trainer note |
| DRAFT and Excel channel | Commit result and tender page | No buyer explanation of the channel label |
| No publish, no participants, no ERP | Hint text and accepted overall chain | Easy to confuse with F1 and with the manual wizard |
| Manual wizard | Back link to `/tenders/new` | Not contrasted in a course |
| Errors and retry | Locale strings above | No single retry table for buyers or support |
| CREATE versus F1 | Separate pages and modes | No side-by-side lesson |
| RU/EN/ZH course | UI strings exist in all three | No reviewed course; ZH and EN must not lead |

## Audiences

| Audience | Needs | Must not receive |
| --- | --- | --- |
| Buyer / procurement user | Steps, sheet meaning, metadata, preview, create, draft result, safe errors | Internal policy names, codes, hashes, tokens, company ids, request keys |
| Buyer administrator / support | Same steps plus flag-off, role denial, 401/403/404/429/5xx, and when to ask the buyer to Preview again | A dump of internal codes. Support notes may say a code line can appear and must be redacted in tickets |
| Internal operator / trainer | Script, demo order, screenshot rules, what the demo must not do | Permission to publish, add participants, or open ERP during the demo |

## Curriculum

Twenty lessons. T1 writes them in RU only. Later waves translate after review.

1. Who sees **Create from Excel**.
2. BuyerManage roles and the unavailable state when the feature flag is off.
3. Where `/tenders/new-from-xlsx` is opened from the tender list.
4. Downloading the blank template, and that download creates nothing.
5. The seven sheets and the rule that data sheets start as headers.
6. Required metadata: owner company, RFx number, title, type, category.
7. Optional description, response deadline, and currency.
8. `.xlsx` only, and the 5 MiB limit.
9. Upload and Preview. Preview does not create the tender.
10. Findings: blocking errors, warnings, and the safe sentence to read.
11. Why a file or metadata change clears the preview.
12. **Create tender** and the double-click guard.
13. The result stays a draft.
14. The tender was created from Excel.
15. Nothing is published automatically.
16. No participants are added.
17. No ERP call is made.
18. Returning to the manual wizard.
19. 401, 403, 404, 413, 422, 429, and 5xx, and the safe repeat from the table above.
20. CREATE versus F1 update of an existing draft.

## Language strategy

1. RU is the first and only implementation wave after this discovery is accepted.
2. EN starts only after the RU guide and trainer script are accepted.
3. ZH starts only after RU is accepted and the glossary below is confirmed.

Do not machine-translate unconfirmed legal or product terms. Quote the existing locale strings when they already exist. Leave product codes such as `SPOT_RFQ`, `FREIGHT`, and `DRAFT` as codes, with a confirmed gloss beside them.

## Glossary plan

These pairs need human confirmation before EN or ZH course text is written. The RU and EN columns quote current UI where it exists. ZH quotes current UI where it exists. Blank cells are not translations.

| Concept | RU in product | EN in product | ZH in product | Needs confirmation |
| --- | --- | --- | --- | --- |
| List action | Создать из Excel | Create from Excel | 从 Excel 创建 | Whether trainers say Excel or XLSX in speech |
| Page title | Создать тендер из Excel | Create tender from Excel | 从 Excel 创建招标 | тендер versus RFx versus закупка |
| Draft | черновик | draft / DRAFT | 草稿 | Whether the status code stays visible |
| Preview | Предпросмотр | Preview | 预览 | — |
| Findings | Замечания | Findings | 检查结果 | ошибка versus предупреждение in speech |
| Lots | Лоты | Lots | 标段 | — |
| Sections | Разделы | Sections | 章节 | — |
| Questions | Вопросы | Questions | 问题 | — |
| Create action | Создать тендер | Create tender | 创建招标 | — |
| Manual return | К ручному созданию | Back to manual creation | 返回手动创建 | — |
| Size unit | МиБ | MiB | MiB | Do not normalize RU to MiB |
| Carrier | Перевозчик | Carrier | 承运商 | Used only to say participants are not added |
| Excel channel | — | `EXCEL` on the tender | — | F4 display label in RU and ZH |
| BuyerManage | Not a UI label | Not a UI label | Not a UI label | Teach the four role names only to support |

`Knockout` stays an untranslated product word where the evaluation UI already does that. It is out of this course except as a term not to redefine.

## Screenshots

Follow the quick-guide habit of a named demo, without copying low-code content.

- One RU desktop sequence in T1: list button, page, template button, filled metadata, preview ready, draft tender.
- Synthetic title and RFx number. No real company name, no account id, no token, no request key, no workbook bytes in the doc.
- Crop the **Code:** line and any owner-company value that is an identifier. Company may appear as a display name only if the fixture name is synthetic.
- File name of the blank template may be shown. File contents of Instructions may be paraphrased. Do not paste formulas or internal payloads.
- Alt text is in the language of that guide.
- EN and ZH screenshots wait for T3, after the RU set is accepted.
- Mobile screenshots are a T4 question, not a T1 requirement.

## Accessibility

The page already moves focus to the error region and to the preview region. T1 text must be usable without screenshots: each step names the visible label. Do not use color as the only distinction between a warning and a blocking error. T4, if authorized later, checks keyboard order from the list button through file, Preview, and Create tender, and checks that the unavailable state still exposes the manual-creation button.

## Recommended architecture

**Combined, with repository Markdown as the only source of truth.**

T1 adds a RU user guide and a trainer script under `docs/rfx-v3/training/`, using the quick-guide shape: audience, who can use it, numbered steps, explicit non-goals. That folder does not exist yet and must not be created in this discovery. It is the RFx place for learner text. It does not replace `docs/rfx-v3/implementation/`, and it does not move the low-code guides.

T4, only after RU acceptance, may add one link from `/tenders/new-from-xlsx` to the published guide. The link is a pointer. The Markdown file remains the source. No second copy in Vue, and no translated prose that diverges from i18n.

| Option | Discoverability | RU/EN/ZH care | Versioning | Screenshots | Accessibility | Browser tests | Drift risk |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Markdown only | Low for buyers | Strong; reviewable diffs | Strong; git | Easy to redact | Text stands alone | Docs checks only | Low if copy quotes i18n |
| In-app help page only | High | Hard; three locales in UI | Tied to releases | Harder to review | Must be built | New page tests | High |
| Contextual help only | Medium | Same as in-app | Same | Easy to omit | Easy to hide from keyboard | Coupled to the panel | High |
| **Markdown source plus a later link** | High after T4; trainers have the file from T1 | Strong | Strong | Redacted in git | Text first; link is extra | T4 checks the link, not a second course | Low if the link does not embed a copy |

## Rejected alternatives

- In-app help as the only course. Review and redaction are weaker, and this discovery does not authorize UI work.
- Contextual help as the only course. Buyers who cannot open the panel, and trainers preparing offline, have no artifact.
- Markdown with no future link. Acceptable for T1 and T2. It is not the end state, because buyers will not find a repository file.
- Translating EN and ZH in the same wave as RU. Rejected until RU wording is accepted.
- Putting the course in `docs/LOW_CODE_PILOT_*`. That tree is a different product.

## Wave plan

Each wave needs its own controller review. None of them starts from this discovery.

| Wave | Scope | Deliverables | Acceptance | Tests | Exclusions | Depends on |
| --- | --- | --- | --- | --- | --- | --- |
| T1 | RU buyer guide and RU trainer script for the twenty lessons | `docs/rfx-v3/training/` RU guide and trainer script | Quotes live RU strings; covers the twenty topics; no secrets or codes; draft-only, no publish, no participants, no ERP | `git diff --check`, link check, secret scan, terminology check against `ru-RU/tenders.json` | EN, ZH, UI, screenshots of real tenants | Accepted discovery |
| T2 | RU read-through with product and support | Review notes on the glossary rows marked for confirmation | Confirmed or explicitly deferred terms; no silent translation | Review checklist | New product behavior | T1 |
| T3 | EN, then ZH | Locale guides that quote existing i18n | No new legal terms; ZH only after glossary confirmation | Same docs checks per locale | UI changes | Accepted T2 |
| T4 | One help link and accessibility pass | Link from the create page to the RU guide; keyboard notes | Link target is the Markdown source; unavailable state remains reachable | Focused browser check of the link and keyboard path; no product mocks | A second copy of the course in Vue; ERP; TMS | Accepted T3 for any non-RU link text; RU link may follow accepted T2 if the controller allows |
| T5 | Final training acceptance | Acceptance note on the roadmap | All authorized locales match the UI; screenshots redacted; implementation still does not change CREATE behavior | Docs gate plus the T4 browser check if T4 was authorized | Feature work | T1–T4 as authorized |

T4 is the only wave that touches UI, and only a link. It stays unauthorized until its own review.

## Acceptance matrix

| Check | This discovery | Later course |
| --- | --- | --- |
| F5 overall stays accepted | Required | Required |
| Training implementation not started | Required | T1 is the first implementation, after a separate accept |
| Twenty topics listed | Required | Each topic present in the RU guide |
| Three audiences | Required | Each artifact names its audience |
| RU before EN before ZH | Required | Wave order |
| No secrets or machine-code catalogue | Required | Required |
| CREATE versus F1 | Required | Side-by-side lesson |
| Controller review | This document | Every wave |

## Risks

- The UI shows a **Code:** line. A screenshot or a copied sentence can leak an internal code into the course. Mitigation: redact, and teach the sentence.
- Preview/create HTTP 401 uses the generic unavailable string, while template download has a session sentence. A guide that says both screens use the same 401 text would be false.
- RU **МиБ** versus EN/ZH **MiB** will look inconsistent if a translator normalizes them.
- Role names in the UI set are not the words BuyerManage. A buyer guide that says BuyerManage will not match the screen.
- F1 and CREATE share workbook sheets. Training that says the blank template updates an existing tender is wrong.
- Flag-off is a hidden button plus an unavailable page, not an error toast on the tender list.

## Out of scope

Training implementation, help UI, product code, OpenAPI, tests, workflows, migrations, ERP, TMS, Award to transport order, carrier Excel training, and marking Frontend Phase 2 accepted.

## Open decisions

1. Confirm the spoken RU term for the object: тендер, as in the UI, or another legal term.
2. Confirm the F4 RU and ZH label for `creation_channel=EXCEL` before lesson 14 is illustrated.
3. Confirm whether T4 may ship a RU link before EN and ZH guides exist.
4. Confirm the support rule for the **Code:** line: redact always, including internal tickets.
5. Confirm the demo company display name. It must be synthetic.
