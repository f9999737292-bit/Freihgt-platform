# Customer Portals 0.1A — current state

Inspected tree: `apps/web-shipper`, `apps/web-carrier`, `apps/web-consignee`, shared frontend packages, `apps/web-admin`, `apps/web-procurement`, `apps/web-finance`, `apps/driver-mobile`, and API gateway route policies. No runtime server was started. No `nuxt build` was executed.

```text
PORTAL_MAY_DEFINE_BACKEND_BUSINESS_FACTS=NO
PORTAL_MAY_DEFINE_ANALYTICS_KPI=NO
PORTAL_MAY_BYPASS_GATEWAY=NO
```

Agent F owns the three customer web apps, their navigation, presentation, API consumption, frontend authorization UX, and portal-local state. Agent F does not own TMS execution facts, EDO facts, NLO planning facts, analytics KPI definitions, billing transactions, identity authority, or gateway security.

## Application inventory

Each app is six source files: `package.json`, `nuxt.config.ts`, `app.vue`, `pages/index.vue`, `server/api/health.get.ts`, `tsconfig.json`, plus a README. There is no `middleware/`, `stores/`, `composables/`, `plugins/`, `components/`, or `tests/` directory.

The page copy states the limit directly: "skeleton UI, no business logic yet."

| Flag | web-shipper | web-carrier | web-consignee |
| --- | --- | --- | --- |
| APP_EXISTS | YES | YES | YES |
| FRAMEWORK | Nuxt 3.15, Vue 3.5 | same | same |
| BUILD_SYSTEM | pnpm workspace, `nuxt build` | same | same |
| DEV_PORT | 3001 | 3002 | 3003 |
| APP_SHELL_IMPLEMENTED | YES | YES | YES |
| NAVIGATION_IMPLEMENTED | NO | NO | NO |
| AUTH_IMPLEMENTED | NO | NO | NO |
| SESSION_HANDLING_IMPLEMENTED | NO | NO | NO |
| TENANT_CONTEXT_IMPLEMENTED | NO | NO | NO |
| RBAC_IMPLEMENTED | NO | NO | NO |
| REAL_API_CLIENT_IMPLEMENTED | NO | NO | NO |
| MOCK_ONLY | NO | NO | NO |
| ERROR_HANDLING_IMPLEMENTED | NO | NO | NO |
| LOADING_STATES_IMPLEMENTED | NO | NO | NO |
| EMPTY_STATES_IMPLEMENTED | NO | NO | NO |
| I18N_IMPLEMENTED | YES | YES | YES |
| RU_LOCALE_PRESENT | YES | YES | YES |
| EN_LOCALE_PRESENT | YES | YES | YES |
| UNIT_TESTS_PRESENT | NO | NO | NO |
| E2E_TESTS_PRESENT | NO | NO | NO |
| PRODUCTION_READY | NO | NO | NO |

`APP_SHELL_IMPLEMENTED=YES` means each app renders `@freight-platform/ui` `AppShell` with a title and a locale switcher. That shell is a header and a main slot. It is not a product navigation shell.

`I18N_IMPLEMENTED=YES` means `@nuxtjs/i18n` is configured through `createFreightI18nOptions()` and the welcome line calls `$t('common.welcome')`. The hint sentence under it is hardcoded English. Shared catalogs contain four keys: `welcome`, `health`, `language`, `home`. Locales on disk are `ru-RU`, `en-US`, and `zh-CN`. Default locale is `ru-RU`.

`MOCK_ONLY=NO` means there is no mock API layer. There is also no real API client. Dependencies declare `@freight-platform/shared-ts` and `@freight-platform/ui`, and the pages do not call either for data.

Health is `GET /api/health` on the Nuxt server, returning `status`, `service`, and `timestamp`. That route is not the API gateway.

Root scripts in `package.json`: `dev:shipper`, `dev:carrier`, `dev:consignee`.

## Shipper functional areas

