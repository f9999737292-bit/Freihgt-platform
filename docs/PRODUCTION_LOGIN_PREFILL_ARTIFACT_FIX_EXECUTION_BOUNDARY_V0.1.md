# Production Login Prefill Artifact Fix Execution Boundary v0.1

## Approved Future Target

| Item | Value |
|---|---|
| production static root | /var/www/bintrans-web-admin |
| staging static root | /var/www/staging-bintrans-web-admin |
| artifact source | current QA-signed web-admin source |
| future deploy type | static artifact refresh only |

## Approved Future Paths

| Path | Future Action |
|---|---|
| /var/www/bintrans-web-admin | replace static artifact content only after backup |
| /root/production-login-prefill-fix-backup-* | production root backup |
| /tmp/bintrans-web-admin-production-prefill-fix*.tar.gz | temporary upload artifact |

## Explicitly Not Approved Paths / Areas

| Path / Area | Reason |
|---|---|
| /var/www/staging-bintrans-web-admin | staging must not be modified |
| /etc/nginx | Nginx changes not required |
| /etc/letsencrypt | Certbot/certs out of scope |
| Docker / containers | backend deploy out of scope |
| database | migrations/writes out of scope |
| source files under apps/ | source changes out of scope unless separately approved |

## Required Future STOP Conditions

```text
1. production endpoints are not healthy before execution.
2. staging endpoints are not healthy before execution.
3. production and staging roots are not distinct.
4. artifact does not contain index.html.
5. artifact source cannot be tied to approved commit/source.
6. production root backup fails.
7. any step would require Nginx/DNS/Certbot/backend/API/database changes.
```

## Required Future Verification

```text
1. production / returns 200.
2. production /login returns 200.
3. production /health returns 200.
4. production login fields are empty.
5. staging / returns 200.
6. staging /login returns 200.
7. staging /health returns 200.
8. no Nginx reload was executed.
```

## Decision

```text
PRODUCTION_LOGIN_PREFILL_ARTIFACT_FIX_EXECUTION_BOUNDARY_CREATED
```
