# Staging Web Root Separation Execution Boundary v0.1

## Approved Files / Paths for Future Execution

| Path | Approved Future Action |
|---|---|
| `/etc/nginx/sites-enabled/staging-bintrans.conf` | change root only |
| `/etc/nginx/sites-available/staging-bintrans.conf` | change root only if this is the actual managed config |
| `/var/www/staging-bintrans-web-admin` | create/populate staging static baseline |
| `/root/staging-nginx-root-separation-backup-*` | backup staging Nginx config |

## Explicitly Not Approved

| Path / Area | Reason |
|---|---|
| `/var/www/bintrans-web-admin` | production root must not be modified |
| `/etc/nginx/sites-enabled/00-bintrans-production.conf` | production vhost must not be edited |
| `/etc/nginx/sites-available/bintrans-production.conf` | production vhost must not be edited |
| `/etc/letsencrypt` | Certbot/certs out of scope |
| Docker / containers | backend/deploy out of scope |
| database | no data writes/migrations |

## Required Server Commands in Future Execution

```text
nginx -t before reload
systemctl reload nginx only after syntax pass
curl verification for staging and production after reload
```

## STOP Conditions

```text
1. staging vhost cannot be identified.
2. production vhost would need editing.
3. staging and production roots cannot be separated.
4. nginx -t fails.
5. production baseline is not healthy before execution.
```

## Decision

```text
STAGING_WEB_ROOT_SEPARATION_EXECUTION_BOUNDARY_CREATED
```
