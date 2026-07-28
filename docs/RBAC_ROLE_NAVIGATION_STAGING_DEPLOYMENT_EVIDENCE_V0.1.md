# RBAC Role Navigation Staging Deployment Evidence v0.1

## Summary

Controlled staging-only deployment for RBAC role navigation in web-admin was **blocked** before any staging web root update.

Production deployment was not executed. Backend, API contracts, migrations, database data, Nginx, DNS, and Certbot were not changed.

## Decision

```text
RBAC_ROLE_NAVIGATION_STAGING_DEPLOYMENT_BLOCKED_SHARED_WEB_ROOT
```

## User Approval

```text
Подтверждаю staging deploy RBAC role navigation. Production не трогать.
```

## Deployed Candidate

```text
aee3a9d feat: implement RBAC role navigation in web-admin
```

## Deployment Scope

| Item                       | Result  |
| -------------------------- | ------- |
| staging deploy executed    | blocked |
| production deploy executed | no      |
| backend deploy executed    | no      |
| migrations executed        | no      |
| database writes executed   | no      |
| Nginx/DNS/Certbot changed  | no      |
| role apps deployed         | no      |

## Root Safety Gate

| Item                | Value                      |
| ------------------- | -------------------------- |
| staging web root    | `/var/www/bintrans-web-admin` |
| production web root | `/var/www/bintrans-web-admin` |
| roots are distinct  | **no**                     |
| safety gate result  | **fail — STOP_SHARED_ROOT** |

### Inspection Evidence

Nginx read-only inspection on `161.104.53.221` (2026-07-28):

- `00-bintrans-production.conf` → `root /var/www/bintrans-web-admin;`
- `staging-bintrans.conf` → `root /var/www/bintrans-web-admin;`

Both production (`xn--80abvubqje.xn--p1ai`) and staging (`staging.xn--80abvubqje.xn--p1ai`) vhosts point to the **same** document root.

Per pack critical STOP rule: deployment was not executed.

## Artifact

| Item                 | Value                                       |
| -------------------- | ------------------------------------------- |
| local artifact path  | not uploaded — deploy blocked before upload |
| remote artifact path | not used                                  |
| build command        | `npm run build`                             |
| build result         | pass (Nuxt build completed)                 |
| static artifact note | `.output/public/index.html` not present after `npm run build` (SSR output); artifact tarball not created for deploy |

## Backup

| Item                | Value        |
| ------------------- | ------------ |
| staging backup path | not created  |
| rollback executed   | no           |

## Endpoint Verification

| Check              | Before | After (no deploy) |
| ------------------ | ------ | ----------------- |
| production /       | 200    | not re-checked after blocked stop |
| production /login  | 200    | not re-checked after blocked stop |
| production /health | 200    | not re-checked after blocked stop |
| staging /          | 200    | not re-checked after blocked stop |
| staging /login     | 200    | not re-checked after blocked stop |
| staging /health    | 200    | not re-checked after blocked stop |

Baseline captured before deploy attempt. No post-deploy change expected because deploy did not run.

## Browser Smoke

| Check                                | Result       |
| ------------------------------------ | ------------ |
| staging app opens                    | not deployed |
| staging login opens                  | not deployed |
| no production pre-filled credentials | not deployed |
| no blank screen                      | not deployed |
| sidebar/auth smoke                   | not deployed |

Pre-deploy staging endpoints returned HTTP 200 at baseline.

## Blocker Detail

```text
STAGING_WEB_ROOT == PRODUCTION_WEB_ROOT (/var/www/bintrans-web-admin)

Updating staging web root would simultaneously change production static content served from the same path.
Pack rule: STOP and do not deploy when roots cannot be clearly separated.
```

## Safety Result

```text
Production changed: no
Staging changed: no
Server changed: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Database writes executed: no
Secrets captured: no
Nginx changed: no
DNS changed: no
Certbot changed: no
Deploy executed: no
```

## Next Recommended Pack

```text
STAGING_PRODUCTION_WEB_ROOT_SEPARATION_PLAN_PACK v0.1
```