| Area | Class | Evidence |
| --- | --- | --- |
| AUTH | SKELETON | No login route. Gateway login exists for other apps. |
| DASHBOARD | SKELETON | `pages/index.vue` welcome only. |
| TRANSPORT_ORDERS | BACKEND_READY_FRONTEND_MISSING | Gateway transport-order policies allow `SHIPPER_ADMIN` to create and `SHIPPER_LOGIST` to read. UI is in web-admin and web-procurement. |
| SHIPMENTS | BACKEND_GAP | Create-from-order and cancel are role-gated. List and detail are tenant-wide. |
| TENDERS | BACKEND_READY_FRONTEND_MISSING | `SHIPPER_ADMIN` buyer-manage. `SHIPPER_LOGIST` buyer-read. UI is web-admin and web-procurement. |
| TRACKING | BACKEND_GAP | Gateway tracking routes exist without a participant check. |
| CONTROL_TOWER | OUT_OF_SCOPE | Operator workspace. Shipper role codes are in the control-tower allow list. That does not make it a shipper portal module. |
| DOCUMENTS | BACKEND_GAP | Get-by-id uses the trusted tenant header. List uses query `tenant_id`. |
| BILLING | BACKEND_READY_FRONTEND_MISSING | Billing-register and payment read policies include shipper codes. UI is web-admin and web-procurement. |
| ANALYTICS | BACKEND_GAP | `GET /api/v1/analytics/kpis/{kpiId}` allows shipper codes. KPI definitions are Agent E. |
| PROFILE | SKELETON | No page. |
| COMPANY | BACKEND_READY_FRONTEND_MISSING | Company read for a member. `SHIPPER_ADMIN` can update and manage members. Create and delete are platform admin. |
| USERS | BACKEND_GAP | Company member routes exist for `SHIPPER_ADMIN`. There is no portal user-admin screen and no identity self-service page in this app. |

## Carrier functional areas

`apps/driver-mobile` is a separate Capacitor and Ionic app (`@freight-platform/driver-mobile`). It already calls driver APIs, has its own auth store, and has Vitest coverage. Carrier portal work must not absorb driver stop, POD, delay, or exception commands.

| Area | Class | Evidence |
| --- | --- | --- |
| AUTH | SKELETON | No login route in web-carrier. |
| DASHBOARD | SKELETON | Welcome page only. |
| TENDER_INBOX | BACKEND_READY_FRONTEND_MISSING | `GET /api/v1/carrier/rfx-events/{id}`. Pages exist under `apps/web-procurement/pages/carrier/tenders/`. |
| BIDS | BACKEND_READY_FRONTEND_MISSING | Carrier respond policy. Bid pages exist in web-procurement. |
| AWARDS | BACKEND_READY_FRONTEND_MISSING | `GET /api/v1/rfx-events/{id}/own-award`. |
| SHIPMENTS | BACKEND_GAP | `GET /api/v1/carrier/transport-orders` is role-gated and still not a carrier-company shipment inbox. Shipment detail is tenant-wide. |
| EXECUTION | BACKEND_GAP | Order-execution read and start are carrier-capable. Multi-stop execution is not a public customer route. |
| DRIVERS | BACKEND_READY_FRONTEND_MISSING | `GET/POST /api/v1/drivers` and assign-driver. Create is `CARRIER_ADMIN`. This is fleet administration, not driver mobile. |
| VEHICLES | BACKEND_READY_FRONTEND_MISSING | `GET/POST /api/v1/vehicles` and assign-vehicle. |
| TRACKING | BACKEND_GAP | Same unscoped tracking routes as shipper. |
| BACKHAUL | BACKEND_READY_FRONTEND_MISSING | Network next-load search is carrier-only at the gateway. Presentation must not redefine NLO facts. |
| DOCUMENTS | BACKEND_GAP | Same document tenant split. |
| SETTLEMENTS | BACKEND_GAP | Freight settlements and billing reads include carrier admin and dispatcher. Payment read includes `CARRIER_ADMIN` and unseeded `CARRIER_ACCOUNTANT`, not dispatcher. |
| ANALYTICS | BACKEND_GAP | Analytics read includes carrier admin, dispatcher, and unseeded accountant. Definitions stay with Agent E. |
| PROFILE | SKELETON | No page. |
| COMPANY | BACKEND_READY_FRONTEND_MISSING | `CARRIER_ADMIN` is a company admin role. Dispatcher is not. |
| USERS | BACKEND_GAP | Member management is `CARRIER_ADMIN` only. No portal screen. |

