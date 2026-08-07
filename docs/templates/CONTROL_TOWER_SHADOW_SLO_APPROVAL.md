# Control Tower Shadow Observation — SLO Approval Sheet

> **Proposed values are not approved production SLOs.**

Complete before enabling provisional alerts and before Day 7 final gate.

| Criterion | Proposed | Ops approved | Product approved | Final |
| --------- | -------: | -----------: | ---------------: | ----: |
| Observation duration | 7 days | | | |
| MATCH after convergence | 100% | | | |
| Sustained mismatch window | 5 min | | | |
| Dead-letter delta | 0 | | | |
| Offset commit errors | 0 | | | |
| Lag recovery | ≤5 min | | | |
| Gateway 5xx regression | 0 | | | |
| Read-model p95 | legacy +20% | | | |
| Incomplete after rebuild | 0 | | | |

---

## Signatures

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Ops lead | | | |
| Product owner | | | |
| Engineering lead | | | |

---

## Notes

- Observation window: 7 consecutive calendar days, minimum 5 business days
- Primary mode remains **disabled** throughout
- Alert rules: `infrastructure/monitoring/prometheus/control_tower_shadow_observation_alerts.provisional.yml` (provisional — requires approval)
