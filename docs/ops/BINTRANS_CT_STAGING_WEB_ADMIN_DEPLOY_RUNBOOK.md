# BINTRANS CT Staging Web-Admin Deploy Runbook

**Status:** Architecture freeze R3.4A — design + local proof only. Staging deploy is R3.4B.

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
| WEB_ADMIN_HOST_BIND | 127.0.0.1:13000 |
| API_BASE_VISIBLE_TO_BROWSER | http://127.0.0.1:18080 |

### Why Nuxt Node (not static nginx)

- Existing `apps/web-admin/Dockerfile` already targets production Node server (`.output/server/index.mjs`).
- Deep-link routes (`/dashboard`, authenticated pages) work without separate SPA fallback rules.
- Lazy i18n bundles load correctly in production mode.
- Pinia session restore is client-side; no SSR dependency for pilot login flow.

### Why dual SSH tunnel (not same-origin proxy)

- `useApi.buildUrl()` uses `new URL()` with absolute `NUXT_PUBLIC_API_BASE_URL` — relative `/api` bases are **not** supported without product changes.
- Pilot scripts already use SSH tunnel to `127.0.0.1:18080`.
- Minimal change surface; no backend/nginx proxy required on CT VM.

## Release identity

| Field | Value |
|-------|-------|
| WEB_ADMIN_IMAGE_NAME | cr.selcloud.ru/bintrans-staging/web-admin |
| WEB_ADMIN_IMAGE_TAG_FORMAT | git-`<first-7-of-DEPLOYED_GIT_SHA>` and full SHA tag |
| WEB_ADMIN_RELEASE_LABEL | bintrans.component=web-admin, bintrans.release.sha=DEPLOYED_GIT_SHA |
| WEB_ADMIN_IMMUTABLE_TAGGING | YES |
| WEB_ADMIN_DIGEST_PINNING | YES — BINTRANS_WEB_ADMIN_IMAGE=@sha256:… in protected env |

## Operator access (frozen)

```bash
ssh -i ~/.ssh/bintrans_selectel_staging \
  -L 13000:127.0.0.1:13000 \
  -L 18080:127.0.0.1:18080 \
  root@161.104.57.152
```

Browser entry:

```text
http://127.0.0.1:13000/login
```

No public :80/:443, no 0.0.0.0 frontend bind, no new DNS, no public SG rule.

## Compose overlay

File: `infrastructure/docker-compose/docker-compose.bintrans-ct-staging-web-admin.yml`

Service: `web-admin`  
Project: `bintrans-ct-staging`  
Port: `127.0.0.1:13000:13000` only

Does **not** join backend database, does **not** restart api-gateway or other services.

## Build (R3.4B)

```bash
export DEPLOYED_GIT_SHA=<40-char-sha>
export BINTRANS_IMAGE_VERSION=git-${DEPLOYED_GIT_SHA:0:7}
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_web_admin_release_build.sh
```

## Rollback

| Field | Value |
|-------|-------|
| WEB_ADMIN_ROLLBACK_MODEL | DIGEST_PIN_RESTORE |

Restore previous `BINTRANS_WEB_ADMIN_IMAGE=@sha256:<digest>` in protected env and recreate **web-admin only**:

```bash
docker compose \
  -f infrastructure/docker-compose/docker-compose.bintrans-ct-staging-web-admin.yml \
  --env-file /protected/bintrans/control-tower-observation/staging.env \
  -p bintrans-ct-staging \
  up -d --no-deps web-admin
```

No backend redeploy, DB migration, or DB restore required.

## Local proof (R3.4A)

```bash
./scripts/ops/bintrans_ct_staging/bintrans_ct_staging_web_admin_local_proof.sh
```

## Restore review requirement when

`RESTORE_REQUIRED_APPROVAL_COUNT_TO_1_WHEN=SECOND_ELIGIBLE_HUMAN_MAINTAINER_ADDED`
