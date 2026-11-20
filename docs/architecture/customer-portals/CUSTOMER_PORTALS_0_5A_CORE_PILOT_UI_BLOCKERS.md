# Customer Portals 0.5A — Core Pilot UI blockers

**Stage:** `CUSTOMER-PORTALS-CORE-PILOT-PATH-0.5A`  
**Base:** `origin/main` (discovery at branch creation)  
**Mode:** discovery + mock Playwright only — **not** a live staging/backend E2E pilot.

## Scope

End-to-end **business** path for a future real pilot:

**Buyer publish/award → Carrier respond → Transport order → Shipment execution (tracking, documents).**

Customer portals involved: `web-shipper` (:3001), `web-carrier` (:3002). Buyer RFx/award today lives in **`web-procurement`** (:3005), not in the shipper portal shell.

## Core Pilot UI blockers

| ID | Phase | Actor | Surface | Gap | API / transition | Cross-company / tenant | Mock smoke |
| --- | --- | --- | --- | --- | --- | --- | --- |
| **CP05A-B1** | Publish tender / invite | Buyer (`SHIPPER_ADMIN`) | `web-shipper` | **MISSING** — no `/tenders`, no create/publish UI | Gateway has buyer RFx write (`POST /api/v1/rfx-events`, …); **not wired** in `useShipperApi.ts` | N/A (UI absent) | **NOT_RUN** — no shipper UI to mock |
| **CP05A-B2** | Award carrier | Buyer (`SHIPPER_ADMIN`) | `web-shipper` | **MISSING** — no evaluation/award pages | `POST /api/v1/rfx-events/{id}/award`, `POST /api/v1/bids/{id}/accept`; `SHIPPER_LOGIST` **denied** for award (API map) | Award must stay server-authoritative; **no new write routes** in this task | **NOT_RUN** |
| **CP05A-B3** | Buyer RFx (canonical today) | Buyer (`PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, …) | `web-procurement` | **EXISTS** — `/tenders`, `/tenders/[id]/evaluation` | Full RFx studio/evaluation stack | Tenant from JWT; company context in procurement app | Out of shipper/carrier mock scope |
| **CP05A-B4** | Role split | `PROCUREMENT_MANAGER` on SHIPPER company | `web-shipper` vs `web-procurement` | **BLOCKER for “shipper-only” pilot** — `canEnterShipperPortal` rejects `PROCUREMENT_MANAGER` (`portal-client/access.ts`) | Same tenant; different apps | Procurement manager **cannot** use shipper office gate | Covered in `core-pilot-path-mock.spec.ts` (denial) |
| **CP05A-C1** | Tender inbox / respond / submit | Carrier dispatcher | `web-carrier` | **EXISTS** — `/tenders`, `/tenders/[id]` | `GET /api/v1/carrier/rfx-events`, response `POST/PATCH/submit` | `carrier_company_id=co-1`, `participant_company_id`; no authority headers | `core-pilot-path-mock.spec.ts` |
| **CP05A-C2** | Read own award | Carrier | `web-carrier` | **EXISTS** (read-only card) | `GET /api/v1/rfx-events/{id}/own-award` | Scoped to selected carrier company | Same mock suite |
| **CP05A-C3** | Award → transport order (UI) | Carrier | `web-carrier` | **PARTIAL** — UI lists TO after backend creates row; **no UI to create TO from award** | `GET /api/v1/carrier/transport-orders`, `GET /api/v1/order-execution/transport-orders/{id}?actor=CARRIER` | `carrier_company_id` on list | Mock: inbox shows **TO-1** after award mock |
| **CP05A-C4** | TO → tracking / documents | Carrier | `web-carrier` | **EXISTS** — nested under transport order (`open-tracking`, `open-documents`); **not** top-level nav | `/api/v1/carrier/shipments/{id}/…`, `/api/v1/carrier/documents?…` | Deep-link cross-carrier shipment → UI denial (`pilot-walkthrough.spec.ts` B.7) | Mock: tracking + `document-number` in path mock |
| **CP05A-S1** | Login + company gate | Shipper admin/logist | `web-shipper` | **EXISTS** | Login + `GET /api/v1/users/{id}/companies?tenant_id=` | Tamper `selectedCompanyId` → gate on reload (`shipper-office.spec.ts`) | `pilot-walkthrough.spec.ts` + path mock |
| **CP05A-S2** | Shipments / tracking / documents | Shipper | `web-shipper` | **EXISTS** (post-award execution leg only) | `GET /api/v1/shipper/shipments?shipper_company_id=`, tracking suffixes, shipper documents | `shipper_company_id` from `requireCompany()`; spoof rejected | `core-pilot-path-mock.spec.ts` |
| **CP05A-X1** | Single-app Shipper→Carrier story | Both | — | **BLOCKER** — cannot walk publish→award inside `web-shipper` | Manual hand-off: procurement award **or** API/ops seed → both portals see downstream artifacts | Cross-app tenancy assumed same `tenant_id` | No cross-app Playwright in repo |
| **CP05A-X2** | Duplicate carrier tender UX | Carrier | `web-carrier` vs `web-procurement/pages/carrier/**` | **DOC risk** — two UIs for carrier bidding | Same gateway contracts | Pick **web-carrier** as customer portal canonical for 0.5A | — |
| **CP05A-X3** | Live E2E pilot | Both | Staging | **BLOCKER** — mock tests **do not** prove storage, isolation, or multi-service orchestration | Real award → order-execution → shipment propagation | Requires staging seed + security review for any **new** public writes | Explicitly out of scope |

## Login, RFx authority, and portal entry (verified in code)

| Check | Result |
| --- | --- |
| Shipper login + company gate | **PASS** — `pages/login.vue`, `CompanyGate.vue`, `freight_shipper_tab_session` |
| Shipper RFx create UI | **ABSENT** — six pages only; no `rfx`/`tender` in app source (e2e denial case for `PROCUREMENT_MANAGER` only) |
| Shipper RFx gateway roles | `SHIPPER_ADMIN` buyer-manage for create/award; `SHIPPER_LOGIST` read-oriented (see `CUSTOMER_PORTALS_0_1A_API_MAP.md`) |
| Shipper portal roles | `SHIPPER_ADMIN`, `SHIPPER_LOGIST` only (`SHIPPER_OFFICE_ROLES`) |
| Procurement tender manage | `PROCUREMENT_MANAGER`, `SHIPPER_ADMIN`, `SHIPPER_LOGIST`, `FORWARDER_MANAGER` (`tenderWorkspaceActions.ts`) |
| Carrier tender respond/submit | **PASS** — `web-carrier` tender detail testids + `carrier-office.spec.ts` journey |
| Award → TO transition | **Backend-driven**; carrier UI **read** after creation |
| Documents + tracking both sides | **PASS** with company-scoped carrier/shipper routes |
| Cross-company | Client: `assertCompanyInMembership`, `assertScopedCompanyFields`; E2E tamper + wrong-company deep links |
| Cross-tenant | Membership fetch binds `tenant_id` from session; login body tenant (no client `X-Tenant-ID`) |

## Mock smoke (0.5A only)

| Spec | Purpose |
| --- | --- |
| `apps/web-shipper/e2e/core-pilot-path-mock.spec.ts` | Post-award execution leg; asserts **no** tender nav; procurement role blocked from shipper shell |
| `apps/web-carrier/e2e/core-pilot-path-mock.spec.ts` | Tender submit → award read → TO → tracking → documents list (mock gateway) |

These suites use **routed mock API** only. They are **not** live E2E and **not** a substitute for staging pilot.

## Recommended pilot actors (future real E2E)

1. **Buyer award:** `SHIPPER_ADMIN` in **web-procurement** (or ops API), not `web-shipper`.
2. **Carrier bid:** `CARRIER_DISPATCHER` in **web-carrier**.
3. **Shipper visibility:** `SHIPPER_LOGIST` or `SHIPPER_ADMIN` in **web-shipper** after shipment exists.
4. **Seed:** align with `CUSTOMER_PORTALS_0_4B_PILOT_SEED_CHECKLIST.md` plus RFx event + award + TO linkage for staging.

## Verdict

`FINAL_VERDICT=PILOT_UI_PATH_ASSESSED`

Primary merge blockers for a **single shipper-app** core pilot: **CP05A-B1**, **CP05A-B2**, **CP05A-B4**, **CP05A-X1**.
