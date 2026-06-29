# Low-code Pilot Week-3 Support Ownership Policy v0.1

## Summary

Defines **support ownership and operating model** for the low-code controlled pilot (PR-GAP-007). **Docs-only** — no support tooling or config changed.

**Decision:** **SUPPORT_OWNERSHIP_PACK_CREATED_PENDING_OWNER_ASSIGNMENT**

**PR-GAP-007:** **SUPPORT_OWNERSHIP_PACK_CREATED_PENDING_OWNER_ASSIGNMENT**

## Purpose

Document who is responsible for pilot support, severity handling, channels, escalation, evidence rules, and forbidden actions before production readiness.

## Scope

- Low-code **runtime** and **admin** modules in controlled pilot
- Incident triage for pilot operators and platform team
- Does **not** close PR-GAP-007 without named support owner and final approval

## Current Status

| Field | Value |
|-------|-------|
| Support Owner | **TBD** |
| Approval | **pending** |
| Production-ready claimed | **no** |
| Controlled pilot | **CONTROLLED_PILOT_APPROVED** — active |
| Support config changed | **no** |

## Support Ownership

| Role | Scope |
|------|-------|
| **Support / Operations / Platform Support Owner** | Owns pilot support triage, escalation, and operator communication |

**Assigned owner:** **TBD**

## Support Channels

| Channel | Use |
|---------|-----|
| Pilot coordinator / PM | Primary intake for operator issues |
| Documented escalation matrix | Severity-based routing |
| Incident notes in feedback log | Sanitized evidence only |

No real phone numbers or personal emails documented in this pack.

## Severity Model

| Severity | Definition |
|----------|------------|
| **P0** | Auth bypass, tenant isolation leak, secrets/JWT/tokens exposure, data corruption risk |
| **P1** | Low-code admin unavailable, migration preview broken, audit evidence missing for critical flow |
| **P2** | Single entity type degraded, non-critical validation issue, UI workflow degraded |
| **P3** | Docs, copy, minor UX, operational clarification |

## Response Rules

| Severity | Target first response |
|----------|----------------------|
| P0 | Immediate — recommend controlled pilot stop / freeze |
| P1 | Same business day |
| P2 | Next business day |
| P3 | Backlog triage |

Reference: `LOW_CODE_PILOT_WEEK3_SUPPORT_ESCALATION_MATRIX_V0.1.md`

## Escalation Rules

- **P0:** Product + Security/Architecture + Support owner (TBD)
- **P1:** Support owner + Platform owner (TBD)
- **P2:** Support owner
- **P3:** Optional escalation via backlog

## Evidence Rules

- Sanitized HTTP status codes, request IDs, endpoint paths allowed
- **No** JWT, tokens, passwords, private keys in support evidence
- **No** raw production personal or financial data
- **No** full DB dumps in tickets or docs

## Forbidden Actions

Without explicit approval:

- Production writes
- Staging writes
- Manual DB edits
- Retention cleanup / log purge
- Template publish / import / migration execute
- Secrets/JWT/tokens sharing
- Raw production data sharing
- Claiming production-ready

## Controlled Pilot Support Boundaries

- Support covers **controlled pilot** scope only (demo tenant, limited users)
- P0 triggers **stop/freeze recommendation** — not automatic production rollback
- Fix Pack v0.1 for P0/P1 suspected incidents (see escalation matrix)

## Owner / Approval Requirements

Before PR-GAP-007 closure:

1. Named support owner assigned
2. Owner role and contact confirmed (contact optional for handover)
3. P0/P1 escalation rules explicitly approved
4. Final support owner sign-off in Support Owner Approval Pack v0.1

## Decision

**SUPPORT_OWNERSHIP_PACK_CREATED_PENDING_OWNER_ASSIGNMENT**

## Next Steps

1. **Low-code Pilot Week-3 Support Owner Approval Pack v0.1**
2. Assign named support owner
3. Do **not** change support tooling or monitoring config in this pack

Reference: `LOW_CODE_PILOT_WEEK3_SUPPORT_OWNER_NOTE_V0.1.md`
