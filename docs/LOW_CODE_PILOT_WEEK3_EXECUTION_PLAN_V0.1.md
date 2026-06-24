# Low-code Pilot Week-3 Execution Plan v0.1

## Summary

Operational execution plan for **Week-3** following Week-2 closure (**CLOSED_WITH_CONDITIONS**). Focus: **monitoring evidence**, **auth-on staging verification**, **operator feedback**, and **controlled expansion decisions** — not broad production rollout.

**Prerequisite:** `LOW_CODE_PILOT_WEEK2_CLOSURE_V0.1.md` signed off.

## Week-3 Goal

Convert Week-2 **documentation readiness** into **operational evidence**:

1. Real monitoring reports (TO ongoing; SH/BR on first writes)
2. Operator feedback for all three entity types
3. Auth-on staging verification repeated
4. Zero P0; P1 owned
5. Explicit expand/hold decision at week end

## Preconditions

- [ ] Week-2 closure **CLOSED_WITH_CONDITIONS** acknowledged
- [ ] Cross-entity decision **GO_WITH_CONDITIONS** acknowledged
- [ ] Monitoring templates distributed (SH, BR, TO daily)
- [ ] Stop conditions briefed to operators
- [ ] Quick guides available (SH, BR)
- [ ] `make health-check` green at Week-3 start

## Workstreams

| # | Workstream | Owner | Priority |
|---|------------|-------|----------|
| 1 | Monitoring Evidence | Operator + pilot lead | **P0** |
| 2 | Auth-on Staging Verification | DevOps + Security | **P1** |
| 3 | Operator Feedback Collection | Operator lead | **P1** |
| 4 | Runtime Fixes If Needed | Backend/Frontend (on trigger) | **P0 when triggered** |
| 5 | Pilot Expansion Decision | PM + pilot lead | **P2 end of week** |

## Workstream 1: Monitoring Evidence

**Goal:** Replace preparatory monitoring docs with **filled reports**.

| Day | Actions |
|-----|---------|
| Daily AM | `make health-check`; active template spot-check; audit baseline |
| After each write | GET after write; audit check; BR: core billing register GET |
| Daily PM | Fill entity-specific monitoring report or "no writes today" |
| Weekly | Audit gap review — target **zero gaps** |

| Entity | Template |
|--------|----------|
| TRANSPORT_ORDER | `LOW_CODE_PILOT_DAILY_REPORT_TEMPLATE_V0.1.md` |
| SHIPMENT | `LOW_CODE_PILOT_WEEK2_SHIPMENT_WRITE_MONITORING_REPORT_TEMPLATE_V0.1.md` |
| BILLING_REGISTER | `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_MONITORING_REPORT_TEMPLATE_V0.1.md` |

**Exit:** ≥3 days with reports OR documented zero-write days.

## Workstream 2: Auth-on Staging Verification

**Goal:** Confirm RBAC on **staging** (not committed to repo).

| Task | Reference |
|------|-----------|
| Enable `LOW_CODE_ADMIN_AUTH_ENABLED=true` on staging low-code-service only | Ops runbook — **do not commit** |
| PLATFORM_ADMIN → admin routes **200** | `LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md` |
| Non-admin → admin routes **403** | Same doc |
| Document matrix in Week-3 daily report | |

**Exit:** Staging auth-on PASS or exceptions with owners and dates.

## Workstream 3: Operator Feedback Collection

**Goal:** First **real** usability feedback.

| Task | Output |
|------|--------|
| 15-min walkthrough TO/SH/BR | Sign-off on checklist items |
| Distribute quick guides | SH + BR guides |
| Submit feedback forms | `LOW_CODE_PILOT_OPERATOR_FEEDBACK_FORM_V0.1.md` |
| Triage P0/P1/P2 | Week-3 issue log |

**Exit:** ≥1 feedback submission per entity type attempted OR documented blocker.

## Workstream 4: Runtime Fixes If Needed

**Goal:** Fix only documented P0/P1 — smallest safe diff.

| Trigger | Action |
|---------|--------|
| P0 stop condition | **Low-code Runtime Pilot Fix Pack v0.1** — halt affected writes |
| P1 bug with repro | Fix pack with PM approval |
| P2/P3 | Backlog — no Week-3 scope creep |

**Constraints:** No API contract changes; no migrations unless explicitly approved.

## Workstream 5: Pilot Expansion Decision

**Goal:** End-of-week written decision — expand or hold.

| Question | Gate |
|----------|------|
| Second SH entity? | Monitoring clean + operator sign-off |
| Second BR entity? | Financial safety clean + operator sign-off |
| Shipper logist SH/BR write? | Product + security approval |
| Production pilot scope doc? | **Not Week-3 default** |
| Broad rollout? | **Rejected unless new decision note** |

**Exit:** Written note in Week-3 review or explicit HOLD.

## Daily Cadence

| Time | Actions |
|------|---------|
| **Morning** | health-check; template/audit baseline; open P0/P1 review |
| **Midday** | error logs; audit scan; operator issues |
| **After writes** | Full after-write checklist (entity-specific) |
| **Evening** | health-check; daily report; stop-condition review; next-day plan |

## Weekly Review

End of Week-3 (or Day 5):

- [ ] Monitoring evidence summary (reports count, audit gaps)
- [ ] Auth-on staging result
- [ ] Operator feedback summary
- [ ] Open P0/P1 status
- [ ] Financial safety incidents (target: **0**)
- [ ] Expand/hold decision
- [ ] Week-4 candidate inputs

## Not In Scope

- Broad production rollout
- Multi-tenant expansion
- Batch migration execute
- Import execute in pilot operator flow
- Template publish without review
- Payment/billing status automation via low-code
- Mobile driver app / ЭТрН/ЭПД
- API contract changes
- Migrations
- Core business logic changes
- Committing auth-on env to tracked config

## Exit Criteria

Week-3 execution successful if:

1. ≥3 daily monitoring reports (or zero-write documented days)
2. Auth-on staging verified or scheduled with owner
3. Operator feedback attempted for TO/SH/BR
4. No unresolved P0
5. No BR financial side effects from pilot writes
6. Expand/hold decision documented

## Next Decision Point

**End of Week-3:** Week-3 review pack or **Monitoring Evidence Pack** completion triggers:

- **Stable:** Limited expansion decision pack (optional)
- **Issues:** Runtime Pilot Fix Pack
- **Default:** Week-3 review + Week-4 candidate plan

**Immediate next pack:** `Low-code Pilot Week-3 Monitoring Evidence Pack v0.1`

---

Inputs: `LOW_CODE_PILOT_WEEK3_CANDIDATE_PLAN_V0.1.md`, `LOW_CODE_PILOT_WEEK2_CLOSURE_V0.1.md`
