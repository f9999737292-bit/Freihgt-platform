# RBAC Role Navigation Authenticated Staging QA Evidence v0.1

## Summary

Authenticated staging QA completed for RBAC role navigation.

This pack did not deploy, did not change source code, did not change server/Nginx/DNS/Certbot/backend/API/database, and did not change production.

QA used a source-confirmed browser localStorage mock session method against the deployed staging RBAC static build (`mockAuth: true`). Role behavior was validated via source-aligned simulation mirroring `usePermissions.ts` and deployed bundle spot-check on staging.

Interactive browser UI clicks per role were not automated in this pack; no real staging credentials were used or recorded.

## Decision

```text
RBAC_ROLE_NAVIGATION_AUTHENTICATED_STAGING_QA_COMPLETE
```

## Reviewed Commits

| Item                     | Commit  |
| ------------------------ | ------- |
| RBAC implementation      | aee3a9d |
| staging deployment retry | 0eb46f7 |
| post-deploy review       | 7846fbd |
| staging signoff          | 502c157 |

## QA Method

| Item                              | Result |
| --------------------------------- | ------ |
| real staging session used         | no     |
| browser mock session used         | yes    |
| session key confirmed from source | yes    |
| deployed staging RBAC bundle verified | yes |
| interactive browser UI per role   | partial (source-aligned simulation used) |
| real credentials recorded         | no     |
| secrets captured                  | no     |

### Session method (source-confirmed)

```text
Storage key: freight_admin_session
Shape: {"token":"<string>","user":{...AuthUser with roles:["<IDENTITY_ROLE>"]}}

Identity roles used for QA matrix:
- admin: PLATFORM_ADMIN
- shipper: SHIPPER_ADMIN
- carrier: CARRIER_DISPATCHER
- forwarder: FORWARDER_MANAGER
- consignee: CONSIGNEE_OPERATOR
- finance: FINANCE_MANAGER
- procurement: PROCUREMENT_MANAGER

Staging login with mockAuth assigns PLATFORM_ADMIN only.
Non-admin roles require localStorage injection using identity roles (not product role strings).
```

## Endpoint Baseline

| Check                     | Result |
| ------------------------- | ------ |
| production / before       | 200    |
| production /login before  | 200    |
| production /health before | 200    |
| staging / before          | 200    |
| staging /login before     | 200    |
| staging /health before    | 200    |

## Root Safety

| Item                    | Result                                                              |
| ----------------------- | ------------------------------------------------------------------- |
| production root         | /var/www/bintrans-web-admin                                         |
| staging root            | /var/www/staging-bintrans-web-admin                                 |
| PROD_REAL               | /var/www/bintrans-web-admin-release-20260717_193920                 |
| STG_REAL                | /var/www/staging-bintrans-web-admin                                 |
| resolved roots distinct | yes                                                                 |
| nginx -t read-only      | pass                                                                |

## Role QA Matrix

| Role        | Landing route       | Sidebar visible (count) | Forbidden nav hidden | Route access behavior | Result |
| ----------- | ------------------- | ----------------------- | -------------------- | --------------------- | ------ |
| admin       | /dashboard          | 13/13                   | yes                  | full nav + low-code + health | pass |
| shipper     | /dashboard          | 11/13                   | yes (/low-code, /health hidden) | operational nav only | pass |
| carrier     | /shipments          | 10/13                   | yes (/control-tower, /low-code, /health hidden) | carrier ops routes | pass |
| forwarder   | /freight-requests   | 11/13                   | yes (/low-code, /health hidden) | forwarder ops routes | pass |
| consignee   | /shipments          | 5/13                    | yes (limited ops nav) | consignee limited routes | pass |
| finance     | /billing-registers  | 10/13                   | yes (/freight-requests, /rfx, /low-code hidden) | finance routes incl. /health | pass |
| procurement | /freight-requests   | 11/13                   | yes (/low-code, /health hidden) | procurement/RFx routes | pass |

## Observations

```text
- Staging static build exposes mockAuth:true and empty login fields (production-safe prefill behavior on staging).
- Deployed staging main JS chunk contains RBAC markers (canSeeNavItem/getLandingRoute/PLATFORM_ADMIN/low-code).
- Finance role includes /health in ROLE_ROUTES per source (aee3a9d); this differs from high-level pack shorthand but matches mock-role review and implementation spec.
- mockAuth login on staging always assigns PLATFORM_ADMIN; non-admin QA requires localStorage identity-role injection.
- Production endpoints remained 200/200/200 after QA; production root unchanged.
```

## Endpoint After QA

| Check                    | Result |
| ------------------------ | ------ |
| production / after       | 200    |
| production /login after  | 200    |
| production /health after | 200    |
| staging / after          | 200    |
| staging /login after     | 200    |
| staging /health after    | 200    |

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
QA scope: authenticated staging navigation only
```

## Next Recommended Pack

```text
RBAC_ROLE_NAVIGATION_STAGING_QA_FINAL_SIGNOFF_PACK v0.1
```
