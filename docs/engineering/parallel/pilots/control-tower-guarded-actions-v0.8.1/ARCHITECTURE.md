# Control Tower v0.8.1 — Guarded Automatic Actions

## Overview

Extends the v0.8.0.1 automation runtime with `guarded_auto` execution and allow-listed Driver Task actions via the Driver Mobile Platform v0.2 internal API.

## Flow

```text
Domain trigger (e.g. eta_at_risk)
  → tenant rule match
  → playbook resolve
  → GuardEvaluator (ALLOW / REQUIRE_APPROVAL / DENY)
  → GuardedAction persistence
  → optional approval
  → Driver Task internal API (HTTP)
  → async WAITING_RESPONSE
  → driver.task_completed / driver.task_expired ingress
  → action continuation + case timeline
```

## Allow-listed actions

| Automation action | Driver task type | Approval default |
|---|---|---|
| REQUEST_DRIVER_DELAY_REASON | REQUEST_DELAY_REASON | none |
| REQUEST_DRIVER_STATUS_CONFIRMATION | REQUEST_STATUS_CONFIRMATION | none |
| REQUEST_DRIVER_ARRIVAL_CONFIRMATION | REQUEST_ARRIVAL_CONFIRMATION | none |
| CREATE_DRIVER_OPERATIONAL_NOTICE | GENERAL_OPERATIONAL_NOTICE | operator |

Forbidden actions (CHANGE_ROUTE, CANCEL_SHIPMENT, payment mutations, arbitrary commands, etc.) are rejected at registry validation and runtime guard evaluation. Tenant policy may only make behavior more restrictive.

## Safety

- Fail closed on unknown/forbidden actions
- Authoritative driver assignment lookup before dispatch
- Stable idempotency keys per execution step + trigger
- Loop protection via automation origin / driver response source exclusions
- No generic CREATE_DRIVER_TASK escape hatch

## Known limitations (v0.8.1)

- Timeout escalation uses case timeline event (no separate operator task entity)

## v0.8.1.1 verification remediation

- Approval RBAC fail-closed: missing `X-User-Permissions` / empty permission context → deny
- Gateway strips client `X-User-Permissions`; approve/reject injects trusted `automation.approve` for PLATFORM_ADMIN
- Global kill switch: `GLOBAL_GUARDED_ACTIONS_ENABLED` (default false)
- Tenant/action kill switches via `automation_tenant_action_policy`
- Real PostgreSQL concurrency/replay matrix: approval, approve/reject race, completion replay/concurrency, timeout replay, completion vs timeout race
- Runtime forbidden-action proof (playbook steps persisted, guard DENY at runtime)
- Full audit chain reconstructable from persistence
- OpenAPI guarded action + approval schemas validated
- Control Tower UI: guarded action panel with DENIED/WAITING_APPROVAL/WAITING_RESPONSE/SUCCEEDED/TIMED_OUT/REJECTED + RBAC-aware approve/reject
