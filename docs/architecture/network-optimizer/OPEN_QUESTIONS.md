# Open questions

These do not block the freeze. They block specific later stages.

| Id | Question | Blocks |
|----|----------|--------|
| Q1 | Who may author city rules, and which official sources are in scope for Moscow and Saint Petersburg? | NLO-0.7, NLO-0.8 |
| Q2 | Is unload dwell a constant policy or a per-facility history? No dwell table exists. | Confidence of NLO-0.2 |
| Q3 | When a shared leg is accepted, is execution one shipment per leg or one shipment with stops? | NLO-0.4 activation |
| Q4 | Which routing vendor is allowed in staging? Port is vendor-neutral. | Executable kilometres |
| Q5 | Should `tracking.eta.updated` be added by tracking-service, or should the optimizer poll ETA? | Event versus read model |
| Q6 | Pallet and linear-metre capture: extend cargo, or only the optimizer projection? | Full fill feasibility |
| Q7 | Trailer master: new aggregate in shipment-service or later? | Private fleet equipment |
| Q8 | Reservation TTL and whether accept is synchronous in one Postgres schema. | Offer races |
| Q9 | Does Control Tower show `chain_at_risk` in the existing workspace or a new projection? | NLO-0.9 UX |
| Q10 | Anonymized marketplace: city centroid versus zone polygon. Polygons are city-rule data. | Publication UX |

No open question authorizes product code, a migration, or a deploy.

NLO-0.5A leaves Q4 open. `STAGING_PROVIDER_APPROVAL_REQUIRED=YES`. The 2GIS adapter exists and is not a staging approval. NLO-0.5B1 enforces a discovery cap of 1000. NLO-0.5B2 enforces a routing cap of 25, 4 matrix calls, 2 route calls, 6 provider calls, a 5 second request timeout, and a 30 second watchdog. NLO-0.5B3 evaluates one-load corridor and four-leg ellipse feasibility and does not close Q4. Routing SLA and provider commercial terms remain unknown. `EXTERNAL_PROVIDER_BENCHMARK=BLOCKED`. `PROVIDER_SLA_PROVEN=NO`. Exact road distance on anonymized loads stays in the existing coarse buckets. Rate absence stays `UNPRICED`.
