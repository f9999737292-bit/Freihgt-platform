# Observability

No metrics are implemented in this freeze. Names are for a later Prometheus exposition, consistent with existing service metrics style.

| Metric | Type | Labels |
|--------|------|--------|
| `optimizer_jobs_total` | counter | `mode`, `objective`, `result` |
| `optimizer_job_duration_seconds` | histogram | `mode`, `scale_class` |
| `candidate_pairs_total` | counter | `stage` |
| `candidate_rejections_total` | counter | `reason_code` |
| `matches_generated_total` | counter | `problem_class` |
| `consolidation_candidates_total` | counter | `pattern` |
| `offers_created_total` | counter | `commercial_mode` |
| `offers_accepted_total` | counter | `commercial_mode` |
| `deadhead_km_estimated` | histogram | `problem_class` |
| `deadhead_km_avoided` | counter | `mode` |
| `capacity_utilization_weight` | histogram | `mode` |
| `capacity_utilization_volume` | histogram | `mode` |
| `reoptimization_total` | counter | `trigger` |
| `chain_at_risk_total` | counter | `reason` |

`NETWORK_UTILIZATION` is not a single scraped gauge without parts. Dashboards must show vehicle utilization, loaded-km ratio, capacity utilization, time utilization, and revenue utilization beside any aggregate.

Control Tower may later project: active shipments (already), `eta_risk`, `chain_at_risk`, future and unmatched capacity, deadhead, empty-space ratios, idle and waiting, match rate, offer acceptance, time to match, time to accept. Those projections are CT read models fed by `network.*` facts. They are not computed inside the freight-cost ledger.

Logs must not contain other tenants' rates. Decision ids are enough to join the audit store under the caller's tenant.
