# RBAC Role Navigation Staging Post-Deploy Review Evidence v0.1

## Summary

Post-deploy review completed for RBAC role navigation on staging.

No deployment, server changes, Nginx changes, DNS changes, Certbot actions, backend changes, migrations, database writes, or source code changes were executed in this review pack.

## Decision

```text
RBAC_ROLE_NAVIGATION_STAGING_POST_DEPLOY_REVIEW_COMPLETE
```

## Reviewed Deployment

| Item                            | Value                               |
| ------------------------------- | ----------------------------------- |
| RBAC implementation commit      | aee3a9d                             |
| staging deployment retry commit | 0eb46f7                             |
| staging root                    | /var/www/staging-bintrans-web-admin |
| production root                 | /var/www/bintrans-web-admin         |

## Endpoint Review

| Check                      | Result             |
| -------------------------- | ------------------ |
| production /               | 200                |
| production /login          | 200                |
| production /health         | 200                |
| staging /                  | 200                |
| staging /login             | 200                |
| staging /health            | 200                |
| staging /dashboard         | 200 (redirect HTML to /login) |
| staging /shipments         | 200 (redirect HTML to /login) |
| staging /freight-requests  | 200 (redirect HTML to /login) |
| staging /billing-registers | 200 (redirect HTML to /login) |

## Root Separation Review

| Item                    | Result                                                                 |
| ----------------------- | ---------------------------------------------------------------------- |
| staging vhost root      | /var/www/staging-bintrans-web-admin                                    |
| production vhost root   | /var/www/bintrans-web-admin                                            |
| PROD_REAL               | /var/www/bintrans-web-admin-release-20260717_193920                    |
| STG_REAL                | /var/www/staging-bintrans-web-admin                                    |
| resolved roots distinct | yes                                                                    |
| nginx -t read-only      | pass                                                                   |
| staging root content    | RBAC static artifact present (`index.html`, `_nuxt/`, SPA route shells) |
| production root touched | no (read-only sample empty at depth 1; symlink unchanged)              |

## Browser Smoke

| Check                                            | Result        |
| ------------------------------------------------ | ------------- |
| staging app opens                                | pass          |
| staging login opens                              | pass          |
| no blank screen                                  | pass          |
| no production pre-filled credentials             | pass          |
| no dev-only prefill/banner                       | pass          |
| auth routes redirect to login if unauthenticated | pass          |
| no Nginx 404 on SPA routes                       | pass          |
| sidebar authenticated smoke                      | not tested    |
| production opens unchanged                       | pass          |

Notes:

- Staging `/login` returns full Nuxt HTML shell with empty email/password fields (no credential prefill).
- Staging shows mock-mode banner (`Mock-режим активен`) and `mockAuth: true` in baked runtime config — expected for staging static build, not dev-only `import.meta.dev` prefill.
- Unauthenticated SPA routes (`/dashboard`, `/shipments`, etc.) return HTTP 200 with meta-refresh redirect to `/login?redirect=...` — no Nginx 404.
- Production `/` and `/login` remain HTTP 200. Production login retains **pre-existing** demo credential prefill (`demo@7rights.local` / `123456`) from the unchanged production static root — not introduced by RBAC staging deploy.

## Source Static Review

| Check                                           | Result |
| ----------------------------------------------- | ------ |
| usePermissions contains RBAC helpers            | pass   |
| AppSidebar uses nav filtering                   | pass   |
| login uses production-safe credentials behavior | pass   |
| index redirects through landing route           | pass   |

Source review summary (read-only, commit `aee3a9d`):

- `usePermissions.ts`: exports `getProductRoles`, `hasProductRole`, `canAccessRoute`, `canSeeNavItem`, `getLandingRoute`.
- `AppSidebar.vue`: `visibleNavItems = navItems.filter(canSeeNavItem)`.
- `login.vue`: credentials prefilled only when `import.meta.dev`; post-login redirect via `getLandingRoute()`.
- `index.vue`: redirects authenticated users to `getLandingRoute()`, others to `/login`.

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
Review scope: read-only post-deploy review
```

## Next Recommended Pack

```text
RBAC_ROLE_NAVIGATION_STAGING_ACCEPTANCE_SIGNOFF_PACK v0.1
```
