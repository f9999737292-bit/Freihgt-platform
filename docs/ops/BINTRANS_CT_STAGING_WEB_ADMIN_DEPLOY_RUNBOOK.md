# BINTRANS CT Staging Web-Admin Deploy Runbook

**Status:** Architecture freeze R3.4A.1 — design + local Docker proof. Staging deploy is R3.4B.

## Governance model

| Field | Value |
|-------|-------|
| GOVERNANCE_MODE | SOLO_MAINTAINER |
| PILOT_MODEL | CONTROLLED_OPERATOR_ASSISTED |
| DB_SCHEMA_VERSION_CURRENT | NOT_VERIFIED (do not assume 19 without live probe) |

## Architecture (frozen)

| Field | Value |
|-------|-------|
| WEB_ADMIN_STAGING_ARCHITECTURE | PRIVATE_NUXT_NODE_CONTAINER |
| WEB_ADMIN_API_ACCESS_MODEL | DUAL_SSH_TUNNEL |
| PUBLIC_INGRESS_REQUIRED | NO |
| WEB_ADMIN_CONTAINER_BIND | 0.0.0.0:13000 (inside container network namespace) |
| WEB_ADMIN_REMOTE_HOST_BIND | 127.0.0.1:13000 (on CT VM — loopback only) |
| WEB_ADMIN_LOCAL_BROWSER_ORIGIN | http://localhost:3000 |
| API_BASE_VISIBLE_TO_BROWSER | http://localhost:18080 |

Container process listens on **0.0.0.0:13000** so Docker host port mapping works. Public exposure remains closed via **127.0.0.1:13000:13000** compose publication.

### CORS-compatible operator origin

api-gateway default `CORS_ALLOWED_ORIGINS` (compose): `http://localhost:3000`, `http://localhost:3001`, `http://localhost:5173`.

Operator browser **must** use `http://localhost:3000` (not `http://127.0.0.1:13000`) so login POST preflight succeeds without api-gateway reconfiguration.

### Why dual SSH tunnel (not same-origin proxy)

- `useApi.buildUrl()` uses `new URL()` with absolute `NUXT_PUBLIC_API_BASE_URL` — relative `/api` bases are **not** supported without product changes.
- Minimal change surface; no backend/nginx proxy required on CT VM.

## Release identity

| Field | Value |
|-------|-------|
| WEB_ADMIN_IMAGE_NAME | cr.selcloud.ru/bintrans-staging/web-admin |
| WEB_ADMIN_IMAGE_TAG_FORMAT | git-`<first-7-of-DEPLOYED_GIT_SHA>` and full SHA tag |
| WEB_ADMIN_RELEASE_LABEL | bintrans.component=web-admin, bintrans.release.sha=DEPLOYED_GIT_SHA |
| WEB_ADMIN_IMMUTABLE_TAGGING | YES |
| WEB_ADMIN_DIGEST_PINNING | YES |

Runtime pin (protected env):

```text
BINTRANS_WEB_ADMIN_IMAGE=cr.selcloud.ru/bintrans-staging/web-admin@sha256:<remote-manifest-digest>
```

Local Docker image ID (`docker image inspect --format '{{.Id}}'`) is **not** the registry manifest digest until after push.

## Operator access (frozen)

**Remote (CT VM loopback):**

| Service | Bind |
|---------|------|
| web-admin | 127.0.0.1:13000 |
| api-gateway | 127.0.0.1:18080 |

**Local (operator workstation via SSH tunnels):**

| Service | Bind |
|---------|------|
| web-admin | localhost:3000 |
| api-gateway | localhost:18080 |

```bash
ssh -i ~/.ssh/bintrans_selectel_staging \
  -L 3000:127.0.0.1:13000 \
  -L 18080:127.0.0.1:18080 \
  root@161.104.57.152
```

Browser entry:

```text
http://localhost:3000/login
```

Do **not** use `http://127.0.0.1:13000` as the operator browser origin.

No public :80/:443, no 0.0.0.0 host frontend publication, no new DNS, no public SG rule.

## Compose overlay

File: `infrastructure/docker-compose/docker-compose.bintrans-ct-staging-web-admin.yml`

Service: `web-admin`  
Project: `bintrans-ct-staging`  
Container: `HOST=0.0.0.0`, `PORT=13000`  
Host publication: `127.0.0.1:13000:13000` only

Does **not** join backend database, does **not** restart api-gateway or other services.

## Build (R3.4B)

```bash
export DEPLOYED_GIT_SHA=<40-char-sha>
export BINTRANS_IMAGE_VERSION=git-${DEPLOYED_GIT_SHA:0:7}
export NUXT_PUBLIC_API_BASE_URL=http://localhost:18080
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_web_admin_release_build.sh
```

After registry push, capture remote manifest digest and set:

```text
BINTRANS_WEB_ADMIN_IMAGE=cr.selcloud.ru/bintrans-staging/web-admin@sha256:<remote-manifest-digest>
```

## Rollback

| Field | Value |
|-------|-------|
| WEB_ADMIN_ROLLBACK_MODEL | DIGEST_PIN_RESTORE |

Restore previous full immutable reference in protected env:

```text
BINTRANS_WEB_ADMIN_IMAGE=cr.selcloud.ru/bintrans-staging/web-admin@sha256:<previous-remote-manifest-digest>
```

Then recreate **web-admin only**:

```bash
docker compose \
  -f infrastructure/docker-compose/docker-compose.bintrans-ct-staging-web-admin.yml \
  --env-file /protected/bintrans/control-tower-observation/staging.env \
  -p bintrans-ct-staging \
  up -d --no-deps web-admin
```

No backend redeploy, DB migration, or DB restore required.

## Local proof (R3.4A.1)

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_web_admin_local_proof.sh
```

Requires Docker daemon. Models operator view with `127.0.0.1:3000:13000` host mapping and CORS preflight proof.

## Restore review requirement when

`RESTORE_REQUIRED_APPROVAL_COUNT_TO_1_WHEN=SECOND_ELIGIBLE_HUMAN_MAINTAINER_ADDED`
