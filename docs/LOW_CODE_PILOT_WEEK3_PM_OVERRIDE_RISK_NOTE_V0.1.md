# Low-code Pilot Week-3 PM Override Risk Note v0.1

## Purpose

Documents risks and limits if PM requests proceeding **without real operator feedback** (Option C). **Not currently selected** — current decision is **PM_OVERRIDE_NOT_REQUESTED** (after **LIVE_SESSION_CONFIRMATION_STILL_PENDING**).

Reference: `LOW_CODE_PILOT_WEEK3_PM_OVERRIDE_DECISION_V0.1.md`

## When This Applies

- PM explicitly requests override because operators cannot be scheduled in acceptable timeframe
- PM wants to proceed to UI/docs polish or expansion without `FB-W3-001+` submissions
- Separate **PM Override Decision Pack v0.1** must be executed before any override takes effect

**Current status:** Override **not requested**.

## Risk Of Proceeding Without Real Feedback

| Risk | Severity | Description |
|------|----------|-------------|
| UX polish targets wrong fields | High | Labels/help may not match operator mental model |
| BR financial safety miscommunication | Critical | Operator may not understand low-code vs core billing status |
| Undetected P1 save/validation confusion | High | Could cause operator error on next write session |
| False production readiness signal | High | Usability claims unsupported by evidence |
| Pilot expansion without sign-off | Medium | Second entities/roles without operator validation |
| Audit visibility gaps unnoticed | Medium | Operators may not find audit history when needed |

## What Would Be Blocked Without Override

| Work | Status without override |
|------|-------------------------|
| Feedback-Based UI/Docs Polish Selection | **Blocked** |
| Pilot expansion decision | **Blocked** |
| Production readiness claim (usability) | **Blocked** |
| Broad rollout | **Blocked** |
| Code fixes from assumed UX issues | **Blocked** |

## What PM Must Explicitly Accept

If override is requested, PM must document acceptance of:

1. Polish/expansion decisions may not reflect real operator needs.
2. BR financial safety operator confirmation is **missing** — elevated risk for billing-adjacent fields.
3. Any P0/P1 issues in production pilot may not have been discovered in dry-run.
4. Monitoring and read-only validation alone are insufficient for usability sign-off.
5. Override does **not** authorize production broad rollout.
6. Override scope must be limited (e.g. docs-only polish candidates, not code fixes from assumptions).

## Minimum Evidence Before Override

Even with override, require:

- [ ] Written PM override decision note with named approver and date
- [ ] Documented reason operators unavailable
- [ ] Scope limit (what packs are allowed under override)
- [ ] Revisit date for real operator sessions when available
- [ ] No invented P0/P1 items — severity must come from technical evidence only

## Not Allowed Even With Override

| Item | Reason |
|------|--------|
| Production broad rollout | Separate approval required |
| Backend/frontend code fixes from assumed UX | No P0/P1 operator evidence |
| Invented P0/P1 feedback items | Policy violation |
| Skipping BR financial safety review indefinitely | Must schedule when operator available |
| Committing auth-on to tracked compose | Deployment policy |
| PUT/save/production writes without separate pack | Out of override scope |

**Code fixes based on assumptions** should remain limited to **docs/help text only** unless separate technical evidence exists (e.g. monitoring P0, not invented operator report).

## Recommended Decision

**Do not override.** Schedule live sessions per `LOW_CODE_PILOT_WEEK3_PM_SCHEDULING_ACTION_PLAN_V0.1.md`.

If operators truly unavailable:

1. Document Option D (stop feedback track; monitoring continuation).
2. Only if business requires limited docs polish → PM Override Decision Pack with narrow scope and risk acceptance.
3. Never claim production readiness or broad rollout under override alone.