## Consignee functional areas

Consignee access is not implied by `consignee_company_id`. That column is shipment and transport-order data. The only seeded identity code is `CONSIGNEE_OPERATOR`. `CONSIGNEE_VIEWER` is referenced and not seeded.

| Area | Class | Evidence |
| --- | --- | --- |
| AUTH | SKELETON | No login route. Role can exist in identity. The app does not load it. |
| DASHBOARD | SKELETON | Welcome page only. |
| INBOUND_SHIPMENTS | BACKEND_GAP | No consignee-scoped list. |
| SHIPMENT_DETAIL | BACKEND_GAP | Tenant-and-id read only. |
| ETA | BACKEND_GAP | ETA route has no consignee check. |
| TRACKING | BACKEND_GAP | Same. |
| DELIVERY_STATUS | BACKEND_GAP | Status is a shipment field without a consignee projection. |
| DELIVERY_SLOT | BACKEND_GAP | Slot read route exists without participant authorization. |
| DELIVERY_EXCEPTION | BACKEND_GAP | Control tower excludes consignee. Driver exceptions are driver routes. |
| POD | BACKEND_GAP | POD upload is `/api/v1/driver/me/**`. |
| DOCUMENTS | BACKEND_GAP | Shared document API, unsafe list tenant query, no consignee policy. |
| REJECTION | NOT_IMPLEMENTED | Disposition write is driver-only. No consignee read. |
| RETURN_REDIRECT | NOT_IMPLEMENTED | Public authorize and hold routes are not registered. |
| PROFILE | SKELETON | No page. Company read may work later for a member. The portal does not call it. |

Shipment event history is the one gateway policy that names `CONSIGNEE_OPERATOR`. That is not an inbound inbox.

## Shared frontend platform

| Package | What it contains | Portal use today |
| --- | --- | --- |
| `packages/ui` | `AppShell`, `LocaleSwitcher`. Version 0.0.1. | Imported by all three portals and by web-finance. |
| `packages/shared-ts` | `AppHealthResponse`, `formatTimestamp`. | Declared by the three portals. Not imported by their pages. |
| `packages/i18n` | `ru-RU`, `en-US`, `zh-CN`, four `common` keys, cookie `freight_locale`. | Wired by the three portal Nuxt configs. |

```text
SHARED_UI_REUSABLE=YES
SHARED_AUTH_PATTERN_FOUND=YES
SHARED_API_CLIENT_PATTERN_FOUND=NO
SHARED_I18N_REUSABLE=YES
```

Reusable patterns that are not in the shared packages:

| Concern | Where it actually lives |
| --- | --- |
| Design system | `apps/web-admin/assets/css`, `components/ui/UiButton`, `UiInput`, `UiSelect`, `UiTable`, `UiCard`, `UiModal`, `UiBadge`, `UiEmptyState`, `UiPageHeader` |
| Layout with sidebar, breadcrumbs, toasts | `apps/web-admin/components/layout/AppShell.vue` |
| Status badges | Feature folders such as `components/rfx/RfxStatusBadge.vue`, `components/companies/CompanyStatusBadge.vue` |
| Notifications | web-admin toast stack in the admin shell |
| API types | `apps/web-admin/types/**` |
| Authentication | `apps/web-admin/stores/auth.ts`, `middleware/auth.ts`, `pages/login.vue`. web-procurement repeats the same middleware shape |
| Route guards | web-admin `auth`, `guest`, `low-code-admin`, `rfx-buyer-manage`. web-procurement `auth`, `guest`, contract-rate and freight-cost workspace middleware |
| Role guards | `apps/web-admin/composables/usePermissions.ts` |
| Localization of product strings | `apps/web-admin` local i18n, not the four-key shared catalog |
| Error presentation | `apps/web-admin/composables/useApi.ts` `formatApiErrorForUser` |
| Loading and empty states | Admin components including `UiEmptyState.vue` |
| API client | `useApi` plus `utils/buildApiRequestHeaders.ts` in web-admin. Not exported from `packages/shared-ts` |

