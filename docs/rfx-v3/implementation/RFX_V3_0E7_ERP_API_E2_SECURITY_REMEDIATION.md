# RFx v3.0E7 — ERP API E2 Security Remediation

**Wave:** E2 (machine authentication remediation)
**Branch:** `feat/rfx-erp-api-e2-machine-auth-v3.0e7`
**Status:** `IMPLEMENTED_PENDING_REVIEW`

## Remediated controller findings

| ID | Fix |
|---|---|
| E2-GW-001 | Central route classifier; public OAuth token exact match bypasses integration bearer middleware |
| E2-GW-002 | Integration JWT/API key limited to manifest integration-protected routes; human routes reject machine tokens |
| E2-IP-001 | Shared `clientip.Resolver` ignores forwarded headers unless peer is in `TRUSTED_PROXY_CIDRS` |
| E2-TEST-125 | Live gateway middleware tests replace mock-only evidence |
| E2-TEST-186 | HTTP 429 via OAuth handler bridge + gateway integration route limiter |
| E2-OAPI-001 | OAuth components scoped to identity-service + unified OpenAPI only |

## Security markers

```
INTEGRATION_JWT_SECRET=SEPARATE_FROM_JWT_SECRET
TOKEN_REVOCATION_WINDOW_SECONDS=900
ONLINE_REVOCATION_IMPLEMENTED=NO
TRUSTED_PROXY_CIDRS=EMPTY_MEANS_NO_TRUSTED_PROXIES
DO_NOT_MERGE=YES
CONTROLLER_VERDICT=PENDING
NEXT_ACTION=REPEAT_INDEPENDENT_CONTROLLER_REVIEW_ERP_API_E2
```

## Residual risks (accepted for E2)

- Issued integration JWT remains valid until expiry (900s) after credential revoke
- Rate limiter remains in-memory per instance (operational limitation documented)
