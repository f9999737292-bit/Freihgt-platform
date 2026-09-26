# NLO-0.3 residual capacity model

Baseline: `origin/main` `6d47cc92`.

## CargoUnit projection

The planning `CargoUnit` is a projection. It is not a new cargo master. Nullable fields stay null. Null is `UNKNOWN`. Null is not zero and not false.

The projection preserves, when present on `transport.cargoes` after `000077` or on the BNO load `CargoConstraints`:

weight, volume, pallet count, pallet type, linear metres, max loaded height, stackable, fragile, cargo type code, packaging type code, food grade, temperature interval, dangerous goods, hazard class, odor emission, odor sensitivity, contamination, required and allowed loading access, required and allowed unloading access.

`transport.cargoes` weight and volume are `gross_weight` and `volume` (`000003`). Item rows add `weight` and `volume` but have no tenant column of their own. A projection that cannot tie an item to one cargo unit leaves that item out rather than summing it into an authoritative total.

`stackable=true` does not create pallet positions. `fragile=true` does not by itself hard-reject. A rule must say so. There is no 3D placement claim.

Confirmed occupancy comes from shipment-service through `ShipmentOnboardCargoProvider`. Network-optimizer-service is not the system of record for physical load or unload. Only `CONFIRMED_ONBOARD` units enter the subtraction. `PLANNED` cargo does not. Shipment status `LOADED`, a cargo linked to the shipment, and a planned quantity are not that proof.

## Residual formula

For one dimension:

`residual = effective vehicle capacity − confirmed onboard occupancy`

only when both sides are known and expressed in the same unit.

| Input | Residual |
| --- | --- |
| vehicle total unknown | `UNKNOWN` |
| onboard occupancy unknown | `UNKNOWN` |
| both known and occupancy exceeds total | `HARD_REJECT` for any added load; the current set is already infeasible |
| both known and occupancy is within total | the non-negative remainder, provenance `DERIVED_FROM_CONFIRMED_CARGO` |

Do not subtract a known weight from an unknown volume and present a mixed remainder as measured capacity. Dimensions are independent. If a dimension is relevant to the cargo and equipment and either the capacity total or the occupancy is unknown, that dimension is `UNKNOWN`. A required unknown dimension blocks `FEASIBLE`. Payload does not prove volume, pallets, linear metres, or height. NLO-0.3B–D have no weight-only full-feasibility path.

Pallet positions use an explicit positive `pallet_equivalences` row from B2. A missing factor for a known pallet type leaves pallet residual `UNKNOWN`. Mixed types are not converted by a hardcoded EUR ratio. Unknown pallet count is not zero pallets.

Linear metres use `vehicles.usable_linear_meters` minus confirmed cargo `linear_meters`. A null on either side leaves linear residual `UNKNOWN`. Height compares `max_loaded_height_mm` with vehicle internal height when both are present. Otherwise height is `UNKNOWN`. Height is not floor placement.

## Provenance

| Provenance | Use |
| --- | --- |
| `ASSET_CONFIRMED` | Vehicle column entered as the asset capability |
| `EXECUTION_CONFIRMED` | Execution event states what is onboard |
| `DERIVED_FROM_CONFIRMED_CARGO` | Residual arithmetic from the two confirmed sides |
| `REFERENCE_DEFAULT` | Catalog default. Never shown as measured residual |
| `UNKNOWN` | Missing fact |

A reference equipment class may explain a compatibility check. It may not fill `payload_remaining_kg`.

## Per-dimension freeze

| Dimension | Residual fact | Unknown result |
| --- | --- | --- |
| payload kg | `payload_remaining_kg` | `UNKNOWN`, not a pass |
| volume m3 | `volume_remaining_m3` | `UNKNOWN` |
| pallet positions | `pallet_positions_remaining` | `UNKNOWN` |
| linear metres | `usable_linear_meters_remaining` | `UNKNOWN` |
| height | comparison only | `UNKNOWN` |
| temperature zones | not a residual number | multi-zone stays unallocated |
| ADR, food grade, odor, contamination | capability and rules, not a remainder | `INDETERMINATE` or rule result |
| loading and unloading access | set intersection of required and allowed | missing access is `INDETERMINATE` |

Access values are `REAR`, `SIDE`, and `TOP`, evaluated independently. One method does not satisfy another. For a current trip, every planned stop that must still load or unload has to preserve the required access. With only one origin and one destination, "every stop" is that pair plus any planning-only insertion stops. The execution model still has no stop rows.

## What residual is not

It is not `PredictedCapacity.capacity_weight_kg`. That value is the vehicle total reserved for the time after unload.
