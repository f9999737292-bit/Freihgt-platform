# Staging Backend DB Isolation Nginx Change Evidence v0.1

## Summary

Evidence for staging Nginx proxy change.

## Result

```text
STAGING_NGINX_PROXY_CHANGED_TO_127_0_0_1_18080
```

## Evidence

| Item                              | Result                |
| --------------------------------- | --------------------- |
| staging site file                 | /etc/nginx/sites-available/staging-bintrans.conf |
| backup created                    | yes                   |
| production vhost changed          | no                    |
| staging proxy before              | http://127.0.0.1:8080 |
| staging proxy after               | http://127.0.0.1:18080 |
| nginx -t                          | pass                  |
| nginx reload                      | executed              |
| production endpoints after reload | pass                  |
| staging endpoints after reload    | pass                  |

## Notes

```text
sites-enabled/staging-bintrans.conf was a standalone file (not symlink).
After editing sites-available, sites-enabled was synced to 127.0.0.1:18080 and nginx reloaded again.
Production vhost 00-bintrans-production.conf remained on 127.0.0.1:8080.
```

## Safety

```text
Production vhost changed: no
Certbot changed: no
DNS changed: no
Secrets captured: no
```
