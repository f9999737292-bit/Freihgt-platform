# ADR-RFX-013: ERP Machine Authentication

**Status:** Accepted
**Date:** 2026-09-16
**Deciders:** E7 Phase 2 ERP architecture stream
**Normative detail:** [RFX_V3_0E7_ERP_API.md](../implementation/RFX_V3_0E7_ERP_API.md) §10

### Controller acceptance evidence

| Field | Value |
|---|---|
| Reviewed head | `c538bcf073f901f2e80a26b77fb14a6f4a9491c6` |
| Controller verdict | `ACCEPT_ERP_API_ARCHITECTURE` |
| HIGH findings open | 0 |
| MEDIUM findings open | 0 |
| Residual LOW findings | Fixed in acceptance-alignment commit |
| Implementation authorization | NO |

---

## Context

ERP requires M2M auth. Repository audit:

| Mechanism | Status | Evidence |
|---|---|---|
| JWT Bearer (user login) | Implemented | `api-gateway/.../middleware/auth.go` |
| OAuth / client credentials | **Not implemented — new dependency** | — |
| API keys (public ERP) | **Not implemented — new dependency** | — |
| `X-Internal-Service-Token` | S2S only; forbidden for public ERP | `RFX_V3_0E7_PHASE2_EXCEL_ERP.md` |
| Integration principal column | Schema only on external links | migration `000073` |

---

## Decision

### Primary: OAuth 2.0 Client Credentials (new)

```
POST /api/v1/integrations/oauth/token
grant_type=client_credentials&client_id=...&client_secret=...
```

Issued by **new** identity integration layer (not existing user login). Token TTL **15 minutes**.

Claims: `sub=integration_principal_id`, `tenant_id`, `company_id`, `scope`, `act.kind=INTEGRATION`.

### Fallback: API key (explicit principals only)

```
Authorization: Bearer bt_live_<random>
```

Only for principals with `credential_type=API_KEY`. Stored hashed; display-once at issuance.

### Auth scheme binding (M-02)

| Principal type | Allowed scheme | Rejected scheme |
|---|---|---|
| `credential_type=OAUTH` | OAuth bearer token | API key → 401 `auth_scheme_denied` |
| `credential_type=API_KEY` | API key bearer | OAuth token → 401 `auth_scheme_denied` |

- No silent downgrade on OAuth failure.
- Gateway validates scheme matches principal type.
- Audit logs `auth_scheme` (never raw secret).

Test: **E7P2-INT-190**.

### Capabilities auth (M-03)

`GET /integrations/erp/capabilities` requires **`rfx:status:read`**. No unauthenticated access. Response scoped to authenticated principal's tenant/company.

### Scopes

| Scope | Permits |
|---|---|
| `rfx:draft:create` | CREATE preview/commit |
| `rfx:draft:read` | GET ERP DTO, external lookup |
| `rfx:draft:preview` | Preview operations |
| `rfx:draft:commit` | Commit operations |
| `rfx:status:read` | Analysis status, capabilities |

### Trust boundary

- Tenant, company, principal from credential record only.
- Client headers `X-Tenant-ID`, `X-Company-ID`, `X-User-ID` stripped (`auth_context.go`).
- Gateway injects `X-Integration-Principal-ID`, `X-Tenant-ID`, `X-Company-ID`.

### Rate limits

IP bucket (existing) + per-principal: 60 preview/min, 30 commit/min.

---

## Consequences

- New identity tables (migration 000074).
- OAuth is **new infrastructure**, not reuse of user JWT login.
- Dual auth path in gateway with scheme discrimination.

---

## References

- [RFX_V3_SECURITY.md](../RFX_V3_SECURITY.md)
- [ADR-RFX-014](./ADR-RFX-014-ERP-PREVIEW-COMMIT-IDEMPOTENCY.md)
