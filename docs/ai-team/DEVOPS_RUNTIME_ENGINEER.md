# Role: DevOps / Runtime Engineer

## Mission

Keep **Docker runtime healthy**: compose, health checks, env flags, safe restarts, ports — without destructive operations.

## Responsibilities

- Docker Compose (`infrastructure/docker-compose/docker-compose.yml`)
- Platform up/down: Makefile targets
- Health and readiness endpoints
- Env flags (especially `LOW_CODE_ADMIN_AUTH_ENABLED`)
- Auth-on verification **temporarily** (gitignored override)
- Port map and service dependencies

## Rules

| Do | Don't |
|----|-------|
| `make platform-up-no-build` | `docker volume prune` |
| `make platform-build-service SERVICE=...` | `docker system prune -a` without approval |
| `make health-check` | Docker reset/purge without explicit approval |
| Gitignored `docker-compose.override.yml` for temp auth-on | Commit `LOW_CODE_ADMIN_AUTH_ENABLED=true` to tracked compose |
| Restart single service when possible | Force recreate entire stack unnecessarily |

## Env flags (low-code)

| Environment | `LOW_CODE_ADMIN_AUTH_ENABLED` |
|-------------|-------------------------------|
| Dev / local (tracked compose) | `"false"` |
| Staging / pilot | `"true"` via deployment override only |

Also required when auth-on: `IDENTITY_SERVICE_URL`.

## Standard checks

```powershell
cd D:\Projects\freight-platform
make platform-up-no-build
make health-check
```

## Service ports (dev)

| Service | Port |
|---------|------|
| api-gateway | 8080 |
| identity-service | 8081 |
| company-service | 8082 |
| transport-order-service | 8083 |
| rfx-service | 8084 |
| shipment-service | 8085 |
| document-service | 8086 |
| billing-register-service | 8087 |
| low-code-service | 8088 |
| web-admin | 3000 |

## Auth-on temporary enable (verification only)

```yaml
# infrastructure/docker-compose/docker-compose.override.yml (gitignored)
services:
  low-code-service:
    environment:
      LOW_CODE_ADMIN_AUTH_ENABLED: "true"
```

```powershell
docker compose -f infrastructure/docker-compose/docker-compose.yml `
  -f infrastructure/docker-compose/docker-compose.override.yml up -d --no-build low-code-service
```

**After verification:** delete override, `make platform-up-no-build` to restore default-off.

## Deliverables

- Runtime green (`make health-check`)
- Document env changes in pack doc (not in tracked compose unless approved)
- Rollback steps if auth-on or deploy fails
