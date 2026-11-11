# Customer Portals 0.5A — Core pilot path map

Reference map for discovery task 0.5A. See **`CUSTOMER_PORTALS_0_5A_CORE_PILOT_UI_BLOCKERS.md`** for blocker IDs.

## Path steps (intended real pilot)

| Step | User action | App | Route / UI | API transition | UI status |
| --- | --- | --- | --- | --- | --- |
| 1 | Login, pick company | `web-shipper` | `/login` → `company-gate` | `POST /api/v1/auth/login`, `GET /api/v1/users/{id}/companies?tenant_id=` | EXISTS |
| 2 | (Buyer) Create/publish RFx | `web-shipper` | — | `POST /api/v1/rfx-events` (and related) | **MISSING** → use `web-procurement` `/tenders` |
| 3 | (Buyer) Award response | `web-procurement` | `/tenders/[id]/evaluation` | `awardResponse` → gateway award | EXISTS (not shipper) |
| 4 | Login, pick company | `web-carrier` | `/login` → `company-gate` | Same auth/membership pattern | EXISTS |
| 5 | Open tender, submit offer | `web-carrier` | `/tenders`, `/tenders/[id]` | `carrier/rfx-events`, `rfx-responses` submit | EXISTS |
| 6 | View award, open transport orders | `web-carrier` | `/transport-orders` | `carrier/transport-orders`, `order-execution/transport-orders/{id}` | EXISTS (read) |
| 7 | Tracking | `web-carrier` | `/shipments/[id]/tracking` | `/api/v1/carrier/shipments/{id}/tracking?carrier_company_id=` | EXISTS |
| 8 | Documents | `web-carrier` | `/shipments/[id]/documents` | `/api/v1/carrier/documents?…` | EXISTS |
| 9 | Shipper execution visibility | `web-shipper` | `/shipments`, tracking, documents | `/api/v1/shipper/shipments?shipper_company_id=`, … | EXISTS |

## Missing pages (shipper portal only)

| Expected capability | Shipper page | Notes |
| --- | --- | --- |
| Tender list | — | No `pages/tenders/**` |
| Tender create/edit | — | Procurement app owns buyer workspace |
| Evaluation / award | — | `evaluation-award` testids only in procurement |
| Transport order list | — | Shipper sees `transport_order_id` on shipment facts only |
| Finance / billing | — | Out of 0.5A core path |

## Missing API wiring (shipper portal client)

`apps/web-shipper` composable surface (`useShipperApi` / domain modules): **no** RFx, award, or transport-order list helpers. Execution read models (shipments, tracking, documents) are wired.

## Security checks (static + existing E2E)

| Control | Evidence |
| --- | --- |
| No client authority headers | Playwright mocks assert missing `X-Tenant-ID`, `X-User-ID` |
| Company scope on business GETs | `shipper_company_id`, `carrier_company_id` query params from selected membership |
| Cross-company deep link (carrier tracking) | `apps/web-carrier/e2e/pilot-walkthrough.spec.ts` |
| Tampered session company | `reloadAndExpectCompanyGate` in office suites |
| Wrong company type in gate | `shipper-office.spec.ts` filters non-SHIPPER memberships |
| Procurement manager on shipper portal | `canEnterShipperPortal` false — `core-pilot-path-mock.spec.ts` |

## Playwright commands (mock only)

```bash
pnpm --filter @freight-platform/web-shipper exec playwright test e2e/core-pilot-path-mock.spec.ts
pnpm --filter @freight-platform/web-carrier exec playwright test e2e/core-pilot-path-mock.spec.ts
```
