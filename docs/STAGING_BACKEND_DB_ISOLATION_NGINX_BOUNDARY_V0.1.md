# Staging Backend DB Isolation Nginx Boundary v0.1

## Summary

Nginx boundary for future staging backend isolation.

This document does not edit Nginx.

Base commit: `5f5ad4c`.

## Decision

```text
STAGING_NGINX_BOUNDARY_CREATED
```

## Current Boundary

Read-only inspection (2026-07-31):

| Host       | Vhost                          | Current Root                        | Current API Proxy     | Current Health Proxy        |
| ---------- | ------------------------------ | ----------------------------------- | --------------------- | --------------------------- |
| production | `00-bintrans-production.conf`  | /var/www/bintrans-web-admin         | http://127.0.0.1:8080 | http://127.0.0.1:8080/health |
| staging    | `staging-bintrans.conf`        | /var/www/staging-bintrans-web-admin | http://127.0.0.1:8080 | http://127.0.0.1:8080/health |

Note: `sites-available/staging-bintrans.conf` still references old root `/var/www/bintrans-web-admin`; enabled site uses `/var/www/staging-bintrans-web-admin`.

## Target Boundary

| Host       | Target Root                         | Target API Proxy       | Target Health Proxy          |
| ---------- | ----------------------------------- | ---------------------- | ---------------------------- |
| production | /var/www/bintrans-web-admin         | http://127.0.0.1:8080  | http://127.0.0.1:8080/health |
| staging    | /var/www/staging-bintrans-web-admin | http://127.0.0.1:18080 | http://127.0.0.1:18080/health |

## Required Future Execution Rules

```text
1. Backup Nginx configs before editing.
2. Change staging vhost only.
3. Production vhost must remain unchanged.
4. Run nginx -t before reload.
5. Reload Nginx only after successful test.
6. Verify production endpoints after reload.
7. Verify staging /health returns staging backend identity/evidence if available.
```

## Not Approved

```text
No Nginx edits are approved in this plan.
No Nginx reload is approved in this plan.
No Certbot action is approved in this plan.
```
