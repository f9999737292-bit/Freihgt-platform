# Virtual AI Team — Freight Platform

## Purpose

This folder defines **virtual team roles** for accelerated development with Cursor. Each role file is a playbook: scope, rules, checks, and expected outputs. Use roles to split planning, implementation, verification, review, and documentation without adding process overhead.

## How to Use

1. Open Cursor in `D:\Projects\freight-platform`.
2. Paste a task using `CURSOR_TASK_TEMPLATE.md` (or reference roles inline).
3. **Assign an owner role** (PM, Backend, Frontend, QA, DevOps, Security, or Docs).
4. List **supporting roles** (e.g. Backend + QA for an API pack).
5. Ask Cursor to **plan first** (PM scope, acceptance criteria, risks).
6. Then **implement** (owner role rules).
7. Then **self-review** (Security if auth/tenant/import; QA checklist).
8. Then **run verification** (role-specific commands).
9. Finish with a **commit report** (PM/Docs final report template).

## Rules

| Rule | Description |
|------|-------------|
| **One owner** | Every Cursor task has exactly one **owner role** responsible for delivery. |
| **QA validates** | QA role (or QA checklist) must verify result before commit. |
| **PM closes** | PM role prepares scope, acceptance criteria, and **final report**. |
| **No scope creep** | Owner stays within task scope; PM blocks unrelated changes. |
| **Safe defaults** | No migrations, API breaks, or core business logic changes without explicit approval. |

## Role Index

| Role | File |
|------|------|
| PM / Delivery Lead | [PM_DELIVERY_LEAD.md](./PM_DELIVERY_LEAD.md) |
| Backend Go Engineer | [BACKEND_GO_ENGINEER.md](./BACKEND_GO_ENGINEER.md) |
| Frontend Vue/Nuxt Engineer | [FRONTEND_VUE_ENGINEER.md](./FRONTEND_VUE_ENGINEER.md) |
| QA / Test Engineer | [QA_TEST_ENGINEER.md](./QA_TEST_ENGINEER.md) |
| DevOps / Runtime Engineer | [DEVOPS_RUNTIME_ENGINEER.md](./DEVOPS_RUNTIME_ENGINEER.md) |
| Security / RBAC Reviewer | [SECURITY_RBAC_REVIEWER.md](./SECURITY_RBAC_REVIEWER.md) |
| Documentation / Release Writer | [DOCUMENTATION_RELEASE_WRITER.md](./DOCUMENTATION_RELEASE_WRITER.md) |

## Workflow Guides

| Guide | File |
|-------|------|
| Accelerated workflow (Fast / Safe / Sprint) | [ACCELERATED_WORKFLOW.md](./ACCELERATED_WORKFLOW.md) |
| Universal Cursor task template | [CURSOR_TASK_TEMPLATE.md](./CURSOR_TASK_TEMPLATE.md) |
| Daily operator checklist | [DAILY_CHECKLIST.md](./DAILY_CHECKLIST.md) |

## Example Prompt

```text
Use Virtual AI Team for this task.

Owner: Backend Go Engineer
Supporting: QA, Security, Docs

Mode: Safe Track Pack

Task: Add handler test for batch migration preview empty entity list.

Follow docs/ai-team/CURSOR_TASK_TEMPLATE.md:
1. PM plan (scope, Do Not Change, acceptance criteria)
2. Backend implementation
3. QA verification (go test, health-check)
4. Security review (tenant isolation N/A — unit test only)
5. Docs pack note if needed
6. Final report
```
