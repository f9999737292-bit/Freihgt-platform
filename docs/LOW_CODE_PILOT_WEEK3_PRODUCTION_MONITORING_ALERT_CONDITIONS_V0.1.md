# Low-code Pilot Week-3 Production Monitoring Alert Conditions v0.1

## Summary

Draft alert conditions for low-code production monitoring (PR-GAP-004). **Policy only** — no alert routing configured.

Reference: `LOW_CODE_PILOT_WEEK3_PRODUCTION_MONITORING_POLICY_V0.1.md`

## Alert Conditions

| Alert ID | Condition | Severity | Owner | Evidence | Action |
|----------|-----------|----------|-------|----------|--------|
| MON-ALERT-001 | low-code-service unavailable | **P1** (short) / **P0** (sustained) | Ops / SRE — TBD | Health-check fail, service down | Escalate on-call; check gateway and DB |
| MON-ALERT-002 | Admin auth bypass suspected | **P0** | Security | Non-admin 200 on admin route | STOP writes; Runtime Pilot Fix Pack |
| MON-ALERT-003 | Tenant isolation issue suspected | **P0** | Security | Cross-tenant data in response | STOP; isolate tenant; Security review |
| MON-ALERT-004 | Runtime active templates unavailable | **P1** | Ops / QA | Runtime GET 5xx or empty for known template | Check template publish state; read-only verify |
| MON-ALERT-005 | Audit events missing after write action | **P1** | Ops / QA | Write succeeded but no audit GET match | Pause risky admin actions; investigate audit pipeline |
| MON-ALERT-006 | Repeated 5xx on low-code API | **P1** / **P2** | Ops — TBD | Error rate threshold exceeded | Check logs (no secrets); rollback assessment |
| MON-ALERT-007 | High latency on low-code runtime GET | **P2** | Ops — TBD | p95 latency above SLO | Performance investigation |
| MON-ALERT-008 | Non-admin can access admin endpoint | **P0** | Security | 200 instead of 403/401 | STOP; auth-on verification; Fix Pack |
| MON-ALERT-009 | Secrets/JWT/tokens observed in logs/docs | **P0** | Security | Pattern match in logs or committed docs | Revoke/rotate; remove from repo; Security |
| MON-ALERT-010 | Publish/migration/import executed without approval | **P0** | Ops / Security | Audit shows unapproved write | STOP; rollback assessment; PM notify |

## Severity Mapping

| Severity | Response time (target) | Escalation |
|----------|------------------------|------------|
| **P0** | Immediate | Security + PM + on-call |
| **P1** | < 1h | Ops owner + PM |
| **P2** | < 4h | Ops owner |
| **P3** | Next business day | Monitoring owner review |

## P0 Conditions

- MON-ALERT-002 — admin auth bypass suspected
- MON-ALERT-003 — tenant isolation issue suspected
- MON-ALERT-008 — non-admin admin endpoint access
- MON-ALERT-009 — secrets/JWT/tokens in logs/docs
- MON-ALERT-010 — unapproved publish/migration/import
- MON-ALERT-001 — sustained service unavailability

## P1 Conditions

- MON-ALERT-001 — short-duration service unavailability
- MON-ALERT-004 — runtime active templates unavailable
- MON-ALERT-005 — audit events missing after write
- MON-ALERT-006 — repeated 5xx (above P1 threshold)

## P2 Conditions

- MON-ALERT-006 — repeated 5xx (below P1 threshold)
- MON-ALERT-007 — high runtime GET latency

## P3 Conditions

- Latency trend warnings
- Non-blocking metric drift
- Pilot monitoring evidence refresh requests

## Notification Rules

- P0 → Security + PM + monitoring on-call (when assigned)
- P1 → Ops owner + PM notification
- P2/P3 → Ops ticket queue
- **No secrets** in alert messages or attached evidence

## Escalation

1. On-call acknowledges P0 within 15 minutes (when routing approved)
2. Security leads auth/tenant P0
3. PM **Феликс Асаев** for operator impact
4. Rollback owner **Артем Асаев** for rollback decision gate (if incident warrants)

## False Positive Handling

- Document known dev/demo tenant behavior separately from production
- Tune thresholds after owner approval
- Mark rehearsal alerts as non-production

## Next Steps

1. Monitoring owner approves conditions and thresholds
2. Configure alert routing in **Monitoring Owner Approval Pack** (future ops work — not this pack)
3. Link to Runtime Pilot Fix Pack for active P0/P1
