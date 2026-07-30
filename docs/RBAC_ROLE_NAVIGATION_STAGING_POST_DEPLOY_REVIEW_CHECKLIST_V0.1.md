# RBAC Role Navigation Staging Post-Deploy Review Checklist v0.1

## Endpoint Checks

| Check | Result |
|---|---|
| production / healthy | yes |
| production /login healthy | yes |
| production /health healthy | yes |
| staging / healthy | yes |
| staging /login healthy | yes |
| staging /health healthy | yes |
| staging SPA routes not Nginx 404 | yes |

## Root Checks

| Check | Result |
|---|---|
| staging root is separated | yes |
| production root unchanged | yes |
| resolved roots distinct | yes |
| nginx -t read-only pass | yes |

## Browser Checks

| Check | Result |
|---|---|
| staging login opens | yes |
| no blank screen | yes |
| no production credential prefill | yes |
| no dev-only banner/prefill | yes |
| unauthenticated routes handled | yes |
| authenticated sidebar checked | partial |

## Safety Checks

| Check | Result |
|---|---|
| no deploy executed | yes |
| no server change | yes |
| no Nginx change/reload | yes |
| no DNS/Certbot | yes |
| no backend/API/migration/DB change | yes |
| no source code change | yes |
| no secrets captured | yes |

## Decision

```text
RBAC_ROLE_NAVIGATION_STAGING_POST_DEPLOY_REVIEW_CHECKLIST_CREATED
```
