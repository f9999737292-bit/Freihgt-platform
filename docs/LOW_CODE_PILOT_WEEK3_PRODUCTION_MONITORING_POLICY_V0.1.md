# Low-code Pilot Week-3 Production Monitoring Policy v0.1

## Summary

Draft **production monitoring / alerting policy** for low-code runtime and admin modules (PR-GAP-004). Defines signals, alert conditions, owners, and incident severity. **Docs-only** — no real monitoring config changed.

**Decision:** **MONITORING_POLICY_DRAFT_CREATED_PENDING_OWNER_APPROVAL**

**PR-GAP-004:** **MONITORING_POLICY_DRAFT_CREATED_PENDING_OWNER_APPROVAL**

## Purpose

Establish what to monitor, which metrics/logs/audit events matter, alert conditions, incident severity, and evidence requirements before production deployment of low-code pilot scope.

## Scope

- Low-code **runtime** API (`/api/v1/low-code` runtime GET)
- Low-code **admin** API (templates, publish, migrate, import, batch)
- **Audit** events for low-code write actions
- **Security** signals (auth-on, tenant isolation, secrets leakage)
- **Health** and **performance** of low-code-service and gateway
- Does **not** configure Prometheus, Grafana, or production alert routing

## Current Status

| Field | Value |
|-------|-------|
| Owner | **TBD** (Ops / Monitoring Owner / SRE) |
| Approval | **pending** |
| Production-ready claimed | **no** |
| Controlled pilot | **CONTROLLED_PILOT_APPROVED** — active |
| Real monitoring config changed | **no** |

## Monitoring Objectives

1. Detect low-code service unavailability before operator impact
2. Detect auth bypass and tenant isolation failures immediately (P0)
3. Verify runtime template reads and admin access controls
4. Ensure write actions produce audit evidence
5. Detect secrets/JWT/tokens in logs or committed docs (P0)
6. Detect unapproved publish/migration/import execution (P0)

## Runtime Monitoring

| Signal | Description |
|--------|-------------|
| Active template read success/failure | Runtime GET returns expected template for entity_type |
| Custom field values read success/failure | Field values returned or graceful empty state |
| Runtime GET latency | p50/p95 latency trend |
| Runtime 4xx/5xx count | Error rate by endpoint and tenant |
| Missing template fallback | Behavior when template not found |
| Tenant-bound response correctness | Responses scoped to `X-Tenant-ID` |

## Admin Monitoring

| Signal | Description |
|--------|-------------|
| Admin templates list/read | Admin routes accessible with admin auth |
| Non-admin forbidden | 403/401 for non-admin on admin routes |
| Clone/export/import preview | Read-only preview endpoints behave correctly |
| Publish/migration endpoints protected | Writes blocked without auth and approval |
| Admin 4xx/5xx count | Error rate on admin low-code routes |

## Audit Monitoring

| Signal | Description |
|--------|-------------|
| Write actions generate audit events | POST/PUT publish, migrate, import produce audit |
| Admin action evidence | Audit GET shows admin actions |
| Migration/import/batch evidence | Batch actions logged |
| Audit read access protected | Non-admin denied on audit admin routes |

## Security Monitoring

| Signal | Description |
|--------|-------------|
| Auth-on status | Admin routes require authentication |
| Non-admin forbidden on admin routes | RBAC enforced |
| Anonymous forbidden on admin routes | No unauthenticated admin access |
| No secrets in logs/docs | No JWT, tokens, passwords in evidence |
| Tenant isolation failure suspected | Cross-tenant data in response |

## Tenant Isolation Monitoring

- All low-code requests must remain tenant-scoped
- Alert on suspected cross-tenant template or field data exposure
- Reference: PR-GAP-006, PR-RISK-007

## Error / Availability Signals

- `low-code-service` health endpoint availability
- Gateway health for low-code routes
- Repeated 5xx on low-code API
- Service restart / crash loop detection

## Performance Signals

- Low-code service health (make health-check 9/9 in dev baseline)
- Gateway health
- DB query latency (if available)
- DB pool metrics (if available)
- Endpoint latency trend (runtime GET, admin list)

## Alert Conditions

Detailed alert IDs in `LOW_CODE_PILOT_WEEK3_PRODUCTION_MONITORING_ALERT_CONDITIONS_V0.1.md`.

## Incident Severity

| Severity | Examples |
|----------|----------|
| **P0** | Auth bypass, tenant leak, secrets in logs, unapproved publish/migrate |
| **P1** | Service down, audit missing after write, runtime templates unavailable |
| **P2** | High latency, repeated 5xx below threshold |
| **P3** | Trend warnings, non-blocking degradation |

## Owner / Escalation

| Role | Responsibility |
|------|----------------|
| **Ops / Monitoring Owner / SRE** | Approve policy, alert routing, on-call (TBD) |
| **Security** | P0 auth/tenant alerts |
| **PM** | Operator communication during incidents |
| **Rollback owner** | Rollback decision gate (PR-GAP-003 closed) |

## Evidence Requirements

- Health-check output (no secrets)
- HTTP status codes and latency (no JWT/tokens)
- Audit event IDs and timestamps (no payload secrets)
- Tenant ID references (UUID only — no production PII)
- Incident ticket references

## Approval Requirements

Before production monitoring go-live:

1. Named **Ops / Monitoring Owner** assigned
2. Alert conditions approved
3. P0/P1 escalation path defined
4. Evidence format approved (no secrets in repo)
5. Final sign-off in **Production Monitoring Owner Approval Pack v0.1**

## Decision

**MONITORING_POLICY_DRAFT_CREATED_PENDING_OWNER_APPROVAL**

## Next Steps

1. Assign monitoring owner
2. Execute **Low-code Pilot Week-3 Production Monitoring Owner Approval Pack v0.1**
3. Do **not** change Prometheus/Grafana or deploy monitoring until approved

Reference: `LOW_CODE_PILOT_WEEK3_PRODUCTION_MONITORING_CHECKLIST_V0.1.md`
