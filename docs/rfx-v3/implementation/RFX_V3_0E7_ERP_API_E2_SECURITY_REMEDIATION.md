# RFx v3.0E7 — ERP API E2 Security Remediation

**Wave:** E2 (machine authentication remediation)
**Branch:** `feat/rfx-erp-api-e2-machine-auth-v3.0e7`
**Status:** `IMPLEMENTED_ACCEPTED`

## Remediated controller findings

| ID | Status | Fix |
|---|---|---|
| E2-GW-001 | CLOSED | Central route classifier; public OAuth token exact match bypasses integration bearer middleware |
| E2-GW-002 | CLOSED | Integration JWT/API key limited to manifest integration-protected routes; human routes reject machine tokens |
| E2-IP-001 | CLOSED | Shared `clientip.Resolver` ignores forwarded headers unless peer is in `TRUSTED_PROXY_CIDRS` |
| E2-IP-002 | CLOSED | Immutable socket peer captured before `chi RealIP`; gateway strips spoofed forwarded headers and injects canonical XFF on proxy |
| E2-TEST-001 | CLOSED | INT-125 live gateway tests cover expired/wrong issuer/audience/nbf/signing key and human-token rejection |
| E2-TEST-186 | CLOSED | HTTP 429 via OAuth handler bridge + gateway integration route limiter |
| E2-OAPI-001 | CLOSED | OAuth components scoped to identity-service + unified OpenAPI only |

## Security markers

```
E2_GW_001_STATUS=CLOSED
E2_GW_002_STATUS=CLOSED
E2_IP_001_STATUS=CLOSED
E2_IP_002_STATUS=CLOSED
CONTROLLER_VERDICT=ACCEPT_ERP_API_E2
CORRECTIVE_COMMIT_REQUIRED=NO
FINAL_SECURITY_CI_RUN=35366297048
FINAL_SECURITY_CI_HEAD=ef86283320190a130f374fc8b303b7509b698b88
INTEGRATION_JWT_SECRET=SEPARATE_FROM_JWT_SECRET
TOKEN_REVOCATION_WINDOW_SECONDS=900
ONLINE_REVOCATION_IMPLEMENTED=NO
RATE_LIMITER_COORDINATION=IN_MEMORY_PER_INSTANCE
RESIDUAL_RISKS_ACCEPTED_FOR_E2=YES
TRUSTED_PROXY_CIDRS=EMPTY_MEANS_NO_TRUSTED_PROXIES
WIDE_CIDR_TRUST_FORBIDDEN=YES
```

## Trust boundary summary

- `CapturePeerMiddleware` stores the TCP peer before `chi RealIP` runs.
- Security-sensitive code uses `clientip.Resolver`, which reads the immutable peer—not poisoned `RemoteAddr`.
- Gateway ingress strips `Forwarded`, `X-Forwarded-For`, and `X-Real-IP` from untrusted peers.
- Gateway reverse proxy replaces outbound forwarded headers with a single canonical client IP.
- Identity accepts forwarded client IP only when the immediate peer is in configured `TRUSTED_PROXY_CIDRS`.

## Deployment configuration (required)

- Gateway and identity must use a dedicated `INTEGRATION_JWT_SECRET`, separate from `JWT_SECRET`.
- Identity `TRUSTED_PROXY_CIDRS` must list only actual gateway peer CIDRs (never `0.0.0.0/0` or `::/0`).
- Empty `TRUSTED_PROXY_CIDRS` means no trusted proxies; forwarded headers from untrusted peers are ignored.
- Production must not rely on test/dev default secrets.

## Residual risks (accepted for E2, not remediated)

- Issued integration JWT remains valid until expiry (900s) after credential revoke (`TOKEN_REVOCATION_WINDOW_SECONDS=900`, `ONLINE_REVOCATION_IMPLEMENTED=NO`).
- Rate limiter remains in-memory per instance (`RATE_LIMITER_COORDINATION=IN_MEMORY_PER_INSTANCE`).
