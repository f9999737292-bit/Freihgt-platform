# BNO-0.1B2 reference data and compatibility

BNO-0.1B2 adds versioned reference catalogs and an explainable hard-feasibility engine. It does not search for the next load, rank matches, or assign a physical trailer.

## Ownership

`transport-order-service` remains the cargo master. Planning facts such as pallet count, linear metres, stackability, food-grade requirement, odor, and contamination are nullable columns on `transport.cargoes`. BNO reads a tenant-scoped planning profile and does not copy those facts into a second cargo aggregate.

`shipment-service` remains the vehicle master. Pallet positions, usable linear metres, internal dimensions, food-grade capability, and ADR capability are nullable effective-combination fields on `transport.vehicles`. They describe the combination the vehicle record already represents. They are not a trailer asset, registration number, or VIN.

`network-optimizer-service` owns catalog versions, equipment type profiles, pallet and packaging catalogs, compatibility rule sets, and evaluation.

## Versioning

A catalog version and a rule set have scope `SYSTEM` or `TENANT`, status `DRAFT`, `ACTIVE`, or `RETIRED`, and a monotonic version. One active version exists per catalog kind and scope. An active version is not edited in place. A change is a new draft that is then activated. System rows are seeded with source `SYSTEM_SEED` and are not writable by a tenant.

## Taxonomies

Cargo type codes are stable identities. Display names are not. The seed hierarchy includes general cargo, food and its children, pharma, chemical, and the other structural categories in migration `000077`. A category name does not imply a compatibility decision.

Equipment unit kind is separate from combination type and body type: `TRUCK_BODY`, `TRAILER`, `SEMITRAILER`, `CONTAINER_CHASSIS`, `SWAP_BODY`, `OTHER`. An equipment type catalog row is a reference profile. When a planning default is used because the asset fact is null, provenance is `REFERENCE_DEFAULT`. A measured vehicle value stays `ASSET_CONFIRMED`. A specific cargo fact stays `CARGO_CONFIRMED` and is not replaced by a weaker catalog default.

Aliases resolve only through an explicit alias row. Text such as "реф" is not inferred.

## Physical checks

Weight, volume, pallet positions, linear metres, and loaded height are independent. A pass on one dimension does not pass another. Unknown is not zero, false, or supported. Mixed pallet types are not summed unless an explicit equivalence row gives a positive factor. `stackable=true` does not create extra positions. Height is a single comparison of `max_loaded_height_mm` with internal height. There is no bin packing.

`required_loading_access` means every listed side is mandatory. `allowed_loading_access` means at least one listed side is acceptable. Unloading uses the same pair of semantics. `REAR`, `SIDE`, and `TOP` stay independent.

## Temperature, hygiene, odor, contamination, ADR

One zone needs a non-empty intersection of the cargo ranges. `+2..+8` with `-25..-18` is a hard reject, `TEMPERATURE_RANGES_INCOMPATIBLE`. Overlap `+2..+8` with `+4..+6` is compatible on the intersection `+4..+6`. More than one independently controlled zone, without a zone allocator, is `INDETERMINATE` / `MULTI_ZONE_ALLOCATION_REQUIRED`.

Food-grade, odor, and contamination decisions come from cargo facts plus active rules. A food-grade requirement against capability `false` is incompatible. Unknown capability is indeterminate. No product name is hardcoded.

A regulatory rule requires `source_reference`. If dangerous goods are present and no sourced ADR rule matches, the result is `INDETERMINATE` / `ADR_COMPATIBILITY_RULE_UNAVAILABLE`. This stage does not invent an ADR segregation matrix.

## Rules and groupage

Rule kinds are `CARGO_CARGO` and `CARGO_EQUIPMENT`. Selectors are explicit tokens such as cargo type, parent, tag, odor class, contamination class, and hazard class. There is no expression evaluator.

Precedence is regulatory hard deny, then platform hard deny, then tenant hard deny, then conditions, then allow. A tenant rule may add a restriction. It cannot clear a higher hard deny, and a tenant cannot create a regulatory rule.

Groupage evaluates every cargo against the equipment and every unordered pair. Any hard reject makes the candidate incompatible. Capacity usage reports the sums that were actually known. The same facts and active versions produce the same status, reasons, and fingerprint. Cargo order is canonicalized before the fingerprint.

## Security

Reference reads and evaluation are available to authenticated shipper and carrier roles. Draft rule-set management is limited to `SHIPPER_ADMIN` and `CARRIER_ADMIN`, scoped to the trusted tenant. Tenant A cannot read or change Tenant B rule sets. Internal cargo and vehicle reads stay tenant-scoped and require `X-Internal-Service-Token`. Evaluation does not scan marketplace loads or capacities.
