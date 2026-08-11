# BINTRANS Staging Authentication Smoke Design

Operator procedure after runtime health PASS. **No fabricated test users.**

## Categories

| Gate | What it verifies | Autonomous in repo? |
|------|------------------|---------------------|
| **PROCESS_HEALTH** | Containers running; `/health` endpoints | `bintrans_ct_staging_runtime_health.sh` |
| **AUTH_CONFIGURATION** | `AUTH_ENABLED=true`; JWT externalized in effective compose | `bintrans_ct_staging_runtime_preflight.sh` |
| **LOGIN_FLOW** | identity-service issues JWT for real credentials | **OPERATOR_DATA_REQUIRED** |
| **AUTHORIZED_API_REQUEST** | Gateway accepts Bearer token on protected route | **OPERATOR_DATA_REQUIRED** |
| **TENANT_DATA_FUNCTIONAL_TEST** | Tenant-scoped API with real data | **OPERATOR_DATA_REQUIRED** + approved cohort |

## PROCESS_HEALTH (no credentials)

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_health.sh
```

Confirms api-gateway and identity-service containers are up. Does not prove login.

## AUTH_CONFIGURATION (static / preflight)

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_runtime_preflight.sh
```

Verifies JWT_SECRET contract and rejects dev placeholder in rendered compose.

## LOGIN_FLOW (operator)

Requires real user credentials from approved cohort manifest or operator-supplied test account.

1. Obtain credentials through operator-approved channel (not stored in Git).
2. POST to identity-service login endpoint via gateway internal routing or documented staging path.
3. Expect HTTP 200 and JWT access token in response body.
4. **Do not** commit or print token in reports.

If no credentials exist: document `OPERATOR_DATA_REQUIRED=YES` and stop.

## AUTHORIZED_API_REQUEST (operator)

1. Use JWT from LOGIN_FLOW as `Authorization: Bearer <token>`.
2. Call a documented authenticated gateway route (e.g. health-adjacent ready check or tenant-scoped GET).
3. Expect non-401 response when token valid.

Without token: classify as `AUTH_TOKEN_REQUIRED`, not a startup failure.

## TENANT_DATA_FUNCTIONAL_TEST

Blocked until `COHORT_APPROVED=YES` and cohort manifest has approved tenants.

## JWT architecture reference

See `docs/BINTRANS_STAGING_JWT_AUDIT.md`.
