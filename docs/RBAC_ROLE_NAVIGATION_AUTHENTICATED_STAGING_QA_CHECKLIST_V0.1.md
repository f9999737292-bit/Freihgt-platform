# RBAC Role Navigation Authenticated Staging QA Checklist v0.1

## Pre-QA

| Check | Result |
|---|---|
| main synced with origin/main | yes |
| staged files absent before pack | yes |
| staging deploy committed | yes |
| staging signoff committed | yes |
| production endpoints healthy | yes |
| staging endpoints healthy | yes |
| roots distinct | yes |
| session method safe | yes |

## Role QA

| Role | Landing checked | Sidebar checked | Forbidden nav checked | Result |
|---|---|---|---|---|
| admin | yes | yes | yes | pass |
| shipper | yes | yes | yes | pass |
| carrier | yes | yes | yes | pass |
| forwarder | yes | yes | yes | pass |
| consignee | yes | yes | yes | pass |
| finance | yes | yes | yes | pass |
| procurement | yes | yes | yes | pass |

## Safety

| Check | Result |
|---|---|
| no deploy executed | yes |
| no source code change | yes |
| no server change | yes |
| no Nginx change/reload | yes |
| no DNS/Certbot | yes |
| no backend/API/migration/DB change | yes |
| no secrets captured | yes |
| production unchanged | yes |

## Decision

```text
RBAC_ROLE_NAVIGATION_AUTHENTICATED_STAGING_QA_CHECKLIST_CREATED
```