web-procurement is the closest working customer-shaped frontend. It has login, a session plugin, auth middleware, tender pages, bid pages, and `pages/carrier/tenders/**` plus `pages/carrier/transport-orders/**`. It is still a procurement app, with contract-rate and freight-cost workspaces, not the carrier portal.

web-finance matches the customer-portal skeleton: welcome page, shared `AppShell`, health route, no auth.

web-admin is the operator shell. Its role navigation already lists shipper, carrier, and consignee landing routes. `docs/ROLE_TO_MODULE_ACCESS_MATRIX_V0.1.md` says those roles use web-admin navigation first and the dedicated apps later. That document is a planning matrix. It is not a portal implementation.

## UX inventory

```text
CURRENT_LAYOUT=shared header plus padded main
CURRENT_NAVIGATION=none
CURRENT_COMPONENT_LIBRARY=packages/ui AppShell and LocaleSwitcher only
CURRENT_TYPOGRAPHY=system-ui, sans-serif; header 1.125rem
CURRENT_RESPONSIVE_SUPPORT=header flex wrap only; no breakpoints
CURRENT_MOBILE_SUPPORT=NO
CURRENT_ACCESSIBILITY_SUPPORT=header and main landmarks; locale select has a visible label; no skip link, no nav landmark, no focus order beyond one control
```

Colors are hardcoded `#1a1a1a`, `#f7f8fa`, `#fff`, `#e5e7eb`, `#6b7280`. There is no shared token file in `packages/ui`.

## Topology options for 0.1B

Not selected here.

| Option | Evidence for it | Evidence against freezing it now |
| --- | --- | --- |
| A. One shared portal shell | The three apps already render the same `AppShell`. web-finance does too. Duplicated `app.vue` files differ only by title. | The admin shell that users actually operate is a different component with sidebar and toasts. A shared shell does not exist at that level. |
| B. Three apps, shared packages | Current workspace: separate package names, ports 3001, 3002, 3003, `pnpm-workspace.yaml` `apps/*`. Role docs still name dedicated apps as the later cabinet. | The apps do not yet share auth, tables, or API clients. Choosing B still requires extracting those patterns. |
| C. One role-aware app | `usePermissions.ts` already maps shipper, carrier, and consignee to different nav sets inside web-admin. Planning docs say "web-admin role nav first". | web-admin is the operator and control-tower shell. Folding customers into it mixes operator automation with customer access. web-procurement already split buyer and carrier pages into a second app instead of staying only in web-admin. |

## Build, CI, deployment

| Check | Shipper | Carrier | Consignee |
| --- | --- | --- | --- |
| `package.json` scripts `dev`, `build`, `preview`, `typecheck` | YES | YES | YES |
| Nuxt config | YES | YES | YES |
| Workspace dependency | YES, `pnpm-lock.yaml` importer | YES | YES |
| Lockfile | YES | YES | YES |
| Typecheck script | YES, not executed | YES, not executed | YES, not executed |
| Nuxt health route | YES | YES | YES |
| Dockerfile | NO | NO | NO |
| Compose / staging service | NO | NO | NO |
| CI job | NO | NO | NO |

```text
SHIPPER_BUILD_READY=NO
CARRIER_BUILD_READY=NO
CONSIGNEE_BUILD_READY=NO
SHIPPER_CI_PRESENT=NO
CARRIER_CI_PRESENT=NO
CONSIGNEE_CI_PRESENT=NO
SHIPPER_STAGING_PACKAGED=NO
CARRIER_STAGING_PACKAGED=NO
CONSIGNEE_STAGING_PACKAGED=NO
BUILD_EXECUTED=NOT_RUN
```

CI builds `frontend-web-admin-build` and `frontend-web-procurement-check` in `.github/workflows/ci.yml`. Staging compose for a frontend image is `infrastructure/docker-compose/docker-compose.bintrans-ct-staging-web-admin.yml` and names only `web-admin`. The only app Dockerfile under `apps/` is `apps/web-admin/Dockerfile`.

`BUILD_READY=NO` because no green build is recorded for these apps. The scripts and lockfile entries are present.

## Readiness

Scores are properties of the portal apps, not of backend routes that the apps do not call.

