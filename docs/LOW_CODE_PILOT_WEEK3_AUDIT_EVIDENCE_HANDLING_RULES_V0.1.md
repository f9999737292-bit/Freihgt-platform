# Low-code Pilot Week-3 Audit Evidence Handling Rules v0.1

## Summary

Defines **allowed**, **restricted**, and **forbidden** audit evidence for low-code pilot production readiness. Complements `LOW_CODE_PILOT_WEEK3_AUDIT_RETENTION_POLICY_V0.1.md`.

**Decision:** **AUDIT_EVIDENCE_HANDLING_RULES_CREATED**

## Purpose

Ensure audit evidence and related documentation support investigations without exposing secrets, raw production data, or compliance-sensitive material.

## Allowed Evidence

- Response codes (HTTP status)
- Request IDs / correlation IDs
- Timestamps (UTC)
- Endpoint names without secrets
- Tenant ID only when approved for the investigation
- Anonymized/synthetic examples
- Audit event IDs without sensitive payloads
- Entity type and action type (e.g. `template.publish`, `migration.preview`)
- Operator session reference (non-PII identifier when approved)

## Restricted Evidence

Requires redaction or Compliance owner approval before sharing:

- Partial request bodies (field names only, no values)
- Tenant IDs in external reports
- User identifiers (use internal IDs only)
- Stack traces (sanitize paths and env vars)
- Error messages that may contain internal hostnames

## Forbidden Evidence

Must **never** appear in audit evidence, docs, or committed artifacts:

- Passwords
- JWT
- Tokens (service, API, session)
- Private keys
- Real production personal data
- Real financial data
- Signed legal documents
- Raw database dumps
- Real database credentials
- Payment card or bank data

## Redaction Rules

1. Replace secrets with `[REDACTED]` — never partial-mask tokens.
2. Use synthetic tenant/user IDs in examples.
3. Strip query strings and headers that may contain tokens.
4. Do not commit curl commands with `-H Authorization:` values.
5. External sharing requires Compliance review.

## Storage Rules

- Evidence artifacts live in approved doc packs or secure ops storage — not in public repo.
- No secrets in git history.
- Retention of evidence copies follows audit retention policy (draft pending approval).
- Production log exports require separate approval.

## Sharing Rules

| Audience | Rule |
|----------|------|
| Internal Security / Compliance | Allowed with access rules |
| PM / pilot lead | Allowed for pilot scope only |
| External auditors | Compliance owner approval required |
| Operators | Summary only — no raw audit payloads |
| Public / repo | Forbidden for sensitive evidence |

## Incident Evidence Rules

For P0/P1 incidents:

1. Capture request ID, timestamp, endpoint, response code — **no secrets**.
2. Escalate to Security + PM immediately for auth/tenant/secrets issues.
3. Do not attach raw logs with tokens to tickets or docs.
4. Retention hold may apply — see audit retention policy.

## Decision

**AUDIT_EVIDENCE_HANDLING_RULES_CREATED**

Reference: `LOW_CODE_PILOT_WEEK3_AUDIT_RETENTION_POLICY_V0.1.md`
