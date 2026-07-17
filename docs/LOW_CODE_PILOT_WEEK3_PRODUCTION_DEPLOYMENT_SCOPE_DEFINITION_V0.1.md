# Production Deployment Scope Definition v0.1

## Summary

Owner provided the missing production deployment scope fields after execution approval was recorded.

This document unblocks preparation of a separate production deployment execution pack.

Production deploy is not executed by this document.

## Prior Approval

Execution approval wording:

```text
OWNER_APPROVES_PRODUCTION_DEPLOYMENT_EXECUTION
```

Decision:

```text
PRODUCTION_DEPLOYMENT_EXECUTION_APPROVAL_RECORDED
```

## Deployment Scope

| Field | Value |
| --- | --- |
| Target environment | current Selectel VM / current staging-to-production promotion |
| Target domain | бинтранс.рф |
| Deployment window | 2026-07-17 23:00–01:00 MSK |
| Responsible operator | Феликс Асаев |
| Go/no-go owner | Феликс Асаев |
| Backup/snapshot required | yes |
| Rollback required | yes |

## Important Environment Boundary

```text
No separate production server is available.
The intended deployment path is promotion of the current Selectel staging VM to production domain бинтранс.рф.
```

Current staging domain:

```text
https://staging.бинтранс.рф/
```

Technical / punycode staging domain:

```text
https://staging.xn--80abvubqje.xn--p1ai/
```

Server IP:

```text
161.104.53.221
```

Target production domain:

```text
https://бинтранс.рф/
```

## Execution Status

```text
Production deployment execution authorized: yes, by owner approval
Production deployment scope defined: yes
Production deployment execution pack: ready to prepare
Production deploy executed: no
```

## Decision

```text
PRODUCTION_DEPLOYMENT_SCOPE_DEFINED
```

## References

```text
docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_EXECUTION_APPROVAL_CAPTURE_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_RUNBOOK_DRAFT_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_PREPARATION_CHECKLIST_V0.1.md
```

## Safety

```text
Backend/frontend source changed during scope definition: no
Docker compose repo changed: no
UFW changed: no
Nginx changed during scope definition: no
DNS changed during scope definition: no
Certbot executed during scope definition: no
Web-admin redeployed during scope definition: no
Production deploy executed: no
POST/PUT/PATCH/DELETE executed: no
Secrets captured: no
Certificate private key captured: no
```
