# Customer Portals main baseline 0.2

Snapshot of the customer portals on current `origin/main`. This file records what is already on that commit. It does not add screens, document writes, signing, or QES. The historical 0.1A and 0.1B blocks in [README.md](README.md) stay as written.

```text
AGENT=F
STAGE=CUSTOMER-PORTALS-MAIN-BASELINE-0.2
BASE_MAIN_SHA=2a99b615a237b6d1f6e6038ca86a7433c567dc78
BASE_MAIN_SUBJECT=Merge pull request #321 from f9999737292-bit/feat/bintrans-autopilot-queue-v0.3b-r4
BASE_MAIN_DATE=2026-10-31 20:06:46 +0300
BRANCH=docs/customer-portals-main-baseline-v0.2
WORKTREE=D:\Projects\freight-platform-wt\customer-portals-main-baseline-v0.2
SHIPPER_STATUS=IMPLEMENTED
CARRIER_STATUS=IMPLEMENTED
CONSIGNEE_STATUS=BLOCKED
FORWARDER_STATUS=NOT_IMPLEMENTED
FINANCE_STATUS=NOT_IMPLEMENTED
SHIPPER_OFFICE_ON_MAIN=YES
CARRIER_OFFICE_ON_MAIN=YES
CONSIGNEE_PORTAL_IMPLEMENTATION_BLOCKED=YES
FORWARDER_APP_PRESENT=NO
FORWARDER_DUAL_SIDE_BACKEND=BLOCKED
FINANCE_PORTAL_SKELETON=YES
PACKAGES_PORTAL_CLIENT_PRESENT=YES
SHIPPER_CI_ON_MAIN=SUCCESS
CARRIER_CI_ON_MAIN=SUCCESS
CONSIGNEE_CI=NOT_IMPLEMENTED
FINANCE_CI=NOT_IMPLEMENTED
MAIN_PUSH_CI_RUN=41222922070
MAIN_PUSH_CI_CONCLUSION=success
MAIN_PUSH_CI_JOBS=75
MAIN_PUSH_CI_SUCCESS=75
CUSTOMER_TRACKING_SAFE=NO_GLOBAL_SHIPPER_AND_CARRIER_ONLY
CUSTOMER_DOCUMENTS_SAFE=NO_GLOBAL_SHIPPER_AND_CARRIER_READ_ONLY
CONSIGNEE_TRACKING=BLOCKED
CONSIGNEE_DOCUMENTS=BLOCKED
DOCUMENT_WRITES=NOT_IMPLEMENTED
EDO_SIGNING=NOT_IMPLEMENTED
QES=NOT_IMPLEMENTED
EXTERNAL_MAP_PROVIDER=NOT_IMPLEMENTED
SHIPPER_DOCUMENT_WRITE_SAFE=NO
CARRIER_DOCUMENT_WRITE_SAFE=NO
SHIPPER_EDO_SIGNING_SAFE=NO
CARRIER_EDO_SIGNING_SAFE=NO
SHIPPER_QES_SAFE=NO
CARRIER_QES_SAFE=NO
CP_API_006_CONSIGNEE=OPEN
CP-CONSIGNEE-002=BLOCKED
CP-CONSIGNEE-003=BLOCKED
PR_13_CONTINUED=NO
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
STAGING_CHANGED=NO
DOCS_ONLY=YES
```

## Offices on this commit

| App | State on `2a99b615` | Latest commit touching the app |
| --- | --- | --- |
| `apps/web-shipper` | Login, tab session, company context, read-only shipment inbox and detail, tracking, ETA, slots, and shipment-scoped documents. Gateway only. | `13a54d04` `2026-10-11 14:42:56 +0300` `fix(shipper): keep company switch on documents and reject cross-shipment documents` |
| `apps/web-carrier` | Login, fleet, tenders, transport orders, tracking, ETA, slots, and read-only documents opened from a transport-order shipment reference. | `14c3fb50` `2026-10-28 13:40:50 +0000` `fix(web-carrier): sync tampered-session e2e on membership refresh` |
| `apps/web-consignee` | `BLOCKED`. Welcome page only. `pages/index.vue` says skeleton UI, no business logic yet. | `f0f3e3b1` `2026-06-21 16:30:54 +0300` `chore: snapshot freight platform runtime and observability work` |
| `apps/web-finance` | `NOT_IMPLEMENTED`. Welcome page only. `pages/index.vue` says skeleton UI, no business logic yet. | `f0f3e3b1` `2026-06-21 16:30:54 +0300` `chore: snapshot freight platform runtime and observability work` |
| `apps/web-forwarder` | `NOT_IMPLEMENTED`. Directory absent. `apps/` has `web-shipper`, `web-carrier`, `web-consignee`, `web-finance`, `web-admin`, `web-procurement`, and `driver-mobile`. | Not applicable |

`apps/web-shipper/README.md` matches the shipper office. `apps/web-carrier/README.md` still says "Nuxt 3 frontend skeleton" while the carrier pages above are on the same commit. The page tree is the office record. This baseline does not edit that README.

Shipper pages on this commit:

- `pages/login.vue`
- `pages/index.vue`
- `pages/shipments/index.vue`
- `pages/shipments/[id]/index.vue`
- `pages/shipments/[id]/tracking.vue`
- `pages/documents/index.vue`

Carrier pages on this commit:

