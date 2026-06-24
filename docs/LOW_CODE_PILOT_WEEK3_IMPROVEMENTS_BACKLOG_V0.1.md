# Low-code Pilot Week-3 Improvements Backlog v0.1

## Summary

Consolidated **improvements backlog** for Week-3 low-code pilot feedback triage. Contains baseline placeholder items while **no real operator submissions** exist, plus structure for P0–P3 items when feedback arrives.

**Backlog status:** Baseline only — **0 real feedback-derived items**. **9** conservative baseline items (BL-W3-000–008). First session **pending operator** — see First Operator Feedback Session v0.1.

Reference: `LOW_CODE_PILOT_WEEK3_FEEDBACK_TRIAGE_AND_BACKLOG_V0.1.md`, `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_EVIDENCE_V0.1.md`, `LOW_CODE_PILOT_WEEK3_FIRST_OPERATOR_FEEDBACK_SESSION_V0.1.md`

## Backlog Status

| Metric | Value |
|--------|-------|
| Total items | **9** |
| Real feedback-derived | **0** |
| Baseline (no-real-feedback) | **9** |
| Open P0 | **0** |
| Open P1 | **0** |
| Open P2 | **0** |
| Open P3 / baseline | **9** |
| Last updated | 2026-06-24 |
| Evidence status | **FIRST_SESSION_PENDING_OPERATOR** — API validation OK; live operator retry required |

## Backlog Table

| id | source | entity_type | category | severity | summary | proposed action | owner | target pack | status | decision |
|----|--------|-------------|----------|----------|---------|-----------------|-------|-------------|--------|----------|
| BL-W3-000 | baseline / feedback evidence pack | TRANSPORT_ORDER | Documentation/runbook | P3 | Collect first TRANSPORT_ORDER baseline operator feedback | Retry: live walkthrough on DEMO-TO-001 with operator; API pre-check passed 3/3 fields | operator lead | First Operator Feedback Session Retry v0.1 | OPEN | GO_WITH_CONDITIONS |
| BL-W3-001 | baseline / Week-3 triage pack | SHIPMENT | Documentation/runbook | P3 | Collect first real operator feedback for SHIPMENT limited write pilot | Retry: walkthrough DEMO-SH-PLANNED; distribute SH quick guide; no Save in session | operator lead | First Operator Feedback Session Retry v0.1 | OPEN | GO_WITH_CONDITIONS |
| BL-W3-002 | baseline / Week-3 triage pack | BILLING_REGISTER | Documentation/runbook | P3 | Collect first real operator feedback for BILLING_REGISTER limited write pilot | Retry: walkthrough DEMO-BR-001; financial safety briefing mandatory | operator lead | First Operator Feedback Session Retry v0.1 | OPEN | GO_WITH_CONDITIONS |
| BL-W3-003 | baseline / auth-on partial | ALL | Permission/auth clarity | P3 | Repeat auth-on verification on remote staging when ops ready | Ops enables deployment config; re-run auth-on runbook curl matrix | DevOps + Security | Remote Auth-On Repeat v0.1 | OPEN | GO_WITH_CONDITIONS |
| BL-W3-004 | baseline / monitoring | ALL | Audit visibility | P3 | Review audit visibility after first real pilot day | After first write: verify audit GET shows event; document operator findability | pilot lead | Monitoring Report Review v0.1 | OPEN | GO_WITH_CONDITIONS |
| BL-W3-005 | baseline / UX | ALL | Field label/help text | P3 | Review field label/help clarity after first operator session | Triage UX feedback; doc-only polish if P3 | pilot lead | UI Help Text Polish v0.1 | OPEN | GO_WITH_CONDITIONS |
| BL-W3-006 | baseline / financial | BILLING_REGISTER | Financial safety wording | P3 | Review financial safety wording for BILLING_REGISTER with operator | Walkthrough BR fields; confirm operator understands low-code does not change core billing/payment status | operator lead + PM | First Operator Feedback Session v0.1 | OPEN | GO_WITH_CONDITIONS |
| BL-W3-007 | baseline / monitoring | ALL | Monitoring/reporting | P3 | Review monitoring report completeness after first real write day | Fill SH/BR daily report template or document zero-write day | pilot lead | Monitoring Report Review v0.1 | OPEN | GO_WITH_CONDITIONS |
| BL-W3-008 | baseline / feedback evidence pack | ALL | Audit visibility | P3 | Review audit visibility with operator during first feedback session | Retry: ask operator to locate audit history during live session | operator lead | First Operator Feedback Session Retry v0.1 | OPEN | GO_WITH_CONDITIONS |

## P0 Items

**None.**

When P0 appears: add row with status **OPEN**, target pack **Low-code Runtime Pilot Fix Pack v0.1**, decision **STOP**.

## P1 Items

**None.**

When P1 appears: add row with owner assigned, target fix pack, decision **GO_WITH_CONDITIONS** until fixed.

## P2 Items

**None.**

Route to weekly review; may batch in future improvement packs.

## P3 Items

All current items are **P3 baseline** (see Backlog Table). No code changes approved from these items alone.

## No-real-feedback Baseline Items

Explicit list (BL-W3-000–008):

1. Collect first TRANSPORT_ORDER baseline operator feedback
2. Collect first SHIPMENT operator feedback
3. Collect first BILLING_REGISTER operator feedback
4. Repeat auth-on on remote staging when ops ready
5. Review audit visibility after first real pilot day
6. Review audit visibility with operator during first session
7. Review field label/help after first operator session
8. Review BR financial safety wording with operator
9. Review monitoring report completeness after first real write day

**Rule:** Do not create backend/frontend code tasks from baseline items without real P0/P1 evidence.

## Candidate Future Packs

| Pack | Trigger | Notes |
|------|---------|-------|
| **Low-code Pilot Week-3 Operator Feedback Evidence Pack v0.1** | Completed — no submissions yet | Documents pending state |
| **Low-code Pilot Week-3 First Operator Feedback Session Pack v0.1** | Completed — pending operator | API validation OK; no live operator |
| **Low-code Pilot Week-3 First Operator Feedback Session Retry Pack v0.1** | **Next Action** | Live operator walkthrough TO/SH/BR |
| **Low-code Pilot Week-3 Remote Auth-On Repeat Pack v0.1** | Ops staging config ready | BL-W3-003 |
| **Low-code Pilot Week-3 Pilot UI Help Text Polish Pack v0.1** | P3 UX themes after operator session | Docs/UI copy only; no API change |
| **Low-code Pilot Week-3 Monitoring Report Review Pack v0.1** | First real write day | BL-W3-004, BL-W3-007 |
| **Low-code Runtime Pilot Fix Pack v0.1** | P0 or blocking P1 | STOP writes until cleared |

## Not Approved Yet

| Item | Reason |
|------|--------|
| Broad production rollout | Week-3 plan rejects without new decision note |
| Second SH/BR demo entity | Requires monitoring clean + operator sign-off |
| Service-level RBAC on runtime PUT | Out of scope v0.1 |
| Code fixes from baseline P3 items alone | No real P0/P1 evidence |
| Committing auth-on to tracked compose | Policy — deployment override only |

### Adding new backlog rows

When real feedback arrives:

1. Copy form from `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORM_TEMPLATE_V0.1.md`
2. Add row to feedback log (`FB-W3-###`)
3. Triage → add/update row here with source `operator feedback FB-W3-###`
4. Link target pack per severity rules
