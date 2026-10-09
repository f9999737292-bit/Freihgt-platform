# Customer Portals 0.1B — gap register

0.1A gaps stay in [CUSTOMER_PORTALS_0_1A_GAPS.md](CUSTOMER_PORTALS_0_1A_GAPS.md). Their descriptions are not rewritten. This file carries them forward and adds the forwarder gaps.

```text
CARRIED_FORWARD_GAPS=23
CARRIED_FORWARD_BLOCKING=13
NEW_FORWARDER_GAPS=5
TOTAL_GAPS=28
BLOCKING_GAPS=17
```

Blocking means the named business model cannot be given to a customer until the gap is closed. Forwarder gaps do not block `CARRIER-MVP-0.2A`.

## Carried forward

| GAP_ID | BUSINESS_MODEL | BLOCKING | OWNER_AGENT | REQUIRED_BEFORE_STAGE |
| --- | --- | --- | --- | --- |
| CP-AUTH-001 | Shipper, Carrier, Consignee | YES | F | CUSTOMER-PORTALS-0.2A for carrier login |
| CP-AUTH-002 | Shipper, Carrier, Consignee | YES | F | 0.2A must follow the 0.1B session policy, not admin localStorage |
| CP-AUTH-003 | Shipper, Carrier, Consignee | YES | A | Before any screen that depends on company-scoped reads the gateway does not enforce |
| CP-SHIPPER-001 | Shipper | YES | F | Shipper wave after carrier MVP |
| CP-SHIPPER-002 | Shipper | NO | F | Shipper role matrix, already frozen in 0.1B |
| CP-CARRIER-001 | Carrier | YES | F | CUSTOMER-PORTALS-0.2A |
| CP-CARRIER-002 | Carrier | NO | F | Boundary already frozen. Driver stays in driver-mobile. |
| CP-CARRIER-003 | Carrier | NO | C | Not in 0.2A |
| CP-CONSIGNEE-001 | Consignee | YES | F | After CP-CONSIGNEE-002 |
| CP-CONSIGNEE-002 | Consignee | YES | C | Before any consignee screen |
| CP-CONSIGNEE-003 | Consignee | YES | A | Before treating `CONSIGNEE_VIEWER` as live |
| CP-SHARED-001 | Shared | NO | F | 0.2A builds the customer shell in `packages/ui` |
| CP-SHARED-002 | Shared | NO | F | 0.2A adds `packages/portal-client` |
| CP-SHARED-003 | Shared | NO | F | 0.2A uses `ru-RU` and `en-US` |
| CP-API-001 | Shipper, Carrier, Consignee | YES | C | Before any shipment inbox |
| CP-API-002 | Carrier | NO | A | Only if a seeded accountant role is required |
| CP-API-003 | Shipper, Carrier, Consignee | NO | C | Before multi-stop customer visibility |
| CP-API-004 | Shipper, Consignee | NO | C | Before disposition or return visibility |
| CP-API-005 | Shipper, Carrier, Consignee | YES | B | Before any document screen |
| CP-API-006 | Shipper, Carrier, Consignee | YES | C | Before any tracking, ETA, or slot screen |
| CP-API-007 | Shipper, Carrier | NO | E | Before a KPI screen |
| CP-TEST-001 | Shipper, Carrier, Consignee | YES | F | 0.2A CI job |
| CP-DEPLOY-001 | Shipper, Carrier, Consignee | YES | A | Carrier image handoff with 0.2A, not in 0.1B |

## CP-FORWARDER-001

| Field | Value |
| --- | --- |
| BUSINESS_MODEL | Forwarder |
| DESCRIPTION | The target app is `apps/web-forwarder`. The directory does not exist. 0.1B chooses it and does not create it. |
| BLOCKING | NO |
| OWNER_AGENT | F |
| EVIDENCE | No `apps/web-forwarder` package. Topology freeze in `CUSTOMER_PORTALS_0_1B_ARCHITECTURE.md`. |
| REQUIRED_BEFORE_STAGE | Forwarder implementation wave, after CP-FORWARDER-002 |

## CP-FORWARDER-002

| Field | Value |
| --- | --- |
| BUSINESS_MODEL | Forwarder |
| DESCRIPTION | `FORWARDER_MANAGER` resolves only as actor kind `BUYER`. Company types `FORWARDER` and `LSP` do the same. There is no supply-side actor. Dual commercial roles are not implemented. |
| BLOCKING | YES |
| OWNER_AGENT | A |
| EVIDENCE | `services/api-gateway/internal/companycontext/actor.go` |
| REQUIRED_BEFORE_STAGE | Any forwarder customer-side or supply-side screen |

## CP-FORWARDER-003

| Field | Value |
| --- | --- |
| BUSINESS_MODEL | Forwarder |
| DESCRIPTION | Customer-side receivables and supply-side payables are not separate forwarder financial chains. A single buyer billing grant is not that split. |
| BLOCKING | YES |
| OWNER_AGENT | G |
| EVIDENCE | Billing and payment policies admit `FORWARDER_MANAGER` as a buyer. No second commercial ledger is exposed for the same company. |
| REQUIRED_BEFORE_STAGE | Forwarder finance navigation |

## CP-FORWARDER-004

| Field | Value |
| --- | --- |
| BUSINESS_MODEL | Forwarder |
| DESCRIPTION | Customer EDO and supply EDO are not two party-scoped chains. Document APIs do not require a commercial party context. |
| BLOCKING | YES |
| OWNER_AGENT | B |
| EVIDENCE | `CUSTOMER_DOCUMENTS_SAFE=NO` from 0.1A, plus no party-context field in the portal document contract. |
| REQUIRED_BEFORE_STAGE | Forwarder documents / EDO navigation |

## CP-FORWARDER-005

| Field | Value |
| --- | --- |
| BUSINESS_MODEL | Forwarder |
| DESCRIPTION | The portal has no read model that shows one physical execution linked to two commercial relationships without merging those relationships. |
| BLOCKING | YES |
| OWNER_AGENT | C |
| EVIDENCE | No customer gateway route for that link. Multi-stop execution itself is still `CP-API-003`. |
| REQUIRED_BEFORE_STAGE | Forwarder execution navigation |

## Count

Forwarder blocking ids: CP-FORWARDER-002, CP-FORWARDER-003, CP-FORWARDER-004, CP-FORWARDER-005.

Carried blocking ids: the 13 listed in the 0.1A count check.

```text
NEW_FORWARDER_GAPS=5
TOTAL_GAPS=28
BLOCKING_GAPS=17
```

## CP-SHIPPER-API-001

Recorded after the 0.1B count above. It does not change those counts. `CP-API-001` stays the historical unscoped shipment-list hole. `CP-SHIPPER-001` stays the portal-screen gap.

| Field | Value |
| --- | --- |
| BUSINESS_MODEL | Shipper |
| DESCRIPTION | Public shipper shipment list and detail require the selected SHIPPER membership and `SHIPPER_ADMIN` or `SHIPPER_LOGIST` on that membership. Legacy tenant-wide shipment GETs are operator-only. |
| BLOCKING | NO |
| OWNER_AGENT | A |
| EVIDENCE | `SHIPPER_COMPANY_CONTEXT_GATEWAY_0_1.md` |
| REQUIRED_BEFORE_STAGE | Closed for the gateway contract. Portal screens remain `CP-SHIPPER-001`. |
