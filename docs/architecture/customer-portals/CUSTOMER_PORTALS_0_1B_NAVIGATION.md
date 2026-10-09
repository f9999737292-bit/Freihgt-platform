# Customer Portals 0.1B — navigation

Information architecture only. No screens are added. A module marked BLOCKED must not be linked, even as a disabled stub that calls the API.

Shared shell regions, same for every customer app: product name, company switcher, locale switcher (`ru-RU` default, `en-US` available), account, and sign out. No control-tower entry. No driver task entry.

## Carrier

`CARRIER-MVP-0.2A` is only: login, authenticated shell, company context, tender inbox, tender detail, bid/response, own award, carrier transport-order list/detail, fleet view, and error/loading/empty states. Company context is the shell company selector. It is not a Company / Users workspace.

| Module | State | Notes |
| --- | --- | --- |
| Dashboard | MVP_SHELL_ONLY | Minimal authenticated landing inside the shell. No KPI cards and no other business widgets in 0.2A. |
| Tenders | MVP | Inbox and detail. Gateway carrier RFx routes. |
| Bids / Responses | MVP | Create, edit, submit. Not buyer evaluation. |
| Awards | MVP | Own award read. |
| Transport orders | MVP | Carrier list and detail from the carrier transport-order routes. |
| Fleet | MVP | View drivers and vehicles. `CARRIER_ADMIN` may perform only the fleet actions the gateway already allows for that code. `CARRIER_DISPATCHER` must not receive admin-only controls. |
| Backhaul / Marketplace | DEFERRED | Network search exists and is carrier-gated. It is not in 0.2A. |
| Settlements | DEFERRED | Not in 0.2A. Payment read does not include `CARRIER_DISPATCHER`. |
| Billing | DEFERRED | Not in 0.2A. |
| Documents | BLOCKED | `CUSTOMER_DOCUMENTS_SAFE=NO` |
| Analytics | DEFERRED | Portal must not define KPIs. Not in 0.2A. |
| Company / Users | DEFERRED | Full company profile and member administration are a later carrier slice. 0.2A keeps only the company selector in the shell. |
| Driver actions | EXCLUDED | `apps/driver-mobile` only. |

## Shipper

| Module | State | Notes |
| --- | --- | --- |
| Dashboard | TARGET | After the carrier MVP. |
| Transport orders | TARGET | `SHIPPER_ADMIN` create. `SHIPPER_LOGIST` read. |
| Shipments | BLOCKED | `SHIPPER_SHIPMENT_INBOX_SAFE=NO` |
| Tenders | TARGET | Admin manage, logist read. |
| Tracking | BLOCKED | `CUSTOMER_TRACKING_SAFE=NO` |
| Documents | BLOCKED | `CUSTOMER_DOCUMENTS_SAFE=NO` |
| Billing | TARGET | Read of billing registers when the shipper wave starts. Not a forwarder receivable. |
| Payments | TARGET | Read of obligations for the shipper codes the gateway already allows. |
| Analytics | DEFERRED | Render only a published KPI id. Do not define it. |
| Company / Users | TARGET | `SHIPPER_ADMIN` manages members. Logist does not. |

## Consignee

```text
CONSIGNEE_PORTAL_IMPLEMENTATION_BLOCKED=YES
```

The target map is frozen so a later wave has a shape. None of it is enabled now.

| Module | State |
| --- | --- |
| Dashboard | BLOCKED |
| Inbound shipments | BLOCKED |
| ETA / Tracking | BLOCKED |
| Delivery slot | BLOCKED |
| Delivery status | BLOCKED |
| Documents | BLOCKED |
| Exceptions | BLOCKED |
| Profile | BLOCKED |

`CONSIGNEE_OPERATOR` is the only live consignee code. `CONSIGNEE_VIEWER` is not seeded.

## Forwarder

The forwarder app, when it exists, shows two commercial columns and one physical column. The columns do not share a settlement or an EDO package.

Customer side:

| Module | State |
| --- | --- |
| Customers | BLOCKED until dual-side actor and customer-side AR exist |
| Customer orders | BLOCKED |
| Customer commercials / receivables | BLOCKED. Owner of the money semantics is Agent G. |

Supply side:

| Module | State |
| --- | --- |
| Carriers | BLOCKED until the supply-side actor exists |
| Procurement / RFx | BLOCKED. `FORWARDER_MANAGER` is buyer-like today. That is not the supply-side product. |
| Carrier orders | BLOCKED |
| Carrier settlements / payables | BLOCKED. Separate from customer receivables. |

Common:

| Module | State |
| --- | --- |
| Execution | BLOCKED until one execution can be shown with two commercial links without merging them. Owner of the physical facts is Agent C. |
| Tracking | BLOCKED. Same participant gate as the other portals. |
| Documents / EDO | BLOCKED. Each row needs a party context. Owner of the chain is Agent B. |
| Finance | BLOCKED. Two ledgers, not one blended balance. Owner is Agent G. |
| Analytics | DEFERRED. No portal-defined KPIs. |

`apps/web-forwarder` is the target package and is not created in 0.1B.
