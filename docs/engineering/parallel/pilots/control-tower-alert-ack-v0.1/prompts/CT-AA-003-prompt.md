# Agent Prompt — CT-AA-003

## Assignment

You are the **frontend-engineer** agent for the 7Rights Freight Platform.

**Task ID:** CT-AA-003

**Repository / worktree:** D:\Projects\freight-platform-wt\ct-alert-ack-frontend

**Branch:** feat/control-tower-alert-ack-frontend-v0.1

**Base SHA:** `<CONTRACT_FREEZE_SHA from CT-AA-001 handoff — NOT origin/main>`

## Objective

Add minimal acknowledge UX to Control Tower critical events panel: show acknowledged state, acknowledge button for authorized unacknowledged events, refresh summary after success, handle errors. Preserve existing design.

## Allowed paths

- `apps/web-admin/types/controlTower.ts`
- `apps/web-admin/composables/useControlTower.ts`
- `apps/web-admin/components/control-tower/CriticalEventsPanel.vue`
- `apps/web-admin/locales/**` (controlTower ack i18n keys)

## Forbidden paths

- `packages/openapi/**`
- `services/**`
- Other apps/web-admin paths unless justified in handoff

## Dependencies

CT-AA-001 complete; types follow frozen OpenAPI

## Acceptance criteria

1. Acknowledged events display acknowledged state (time/actor if API provides).
2. Authorized users see acknowledge action on unacknowledged events (`canAccessControlTower()`).
3. Success refreshes summary; UI updates deterministically.
4. API errors handled gracefully.
5. No unrelated Control Tower redesign.
6. Lint/build PASS or NOT_RUN documented.

## Required validation level

1

## Safety rules (mandatory)

1. Git context commands first.
2. Read Task Contract CT-AA-003 and frozen OpenAPI types.
3. Read `.cursor/rules/05-parallel-engineering.mdc`.
4. No destructive Git.
5. Allowed paths only.

## Worktree creation

```powershell
git worktree add D:\Projects\freight-platform-wt\ct-alert-ack-frontend -b feat/control-tower-alert-ack-frontend-v0.1 <CONTRACT_FREEZE_SHA>
```

**Wait for CT-AA-001 CONTRACT_FREEZE_SHA before creating this worktree.**
