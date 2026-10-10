# Customer Portals 0.1B — architecture freeze

Policy only. This document does not add apps, routes, migrations, or staging. 0.1A readiness stays 2% for shipper, carrier, and consignee. Implementation order stays carrier, then shipper, then consignee. Forwarder is added as a fourth business model, not as a reason to reopen that order.

Baseline: `origin/main` `c050c3a38f1474dbef736cc0b1ee37e5426d06eb`.

```text
CUSTOMER_PORTALS_ARCHITECTURE_FROZEN=YES
PORTAL_APP_TOPOLOGY=SEPARATE_ROLE_APPS_WITH_SHARED_CUSTOMER_PLATFORM
SHIPPER_PRODUCT_SURFACE=YES
CARRIER_PRODUCT_SURFACE=YES
CONSIGNEE_PRODUCT_SURFACE=YES
FORWARDER_PRODUCT_SURFACE_REQUIRED=YES
SEPARATE_WEB_FORWARDER_APP=YES
FORWARDER_IS_TWO_SIDED_MARKET_ACTOR=YES
FORWARDER_DUAL_SIDE_BACKEND_READY=NO
ONE_EXECUTION_CAN_HAVE_MULTIPLE_COMMERCIAL_RELATIONSHIPS=YES
CUSTOMER_SETTLEMENT_SEPARATE_FROM_SUPPLY_SETTLEMENT=YES
CUSTOMER_EDO_CHAIN_SEPARATE_FROM_SUPPLY_EDO_CHAIN=YES
EDO_PARTY_CONTEXT_MANDATORY=YES
DRIVER_PORTAL=NO
DRIVER_SURFACE=apps/driver-mobile
CUSTOMER_PORTAL_IS_WEB_ADMIN=NO
SHARED_CUSTOMER_SHELL=packages/ui customer shell
SHARED_API_CLIENT=packages/portal-client
SHARED_DESIGN_SYSTEM=packages/ui customer tokens and components
JWT_TENANT_IS_AUTHORITY=YES
CLIENT_TENANT_IS_AUTHORITY=NO
PORTAL_MAY_BYPASS_GATEWAY=NO
PORTAL_MAY_DEFINE_BACKEND_BUSINESS_FACTS=NO
PORTAL_MAY_DEFINE_ANALYTICS_KPI=NO
CURRENT_MVP_SESSION_POLICY=TAB_SESSION_STORAGE_ACCESS_TOKEN
TARGET_SESSION_POLICY=HTTPONLY_COOKIE
MIGRATION_REQUIRED_LATER=YES
SHIPPER_SHIPMENT_INBOX_SAFE=YES
CONSIGNEE_PORTAL_IMPLEMENTATION_BLOCKED=YES
CUSTOMER_TRACKING_SAFE=NO
CUSTOMER_DOCUMENTS_SAFE=NO
RECOMMENDED_FIRST_PORTAL=CARRIER
CARRIER_MVP_FIRST_SLICE=login, authenticated shell, company context, tender inbox, tender detail, bid/response, own award, carrier transport-order list/detail, fleet view, error/loading/empty states
CUSTOMER_PORTAL_CI_REQUIRED=YES
```

## Business models

Four customer business models are distinct. They are not skins of one another.

| Model | Who it serves | Must not be |
| --- | --- | --- |
| Shipper | The cargo owner who buys transport | A forwarder alias or a procurement-only role |
| Carrier | The company that performs transport | A driver handset, and not a forwarder with fewer buttons |
| Consignee | The receiver of the goods | A row implied by `consignee_company_id` |
| Forwarder / LSP | A two-sided market actor | A carrier with extra buttons, a shipper alias, or `PROCUREMENT_MANAGER` |

```text
FORWARDER_IS_TWO_SIDED_MARKET_ACTOR=YES
ONE_EXECUTION_CAN_HAVE_MULTIPLE_COMMERCIAL_RELATIONSHIPS=YES
```

Customer side: shipper or customer to forwarder. Supply side: forwarder to carrier. One physical execution may sit under both commercial relationships. This freeze does not create those relationships in the database.

Driver stays `apps/driver-mobile` and `/api/v1/driver/me/**`. `DRIVER` does not enter a customer web app. `web-admin` stays the operator and control-tower workspace.

## Commercial document and finance boundary

The two commercial chains may point at the same physical `TransportExecution`. They do not share one settlement package or one EDO package.

```text
CUSTOMER_SETTLEMENT_SEPARATE_FROM_SUPPLY_SETTLEMENT=YES
CUSTOMER_EDO_CHAIN_SEPARATE_FROM_SUPPLY_EDO_CHAIN=YES
EDO_PARTY_CONTEXT_MANDATORY=YES
```

