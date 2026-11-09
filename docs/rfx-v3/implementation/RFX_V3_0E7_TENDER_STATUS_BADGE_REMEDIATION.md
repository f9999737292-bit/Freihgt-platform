# RFx tender status badge remediation

```text
RFX_TENDER_STATUS_BADGE_REMEDIATION_STATUS=IMPLEMENTED_AWAITING_CONTROLLER_REVIEW
TRAINING_T1_RU_STATUS=BLOCKED_BY_PRODUCT_UI_DEFECT
T1_RU_SCREENSHOTS_STATUS=FIVE_VALID_ONE_BLOCKED
PR_161_STATUS=OPEN_DRAFT_DO_NOT_MERGE
NEXT_ACTION=INDEPENDENT_CONTROLLER_REVIEW_RFX_TENDER_STATUS_BADGE_REMEDIATION
```

## Reproduction

On the isolated CREATE browser stack, a buyer completed download, upload, preview, and create. The API event was `status=DRAFT` and `creation_channel=EXCEL`, and the browser redirected to `/tenders/{event_id}`. The overview row «Статус» was empty. The live DOM was:

```html
<badge status="DRAFT"></badge>
```

The status value reached the template attribute. The element had no text.

## Root cause

`components/ui/Badge.vue` accepts `status: string` and an optional `tone`. Its template renders that status as text inside `<span class="ui-badge">`. Nuxt auto-import registers the file as `UiBadge`, not `Badge`. `pages/tenders/[id]/index.vue` used `<Badge :status="event.status" />` without an import, so Vue treated it as an unknown HTML element.

`pages/tenders/[id]/evaluation.vue` already imports `~/components/ui/Badge.vue` explicitly. The tender card did not. The CREATE browser harness sets `NUXT_E2E_DISABLE_SSR=true`, so this DOM was produced by the client render. No separate SSR warning log was retained.

The badge prints the status code. Tender i18n does not replace `DRAFT` with another label on this card. The visible contract is `DRAFT`.

The same unresolved `<Badge>` pattern exists on other procurement pages, including the tender list. Those pages are outside this fix. The authorized change is the created-tender card.

## Fix

`pages/tenders/[id]/index.vue` now imports the existing Badge component, matching the evaluation page. No second component, registry change, backend change, or API change.

## Evidence

Before the import, `tests/tenderStatusBadge.dom.test.ts` failed with `expected '' to be 'DRAFT'`. After the import, the same test expects the resolved `.ui-badge` text `DRAFT` and rejects an unknown `<badge>` element.

The CREATE live spec and overall-chain spec, after the redirect, expect:

- `.ui-badge` text `DRAFT` inside `tender-status`;
- no unknown `badge` element;
- `tender-rfx-number` equal to the created number;
- `tender-creation-channel` text `Created from Excel` under the suite locale `en-US`.

Those specs keep `retries: 0` and `forbidOnly: true`. They do not use `page.route`. Local live browser execution is `NOT_RUN` without a safe `TEST_DATABASE_URL`. CI job `rfx-buyer-xlsx-create-browser-e2e` is the live proof.

## Relation to PR #161

PR #161 stays the Russian training draft. This remediation does not add screenshots and does not edit that branch. Screenshot capture stays stopped until this fix is accepted and merged.
