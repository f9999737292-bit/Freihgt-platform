# ADR-RFX-013: ERP Machine Authentication

**Status:** Accepted (architecture freeze — pending controller review)  
**Date:** 2026-09-16  
**Deciders:** E7 Phase 2 ERP architecture stream  
**Normative detail:** [RFX_V3_0E7_ERP_API.md](../implementation/RFX_V3_0E7_ERP_API.md) §5

---

## Context

ERP integration requires **machine-to-machine authentication** without human login. Repository audit findings:

| Mechanism | Status | Evidence |
|---|---|---|
| JWT Bearer (user login) | Implemented | `api-gateway/internal/http/middleware/auth.go` |
| OAuth / client credentials | Not implemented | — |
| API keys (public) | Not implemented | — |
| `X-Internal-Service-Token` | S2S only | `packages/shared-go/internalauth/auth.go` |
| Integration principal | Schema column only | `rfx_external_object_links.integration_principal_id` |
| Public internal token for ERP | **Forbidden** | `RFX_V3_0E7_PHASE2_EXCEL_ERP.md` |

RFx security model (`RFX_V3_SECURITY.md`):

```
CLIENT_SUPPLIED_COMPANY_AUTHORITY=FORBIDDEN
TENANT_AUTHORITY=SERVER_VERIFIED
```

---

## Decision

### Primary: OAuth 2.0 Client Credentials

New token endpoint (proposed):

```
POST /api/v1/integrations/oauth/token
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials
&client_id=...
&client_secret=...
```

Response: short-lived access token (JWT or opaque) with embedded/scoped claims:

| Claim | Source |
|---|---|
| `sub` | `integration_principal_id` |
| `tenant_id` | Credential record |
| `company_id` | Credential record (buyer company) |
| `scope` | Space-delimited scopes |
| `act` kind | `INTEGRATION` (distinct from human `USER`) |

Token TTL: **15 minutes** (proposed; shorter than user JWT 60m).

### Fallback: Hashed API key

For integrators unable to use OAuth immediately:

```
Authorization: Bearer bt_live_<random>
```

- Stored as bcrypt/argon2 hash; plaintext shown once at issuance
- Same scope and binding model as OAuth principal
- Marked `credential_type=API_KEY` for audit

### Optional: mTLS

- Terminated at ingress/load balancer
- Binds client certificate CN/SAN to `integration_principal_id`
- Additional control layer; not required for v1 implementation start

### Explicitly rejected

| Approach | Reason |
|---|---|
| User JWT via service account password | No service account entity; couples ERP to human identity |
| Public `X-Internal-Service-Token` | Forbidden by controller decision |
| Client-supplied tenant/company headers | Spoofing risk; gateway strips these |

---

## Credential model

| Attribute | Policy |
|---|---|
| Owner | Tenant admin provisions integration principal |
| Tenant binding | Immutable on principal; encoded in token |
| Company binding | One primary buyer `company_id`; optional secondary list |
| Scopes | See below |
| Environment binding | `client_id` prefix distinguishes staging/production |
| Expiry | OAuth token 15m; API key until rotation |
| Rotation | Issue new secret; grace window 24h for dual-active |
| Revocation | Immediate; cached token invalid after ≤60s |
| Secret storage | Hash only in DB; KMS for encryption at rest |
| Display-once | API key plaintext shown once at create |
| IP allowlist | Optional per principal (`allowed_cidrs[]`) |
| Audit | All auth failures and rotations logged |

### Scopes (frozen names)

| Scope | Permits |
|---|---|
| `rfx:draft:create` | CREATE preview/commit |
| `rfx:draft:read` | GET event, external lookup |
| `rfx:draft:preview` | Preview operations |
| `rfx:draft:commit` | Commit operations |
| `rfx:status:read` | Analysis status, capabilities |

Naming follows existing policy style (`PolicyBuyerManage` maps to human RBAC; scopes are OAuth strings for M2M).

### Gateway integration

1. New middleware `IntegrationAuth` before ERP routes (or extend `auth.go` with actor kind `INTEGRATION`).
2. Continue stripping spoofed identity headers (`auth_context.go`).
3. Inject trusted `X-Tenant-ID`, `X-Integration-Principal-ID`, `X-Company-ID` from credential record.
4. Map scopes to route requirements (parallel to `rfxrbac.PolicyBuyerManage` for human actors).

### Rate limiting

| Layer | Limit (proposed) |
|---|---|
| Global IP | Existing gateway token bucket |
| Per integration principal | 60 req/min preview; 30 req/min commit |
| Burst | 2× sustained for 10s |

429 response includes `Retry-After`.

### Replay protection

- Short token TTL
- Idempotency-Key on commits (ADR-RFX-014)
- Optional `X-Request-Timestamp` + skew window (future hardening)

---

## Consequences

### Positive

- Clear separation of human vs machine actors
- Aligns with external object link `integration_principal_id`
- OAuth industry standard for SAP/Oracle integrators

### Negative / cost

- New identity tables and admin UI/API for credential lifecycle
- Dual auth path (human JWT + integration token) in gateway
- Controller must authorize before implementation

---

## Security

- Never trust client `tenant_id`, `company_id`, `actor_id`, or internal auth headers
- Cross-tenant credential use → 403 (not 404) on auth layer
- Revoked credential → 401 with `credential_revoked`
- Expired credential → 401 with `credential_expired`

---

## References

- [RFX_V3_SECURITY.md](../RFX_V3_SECURITY.md)
- [ADR-RFX-015](./ADR-RFX-015-ERP-EXTERNAL-IDS-REFERENCE-MAPPING.md)
- `services/api-gateway/internal/http/middleware/auth_context.go`