A forwarder screen may show both sides in one session. It must label the party context on every document, signature, invoice, and settlement. It must not merge customer-side receivables with supply-side payables into one total, one package, or one signature ceremony.

Ownership, without defining the other agents' rules:

| Agent | Owns |
| --- | --- |
| F | Portal UX, navigation, presentation, client state |
| B | EDO lifecycle, signatures, packages, operator evidence |
| G | AR/AP, billing, accounting documents, financial semantics |
| C | Physical execution facts |

The portal does not define execution facts, KPI formulas, EDO validity, or accounting entries.

## Topology

Chosen: `SEPARATE_ROLE_APPS_WITH_SHARED_CUSTOMER_PLATFORM`.

Rejected: one role-aware customer app. Rejected: folding customers into `web-admin`. Rejected: hosting Forwarder inside `web-shipper` or `web-carrier`.

Why this one:

- `apps/web-shipper`, `apps/web-carrier`, and `apps/web-consignee` are already separate Nuxt packages on ports 3001, 3002, and 3003.
- `web-admin` is the operator shell. `CUSTOMER_PORTAL_IS_WEB_ADMIN=NO`.
- Forwarder is two-sided. Putting it in the carrier app or the shipper app would collapse the business model this freeze exists to protect.
- One combined customer app would ship four trust boundaries and both commercial sides in one origin and one bundle. Role checks would live mostly in the client.
- `web-procurement` remains reference evidence for tender and carrier screens. It is not a customer portal and it is not the forwarder app.

Deployment: each business model is its own image and compose service when that model is released. Shared packages are build dependencies, not a runtime service. The first image is the carrier app. Consignee and forwarder images wait on their gates. Agent A owns staging and release. Agent F does not deploy.

Security: a carrier origin does not contain shipper create flows or forwarder receivables. The gateway remains the authorization authority. Separate apps reduce the blast radius of a client bug. They do not replace gateway RBAC.

Reuse: shell, tokens, forms, tables, i18n, and the portal API client are shared. Feature pages stay in the app that owns the business model.

Role isolation: each app allows only its entry roles, listed below. A token for another model is a forbidden state in that app, not a hidden navigation set.

Forwarder placement: future `apps/web-forwarder`. Not created in 0.1B.

```text
SEPARATE_WEB_FORWARDER_APP=YES
FORWARDER_PRODUCT_SURFACE_REQUIRED=YES
```

`YES` is the frozen target. It is not an instruction to add the directory in this stage.

## Shared customer platform

Do not copy `apps/web-admin` into the customer apps. Admin layout, control tower, low-code, and operator automation stay in `web-admin`.

| Concern | Frozen home |
| --- | --- |
| Customer shell, nav primitives, tokens, forms, tables, modals, badges, empty, loading, error, toast | `packages/ui` |
| Locale catalogs and Nuxt i18n options | `packages/i18n` |
| Generic types: API error, session snapshot, company selection | `packages/shared-ts` |
| Gateway HTTP client | `packages/portal-client` |

```text
SHARED_CUSTOMER_SHELL=packages/ui customer shell
SHARED_API_CLIENT=packages/portal-client
SHARED_DESIGN_SYSTEM=packages/ui customer tokens and components
```

`packages/portal-client` is a package boundary only. This stage does not add the package. The client may call the API gateway and nothing else. It attaches `Authorization` from the session policy. It may attach `X-Company-ID` only as the user's selected company. It must not treat `X-Tenant-ID`, `X-User-ID`, `X-User-Email`, or browser-supplied role codes as authority.

The existing `packages/ui` `AppShell` is a header and a slot. The customer shell replaces that skeleton for customer apps. It is not `apps/web-admin/components/layout/AppShell.vue`.

## Authentication

```text
PORTAL_MAY_BYPASS_GATEWAY=NO
JWT_TENANT_IS_AUTHORITY=YES
CLIENT_TENANT_IS_AUTHORITY=NO
```

The browser talks only to the API gateway. Domain service hosts are not customer origins.

Identity today is `POST /api/v1/auth/login`. The body includes `tenant_id`, `email`, and `password` because `identity-service` requires that body (`auth_handler.go`). That field selects the account to authenticate. It is not a tenant override after login. The response is JSON: `access_token`, `token_type`, `expires_in`, `user`. There is no `Set-Cookie` and no refresh token.

After login, the gateway strips client `X-Tenant-ID`, `X-User-ID`, and `X-User-Email` and writes them from the JWT. The portal must not send those headers as a source of truth.

