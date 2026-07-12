# Low-code Pilot Week-3 Web-admin Deploy Plan v0.1

## Summary

Docs-only deploy plan for `apps/web-admin` (Nuxt 3) on Selectel staging server.

No build executed on server, no Nginx changes, no Docker image push, no secrets written, no production-ready claim.

## Decision

```text
WEB_ADMIN_DEPLOY_PLAN_CREATED_PENDING_EXECUTION
```

## Target Application

Path:

```text
apps/web-admin
```

Framework:

```text
Nuxt 3
```

Default dev port:

```text
3000 — must not be exposed publicly
```

Runtime config (public):

```text
NUXT_PUBLIC_API_BASE_URL
NUXT_PUBLIC_APP_NAME
NUXT_PUBLIC_DEFAULT_LOCALE
NUXT_PUBLIC_DEFAULT_TENANT_ID
NUXT_PUBLIC_MOCK_AUTH=false
```

## Staging Targets

Current API (active):

```text
http://161.104.53.221/api/v1
```

Future API (after DNS + HTTPS):

```text
https://staging.bintrans.ru/api/v1
```

Current web-admin URL (planned, IP interim):

```text
http://161.104.53.221/
```

Future web-admin URL:

```text
https://staging.bintrans.ru/
```

Server deploy root (existing):

```text
/opt/bintrans/freight-platform
```

## Prerequisites

| # | Prerequisite | Status |
| - | ------------ | ------ |
| 1 | API staging healthy | **pass** — `/health` 200 |
| 2 | Read-only API smoke | **pass** — see smoke evidence |
| 3 | Trusted SSH available | **pass** |
| 4 | Node.js 20+ on server | verify on execution |
| 5 | Nginx reverse proxy | verify on execution |
| 6 | DNS `staging.bintrans.ru` | **pending** |
| 7 | HTTPS Certbot | **pending** — after DNS |
| 8 | STG-LIM-003 external scan | **deferred** |

## Recommended Deploy Architecture

```text
Internet -> Nginx :80/:443 -> web-admin static or Node preview :3000 (localhost only)
                         -> api-gateway :8080 (/api/v1)
```

Public exposure:

* **80/443** via Nginx only
* **3000/5173** blocked externally (UFW deny — already configured)
* **8080** internal only behind Nginx `/api/`

## Phase 1 — Local Build Verification (docs-only command reference)

On operator workstation or CI (not executed in this pack):

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm ci
$env:NUXT_PUBLIC_API_BASE_URL="http://161.104.53.221/api/v1"
$env:NUXT_PUBLIC_DEFAULT_TENANT_ID="74519f22-ff9b-4a8b-8fff-a958c689682f"
$env:NUXT_PUBLIC_MOCK_AUTH="false"
npm run build
```

## Phase 2 — Server Deploy (execute only after operator approval)

On staging server via SSH:

```bash
cd /opt/bintrans/freight-platform
git pull origin main
cd apps/web-admin
npm ci
export NUXT_PUBLIC_API_BASE_URL=http://161.104.53.221/api/v1
export NUXT_PUBLIC_DEFAULT_TENANT_ID=74519f22-ff9b-4a8b-8fff-a958c689682f
export NUXT_PUBLIC_MOCK_AUTH=false
npm run build
```

Output options:

| Option | Description | Recommended |
| ------ | ----------- | ----------- |
| A. Static + Nginx | Serve `.output/public` via Nginx | **yes** |
| B. Node preview | `node .output/server/index.mjs` behind Nginx | optional |
| C. Dev server | `npm run dev` on 3000 | **no** for staging |

## Phase 3 — Nginx (docs-only — do not apply without approval)

Interim IP-based config:

```nginx
server {
    listen 80;
    server_name 161.104.53.221 staging.bintrans.ru;

    location / {
        root /opt/bintrans/freight-platform/apps/web-admin/.output/public;
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8080/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /health {
        proxy_pass http://127.0.0.1:8080/health;
    }
}
```

After DNS + HTTPS: extend with Certbot per Bintrans HTTPS pack.

## Phase 4 — Post-deploy Verification

| Check | Command / action | Expected |
| ----- | ---------------- | -------- |
| UI loads | browser `http://161.104.53.221/` | login page or app shell |
| API proxy | browser network calls to `/api/v1` | 200/401 as expected |
| No public 3000 | external scan port 3000 | filtered/denied |
| Health | `http://161.104.53.221/health` | 200 |

## STG-LIM-004 Impact

| Status before | Status after this pack |
| ------------- | ---------------------- |
| OPEN | OPEN — deploy plan created; execution pending |

## Production-ready

```text
not claimed
```

## Safety

Secrets in docs:

```text
no
```

Backend code changed:

```text
no
```

Frontend code changed:

```text
no
```

## Next Pack

```text
Web-admin Deploy Execution Pack v0.1 (operator approval required)
```

## Related Docs

```text
docs/LOW_CODE_PILOT_WEEK3_WEB_ADMIN_DEPLOY_CHECKLIST_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_STAGING_API_READ_ONLY_SMOKE_EVIDENCE_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_BINTRANS_HTTPS_CERTBOT_PREPARATION_PACK_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_STAGING_DEPLOY_RUNBOOK_V0.1.md
```
