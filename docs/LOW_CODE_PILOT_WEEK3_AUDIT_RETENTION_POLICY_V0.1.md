# Low-code Pilot Week-3 Audit Retention Policy v0.1

## Summary

Draft **audit retention policy** for low-code runtime and admin modules (PR-GAP-005). Defines audit events in scope, retention rules, access rules, evidence requirements, and forbidden data. **Docs-only** — no real retention config changed.

**Decision:** **AUDIT_COMPLIANCE_OWNER_FINAL_APPROVAL_CAPTURED**

**PR-GAP-005:** **CLOSED_APPROVED_BY_OWNER**

## Purpose

Establish what low-code audit events are retained, for how long, who may access them, what evidence may be recorded, what is forbidden, and which approvals are required before production deployment.

## Scope

- Low-code **runtime** custom field value writes
- Low-code **admin** template lifecycle (create, update, clone, import, export, publish)
- Low-code **migration** preview and execute (when separately approved)
- **Batch** migration preview and execute
- **Admin access** and forbidden access attempts
- **Tenant-bound** low-code admin actions
- **Audit read** access evidence
- Does **not** configure production log retention, database TTL, or automated purge jobs

## Current Status

| Field | Value |
|-------|-------|
| Owner | **Феликс Асаев** (Audit / Compliance / Security Owner) |
| Approval | **approved** |
| Current Approval Status | **APPROVED_BY_AUDIT_COMPLIANCE_OWNER** |
| Approved By | **Феликс Асаев** |
| Production-ready claimed | **no** |
| Controlled pilot | **CONTROLLED_PILOT_APPROVED** — active |
| Real retention config changed | **no** |

**Important:** This approval is **docs-only**. No real retention config was changed. Production-ready is **not** claimed.

## Audit Events In Scope

| Category | Events |
|----------|--------|
| Template lifecycle | create, update, clone, import, export, publish |
| Migration | preview (always); execute (only when separately approved) |
| Batch migration | preview, execute (only when separately approved) |
| Custom field writes | runtime PUT / custom field value write events |
| Admin access | successful admin access to low-code admin routes |
| Forbidden access | non-admin or anonymous forbidden attempts on admin routes |
| Tenant-bound actions | admin actions scoped to `X-Tenant-ID` |
| Audit read access | audit GET / audit list access evidence when available |

## Audit Events Out Of Scope

- Core billing/payment ledger mutations outside low-code module
- Non-low-code service logs unless linked to low-code incident
- Operator personal notes or offline spreadsheets
- Full HTTP request/response bodies containing secrets
- Production database backups used as audit substitutes

## Retention Rules

| Rule | Draft proposal | Status |
|------|----------------|--------|
| Minimum retention (controlled pilot / dev) | 30 days | **proposed — pending owner approval** |
| Minimum retention (production) | 90 days | **proposed — pending owner approval** |
| Security incident audit events | 365 days | **proposed — pending owner approval** |
| Purge / deletion | Automated purge **blocked** until owner approval and ops runbook | **pending** |
| Retention config change | Requires Audit/Compliance owner approval + separate ops pack | **pending** |

## Access Rules

| Role | Access |
|------|--------|
| Audit / Compliance owner | Full audit read for approved investigations |
| Security | Read for P0/P1 security incidents |
| QA / pilot lead | Read for pilot verification (dev/demo tenant) |
| Operators | No direct audit admin access unless separately granted |
| Non-admin users | **Denied** on audit admin routes |
| Anonymous | **Denied** |

Audit read access must be **protected** by authentication and tenant scoping.

## Evidence Rules

Allowed in audit evidence and related docs:

- Response codes (HTTP status)
- Request IDs / correlation IDs
- Timestamps (UTC)
- Endpoint names **without** secrets
- Tenant ID **only when approved** for the investigation
- Anonymized or synthetic examples
- Audit event IDs **without** sensitive payloads

See `LOW_CODE_PILOT_WEEK3_AUDIT_EVIDENCE_HANDLING_RULES_V0.1.md`.

## Security Rules

1. Audit evidence must **never** contain secrets (passwords, JWT, tokens, private keys).
2. Audit read routes require admin/auth-on in production target.
3. Cross-tenant audit access is **forbidden**.
4. Audit export for external sharing requires Compliance owner approval.
5. No production retention purge without documented approval.

## Tenant Isolation Requirements

- Audit events must be **tenant-scoped** where applicable.
- Audit read queries must enforce tenant boundary.
- Cross-tenant audit leakage is **P0** — escalate to Security + PM.
- Tenant isolation production evidence remains **PR-GAP-006** (separate pack).

## Forbidden Data In Audit Evidence

Audit evidence must **not** include:

- Passwords
- JWT
- Service tokens
- Private keys
- Real database credentials
- Raw production personal data
- Signed legal documents
- Payment data
- Full production dumps

## Deletion / Redaction Rules

| Action | Rule |
|--------|------|
| Manual deletion | **Forbidden** without Audit/Compliance owner approval |
| Automated purge | **Blocked** until retention period approved and ops runbook exists |
| Redaction | Required before sharing evidence externally |
| Incident hold | Security may request retention hold for P0/P1 — owner notified |
| Logs/audit cleanup | **Not executed** in this pack |

## Owner / Approval Requirements

Before PR-GAP-005 closure:

1. Named **Audit / Compliance / Security** owner assigned
2. Retention periods confirmed for production
3. Access rules and audit read protection accepted
4. Forbidden evidence rules accepted
5. Explicit final approval captured in **Audit Compliance Owner Approval Pack v0.1**

## Decision

**AUDIT_RETENTION_POLICY_DRAFT_CREATED_PENDING_OWNER_APPROVAL**

## Next Steps

1. Execute **Low-code Pilot Week-3 Audit Compliance Owner Approval Pack v0.1**
2. Do **not** change real retention config or purge logs until approved
3. Do **not** claim production-ready while PR-GAP-005 open

Reference: `LOW_CODE_PILOT_WEEK3_AUDIT_RETENTION_CHECKLIST_V0.1.md`
