# Low-code Pilot Week-3 Live Operator Session Confirmation Checklist v0.1

## Purpose

Checklist for **Virtual PM / Pilot Coordinator** to confirm live operator feedback sessions before execution. Use before marking sessions **SCHEDULED** or running Capture Retry Pack.

Reference: `LOW_CODE_PILOT_WEEK3_LIVE_OPERATOR_SESSION_CONFIRMATION_V0.1.md`

## PM Owner

**Virtual PM / Pilot Coordinator** (virtual)

## Confirmation Checklist

| # | Item | TO | SH | BR | All |
|---|------|----|----|-----|-----|
| 1 | Real operator named | [ ] | [ ] | [ ] | — |
| 2 | Operator availability confirmed | [ ] | [ ] | [ ] | — |
| 3 | Date/time confirmed (not proposed only) | [ ] | [ ] | [ ] | — |
| 4 | Calendar invite sent | [ ] | [ ] | [ ] | — |
| 5 | Environment confirmed (local/staging) | [ ] | [ ] | [ ] | [x] local proposed |
| 6 | Demo entity verified (`make seed-lowcode-demo`) | [ ] | [ ] | [ ] | — |
| 7 | Facilitator assigned | — | — | — | [ ] |
| 8 | Platform admin observer assigned | — | — | — | [ ] |
| 9 | Feedback form + live checklist distributed | [ ] | [ ] | [ ] | [x] docs ready |
| 10 | BR financial safety briefing scheduled | — | — | [x] required | — |
| 11 | Stop rules communicated | [ ] | [ ] | [ ] | [ ] |

**Current status (2026-06-24):** All session rows **unchecked** — **LIVE_SESSION_CONFIRMATION_PENDING**.

## Required Participants

| Role | TO | SH | BR | Assigned |
|------|----|----|-----|----------|
| Logistics / transport operator | required | optional | — | **TBD** |
| Shipment / logistics operator | — | required | — | **TBD** |
| Billing / finance operator | — | — | **required** | **TBD** |
| Platform admin observer | support | support | support | **TBD** |
| Pilot lead (facilitator) | required | required | required | **TBD** |
| Virtual PM / Pilot Coordinator | coordinator | coordinator | coordinator | **assigned** |

## Required Dates

| Session | Proposed | Confirmed |
|---------|----------|-----------|
| TO baseline | 2026-06-30 09:00 | **no — TBD** |
| SH limited | 2026-06-30 14:00 | **no — TBD** |
| BR limited | 2026-07-01 09:00 | **no — TBD** |
| PM wrap-up | 2026-07-01 10:00 | **no — TBD** |

## Required Environment

| Item | Value | Confirmed |
|------|-------|-----------|
| Web UI | `http://localhost:3000` | **proposed** |
| API | `http://localhost:8080` | **proposed** |
| Tenant | `74519f22-ff9b-4a8b-8fff-a958c689682f` | **yes** (demo) |
| Staging alternative | when available | **TBD** |

## Required Demo Entities

| Session | Demo | entity_id | Confirmed seeded |
|---------|------|-----------|------------------|
| TO | DEMO-TO-001 | `2db04b49-665c-469f-bcb1-ffeb1274fedb` | [ ] before session |
| SH | DEMO-SH-PLANNED | `14d405e2-0152-4030-b356-eec464a3cc66` | [ ] before session |
| BR | DEMO-BR-001 | `cf7dbc77-395f-42a2-9717-476e4cd93796` | [ ] before session |

## Required Feedback Forms

- [ ] `LOW_CODE_PILOT_WEEK3_LIVE_OPERATOR_FEEDBACK_CHECKLIST_V0.1.md` — **ready**
- [ ] `LOW_CODE_PILOT_WEEK3_OPERATOR_FEEDBACK_FORM_TEMPLATE_V0.1.md` — **ready**
- [ ] Facilitator knows where to file `FB-W3-001+` — [ ]

## Stop Rules

| Condition | Action |
|-----------|--------|
| P0 during session | Stop; Runtime Fix Pack same day |
| Operator no-show | Do not invent feedback; reschedule |
| Session not confirmed | Do **not** run Capture Retry Pack |
| Proposed slot ≠ confirmed | Keep status NEEDS_CONFIRMATION |

## Ready For Capture Criteria

All must be **yes** before **First Real Operator Feedback Capture Retry Pack v0.1**:

| Criterion | Met |
|-----------|-----|
| At least one real operator assigned | **no** |
| Session date/time confirmed (per entity) | **no** |
| Environment selected | **proposed only** |
| Feedback form/checklist ready | **yes** |
| Facilitator assigned | **no** |
| Stop rules understood | **pending** |
| Live sessions completed | **no** |
| Real feedback forms exist | **no** |

**Ready for capture retry:** **no**
