# Low-code Pilot Week-3 Tenant Isolation Evidence Request v0.1

## Summary

Defines **tenant isolation evidence requirements** for low-code runtime and admin modules (PR-GAP-006). **Docs-only / read-only** — no code changes, no production-ready claim.

**Decision:** **TENANT_ISOLATION_EVIDENCE_PACK_CREATED_PENDING_REVIEW**

**PR-GAP-006:** **TENANT_ISOLATION_EVIDENCE_PACK_CREATED_PENDING_REVIEW**

## Purpose

Document which tenant-bound checks must be confirmed, which endpoint groups are in scope, what evidence is allowed, what is forbidden, and which read-only checks may be performed before production readiness review.

## Scope

- Low-code **runtime** (public) API
- Low-code **admin** API
- **Audit** events for low-code actions
- **Migration** preview and execute (metadata / source review)
- **Batch** migration preview and execute (metadata / source review)
- Does **not** close PR-GAP-006 without formal evidence review

## Current Status

| Field | Value |
|-------|-------|
| Owner | Security / Architecture / Platform Owner — **TBD** |
| Review | **pending** |
| Production-ready claimed | **no** |
| Controlled pilot | **CONTROLLED_PILOT_APPROVED** — active |
| Code changed | **no** |
| Write operations executed | **no** |

## Tenant Isolation Areas

| Area | Requirement |
|------|-------------|
| Runtime active templates | Queries scoped by tenant; no cross-tenant template leak |
| Custom field values | Reads/writes scoped by tenant + entity ownership |
| Audit events | Tenant filter required; events include tenant context |
| Admin templates | List/detail/mutate paths tenant-bound |
| Admin clone/export/import/publish | Operations tenant-bound |
| Migration preview | Source/target tenant ownership validated |
| Migration execute | Tenant-bound ownership before execute (source/docs) |
| Batch migration | Batch scoped to tenant (source/docs) |

## Endpoint Groups In Scope

| Group | Endpoints / operations |
|-------|------------------------|
| Public runtime active form templates | Runtime GET active template by entity_type |
| Public runtime custom field values | Runtime GET/PUT custom field values |
| Public runtime audit events | Audit GET (where exposed) |
| Admin form templates | Admin list, detail, create, update |
| Admin template clone/export/import/publish metadata | Admin clone, export, import preview, publish |
| Migration preview | Admin migration preview |
| Migration execute metadata | Admin migrate-to-active (docs/source review only) |
| Batch migration preview/execute metadata | Batch preview/execute (docs/source review only) |

## Evidence Required

- Tenant-bound query/filter evidence (source or sanitized read-only GET)
- Tenant-bound write ownership evidence (source/docs only — no writes in this pack)
- No cross-tenant read evidence (negative test or code review)
- No cross-tenant write evidence (negative test or code review)
- Tenant-bound audit event evidence
- Active template selection per tenant evidence
- Admin endpoints tenant-bound evidence

## Evidence Not Allowed

- Secrets
- Passwords
- JWT
- Service tokens
- Private keys
- Raw production personal data
- Raw production financial data
- Full DB dumps
- Signed legal documents

## Read-only Checks

| Check | Allowed |
|-------|---------|
| Local source inspection | **yes** |
| Docs inspection | **yes** |
| Local/dev read-only GET | **yes** (sanitized evidence only) |
| Staging/production GET | **no** in this pack |
| POST/PUT/PATCH/DELETE | **no** |

## Review Requirements

Before PR-GAP-006 closure:

1. Security / Architecture owner reviews evidence log and checklist
2. Cross-tenant negative tests confirmed (runtime/staging as applicable)
3. Residual risks (e.g. tenant header source) accepted or mitigated
4. Explicit review sign-off in **Tenant Isolation Evidence Review Pack v0.1**

## Decision

**TENANT_ISOLATION_EVIDENCE_PACK_CREATED_PENDING_REVIEW**

## Next Steps

1. Execute **Low-code Pilot Week-3 Tenant Isolation Evidence Review Pack v0.1**
2. Optionally run sanitized local read-only GET matrix (two tenants)
3. Do **not** claim production-ready while PR-GAP-006 open

Reference: `LOW_CODE_PILOT_WEEK3_TENANT_ISOLATION_EVIDENCE_CHECKLIST_V0.1.md`
