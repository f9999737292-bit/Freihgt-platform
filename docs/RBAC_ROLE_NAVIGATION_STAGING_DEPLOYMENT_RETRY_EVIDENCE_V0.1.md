# RBAC Role Navigation Staging Deployment Retry Evidence v0.1

## Summary

Controlled RBAC role navigation staging deployment retry completed after staging and production web roots were separated.

Deployment was executed only to staging static root:

```text
/var/www/staging-bintrans-web-admin
```

Production root was not modified.

## Decision

```text
RBAC_ROLE_NAVIGATION_STAGING_DEPLOYMENT_RETRY_COMPLETE
```

## User Approval

```text
Подтверждаю RBAC_ROLE_NAVIGATION_STAGING_DEPLOYMENT_RETRY_PACK. Разрешаю deploy только в staging root /var/www/staging-bintrans-web-admin. Production не трогать.
```

## Preconditions

| Item                               | Result                              |
| ---------------------------------- | ----------------------------------- |
| staging/production roots separated | yes                                 |
| staging root                       | /var/www/staging-bintrans-web-admin |
| production root                    | /var/www/bintrans-web-admin         |
| resolved roots distinct            | yes                                 |
| RBAC implementation commit present | yes (`aee3a9d`)                     |
| source diff before deploy          | empty                               |
| build/generate result              | pass                                |

## Artifact

| Item                         | Value                                                                                      |
| ---------------------------- | ------------------------------------------------------------------------------------------ |
| local artifact path          | `C:\Users\Пользователь\AppData\Local\Temp\bintrans-web-admin-rbac-staging-retry-20260730_184529.tar.gz` |
| remote artifact path         | /tmp/bintrans-web-admin-rbac-staging-retry.tar.gz                                          |
| artifact contains index.html | yes                                                                                        |
| build command                | `npx nuxi generate` (no `generate` script in package.json)                                 |
| artifact size                  | 285928 bytes                                                                               |

## Server Deployment

| Item                     | Result                                            |
| ------------------------ | ------------------------------------------------- |
| deploy executed          | yes                                               |
| deploy target            | /var/www/staging-bintrans-web-admin               |
| staging root backup path | /root/rbac-staging-deploy-retry-backup-20260730_184556 |
| production root modified | no                                                |
| Nginx changed            | no                                                |
| Nginx reload executed    | no                                                |
| backend deploy executed  | no                                                |
| migrations executed      | no                                                |
| database writes executed | no                                                |
| DNS changed              | no                                                |
| Certbot executed         | no                                                |

## Root Safety Gate

| Item                | Value                                                                 |
| ------------------- | --------------------------------------------------------------------- |
| staging vhost root  | `/var/www/staging-bintrans-web-admin`                                 |
| production vhost root | `/var/www/bintrans-web-admin`                                       |
| PROD_REAL           | `/var/www/bintrans-web-admin-release-20260717_193920`                 |
| STG_REAL            | `/var/www/staging-bintrans-web-admin`                                 |
| resolved roots distinct | yes                                                               |
| safety gate result  | `ROOT_SAFETY_GATE_PASS`                                               |

## Endpoint Verification

| Check              | Before    | After     |
| ------------------ | --------- | --------- |
| production /       | 200       | 200       |
| production /login  | 200       | 200       |
| production /health | 200       | 200       |
| staging /          | 200       | 200       |
| staging /login     | 200       | 200       |
| staging /health    | 200       | 200       |

## Browser Smoke

| Check                                | Result   |
| ------------------------------------ | -------- |
| staging app opens                    | pass     |
| staging login opens                  | pass     |
| no blank screen                      | pass     |
| no production pre-filled credentials | pass     |
| no dev-only prefill/banner           | pass     |
| sidebar/auth smoke                   | partial  |
| production opens unchanged           | pass     |

Notes:

- Staging `/login` returns full Nuxt HTML shell (title: 7Rights Freight Platform Admin).
- Staging `/`, `/dashboard`, and entity routes redirect to `/login` when unauthenticated (expected SPA behavior).
- No `admin@example.com`, `dev-only`, or prefill markers detected in fetched HTML.
- Production `/` and `/login` remain HTTP 200 with unchanged redirect/login behavior.

## Safety Result

```text
Production changed: no
Production deploy executed: no
Production root modified: no
Staging changed: yes, static web-admin content only
Server changed: yes, staging static web root content only
Nginx changed: no
Nginx reload executed: no
DNS changed: no
Certbot changed: no
Backend changed: no
API contracts changed: no
Migrations changed: no
Database writes executed: no
Secrets captured: no
Deploy scope: staging web-admin static only
```

## Next Recommended Pack

```text
RBAC_ROLE_NAVIGATION_STAGING_POST_DEPLOY_REVIEW_PACK v0.1
```
