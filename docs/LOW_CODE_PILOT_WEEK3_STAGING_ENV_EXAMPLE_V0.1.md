# Low-code Pilot Week-3 Staging Env Example v0.1

## Purpose

**Documentation only** — example staging environment variables for Ops. This is **NOT** a real `.env` file. Do **not** copy into git.

Reference: `LOW_CODE_PILOT_WEEK3_STAGING_DEPLOY_RUNBOOK_V0.1.md`

## Important Security Warning

- **Never commit** real `.env`, `.env.staging`, or secrets to the repository
- Replace all `<placeholder>` values on the server only
- Store production and staging secrets in separate secure stores
- Docs may only state: *credentials provided separately / not stored*

## Example Variables

```text
# Example only. Do not commit real .env files.
# Store on staging server outside git, e.g. /opt/freight-platform/staging.env

ENVIRONMENT=staging

# --- Database (placeholder) ---
POSTGRES_DB=<staging-db-name>
POSTGRES_USER=<staging-db-user>
POSTGRES_PASSWORD=<provided-by-ops>
DATABASE_URL=postgres://<user>:<password>@postgres:5432/<db>?sslmode=disable

# --- Gateway / API (placeholder) ---
API_GATEWAY_PORT=8080
API_BASE_URL=https://<staging-domain>/api/v1
LOW_CODE_API_URL=https://<staging-domain>/api/v1/low-code

# --- Auth / JWT (placeholder) ---
JWT_SECRET=<provided-by-ops>
JWT_ACCESS_TOKEN_TTL_MINUTES=60

# --- Low-code auth-on (staging pilot) ---
LOW_CODE_ADMIN_AUTH_ENABLED=true
IDENTITY_SERVICE_URL=http://identity-service:8081

# --- Observability (optional placeholders) ---
LOG_LEVEL=info
```

## Low-Code Auth-On Variables

| Variable | Staging value | Notes |
|----------|---------------|-------|
| `LOW_CODE_ADMIN_AUTH_ENABLED` | `true` | Required for PR-GAP-001 remote matrix |
| `IDENTITY_SERVICE_URL` | Internal service URL | low-code-service → identity-service |

After change: **restart low-code-service** only (preferred) or full stack per Ops policy.

## Gateway / API Variables

| Variable | Example placeholder |
|----------|---------------------|
| `API_GATEWAY_PORT` | `8080` (internal) |
| Public URL | `https://<staging-domain>` |
| Low-code API path | `/api/v1/low-code` |

Reverse proxy terminates TLS; upstream HTTP to gateway container.

## Database Variables

| Variable | Rule |
|----------|------|
| `DATABASE_URL` | Staging-only database |
| `POSTGRES_PASSWORD` | Unique; not production |
| Volumes | Staging-named Docker volume |

**No production data** in staging DB.

## Auth / JWT Variables

| Variable | Rule |
|----------|------|
| `JWT_SECRET` | Staging-only secret; rotate independently |
| Login | `POST https://<staging-domain>/api/v1/auth/login` (confirm with gateway routes) |

Passwords for test users: **not stored in this doc**.

## Observability Variables

Optional placeholders:

```text
LOG_LEVEL=info
# METRICS_ENABLED=true
# SENTRY_DSN=<provided-by-ops-if-used>
```

## What Must Never Be Committed

- Real passwords
- Real JWT secrets or tokens
- Real service account keys
- Real `DATABASE_URL` with credentials
- Private TLS keys
- Production credentials
- ngrok / Cloudflare tunnel tokens
- Completed staging input form with secrets pasted in

## Ops Confirmation Checklist

Before handoff to QA:

- [ ] Env file exists **only on server** — not in git
- [ ] `LOW_CODE_ADMIN_AUTH_ENABLED=true` set
- [ ] low-code-service restarted after auth-on change
- [ ] `make health-check` (or equivalent) passes on staging
- [ ] Staging input form filled (UUIDs only — passwords separate)
- [ ] Read-only verification permission **yes**

Next: `LOW_CODE_PILOT_WEEK3_STAGING_INPUT_FORM_V0.1.md`
