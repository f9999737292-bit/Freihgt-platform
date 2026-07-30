# Live Data Demo Workflow Approval v0.1

## Summary

Approval boundary prepared for the future live-data demo workflow.

This pack approves the planning boundary only. It does not create credentials, seed data, users, tenants, records, sessions, source changes, backend changes, API changes, database writes, migrations, Nginx changes, deploys, Docker restarts, or server changes.

Base commit: `43ab15f` (`docs: plan live data demo workflow`).

Approval date: 2026-07-30.

## Decision

```text
LIVE_DATA_DEMO_WORKFLOW_APPROVAL_BOUNDARY_COMPLETE
LIVE_DATA_DEMO_ENVIRONMENT_STAGING_FIRST_APPROVED
LIVE_DATA_DEMO_V0_1_ROLE_SCOPE_APPROVED
LIVE_DATA_DEMO_CREDENTIALS_NOT_APPROVED_YET
LIVE_DATA_DEMO_SEED_DATA_NOT_APPROVED_YET
LIVE_DATA_DEMO_EXECUTION_NOT_APPROVED_YET
PRODUCTION_LIVE_DATA_DEMO_NOT_APPROVED
```

## Selected Environment Strategy

```text
Staging-first authenticated workflow.
```

Production remains limited to controlled static walkthrough until a separate explicit production live-data approval is given.

## Approved v0.1 Role Scope

| Role            | Status                           | Purpose                                                |
| --------------- | -------------------------------- | ------------------------------------------------------ |
| PLATFORM_ADMIN  | approved for future staging demo | platform overview / companies / admin concept          |
| SHIPPER_ADMIN   | approved for future staging demo | shipper workflow / freight requests / transport orders |
| CARRIER_ADMIN   | approved for future staging demo | shipment execution concept                             |
| FINANCE_MANAGER | approved for future staging demo | billing register concept                               |

## Deferred Roles

| Role                | Status   |
| ------------------- | -------- |
| FORWARDER_ADMIN     | deferred |
| CONSIGNEE_ADMIN     | deferred |
| PROCUREMENT_MANAGER | deferred |

## Approved Future Workflow Shape

```text
1. Login with dedicated approved demo credentials.
2. Open dashboard.
3. Open companies.
4. Open freight requests / RFx.
5. Open transport orders.
6. Open shipments.
7. Open documents.
8. Open billing registers.
9. Show role-based navigation difference with approved demo roles.
10. Logout / close browser session.
```

## Not Approved Yet

```text
Demo credentials creation.
Seed data creation.
Staging writes.
Production writes.
Authenticated production demo.
Source code changes.
Backend/API changes.
Database migrations/writes.
Nginx changes.
Deploys.
Fake production sessions.
Real credentials use.
```

## Required Before Execution

```text
1. DEMO_CREDENTIALS_AND_SEED_DATA_APPROVAL_PACK v0.1.
2. Exact demo tenant decision.
3. Exact demo user list and roles.
4. Credentials handling policy.
5. Seed dataset approval.
6. Staging execution approval.
7. Cleanup/rollback plan.
8. Post-execution review plan.
```

## Safety Result

```text
Production changed in this pack: no
Production deploy executed in this pack: no
Staging deploy executed in this pack: no
Server changed in this pack: no
Nginx changed: no
Nginx reload executed: no
DNS changed: no
Certbot changed: no
Docker restarted: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Database writes executed: no
Source code changed: no
Ports opened: no
Secrets captured: no
Credentials entered: no
Credentials created: no
Seed data created: no
Fake session created: no
Approval scope: live-data demo workflow boundary only
```

## Next Recommended Pack

```text
DEMO_CREDENTIALS_AND_SEED_DATA_APPROVAL_PACK v0.1
```

See also:

- `docs/LIVE_DATA_DEMO_ENVIRONMENT_DECISION_V0.1.md`
- `docs/DEMO_CREDENTIALS_AND_SEED_DATA_APPROVAL_BOUNDARY_V0.1.md`
- `docs/LIVE_DATA_DEMO_WORKFLOW_EXECUTION_BOUNDARY_V0.1.md`
- `docs/LIVE_DATA_DEMO_PRODUCTION_BOUNDARY_V0.1.md`
- `docs/LIVE_DATA_DEMO_WORKFLOW_APPROVAL_CHECKLIST_V0.1.md`
