# NLO-0.5B test strategy

B1 covers discovery. B2 covers the routing stage. B3 covers one-load feasibility. None of them call 2GIS.

| Gate | Expected |
| --- | --- |
| Discovery | Visible marketplace loads stop at 1,000 |
| Routing | Cheap-prefilter survivors sent to the provider stop at 25 |
| Deadline | A fake clock past 30 seconds starts no further provider call |
| Provider ceiling | An ellipse of 25 survivors issues 4 matrix calls and 2 route calls |
| Matrix dimension | One call has at most 25 origins and at most 25 destinations. A 25 by 25 page is one call |
| Partial matrix | Cells with errors stay unknown; successful cells remain |
| Timeout and 429 | No retry, no Haversine, no later provider call after a request-level failure |
| No route | `ROUTE_NOT_FOUND` does not invent a road distance |
| Large pool | 1,000 discovered rows do not produce 160 matrix calls |
| Tenant and privacy | A viewer's own load and a private load stay out. Anonymized responses omit exact coordinates, location ids, and facility labels |
| Duplicates | One load id appears once |
| Ordering | Repeated searches keep the same routed ids |

`EXTERNAL_PROVIDER_BENCHMARK=BLOCKED`. Local timings are not a production routing SLA.

## B3 one-load feasibility

| Gate | Expected |
| --- | --- |
| Corridor direction | Delivery-to-target road kilometres strictly smaller than pickup-to-target. Equal distance is `DELIVERY_NOT_TOWARD_TARGET` |
| Unknown corridor leg | A nil road fact is `ROAD_DISTANCE_UNKNOWN`. A measured zero remains zero |
| Ellipse | Increase is the four-leg insertion. The three-leg pickup detour is not the result |
| Missing ellipse leg | No increase is stored. A zero substitute that would pass the cap still rejects |
| One load | A feasible load and an unknown load stay separate. They are not one chain |
| Radius | `RADIUS` does not apply the corridor or ellipse verdict |
| Budget | Feasibility adds no provider call beyond the B2 ceiling |
| Early arrival | Waiting minutes are recorded |
| Missed window | `PICKUP_WINDOW_MISSED` |
| Service duration | A plan with no service-duration policy stays not executable |
| Safety | Own loads and private foreign loads stay out. Anonymized responses omit exact coordinates. Haversine is not an executable distance |