`X-Company-ID` is a selected company. It is not authorization. The gateway must revalidate membership. Shipment list, shipment detail, and tracking do not do that today. Those screens stay out of customer portals until they do.

Portal behavior:

| Condition | Portal behavior |
| --- | --- |
| Token missing | Show login. Do not call product APIs. |
| Token expired or rejected | Clear the tab session. Return to login. Do not reuse a stored tenant as a substitute credential. |
| Membership missing | Stay authenticated. Block product routes. Show an empty company state. Do not invent a company id. |
| Selected company invalid | Drop the selected company. Stay on the company gate. Do not retry with another id chosen by the client. |
| Role insufficient | Stay in that app. Show a forbidden state. Do not switch the user into another portal. |
| Upstream unavailable | Show an unavailable state. Do not present cached business records as live. |

## Session policy

Evidence on this baseline:

| App | Storage |
| --- | --- |
| `web-admin` | `localStorage` key `freight_admin_session`, plus tenant and company keys |
| `web-procurement` | `localStorage` key `freight_procurement_session`, plus tenant and company keys |
| `driver-mobile` | Capacitor Preferences, with `sessionStorage` as fallback |

Customer web apps must not copy the admin localStorage keys and must not use the driver preference store.

`localStorage` keeps a bearer token across browser restarts and is readable by any script on the origin. `sessionStorage` keeps it for one tab and clears it when the tab closes. A memory-only token cannot survive reload, and identity has no refresh grant to restore it. An HttpOnly cookie is the target and does not exist on login today.

```text
CURRENT_MVP_SESSION_POLICY=TAB_SESSION_STORAGE_ACCESS_TOKEN
TARGET_SESSION_POLICY=HTTPONLY_COOKIE
MIGRATION_REQUIRED_LATER=YES
```

MVP stores only the access token and the server-returned user snapshot in `sessionStorage` for that tab. Company selection for the tab may sit beside it and is still not authority. Tenant id in storage is display context copied from the login response, never a header the client relies on for authorization.

The later move to an HttpOnly cookie and a refresh or session endpoint is identity and gateway work. Agent F does not invent that API here.

## Role entry

| Model | Live entry roles | Excluded |
| --- | --- | --- |
| Shipper | `SHIPPER_ADMIN`, `SHIPPER_LOGIST` | Do not collapse these codes. Logist is not buyer-manage for RFx and is not transport-order create at the gateway. |
| Carrier | `CARRIER_ADMIN`, `CARRIER_DISPATCHER` | `DRIVER`. `CARRIER_ACCOUNTANT` is not seeded. |
| Consignee | `CONSIGNEE_OPERATOR` | `CONSIGNEE_VIEWER` is referenced and not seeded. It is not a live role. |
| Forwarder | `FORWARDER_MANAGER` | Not `PROCUREMENT_MANAGER`, not `SHIPPER_ADMIN`, not `CARRIER_ADMIN` |

`FORWARDER_MANAGER` currently returns actor kind `BUYER` in `services/api-gateway/internal/companycontext/actor.go`, in the same arm as shipper and procurement roles. Company type `FORWARDER` and `LSP` also fall through to `BUYER`. There is no supply-side actor for the same user.

```text
FORWARDER_DUAL_SIDE_BACKEND_READY=NO
```

That limitation does not remove Forwarder from the product model. It blocks forwarder commercial screens until Agent A can represent both sides and Agents B, G, and C expose the matching chains.

## Company context

```text
TENANT_CONTEXT != COMPANY_CONTEXT
```

| Concept | Meaning in the portal |
| --- | --- |
| Tenant | Isolation boundary inside the JWT. The portal does not choose it after login. |
| Company | A membership inside the tenant. The user may have more than one. |
| Role | A seeded code on the membership. It decides which app and which actions are eligible. |
| Business actor | Shipper, carrier, consignee, or forwarder. Not the same string as company type. |
| Commercial relationship | Customer-side or supply-side contract between two companies. Independent of the physical execution. |

A tenant may contain many companies. The portal must not set tenant equal to company. The selected company is sent only as `X-Company-ID` and only after a server-returned membership list. The gateway revalidates it.

## Safety gates carried from 0.1A

The block below is the 0.1A snapshot and is not rewritten. The current shipper inbox gate is `SHIPPER_SHIPMENT_INBOX_SAFE=YES` in the freeze above and in `SHIPPER_COMPANY_CONTEXT_GATEWAY_0_1.md`.

