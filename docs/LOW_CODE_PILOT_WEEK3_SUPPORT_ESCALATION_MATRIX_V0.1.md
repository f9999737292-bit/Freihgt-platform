# Low-code Pilot Week-3 Support Escalation Matrix v0.1

## Summary

Escalation matrix for low-code controlled pilot support (PR-GAP-007). **No real contact details** — owner TBD.

Reference: `LOW_CODE_PILOT_WEEK3_SUPPORT_OWNERSHIP_POLICY_V0.1.md`

## Escalation Matrix

| Severity | Trigger | First Response | Escalation | Owner | Evidence | Notes |
|----------|---------|----------------|------------|-------|----------|-------|
| **P0** | Auth bypass, tenant leakage, secrets exposure, data corruption | Immediate controlled pilot stop / freeze recommendation | Product + Security/Architecture + Support owner | **TBD** | Sanitized status codes, request IDs, endpoints only | Trigger **Low-code Runtime Pilot Fix Pack v0.1**; no writes without approval |
| **P1** | Critical low-code workflow unavailable (admin down, migration preview broken, missing audit for critical flow) | Same business day | Support owner + Platform owner | **TBD** | Sanitized logs/tickets; no secrets | May escalate to Fix Pack if blocking |
| **P2** | Degraded controlled pilot workflow (single entity type, non-critical validation, UI degraded) | Next business day | Support owner | **TBD** | Sanitized reproduction steps | Backlog triage acceptable |
| **P3** | Docs, UX copy, process clarification | Backlog triage | Optional | **TBD** | Docs/feedback references | No code change from baseline P3 alone |

## Response Targets

| Severity | Stop/freeze | Fix Pack |
|----------|-------------|----------|
| P0 | **Recommend stop** | **Yes** — immediate |
| P1 | Evaluate case-by-case | **Yes** if blocking |
| P2 | No | Only if becomes P1 |
| P3 | No | No |

## Evidence Format

| Allowed | Forbidden |
|---------|-----------|
| HTTP status code | JWT / tokens |
| Request ID | Passwords |
| Endpoint path (no secrets in query) | Raw production payloads |
| Sanitized tenant label (dev/demo) | Personal/financial production data |

## Decision

**SUPPORT_OWNERSHIP_PACK_CREATED_PENDING_OWNER_ASSIGNMENT**

## Next Pack

**Low-code Pilot Week-3 Support Owner Approval Pack v0.1**
