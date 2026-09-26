# Consolidation model

`CONSOLIDATION ENGINE` decides which cargo units may travel together. It is a logical component of the one optimization core, not a second product.

## Pipeline position

Hard constraints run before scoring. Scoring never repairs a hard failure.

NLO-0.3A proposes the first implementation waves as feasibility only, pending controller acceptance. BNO-0.1C2 `MatchScore` stays one capacity plus one load. It is not applied to a cargo set. A consolidation score, if authorized later, is a separate profile and is not frozen here. Multi-stop results stay `PROPOSED` planning records. They do not become an active shipment. See [NLO_0_3_CONSOLIDATION_FEASIBILITY.md](NLO_0_3_CONSOLIDATION_FEASIBILITY.md).

```text
cargo units + capacity + route skeleton
        ↓
hard constraints
        ↓
HARD_REJECT  or  feasible set
        ↓
score only the feasible set
```

## Hard constraints

Any one of these, when required data is present and violated, yields `HARD_REJECT`:

- payload
- volume
- pallet positions
- linear metres
- cargo incompatibility
- ADR
- temperature
- equipment / body
- pickup reachability
- delivery reachability
- pickup windows
- delivery windows
- vehicle access (including city-rule access)
- required documents
- carrier permissions

If the data needed to evaluate a hard constraint is missing, the result is `INDETERMINATE`, not a pass. NLO-0.3B, NLO-0.3C, and NLO-0.3D do not use `FEASIBILITY_PARTIAL`. A required unknown fact cannot become `FEASIBLE`, and a weight-only result is not full feasibility while volume, pallets, linear metres, or height remain unknown and required. Any later advisory partial-evidence mode is future, non-executable, not authorized, and not `FEASIBLE`. It is outside NLO-0.3B–D.

## Patterns

| Pattern | Meaning |
|---------|---------|
| same-origin / same-destination | Co-load on one O–D |
| multi-pick / one-drop | Several pickups, one delivery |
| one-pick / multi-drop | One pickup, several deliveries |
| multi-pick / multi-drop | Both |
| hub consolidation | Deliver into a hub, then a linehaul leg |
| cross-dock | Short dwell; inbound and outbound legs; no execution automation in the first implementation |

## Residual capacity

Current-trip fill, when later authorized, uses remaining payload, volume, pallet positions, and linear metres after cargo confirmed onboard. Each dimension is independent. An unknown required dimension stays `UNKNOWN` and the candidate stays `INDETERMINATE`. Never coerce an unknown value to zero, and never treat a weight-only remainder as full feasibility.

## Load order and unloading feasibility

For multi-drop, a later capability checks placement, unloading sequence, and rehandling. A unit that must unload first must not sit behind a later unit unless rehandling is allowed.

v0.1 contract, without 3D bin packing:

```text
placement_check = NOT_EVALUATED | SEQUENCE_OK | SEQUENCE_CONFLICT | REHANDLE_REQUIRED
```

`SEQUENCE_CONFLICT` is `HARD_REJECT` when the shipper or cargo policy forbids rehandling. `NOT_EVALUATED` is the v0.1 default and is visible in `score_explanation`. It does not claim a physical fit.

## Cross-shipper sets

A consolidation candidate may include cargo from more than one owner tenant only when every participating opportunity was published with a scope that allows network consolidation. Three or more shippers are the same rule as two: Shipper A, Shipper B, and Shipper C may share one vehicle only as co-loaded cargo units. The candidate stores each owner tenant id internally. The shared view follows [MARKETPLACE_DATA_VISIBILITY_MATRIX.md](MARKETPLACE_DATA_VISIBILITY_MATRIX.md). Shipper A does not receive B's or C's rate, contract, customer identity, internal ids, or tender. The same holds for every other pair.