| Domain | Score | Reason, same for all three apps |
| --- | --- | --- |
| FOUNDATION | 25 | Nuxt app, health route, i18n module, shared header shell |
| AUTH | 0 | No login or session |
| RBAC_TENANCY | 0 | No route guard and no tenant store |
| NAVIGATION | 0 | No nav and one page |
| CORE_WORKFLOWS | 0 | Welcome text only |
| TRACKING | 0 | No UI |
| DOCUMENTS | 0 | No UI |
| FINANCE | 0 | No UI |
| ANALYTICS | 0 | No UI |
| TESTING | 0 | No unit or e2e tests |
| DEPLOYMENT | 0 | No CI job, image, or staging service |

Unweighted mean: `25 / 11 = 2.27`, rounded to the nearest integer.

```text
SHIPPER_PORTAL_READINESS=2
CARRIER_PORTAL_READINESS=2
CONSIGNEE_PORTAL_READINESS=2
CUSTOMER_PORTALS_OVERALL_READINESS=2
```

Overall is the same mean. Backend maturity differs by portal and is recorded in the API map. It does not raise a portal that has no screens.

## Product order

```text
RECOMMENDED_FIRST_PORTAL=CARRIER
RECOMMENDED_SECOND_PORTAL=SHIPPER
RECOMMENDED_THIRD_PORTAL=CONSIGNEE
```

Carrier is first because the gateway already has carrier-named routes for RFx, own-award, carrier transport orders, fleet, and next-load search. Seeded roles `CARRIER_ADMIN` and `CARRIER_DISPATCHER` are enforced on those routes. `apps/web-procurement/pages/carrier/**` is a working presentation to study. `apps/driver-mobile` draws a boundary around driver actions, so the carrier portal scope is the company office, not the driver handset.

Shipper is second because buyer routes for transport orders, RFx, shipment create, and billing reads are real, and web-admin plus web-procurement already render them. The surface is wider, `SHIPPER_LOGIST` and `SHIPPER_ADMIN` must stay distinct, and shipment list is not safe to put on a customer home until company scope exists.

Consignee is third because a seeded role is not an inbox. There is no consignee actor kind, no consignee-scoped shipment query, and the viewer role code is not seeded. Tracking and slot routes that ignore the caller would be an unsafe shortcut.

## 0.1B freeze inputs

0.1A does not decide these. Repository evidence leaves more than one valid option except where a flag is already true.

| Topic | What 0.1B must freeze | What is already true |
| --- | --- | --- |
| PORTAL_APP_TOPOLOGY | A, B, or C above | Three app directories exist and are skeletons |
| SHARED_SHELL_POLICY | Whether customer apps use `packages/ui` AppShell, the admin shell, or a new shell | Two shells exist |
| AUTH_FLOW | Login through `POST /api/v1/auth/login` and where the JWT is stored | Gateway JWT is canonical. Portal storage is absent |
| TENANT_CONTEXT | Portal reads tenant from the login response and does not authorize with a client tenant header | Gateway overwrites `X-Tenant-ID` from the JWT |
| ROLE_MATRIX | Which seeded codes open which portal | Matrix in the role document |
| API_GATEWAY_ONLY_POLICY | Browser calls only the gateway | `PORTAL_MAY_BYPASS_GATEWAY=NO` |
| NAVIGATION_MODEL | Per-portal information architecture | No nav exists |
| ERROR_MODEL | How gateway errors are shown | Admin `formatApiErrorForUser` is the only pattern |
| I18N_POLICY | Shared catalogs versus app catalogs. Default remains `ru-RU` unless 0.1B changes it | Shared catalog is four keys |
| RESPONSIVE_POLICY | Desktop, tablet, mobile browser. Driver mobile stays separate | Customer apps have no breakpoints |
| DESIGN_SYSTEM_POLICY | Extract admin UI kit or keep it app-local | Kit is app-local |
| FRONTEND_TEST_POLICY | Unit and e2e bar before a portal is customer-usable | No customer-portal tests |
| DEPLOYMENT_BOUNDARY | Who publishes images and compose. Agent F does not own staging runtime | Only web-admin is packaged |
