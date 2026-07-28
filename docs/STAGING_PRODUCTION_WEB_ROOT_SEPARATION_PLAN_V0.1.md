# Staging / Production Web Root Separation Plan v0.1

## Summary

RBAC staging deployment was blocked because staging and production currently share the same web root.

This plan defines a safe separation approach before any staging deployment can continue.

This pack is planning-only. It does not change source code, server configuration, Nginx, DNS, Certbot, production, staging, API contracts, migrations, or database data.

## Decision

```text
STAGING_PRODUCTION_WEB_ROOT_SEPARATION_PLAN_COMPLETE
```

## Current Blocker

```text
RBAC_ROLE_NAVIGATION_STAGING_DEPLOYMENT_BLOCKED_SHARED_WEB_ROOT
```

## Current State

| Environment | Domain                          | Vhost File                         | Current Web Root            |
| ----------- | ------------------------------- | ---------------------------------- | --------------------------- |
| production  | xn--80abvubqje.xn--p1ai         | `00-bintrans-production.conf`      | `/var/www/bintrans-web-admin` |
| staging     | staging.xn--80abvubqje.xn--p1ai | `staging-bintrans.conf`            | `/var/www/bintrans-web-admin` |

Read-only inspection (2026-07-28, `161.104.53.221`):

- `/var/www/bintrans-web-admin` exists as symlink → `/var/www/bintrans-web-admin-release-20260717_193920`
- `/var/www/staging-bintrans-web-admin` does **not** exist

Endpoint baseline: production and staging root/login/health all HTTP 200.

## Problem

```text
Staging and production use the same document root.
Any staging static frontend update would also update production static content.
Therefore staging deployment must remain blocked until roots are separated.
```

## Target State

| Environment | Domain                          | Target Web Root                     |
| ----------- | ------------------------------- | ----------------------------------- |
| production  | xn--80abvubqje.xn--p1ai         | `/var/www/bintrans-web-admin`       |
| staging     | staging.xn--80abvubqje.xn--p1ai | `/var/www/staging-bintrans-web-admin` |

## Recommended Separation Strategy

```text
1. Keep production root unchanged.
2. Create a dedicated staging root: /var/www/staging-bintrans-web-admin.
3. Copy current production/static content into the new staging root as baseline.
4. Update only staging Nginx vhost (staging-bintrans.conf) root to /var/www/staging-bintrans-web-admin.
5. Run nginx -t.
6. Reload Nginx only after syntax passes.
7. Verify staging root/login/health.
8. Verify production root/login/health.
9. Only after this, retry RBAC staging deployment.
```

## Explicit Non-goals

```text
No production deployment.
No production web root update.
No backend deployment.
No database migration.
No DNS change.
No Certbot action.
No role app deployment.
No pilot user onboarding.
```

## Required Next Approval

```text
STAGING_WEB_ROOT_SEPARATION_APPROVAL_PACK v0.1
```

## Safety Result

```text
Production changed: no
Staging changed: no
Server changed: no
Nginx changed: no
DNS changed: no
Certbot changed: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Database writes executed: no
Secrets captured: no
Deploy executed: no
```

## Next Recommended Pack

```text
STAGING_WEB_ROOT_SEPARATION_APPROVAL_PACK v0.1
```