```text
SHIPPER_SHIPMENT_INBOX_SAFE=NO
CONSIGNEE_PORTAL_IMPLEMENTATION_BLOCKED=YES
CUSTOMER_TRACKING_SAFE=NO
CUSTOMER_DOCUMENTS_SAFE=NO
```

Shipment list and detail were tenant-scoped, not company-scoped or consignee-scoped (`CP-API-001`, `CP-CONSIGNEE-002`). A customer portal must not render a tenant-wide shipment list. The shipper customer contract is now the company-scoped gateway route in `SHIPPER_COMPANY_CONTEXT_GATEWAY_0_1.md` (`CP-SHIPPER-API-001`). Legacy `GET /api/v1/shipments` remains an operator path.

Consignee implementation stays blocked until a consignee authorization principal, a consignee-scoped inbound list, a safe detail read, and safe tracking or ETA participation checks exist. `consignee_company_id` does not open that gate.

Tracking, ETA, and slot reads required a JWT and did not prove the caller participates in the shipment (`CP-API-006`). That historical hole stays open for carrier and consignee. The shipper customer contract is the membership-gated gateway route in `SHIPPER_TRACKING_GATEWAY_0_1.md`. `CUSTOMER_TRACKING_SAFE=NO_GLOBAL_SHIPPER_ONLY`.

Document get-by-id uses the trusted tenant header. Document list requires query `tenant_id`, and several writes take `tenant_id` from the body (`CP-API-005`). No customer document screen until list and write ignore caller-supplied tenant.

## Design language

One customer design language, implemented later in `packages/ui`. Not the admin control-tower chrome.

| Rule | Freeze |
| --- | --- |
| Viewports | Desktop, tablet, and mobile web. Driver native UI stays in `apps/driver-mobile`. |
| Keyboard | Every action in the customer shell is reachable by keyboard. |
| Focus | Visible focus on links, buttons, inputs, and dialogs. |
| Loading | A skeleton for page structure, a spinner only for a single action. Do not mix both on the same region. |
| Empty | A named empty state with the reason. Not a blank table. |
| Error | Inline for a field or a block. A toast only for a completed or failed action that does not replace the page. |
| Toast | Success and failure of an action. Not a substitute for a 403 or an unavailable page. |
| Tables | Horizontal overflow stays inside the table region. The page itself does not grow a second scrollbar for columns. |
| Forms | Show field errors from the gateway validation payload. Disable submit while the request is in flight. |
| Language | `ru-RU` is primary. `en-US` is supported. `zh-CN` stays in the shared catalog and is not a 0.2A launch locale. |

## Test and CI gate

A customer portal change is not mergeable without its own CI job.

```text
CUSTOMER_PORTAL_CI_REQUIRED=YES
```

The job for an app that has product code must include unit tests, component tests where the UI branches, API contract tests against the gateway error and auth shapes, auth guard tests, role tests, and tenant or company spoof tests. Critical journeys need Playwright: for the carrier MVP that is login, company selection, tender inbox, response submit, and own-award read.

0.1B itself is docs-only. It does not add that job. The first implementation wave does.

## Deployment handoff

Agent F prepares the app. Agent A publishes and runs it. Agent F does not deploy staging.

Handoff for each released app:

| Item | Requirement |
| --- | --- |
| Dockerfile | App image, not the web-admin image |
| Health | Existing Nuxt `GET /api/health` kept as the process health |
| Build | `pnpm --filter <package> build` |
| Runtime env | Public gateway base URL only. No domain-service URLs. No secrets in the client bundle. |
| Image manifest | Entry owned by Agent A |
| Compose | Service handoff owned by Agent A |
| Smoke | Login page and health route |

## Carrier first slice

`CARRIER-MVP-0.2A` is the first implementation wave. It is only:

login and tab session, authenticated customer shell, company context, tender inbox, tender detail, bid or response flow, own award, carrier transport-order list and detail, fleet view, and error, loading, and empty states.

Not in that slice: tracking, settlements, documents, analytics, Control Tower, backhaul, driver actions, and any finance beyond what those excluded modules are. `CARRIER_DISPATCHER` and `CARRIER_ADMIN` stay distinct. Dispatcher does not see fleet create if the gateway still reserves create for `CARRIER_ADMIN`.

Shipper shipment inbox, consignee portal, and forwarder commercial screens are outside 0.2A.

## What 0.2A may assume

The architecture above is frozen. These are not frozen as built software: `packages/portal-client`, the customer shell components, `apps/web-forwarder`, consignee authorization, company-scoped shipment reads, participant-scoped tracking, and trusted document list.
