# Low-code Pilot Week-3 Production Data Policy v0.1

## Summary

Draft **production data policy** for the Week-3 low-code controlled pilot. Defines allowed, restricted, and forbidden data classes and operations. **Final owner approval pending** — production data use **not approved**.

**Decision:** **DATA_POLICY_DRAFT_CREATED_PENDING_OWNER_APPROVAL**

**PR-GAP-002:** **DATA_POLICY_DRAFT_CREATED_PENDING_OWNER_APPROVAL**

## Purpose

Establish rules for what data may be used in low-code pilot environments (dev, demo, staging, production) and what approvals are required before production data access or writes.

## Scope

- Low-code custom fields on **TRANSPORT_ORDER**, **SHIPMENT**, **BILLING_REGISTER**
- Operator feedback, audit evidence, verification logs in docs
- Tenant-bound configuration and template data
- Does **not** approve production deployment or production-ready status

## Current Status

| Field | Value |
|-------|-------|
| Owner | **TBD** (Product / Legal / Data Owner) |
| Approval | **pending** |
| Production-ready claimed | **no** |
| Controlled pilot | **CONTROLLED_PILOT_APPROVED** — active |
| Production data use approved | **no** |

## Data Classification

| Class | Description | Default for controlled pilot |
|-------|-------------|------------------------------|
| **Allowed** | Demo, synthetic, non-sensitive test data | Yes |
| **Restricted** | Staging/tenant data requiring explicit approval | Approval required |
| **Forbidden** | Production PII, financial, legal, secrets | No |

## Allowed Data

For **controlled pilot** only:

- Demo data
- Synthetic data
- Non-sensitive test data
- Operator feedback **without secrets**
- Response codes and verification evidence **without personal or secret data**

## Restricted Data

Requires **explicit approval** before use:

- Limited staging data — only after staging deployment approval
- Tenant-bound data — only with tenant owner approval
- Audit evidence — allowed only **without secrets/tokens**

## Forbidden Data

Without separate documented approval:

- Real production personal data
- Real production financial data
- Signed legal documents
- Payment data
- Secrets, passwords, JWT, tokens
- Private keys
- Real customer confidential data
- Raw production database dumps

## Allowed Operations

For **controlled pilot** (dev/demo tenant):

- Read-only runtime GET verification
- Read-only audit GET verification
- Operator feedback collection (no secrets)
- Docs-only policy and gap closure packs
- Template configuration review (no publish without approval)

## Forbidden Operations

Without approved policy and owner sign-off:

- Production data writes
- Staging data writes (unless separately approved in staging pack)
- Migration execute
- Batch migration execute
- Import execute
- Template publish to production
- Manual DB edits
- Copying production dumps into repo or docs

## Low-Code Custom Fields Policy

Low-code custom fields are **configuration/advisory** fields for pilot scope. They support operator workflows and demos; they do **not** replace core domain records without approval.

## Source-of-Truth Policy

**Decision:** **LOW_CODE_FIELDS_NOT_FINANCIAL_OR_LEGAL_SOURCE_OF_TRUTH_WITHOUT_APPROVAL**

Low-code custom fields are **advisory/configuration fields only** unless separately approved.

Low-code fields **must not** become source of truth for:

- Shipment legal status
- Billing register status
- Payment status
- Signed documents
- Legal documents
- Financial closing
- Government reporting
- ЭТрН / ЭПД official records

Reference: PR-GAP-010, PR-RISK-006

## Tenant Data Policy

- All low-code operations remain **tenant-scoped** (`X-Tenant-ID`)
- Cross-tenant data access is **forbidden**
- Production tenant data requires tenant owner + data policy approval
- Demo tenant only for current controlled pilot

## Audit Requirements

- Audit evidence may be referenced in docs **without secrets**
- No JWT, tokens, or credentials in audit log excerpts committed to repo
- Audit GET used for read-only verification only in pilot

## Retention Notes

- Operator feedback retained per feedback log process
- No production data retention policy approved in this draft
- Final retention rules require **Audit Retention Policy Pack** (PR-GAP-005)

## Approval Requirements

Before production data use or production-ready claim:

1. **Product / Data Owner** — assign and approve policy
2. **Legal / Compliance** — approve restricted/forbidden classes
3. **Finance** — if financial data involved
4. Final sign-off captured in **Production Data Owner Approval Pack v0.1**

## Decision

**DATA_POLICY_DRAFT_CREATED_PENDING_OWNER_APPROVAL**

## Next Steps

1. Assign Product / Data Owner and Legal / Compliance owner
2. Execute **Low-code Pilot Week-3 Production Data Owner Approval Pack v0.1**
3. Do **not** use production data until final approval

Reference: `LOW_CODE_PILOT_WEEK3_PRODUCTION_DATA_POLICY_CHECKLIST_V0.1.md`, `LOW_CODE_PILOT_WEEK3_PRODUCTION_DATA_OWNER_NOTE_V0.1.md`
