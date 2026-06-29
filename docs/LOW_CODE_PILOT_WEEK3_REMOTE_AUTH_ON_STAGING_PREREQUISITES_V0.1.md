# Low-code Pilot Week-3 Remote Auth-On Staging Prerequisites v0.1

## Summary

Prerequisites for remote auth-on staging repeat verification (PR-GAP-001). Defines required inputs, allowed verification mode, and forbidden operations. **No repeat executed in this pack.**

**Decision:** **REMOTE_AUTH_ON_STAGING_PREREQUISITES_CREATED_PENDING_STAGING_DETAILS**

## Purpose

Document what must exist before **Low-code Pilot Week-3 Remote Auth-On Staging Repeat Pack v0.1** can run safely.

Reference: `LOW_CODE_PILOT_WEEK3_REMOTE_AUTH_ON_REPEAT_V0.1.md`, `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_AUTH_ON_TEST_MATRIX_V0.1.md`

## Required Inputs

| Input | Required | Stored in docs |
|-------|----------|----------------|
| Staging web-admin URL | yes | URL only (no credentials) |
| Staging API gateway URL | yes | URL only |
| Auth mode confirmation (`LOW_CODE_ADMIN_AUTH_ENABLED=true`) | yes | yes/no |
| Admin user available | yes | UUID/email only — **no password** |
| Non-admin user available | yes | UUID/email only — **no password** |
| Tenant ID available | yes | UUID only |
| Read-only verification approved | yes | yes/no |
| Credentials | yes (secure channel) | **never in docs** |

## Required Environment State

- Remote staging server provisioned per intake form
- Docker stack running (or ready to start per approved runbook)
- Auth-on enabled on `low-code-service` for staging
- Health-check endpoints reachable (sanitized GET)
- Local auth-on repeat **PASS** (2026-06-23) — baseline only; does not replace remote repeat

## Allowed Verification Mode

- **Sanitized GET checks only** (HTTP status codes, public routes)
- Document request IDs and endpoint paths without secrets
- Verify admin routes require auth; non-admin denied
- Verify runtime GET compatibility (read-only)
- **No secret capture**
- **No JWT capture**
- **No POST / PUT / PATCH / DELETE** during repeat pack unless explicitly approved in future pack

## Forbidden Operations

- Production writes
- Staging writes (POST/PUT/PATCH/DELETE)
- Deploy without approved runbook
- SSH to server from this docs pack
- Docker commands on remote server from this docs pack
- Storing passwords, JWT, tokens, or `.env` values in docs
- Storing raw production data
- Claiming production-ready
- Closing PR-GAP-001 without completed repeat evidence

## Decision

**REMOTE_AUTH_ON_STAGING_PREREQUISITES_CREATED_PENDING_STAGING_DETAILS**

## Next Pack

**Low-code Pilot Week-3 Remote Auth-On Staging Repeat Pack v0.1**

**Trigger:** Remote staging details provided via intake form

Reference: `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_DETAILS_INTAKE_FORM_V0.1.md`
