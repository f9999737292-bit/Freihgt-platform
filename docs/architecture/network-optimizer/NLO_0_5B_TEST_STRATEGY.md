# NLO-0.5B test strategy

B1 covers discovery. B2 covers the routing stage. Neither calls 2GIS.

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
