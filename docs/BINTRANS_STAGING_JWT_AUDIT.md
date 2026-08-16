# BINTRANS Staging JWT Audit

Static trace of JWT usage for BINTRANS dedicated Control Tower staging runtime preparation.

## Summary

| Item | Finding |
|------|---------|
| Token issuer | **identity-service** |
| Token verifier | **api-gateway** |
| Shared secret required | **YES** — both services read `JWT_SECRET` |
| Algorithm | **HS256** |
| Other `JWT_SECRET` consumers | **None** in runtime service set |
| Restart on secret change | **YES** — both services load secret at startup |
| Staging compose override | `docker-compose.bintrans-ct-staging.yml` sets `JWT_SECRET: ${JWT_SECRET}` for api-gateway and identity-service |

## identity-service (signs tokens)

- Config: `services/identity-service/internal/config/config.go` reads `JWT_SECRET`; defaults to `dev_secret_change_me` if unset.
- Signing: `services/identity-service/internal/platform/security/jwt.go` uses `jwt.SigningMethodHS256`.
- Wired in: `services/identity-service/cmd/server/main.go` → `security.NewJWTService(cfg.JWTSecret, ...)`.

## api-gateway (verifies tokens)

- Config: `services/api-gateway/internal/config/config.go` reads `JWT_SECRET`; defaults to `dev_secret_change_me` if unset.
- Middleware: `services/api-gateway/internal/http/router.go` → `gwmiddleware.Auth(cfg.AuthEnabled, cfg.JWTSecret)`.
- Verification: `services/api-gateway/internal/http/middleware/auth.go` accepts **HS256** only.

When `AUTH_ENABLED=true` (required for staging runtime), gateway rejects requests without valid JWT signed with the same secret identity-service uses.

## Compose layering

| Layer | JWT_SECRET effective value |
|-------|----------------------------|
| `docker-compose.yml` (base) | Hardcoded `dev_secret_change_me` |
| `docker-compose.bintrans-ct-staging.yml` | `${JWT_SECRET}` from protected env |
| `docker-compose.staging-shadow.yml` | Does not override JWT |

BINTRANS staging runtime preflight renders compose with shadow + images profiles and **rejects** effective `JWT_SECRET: dev_secret_change_me`.

## Minimum practical requirements

Repository does not enforce secret length in Go code (empty → dev default). Staging operator scripts enforce:

- exactly one `JWT_SECRET=` assignment in protected env
- non-empty, non-placeholder
- minimum **32 characters** (`bintrans_require_nonplaceholder_jwt_secret`)

## Control Tower mode (separate from JWT)

| `CONTROL_TOWER_READ_MODEL_MODE` | Public API behavior |
|-----------------------------------|---------------------|
| `shadow` | Legacy aggregate remains public response; read-model fetched for comparison |
| `primary` | Read-model becomes public source (with legacy fallback) |
| `disabled` | Read-model path off (base compose default) |

BINTRANS staging requires **shadow**. Runtime preflight extracts effective mode from rendered `api-gateway` service block.

## Observation vs runtime

- `JWT_TOKEN` in protected env is for **Day 0 observation tooling** only (`scripts/ops/control_tower_shadow_observation/`).
- **Not required** for container startup or runtime health infrastructure smoke.

## Operator action

Before first runtime deploy, provision a strong unique `JWT_SECRET` in `/protected/bintrans/control-tower-observation/staging.env` only. Restart identity-service and api-gateway after change.
