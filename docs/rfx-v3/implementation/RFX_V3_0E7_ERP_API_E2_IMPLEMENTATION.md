# RFx v3.0E7 — ERP API E2 Implementation

**Wave:** E2 (machine authentication)
**Branch:** `feat/rfx-erp-api-e2-machine-auth-v3.0e7`
**Status:** `IMPLEMENTED_ACCEPTED`

## Authorization markers

```
ERP_API_E2_AUTHORIZED=YES
ERP_API_E2_STATUS=IMPLEMENTED_ACCEPTED
ERP_API_E2_IMPLEMENTATION_STARTED=YES
OPERATION_ID=post_obtain_oauth_access_token_via_client_credentials
TOKEN_ENDPOINT=/api/v1/integrations/oauth/token
TOKEN_TTL=900
TOKEN_SIGNING_ALGORITHM=HS256
TOKEN_ISSUER=bintrans/integrations
TOKEN_AUDIENCE=bintrans/api
BCRYPT_COST=12
MIGRATION_000075_CREATED=NO
PUBLIC_ERP_BUSINESS_ROUTES_ADDED=NO
ERP_PREVIEW_IMPLEMENTED=NO
ERP_COMMIT_IMPLEMENTED=NO
ERP_GET_IMPLEMENTED=NO
AUTO_PUBLISH_IMPLEMENTED=NO
INT_196_OCCUPIED=NO
ERP_API_E3_STATUS=NOT_STARTED
ERP_API_E3_AUTHORIZED=NO
CONTROLLER_VERDICT=ACCEPT_ERP_API_E2
CORRECTIVE_COMMIT_REQUIRED=NO
E2_GW_001_STATUS=CLOSED
E2_GW_002_STATUS=CLOSED
E2_IP_001_STATUS=CLOSED
E2_IP_002_STATUS=CLOSED
FINAL_SECURITY_CI_RUN=35366297048
FINAL_SECURITY_CI_HEAD=ef86283320190a130f374fc8b303b7509b698b88
INTEGRATION_JWT_SECRET=SEPARATE_FROM_JWT_SECRET
TOKEN_REVOCATION_WINDOW_SECONDS=900
ONLINE_REVOCATION_IMPLEMENTED=NO
RATE_LIMITER_COORDINATION=IN_MEMORY_PER_INSTANCE
RESIDUAL_RISKS_ACCEPTED_FOR_E2=YES
```

See also: `RFX_V3_0E7_ERP_API_E2_SECURITY_REMEDIATION.md`

## Delivered scope

| Area | Deliverable |
|---|---|
| OAuth token | `POST /api/v1/integrations/oauth/token` (identity-service) |
| API-key foundation | Verifier + internal verify-bearer + gateway middleware |
| Trusted headers | Strip/inject integration identity headers |
| Audit | `rfx.erp.client.authenticated.v1`, `rfx.erp.access.denied.v1` |
| OpenAPI | OAuth token operation + schemas |
| Route manifest | `packages/shared-go/rfx/e7_erp_machine_auth_routes.go` |
| Tests | E7P2-INT-120..131, 186, 190 |

## Explicitly not in E2

ERP Preview/Commit/GET routes, parser/mapping, migration 000075, UI, staging deploy.

## Primary tests

`services/rfx-service/internal/integration/erp/e2_machine_auth_test.go`

## Deployment configuration (required)

- Gateway and identity must use a dedicated `INTEGRATION_JWT_SECRET`, separate from `JWT_SECRET`.
- Identity `TRUSTED_PROXY_CIDRS` must list only actual gateway peer CIDRs (never `0.0.0.0/0` or `::/0`).
- Empty `TRUSTED_PROXY_CIDRS` means no trusted proxies; forwarded headers from untrusted peers are ignored.
- Production must not rely on test/dev default secrets.