- `pages/login.vue`
- `pages/index.vue`
- `pages/fleet/index.vue`
- `pages/tenders/index.vue`
- `pages/tenders/[id].vue`
- `pages/transport-orders/index.vue`
- `pages/transport-orders/[id].vue`
- `pages/shipments/[id]/tracking.vue`
- `pages/shipments/[id]/documents.vue`

Document and tracking behavior stays the behavior already recorded in [SHIPPER_DOCUMENTS_UI_0_1.md](SHIPPER_DOCUMENTS_UI_0_1.md), [CARRIER_DOCUMENTS_UI_0_2D.md](CARRIER_DOCUMENTS_UI_0_2D.md), [CUSTOMER_PORTALS_0_2C_SHIPPER_TRACKING_UI.md](CUSTOMER_PORTALS_0_2C_SHIPPER_TRACKING_UI.md), and [CARRIER_TRACKING_UI_0_1.md](CARRIER_TRACKING_UI_0_1.md).

`packages/portal-client` is present (`client.ts`, `session.ts`, `access.ts`, `headers.ts`). Both portal CI jobs typecheck and test `@freight-platform/portal-client` before the app build.

## CI

Push of `2a99b615a237b6d1f6e6038ca86a7433c567dc78` is GitHub Actions run [41222922070](https://github.com/f9999737292-bit/Freihgt-platform/actions/runs/41222922070). Conclusion `success`. 75 jobs, 75 `success`.

| Job | Result on that run | What the workflow runs |
| --- | --- | --- |
| `frontend-web-shipper-check` | `success` | portal-client typecheck and test; web-shipper typecheck, test, and build; Playwright Chromium `pnpm exec playwright test` in `apps/web-shipper` |
| `frontend-web-carrier-check` | `success` | portal-client typecheck and test; web-carrier typecheck, test, and build; Playwright Chromium `pnpm exec playwright test` in `apps/web-carrier` |
| `frontend-web-consignee-check` | `NOT_IMPLEMENTED`. Absent from `.github/workflows/ci.yml` | none |
| `frontend-web-finance-check` | `NOT_IMPLEMENTED`. Absent from `.github/workflows/ci.yml` | none |

`SHIPPER_BUILD_READY=NO` and `CARRIER_BUILD_READY=NO` in the 0.1A block were the discovery record before these jobs existed. On this commit both apps are built by the jobs above, and those jobs succeeded.

## Consignee is BLOCKED

`CONSIGNEE_STATUS=BLOCKED`. `CONSIGNEE_PORTAL_IMPLEMENTATION_BLOCKED=YES`.

`CP-CONSIGNEE-002=BLOCKED`. `services/api-gateway/internal/companycontext/actor.go` on this commit defines `ActorBuyer` and `ActorCarrier` only. `buyerCompanyTypes` is `SHIPPER`, `FORWARDER`, and `LSP`. `carrierCompanyTypes` is `CARRIER`. There is no consignee actor kind.

`CP-CONSIGNEE-003=BLOCKED`. No file under `infrastructure/migrations` on this commit contains `CONSIGNEE_VIEWER`.

`CONSIGNEE_TRACKING=BLOCKED`. `CONSIGNEE_DOCUMENTS=BLOCKED`. `CP_API_006_CONSIGNEE=OPEN`. Shipper and carrier tracking gateway routes are the recorded customer tracking surface.

## Forwarder is NOT_IMPLEMENTED

`FORWARDER_STATUS=NOT_IMPLEMENTED`. `apps/web-forwarder` is absent. This baseline does not create that app.

`FORWARDER_DUAL_SIDE_BACKEND=BLOCKED`. `DeriveActorKind` returns `BUYER` for `FORWARDER_MANAGER` and for company types `FORWARDER` and `LSP`. `ExecutionActorFromMembership` returns `CARRIER` or `BUYER`. There is no supply-side forwarder actor.

## Finance is NOT_IMPLEMENTED

`FINANCE_STATUS=NOT_IMPLEMENTED`. `apps/web-finance/pages/index.vue` is the welcome skeleton. `FINANCE_CI=NOT_IMPLEMENTED`.

## Unfinished customer gates

On this commit, for the shipper and carrier offices that already exist:

- `DOCUMENT_WRITES=NOT_IMPLEMENTED`
- `EDO_SIGNING=NOT_IMPLEMENTED`
- `QES=NOT_IMPLEMENTED`
- `EXTERNAL_MAP_PROVIDER=NOT_IMPLEMENTED`
- `CONSIGNEE_TRACKING=BLOCKED`
- `CONSIGNEE_DOCUMENTS=BLOCKED`

`CUSTOMER_TRACKING_SAFE` and `CUSTOMER_DOCUMENTS_SAFE` stay limited to shipper and carrier.

Shipper document tests reject write and signing controls. The shipper office README states that document writes, signing, and map providers are not included.

## Pull request #13

PR #13, `feat(driver-mobile): native Android/iOS + responsive qualification v0.9.6`, remains `OPEN` on `feat/driver-mobile-native-qualification-v0.9.6` (`8383ebf25b694abd2bc459f3834e825afc7045bc`). That ref is 1497 commits behind `origin/main`. Updated `2026-08-16T18:57:55Z`. This baseline does not continue it. `apps/driver-mobile` on main is outside this slice.

## This slice

Allowed output is this file. Product code, OpenAPI, migrations, CI, staging, and the autopilot tree are unchanged. The consignee office stays `BLOCKED` on `CP-CONSIGNEE-002` (owner C) and `CP-CONSIGNEE-003` (owner A). The forwarder app stays `NOT_IMPLEMENTED`.
