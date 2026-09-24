# Marketplace data visibility matrix

Blocking rule: marketplace optimization must not bypass tenant isolation. The matching engine does not run `SELECT * FROM shipment` across tenants. It reads `LoadOpportunity` and `Capacity` rows that an owner explicitly projected.

```text
Tenant shipment / order
        ↓
explicit publication
        ↓
marketplace load projection (allowed fields only)
        ↓
network optimizer
```

## Publication scopes — loads

| Scope | Who may see the projection |
|-------|----------------------------|
| `PRIVATE` | Owning tenant and named internal users |
| `INVITED_CARRIERS` | Named carrier companies |
| `MARKETPLACE` | Carriers allowed on the marketplace surface, with published commercial fields |
| `ANONYMIZED_MARKETPLACE` | Same, without shipper legal name or customer identity; geography at city or zone |
| `NETWORK_OPTIMIZATION_ONLY` | Optimizer and owning tenant; not a human advertisement |

A transport order is not public because it exists.

## Publication scopes — capacity

| Scope | Who may see it |
|-------|----------------|
| `PRIVATE` | Owning carrier tenant |
| `SHIPPER_NETWORK` | Shippers that carrier has admitted |
| `MARKETPLACE` | Marketplace carriers' counterparties per policy |
| `ANONYMIZED` | Equipment, rough location, window; no plate and no driver name |

## Visibility codes

`FULL` own record. `OPS` operational facts required to meet or share a stop. `RATE` commercial terms. `ID` legal identity. `NONE` not visible. `INTERNAL` platform service only.

| Entity | Owning shipper | Other shipper | Candidate carrier | Assigned carrier | Driver | Platform service | Platform admin |
|--------|----------------|---------------|-------------------|------------------|--------|------------------|----------------|
| CargoUnit | FULL | NONE | NONE | OPS of assigned units only | OPS of own task stops | INTERNAL | Audit, not a backdoor export |
| LoadOpportunity | FULL | NONE | Published fields for scope | Published fields | NONE until a task exists | INTERNAL including owner key | Audit |
| Capacity | NONE unless scope includes that shipper | NONE | FULL if they own it; else scope | FULL if they own it | OPS of own assignment | INTERNAL | Audit |
| Match | Own candidates | NONE | Own candidates and own rejects | Own | NONE | INTERNAL | Audit |
| Offer | Offers on own loads, without other shipper secrets inside a co-load | NONE | FULL for own offer | FULL for own offer | NONE | INTERNAL | Audit |
| RoutePlan | Own stops and own cargo actions | OPS of own stops only | FULL for own plan | FULL | Own stop list | INTERNAL | Audit |
| Commercial terms | Own rate | NONE | Rate on the offer they received | Same | NONE | INTERNAL | Audit |
| Shipment | Own tenant shipment | NONE | Own assigned shipment | FULL own execution | Own task | Read via owning tenant API | Audit |
| Counterparty identity | Own parties | NONE | Identity only if scope is not anonymized | Same | Site contact on own stop if operations require it | INTERNAL | Audit |

Platform admin access is an audited break-glass read. It is not a routine marketplace query and it is not how matching works.

## Cross-shipper consolidation

Shipper A, Shipper B, and Shipper C may share one vehicle. The rules below are pairwise. "B" means every other shipper on the same vehicle.

Shipper A must not receive, by default:

- B commercial rate
- B contract
- B customer details beyond the operational facts below
- B internal identifiers
- B tender information

Permitted shared operational data for A:

- that a co-load exists
- shared stop location at the precision required to execute A's cargo (site if A picks or delivers there; otherwise city or zone)
- time window that changes A's arrival
- vehicle equipment and ETA relevant to A's cargo
- A's own weight, volume, and sequence position
- a statement that residual capacity exists, without B's cargo description

Another shipper's cargo description, commodity, and quantities stay hidden unless every participating publication policy sets `share_commodity_with_co_load = true`.

## What each party sees

| Party | Sees | Does not see |
|-------|------|----------------|
| Owning shipper | Own cargo, own rate, own plan stops, co-load existence, shared windows that affect own cargo | Other shippers' rates, contracts, customers, internal ids, tenders |
| Another shipper | Nothing, unless they are a co-load partner, and then only the operational share above | The same commercial and identity fields |
| Candidate carrier | Published fields for the capacity or load scope they are allowed to see | Unpublished shipments, other tenants' source ids |
| Assigned carrier | The accepted plan, equipment, stops they must run, and the rate on their offer | Other shippers' contracts and tenders |
| Driver | Own task stops and the site contact required for those stops | Rates, contracts, other shippers' commercial terms, marketplace search |
| BNO internal service | Owner keys, snapshots, scores, and reject reasons inside the trust zone | A license to query `transport.shipments` across tenants |

## Engine versus humans

| Data | BNO internal service | Carrier | Other shipper |
|------|-----------------|---------|---------------|
| Owner tenant id | yes, inside trust zone | only if scope is not anonymized | no |
| Source shipment id | yes | no | no |
| Unpublished shipment rows | no | no | no |
| Plate number | yes if the capacity owner sent it | yes if they own the capacity; anonymized scope hides it from others | no |
| Rate | yes for scoring the offer | the rate offered to them | no |

## Carrier capacity privacy

A carrier can withhold capacity from the marketplace and still use private-fleet mode inside their tenant. Predicted capacity from a shipment is visible to the owning carrier. It becomes visible to shippers only after a publication scope says so. A prediction is not an advertisement.
