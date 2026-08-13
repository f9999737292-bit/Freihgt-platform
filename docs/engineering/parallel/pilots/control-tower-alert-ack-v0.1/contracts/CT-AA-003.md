# Task Contract

## Task ID

CT-AA-003

## Title

Control Tower alert acknowledgement — frontend UI

## Owner

orchestrator

## Role

frontend-engineer

## Repository

D:\Projects\freight-platform-wt\ct-alert-ack-frontend

## Base branch

CONTRACT_FREEZE_SHA (from CT-AA-001 handoff)

## Base SHA

`<CONTRACT_FREEZE_SHA>` — do not use floating origin/main

## Working branch

feat/control-tower-alert-ack-frontend-v0.1

## Worktree

D:\Projects\freight-platform-wt\ct-alert-ack-frontend

---

## Objective

Minimal Control Tower UX: show acknowledgement state on critical events; allow authorized users to acknowledge unacknowledged events; refresh summary after success; handle API errors. Preserve existing Control Tower design conventions.

## In scope

- Extend `ControlTowerEvent` TypeScript type with acknowledgement fields from frozen OpenAPI
- API call: POST `/api/v1/control-tower/critical-events/{eventId}/acknowledge` via `useApi`
- Update `CriticalEventsPanel.vue`: acknowledged badge, acknowledge button (authorized + unacknowledged only)
- i18n keys under `controlTower.events.ack*` (locales as needed)
- Composable/helper in `useControlTower.ts` for acknowledge action + summary refresh
- Respect `canAccessControlTower()` for showing acknowledge action

## Out of scope

- Dashboard redesign, KPI changes, unrelated Control Tower panels
- OpenAPI edits
- Backend changes
- Bulk acknowledge, unacknowledge, comments
- Demo mode acknowledge (disable or no-op in demo)

## Allowed paths

- `apps/web-admin/types/controlTower.ts`
- `apps/web-admin/composables/useControlTower.ts`
- `apps/web-admin/components/control-tower/CriticalEventsPanel.vue`
- `apps/web-admin/locales/**/controlTower*.json` (or existing locale files with controlTower keys)
- `apps/web-admin/pages/control-tower/index.vue` (wire only if needed — prefer component-level)

## Forbidden paths

- `packages/openapi/**`
- `services/**`
- `infrastructure/**`
- Other `apps/web-admin/**` paths not listed above
- `Makefile`, root workspace files

## Dependencies

- CT-AA-001 complete with CONTRACT_FREEZE_SHA (types follow frozen schema)
- Backend endpoint may be stubbed for UI dev; integration expects CT-AA-002 merged or available on integration branch before E2E

## Security invariants

- No client-supplied tenant_id or user_id in acknowledge request
- Use existing authenticated `useApi` (JWT bearer)
- Do not expose acknowledge action when `canAccessControlTower()` is false

## Acceptance criteria

1. Acknowledged events show visible acknowledged state (timestamp and/or actor if API provides).
2. Unacknowledged events show acknowledge action for authorized users.
3. Successful ack triggers summary refresh; UI reflects new state.
4. API errors (403/404/503) show appropriate user feedback without crash.
5. Existing critical events list layout preserved (no unrelated redesign).
6. Frontend lint/build for web-admin PASS or NOT_RUN documented.

## Required validation

Level: 1

Commands:

- `git diff --check`
- `pnpm --filter web-admin lint` (or project-equivalent)
- `pnpm --filter web-admin build` (if feasible)

## Required deliverables

- UI diff within Allowed Paths
- Handoff with screenshots description or manual test notes

## Integration target

int/control-tower-alert-ack-v0.1 → main

## Handoff requirements

- CONTRACT_FREEZE_SHA documented as base
- List changed Vue/TS/locale files
- Validation results
