# Low-code Pilot Week-3 Operator Feedback Scheduling Note v0.1

## Purpose

PM / pilot lead scheduling note to obtain **first real operator feedback** after two unsuccessful session attempts (FIRST_SESSION_PENDING_OPERATOR, RETRY_PENDING_OPERATOR).

**Reference:** `LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_SESSION_RETRY_V0.1.md`, `LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_ACTION_PLAN_V0.1.md`

## Current Status

| Item | Value |
|------|-------|
| Real operator submissions | **0** |
| Session attempts without operator | **2** (W3-FB-SESSION-001, W3-FB-RETRY-001) |
| Runtime/API readiness | **PASS** — demo entities and fields available |
| Feedback process docs | **Ready** |
| Blocker | **Operator not scheduled / unavailable** |

## Why Feedback Is Required

Week-3 execution plan exit criteria require **≥1 feedback submission per entity type** (TO, SH, BR) before expansion or polish selection decisions.

Without operator input:

- UX clarity, validation, audit findability unverified by users
- SH/BR limited-write enablement lacks operator sign-off
- Financial safety perception for BILLING_REGISTER unconfirmed
- Improvement backlog remains baseline P3 only — **no evidence-driven changes**

## Required Participants

| Role | Responsibility |
|------|----------------|
| **PM** | Schedule session; assign owner; escalate if delayed |
| **Operator lead** | Nominate operator(s); distribute quick guides |
| **Pilot lead / facilitator** | Run walkthrough; record feedback log |
| **Operator** | Primary feedback provider (e.g. shipper logist) |
| **Optional:** Platform admin | Facilitator login / technical support only |

**Suggested operator account (dev):** `shipper@7rights.local` (SHIPPER_LOGIST) — or production-designated pilot operator.

## Required Sessions

| # | Entity | Demo | Duration | Type |
|---|--------|------|----------|------|
| 1 | TRANSPORT_ORDER | DEMO-TO-001 | ~15 min | Baseline read-only review |
| 2 | SHIPMENT | DEMO-SH-PLANNED | ~15 min | Limited-write pilot review (read-first) |
| 3 | BILLING_REGISTER | DEMO-BR-001 | ~15 min | Limited-write + financial safety review |

**Total timebox:** ~45–60 minutes (can split across 2 calendar days if needed).

## Suggested Timebox

| Block | Activity |
|-------|----------|
| 0–5 min | Brief pilot scope; no Save unless separate approval |
| 5–15 min per entity | Open entity page; review low-code panel; complete form template |
| 5 min | Wrap-up; severity + GO/GO_WITH_CONDITIONS/STOP |

**Environment:** local dev `http://localhost:3000` or staging when available.

**Login (dev facilitator):** `admin@7rights.local` / see `docs/AUTH_RBAC.md`

## Required Evidence

Per entity, capture via `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORM_TEMPLATE_V0.1.md`:

- Panel visible, values correct, labels clear
- Validation / save clarity (or N/A read-only)
- Audit visibility
- Permission/access issues
- BR: financial/core safety concerns
- Severity P0–P3
- Operator decision

Log rows: `FB-W3-001`, `FB-W3-002`, `FB-W3-003` (or one row per issue).

## If Operator Is Unavailable

After **2** failed scheduling attempts (current state):

1. **PM escalation** — formal owner assignment + target date
2. Document blocker in feedback log (NEEDS_INFO)
3. **Do not** fabricate feedback or proceed to UI polish selection pack
4. Next pack: **Operator Feedback Scheduling & PM Escalation Pack v0.1** (operational closure)
5. Optional: accept written async feedback via form template if live session impossible — must be **real** operator, not AI-generated

## Owner Actions

| # | Action | Owner | Target date |
|---|--------|-------|-------------|
| 1 | Confirm operator name/role for session | operator lead | __________ |
| 2 | Book calendar slot (45–60 min) | PM | __________ |
| 3 | Send quick guides (SH, BR) + checklist | operator lead | Before session |
| 4 | Pre-check: `make health-check` + seed | pilot lead | Session day AM |
| 5 | Run scenarios; update feedback log | pilot lead | Session day |
| 6 | Triage P0/P1 same day | PM | Session day |

## Next Decision

| Outcome | Next pack |
|---------|-----------|
| Real feedback collected; no P0/P1 | Feedback-Based UI/Docs Polish Selection v0.1 |
| Real feedback collected; P0/P1 | Runtime Pilot Fix Pack v0.1 |
| Operator still unavailable after PM escalation | Week-3 review with **HOLD** on expansion; document in weekly report |

**Current recommendation:** Schedule live session within **3 business days** or assign async form submission with named operator.
