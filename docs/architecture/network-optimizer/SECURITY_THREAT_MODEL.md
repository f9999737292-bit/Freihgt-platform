# Security threat model

Maps to the current gateway model: JWT at `api-gateway`, client identity headers stripped, `X-Tenant-ID` and `X-User-ID` set from claims (`services/api-gateway/internal/http/middleware/auth.go`). Downstream repositories must keep `tenant_id` predicates. There is no separate ABAC product; carrier versus shipper is RBAC plus company context.

| Threat | Abuse | Control |
|--------|-------|---------|
| Cross-tenant leakage | Optimizer reads all shipments | Matching reads only published projections in the network context. No cross-tenant shipment query. Tests must prove a foreign `shipment_id` is not readable. |
| Rate leakage | Co-load partner sees the other contract | Visibility matrix. Decision snapshots store rates internally. APIs for shipper A filter B's commercial fields. |
| Customer leakage | Delivery site and contact of the other shipper | Anonymized scopes expose city or zone. Site contact only on the viewer's own stop. |
| Location privacy | Precise live truck shown to every shipper | Position stays in tracking under the shipment tenant. Predicted location on a published capacity follows capacity scope. |
| Vehicle tracking leakage | Plate and trail of a carrier harvested | `ANONYMIZED` capacity hides plate and driver. History is not a public API. |
| Offer replay | Accept the same offer twice | Idempotency key on accept. Offer status transition is compare-and-swap. |
| Double assignment | Two carriers win one load | Single reservation token. Load status `PUBLISHED → RESERVED → ASSIGNED` with version check. Loser receives conflict, not a silent success. |
| Spoofed carrier identity | Client sends another carrier's company id | Gateway sets company from membership. Body `carrier_id` must match the actor or the call is rejected. |
| Spoofed capacity | Attacker publishes a truck they do not operate | Capacity owner tenant must match JWT tenant. Carrier company must be a membership of that user. |
| Manipulated GPS | Fake position creates a false next load | Prediction records source and freshness. Low trust cannot auto-offer. Tracking ingest remains the tracking owner's problem; the optimizer does not accept raw coordinates from the public match API. |
| Stale prediction | Offer after the truck has already left | Offers bind `prediction_id` and expiry. ETA change emits re-evaluation. Expired predictions cannot be accepted. |
| Unauthorized publication | Order becomes a public load by default | Publication is a separate command. Default scope is `PRIVATE`. |
| Optimizer data poisoning | False loads or capacities steer the network | Only authenticated publishers. Anomalies are audit events. Platform admin bulk write is out of band and audited. |
| IDOR on projections | Guess `load_opportunity_id` | Get-by-id checks tenant or publication scope. Missing and forbidden both hide existence for out-of-scope callers (same pattern the gateway already requires for tenant objects). |

## Service identity

Platform service calls use the existing trusted integration headers (`packages/shared-go/integrationauth`). A service credential is not a license to drop `tenant_id` predicates. Internal jobs iterate owner tenants or read the projection table, which already contains only published fields.

## Audit

Every recommendation and assignment attempt writes `OptimizationDecision`. Security reviews of a future implementation must include cross-tenant tests before any primary runtime flag.
