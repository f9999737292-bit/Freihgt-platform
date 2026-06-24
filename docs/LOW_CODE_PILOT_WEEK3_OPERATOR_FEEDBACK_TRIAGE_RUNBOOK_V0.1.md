# Low-code Pilot Week-3 Operator Feedback Triage Runbook v0.1

## Purpose

Daily triage procedures for Week-3 low-code pilot operator feedback. Links feedback to severity, owners, fix packs, and stop conditions.

**Reference:** `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_COLLECTION_V0.1.md`, `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_LOG_V0.1.md`

## Daily Triage Cadence

| Time | Action |
|------|--------|
| **Morning** | Review NEW entries from prior day; check for overnight P0 |
| **After walkthroughs** | Log forms within 4h; initial severity assignment |
| **Midday** | Re-triage P1 aging >12h without owner |
| **Evening** | Update log statuses; summary for PM; plan next day |

**Minimum:** one triage pass per business day while Week-3 feedback collection is active.

## Severity Rules

| Severity | Criteria | SLA |
|----------|----------|-----|
| **P0** | Stop pilot: security, tenant leak, data corruption, financial unsafe effect, service down, auth bypass | Immediate (same hour) |
| **P1** | Blocks pilot flow; reproducible error; operator cannot complete approved scenario | Fix before next pilot day (≤24h) |
| **P2** | UX/wording/non-blocking; workaround exists | Backlog before expansion |
| **P3** | Suggestion; note only | Log; optional Week-4+ |

When in doubt between P1 and P2 → **P1** if operator marked STOP or cannot complete demo scenario.

## P0 Handling

1. **STOP** affected entity writes (SH/BR/TO as applicable).
2. Set log status **TRIAGED** → **FIX_PLANNED**; owner = PM + pilot lead.
3. Open **Low-code Runtime Pilot Fix Pack v0.1** immediately.
4. Preserve repro: form, audit GET, logs (no secrets in repo).
5. Notify stakeholders; document in daily monitoring report.
6. Do **not** resume writes until PM clears with verification evidence.

## P1 Handling

1. Assign owner (backend / frontend / docs per area).
2. Require repro steps and entity/field scope.
3. Link to **Fix Pack** or approved hotfix PR.
4. Hold entity expansion until fixed or PM waives.
5. Update log: **FIX_PLANNED** → **FIXED** → **CLOSED** after operator confirms or QA verifies.
6. Target resolution ≤24h from triage.

## P2 Handling

1. Log as **ACCEPTED** or **TRIAGED** with backlog reference.
2. No mandatory Week-3 code change.
3. Include in **Feedback Triage & Improvements Backlog Pack v0.1**.
4. PM prioritizes before pilot expansion decisions.

## P3 Handling

1. Log as **ACCEPTED** or **CLOSED** with note.
2. Optional doc/quick-guide update without code.
3. No fix pack unless PM promotes to P2/P1.

## How To Link Feedback To Fix Packs

| Condition | Target pack |
|-----------|-------------|
| P0 any entity | **Low-code Runtime Pilot Fix Pack v0.1** |
| P1 with backend repro | Fix Pack (backend owner) |
| P1 with UI repro | Fix Pack (frontend owner) |
| P2/P3 batch | **Low-code Pilot Week-3 Feedback Triage & Improvements Backlog Pack v0.1** |
| Auth/RBAC P0/P1 | Fix Pack + Security review |
| BR financial safety P0 | Fix Pack + STOP BR writes |

Update feedback log **target pack** column when linked.

## How To Decide Rejection / Acceptance

**Accept** when:

- Reproducible or credible operator report
- In pilot scope (TO/SH/BR low-code panel)
- Actionable summary provided

**Reject** when:

- Duplicate of existing FB-W3 entry (link duplicate id)
- Out of scope (production rollout request, unrelated module)
- Cannot reproduce after NEEDS_INFO follow-up
- Expected behavior documented in operator note/quick guide

**NEEDS_INFO** when:

- Missing entity_id, steps, or severity justification
- Operator unavailable for clarification

Document rejection reason in log **summary** or **notes** (separate triage note if needed).

## Required Evidence

For P0/P1 acceptance into Fix Pack:

| Evidence | Required |
|----------|----------|
| Completed form template | Yes |
| Entity type + demo entity | Yes |
| Steps to reproduce | Yes |
| Expected vs actual | Yes |
| Severity rationale | Yes |
| Audit GET snapshot (if write-related) | When applicable |
| Screenshot/error reference | Recommended (secure storage) |

Read-only feedback (UX clarity) may proceed with form only.

## Escalation Rules

| Condition | Escalate to |
|-----------|-------------|
| P0 | PM + pilot lead immediately |
| P1 unresolved >24h | PM |
| BR financial safety concern (any severity) | PM + finance owner |
| Auth bypass report | Security + PM |
| ≥3 P1 same category in 48h | PM — pattern review |
| Zero feedback by mid-week | Operator lead — schedule sessions |

## Stop Conditions

**STOP** all pilot writes and trigger P0 handling if triage confirms:

- Wrong tenant data visible
- Non-admin admin access in auth-on mode
- Unaudited write or missing audit after approved write
- Operator reports core billing register changed unexpectedly
- Repeated low-code 5xx blocking all operators

After STOP: only Fix Pack and read-only verification until PM clears.

**Cross-reference:** monitoring runbook P0 procedure (`LOW_CODE_PILOT_WEEK3_MONITORING_RUNBOOK_V0.1.md`).
