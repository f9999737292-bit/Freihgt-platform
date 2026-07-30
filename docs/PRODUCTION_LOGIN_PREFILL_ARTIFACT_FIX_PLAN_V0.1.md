# Production Login Prefill Artifact Fix Plan v0.1

## Summary

Production `/login` has a pre-existing demo credential prefill from an older production web-admin artifact.

This issue is separate from the RBAC staging deployment. RBAC staging deployment and QA are complete, and staging login behavior is production-safe.

This pack is planning-only. It does not deploy, does not change source code, does not change server/Nginx/DNS/Certbot, and does not modify production.

## Decision

```text
PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_PLAN_COMPLETE
```

## Current State

| Area                             | Result                                  |
| -------------------------------- | --------------------------------------- |
| RBAC staging QA chain            | complete                                |
| staging login prefill              | not observed (empty credential fields)  |
| production login prefill         | observed as pre-existing artifact issue |
| production endpoints             | 200 / 200 / 200                         |
| staging endpoints                | 200 / 200 / 200                         |
| source production-safe login     | pass (`import.meta.dev` guard present)  |
| production deploy approved       | no                                      |
| source code changed in this pack | no                                      |
| deploy executed in this pack     | no                                      |

## Problem

```text
Production currently serves an older web-admin artifact where login fields are prefilled with demo credentials.
This creates a production-readiness and demo-safety issue.
```

Live verification (2026-07-30): production `/login` HTML contains prefilled demo email field value; staging `/login` does not.

## Root Cause Hypothesis

```text
The production static artifact is older than the current QA-signed web-admin source/staging artifact.
The current source and staging artifact use production-safe login behavior, while production still serves an old build.
Production root resolves to /var/www/bintrans-web-admin-release-20260717_193920.
Staging root is /var/www/staging-bintrans-web-admin with RBAC artifact from 2026-07-30 retry deploy.
```

## Target State

```text
Production /login must render with empty credential fields and no dev-only prefill/banner.
Production must keep HTTPS/root/login/health healthy.
Production deploy must be separately approved before execution.
```

## Fix Options

### Option A — Preferred: production web-admin artifact refresh after readiness approval

```text
Deploy the QA-signed web-admin static artifact/source to production root after a dedicated production readiness and execution approval.
This would also promote RBAC role navigation to production.
```

Pros:

* aligns production with staging QA result;
* avoids special hotfix drift;
* uses already validated RBAC staging chain.

Cons:

* this is a production deploy;
* requires explicit production readiness approval and rollback plan.

### Option B — Hotfix-only production artifact refresh

```text
Prepare a minimal production artifact refresh focused only on removing login prefill, without broad product readiness signoff.
```

Pros:

* narrower visible change.

Cons:

* may create drift from staging;
* still requires production deploy approval;
* may be less clean than promoting the QA-signed staging artifact.

## Recommended Path

```text
Proceed with PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_APPROVAL_PACK v0.1 first.
Then choose either:
1. PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_EXECUTION_PACK v0.1, or
2. RBAC_ROLE_NAVIGATION_PRODUCTION_READINESS_PLAN_PACK v0.1.
```

## Not Approved

```text
Production deploy is not approved.
Production root modification is not approved.
Nginx/DNS/Certbot changes are not approved.
Backend/API/migration/database changes are not approved.
Source code changes are not approved in this pack.
```

## Safety Result

```text
Production changed: no
Production deploy executed: no
Staging deploy executed: no
Server changed: no
Nginx changed: no
Nginx reload executed: no
DNS changed: no
Certbot changed: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Database writes executed: no
Source code changed: no
Secrets captured: no
Plan scope: production login prefill artifact issue only
```

## Next Recommended Pack

```text
PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_APPROVAL_PACK v0.1
```
