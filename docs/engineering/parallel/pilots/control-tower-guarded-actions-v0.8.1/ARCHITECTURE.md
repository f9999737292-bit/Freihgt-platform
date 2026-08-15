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

## v0.8.1.1 Typecheck Certification (WEB-ADMIN TYPECHECK BASELINE REMEDIATION v0.1)

**Original blocker:** `GATE_31_FRONTEND_TYPECHECK=FAIL` — full `apps/web-admin` TypeScript baseline (101 errors; reported focus `utils/lowCodeValidationContext.ts`).

**Root causes (summary):**

| Area | Classification | Fix |
|---|---|---|
| `lowCodeValidationContext.ts` | B/D — `PreviewRuleContext` union access | `'field' in context` guards before extended-field reads |
| `useOperationalCases.ts`, `useOperatorWorkspace.ts` | F — reversed `pushToast` args; `statusCode` typo; boolean query | Correct arg order; `error.status`; widen API query type |
| `useApi.ts` | F — `isApiUnavailableError` not exported | Export from composable; allow boolean query values |
| `useToast.ts` / `ui.ts` | F — missing `warning` toast type | Add `'warning'` union + CSS |
| Company modals | B — optional payload vs `UiInput` string v-model | `CompanyFormState` / `CreateCompanyFormState` / `CompanyEditFormState` |
| `OperatorWorkQueue.vue` | B — nested ref not unwrapped | Destructure refs from `useOperationalCases()` |
| `OperatorCaseDetailsDrawer.vue` | B — `User.displayName` vs `full_name` | Use canonical `User.full_name` |
| `OperatorCaseFromWorkItemModal.vue` | B — partial duplicate mapping | `ControlTowerCaseDuplicateCandidate` return type |
| Low-Code wizard/modals | D — impossible phase comparisons | `isRequestInFlight` / remove redundant phase check |
| `audit/index.vue` | A — numeric limit vs `UiSelect` string options | String limit filter + numeric coercion at API boundary |
| `custom-field-values/index.vue` | D — empty entity type | Guard before `resolveTemplateCodeForType` |
| `nuxt.config.ts` | I — missing `process` types | `nuxt-config-env.d.ts` ambient declaration |

**Verification (2026-08-15):**

```text
npm run typecheck  → PASS (0 errors)
npm run build      → PASS
modified-file lint → PASS (0 errors)
GUARDED_ACTION_UI  → PASS (ControlTowerGuardedActionsPanel lint clean; included in full typecheck/build)
LOW_CODE_REGRESSION→ PASS (typecheck + build; no web-admin unit tests for validation utils)
BACKEND_MODIFIED   → NO
OPENAPI_CHANGED    → NO
GATE_31            → PASS
ALL_42_GATES       → PASS
FINAL_VERDICT      → PASS
PILOT_EXECUTION    → NOT_STARTED
GLOBAL_GUARDED_ACTIONS_ENABLED=false
```
