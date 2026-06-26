# Low-code Pilot Week-3 Monitoring Trigger Matrix v0.1

## Trigger Matrix

| trigger | condition | required action | next pack | owner | urgency |
|---------|-----------|-----------------|-----------|-------|---------|
| PM assigns operators/dates | Human PM names TO/SH/BR operators and proposes or confirms calendar slots | Update session confirmation docs; run minimum read-only checks (health + audit) | Live session confirmation follow-up or Capture Retry prep | Human PM / Virtual PM | P2 |
| `LIVE_SESSION_CONFIRMED` | Operators and dates/times confirmed in writing (not TBD) | Run health + audit + active templates; schedule live sessions | **First Real Operator Feedback Capture Retry Pack v0.1** | Virtual PM / Pilot Coordinator | P2 |
| Real feedback collected | At least one completed operator feedback form (FB-W3-001+) | Triage per feedback runbook; update feedback log and backlog | Feedback-Based UI/Docs Polish Selection (if no P0/P1) | Pilot lead | P2 |
| Ops ready for auth-on repeat | Staging deployment config available; remote env reachable | Execute auth-on curl matrix on remote staging | **Remote Auth-On Repeat Pack v0.1** | DevOps + Security | P3 |
| P0/P1 suspected | Health/smoke failure, operator blocking issue, data loss risk | Stop write packs; diagnose; document incident | **Runtime Pilot Fix Pack v0.1** | Engineering | P0/P1 |
| Runtime/template change | Deploy, template publish, migration execute, or API contract change in pilot env | health + active templates + audit delta; smoke/build if warranted | Monitoring Evidence Refresh Pack v0.1 or Fix Pack if broken | Pilot lead / Engineering | P1–P2 |
| Stakeholder requests fresh evidence | PM, ops, or sponsor asks for current platform proof | Full runbook checks; new evidence snapshot doc | **Monitoring Evidence Refresh Pack v0.1** | Pilot lead | P3 |
| No changes for one week | No PM/operator/ops/runtime updates since last evidence | Optional spot-check: health + audit only; note "no delta" | None (unless spot-check finds issue) | Virtual PM / Pilot Coordinator | P3 |

## Cadence Context

| Field | Value |
|-------|-------|
| Selected cadence | **CADENCE_AD_HOC_ON_EVENT** |
| Automatic continuation v0.8+ | **disabled** |
| Decision doc | `LOW_CODE_PILOT_WEEK3_MONITORING_CADENCE_DECISION_V0.1.md` |
| Runbook | `LOW_CODE_PILOT_WEEK3_MONITORING_CADENCE_RUNBOOK_V0.1.md` |

## Default State (2026-06-26)

| Trigger | Current status |
|---------|----------------|
| PM assigns operators/dates | **not met** — TBD |
| `LIVE_SESSION_CONFIRMED` | **not met** |
| Real feedback collected | **not met** — count 0 |
| Ops ready for auth-on repeat | **not met** — pending |
| P0/P1 suspected | **not met** |
| Runtime/template change | **not met** since v0.7 snapshot |
| Stakeholder fresh evidence | **not requested** |
| One week no changes | **monitor** — optional spot-check after 7 days |

## Blocked Until Trigger

| Work | Waiting on |
|------|------------|
| Capture Retry Pack | `LIVE_SESSION_CONFIRMED` |
| UI/docs polish selection | Real feedback |
| Pilot expansion | Operator sessions + triage |
| Production readiness claim | Real feedback |
| Remote Auth-On Repeat | Ops staging ready |
| Automatic monitoring continuation v0.8+ | **Any trigger above** (not calendar) |
