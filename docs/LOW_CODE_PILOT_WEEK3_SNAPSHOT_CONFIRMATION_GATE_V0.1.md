# Snapshot Confirmation Gate v0.1

## Summary

Production deployment execution is blocked until a Selectel VM snapshot or equivalent backup is confirmed.

This document does not execute production deploy.

This document does not change server state.

## Current Deployment State

| Item | Status |
| --- | --- |
| Production deployment scope | DEFINED |
| Execution approval | RECORDED |
| Execution pack | BLOCKED_PENDING_SNAPSHOT_CONFIRMATION |
| Backup/snapshot required | yes |
| Rollback required | yes |
| Production deploy | not executed |

## Target Scope

| Field | Value |
| --- | --- |
| Target environment | current Selectel VM / current staging-to-production promotion |
| Target domain | бинтранс.рф |
| Server IP | 161.104.53.221 |
| Responsible operator | Феликс Асаев |
| Go/no-go owner | Феликс Асаев |

## Required Snapshot Confirmation

Before production deployment execution pack can run, owner/operator must provide:

```text
SNAPSHOT_CONFIRMED

Provider: Selectel
Server: 161.104.53.221
Snapshot/backup name: <snapshot or backup name>
Created at: <YYYY-MM-DD HH:MM MSK>
Retention: <manual snapshot / 7 days / other>
Rollback allowed: yes
Owner: Феликс Асаев
```

## Current Decision

```text
SNAPSHOT_CONFIRMATION_REQUIRED
```

## Execution Status

```text
Production deployment execution pack: BLOCKED_PENDING_SNAPSHOT_CONFIRMATION
Production deploy executed: no
```

## References

```text
docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_SCOPE_DEFINITION_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_RUNBOOK_DRAFT_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_PRODUCTION_DEPLOYMENT_PREPARATION_CHECKLIST_V0.1.md
```

## Safety

```text
Backend/frontend source changed during snapshot gate pack: no
Docker compose repo changed: no
UFW changed: no
Nginx changed during snapshot gate pack: no
DNS changed during snapshot gate pack: no
Certbot executed during snapshot gate pack: no
Web-admin redeployed during snapshot gate pack: no
Production deploy executed: no
POST/PUT/PATCH/DELETE executed: no
Secrets captured: no
Certificate private key captured: no
```
