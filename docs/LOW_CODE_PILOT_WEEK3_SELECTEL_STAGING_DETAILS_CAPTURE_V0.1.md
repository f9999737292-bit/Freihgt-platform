# Low-code Pilot Week-3 Selectel Staging Details Capture v0.1

## Summary

Selectel staging server details have been provided.

The server hardware profile is acceptable for remote staging, but the environment is not ready for Remote Auth-On Staging Repeat Pack v0.1 because security and runtime prerequisites are incomplete.

## Provider

Provider: Selectel

Public IP: 161.104.53.221

Domain/subdomain: localhost

Domain status:

```text
INVALID_FOR_REMOTE_STAGING
```

Recommended domain:

```text
staging.7rights.ru
```

or:

```text
pilot.7rights.ru
```

## Server Profile

OS: Ubuntu 24.04 LTS 64-bit

CPU: 8 dedicated vCPU, AMD 2.2–2.4 GHz

RAM: 32 GB

Disk: 500 GB SSD Fast v2

IOPS: 25000

GPU: no

## Access

SSH user: root

SSH key access: yes

Sudo/root access: yes

## Network Status

80 open: yes

443 open: yes

22 restricted by IP: no

PostgreSQL external access closed: no

Redis external access closed: no

Internal service ports external access closed: yes

## Runtime Status

Docker installed: no

Docker Compose installed: no

Repo cloned: no

Branch: main

.env staging prepared: values not included

LOW_CODE_ADMIN_AUTH_ENABLED=true: yes

## URLs

Web-admin URL: not provided

API gateway URL: not provided

Low-code service URL, internal only: not provided

## Backups

Daily backup enabled: yes

Retention: 7 days

## Security Assessment

Current status:

```text
STAGING_DETAILS_CAPTURED_BUT_NOT_READY
```

Critical blockers:

* SSH 22 is not restricted by IP
* PostgreSQL 5432 external access is not closed
* Redis 6379 external access is not closed
* Domain/subdomain is localhost and not valid for remote staging
* Docker is not installed
* Docker Compose is not installed
* Repository is not cloned
* Runtime URLs are not available

## Decision

Decision:

```text
SELECTEL_STAGING_DETAILS_CAPTURED_HARDENING_REQUIRED
```

PR-GAP-001:

```text
BLOCKED_WAITING_FOR_STAGING_HARDENING_AND_RUNTIME_PREPARATION
```

Production-ready claimed:

```text
no
```

Remote Auth-On Staging Repeat allowed:

```text
no
```
