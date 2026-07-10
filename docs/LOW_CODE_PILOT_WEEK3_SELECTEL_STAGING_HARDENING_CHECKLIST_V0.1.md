# Low-code Pilot Week-3 Selectel Staging Hardening Checklist v0.1

## Summary

This checklist must be completed before Remote Auth-On Staging Repeat Pack v0.1 can be executed.

## Network Hardening

| Item                                     | Required | Current | Status |
| ---------------------------------------- | -------- | ------- | ------ |
| SSH 22 restricted by IP                  | yes      | no      | FAIL   |
| HTTP 80 open                             | yes      | yes     | PASS   |
| HTTPS 443 open                           | yes      | yes     | PASS   |
| PostgreSQL 5432 closed externally        | yes      | no      | FAIL   |
| Redis 6379 closed externally             | yes      | no      | FAIL   |
| Internal service ports closed externally | yes      | yes     | PASS   |

## Runtime Preparation

| Item                                            | Required | Current | Status  |
| ----------------------------------------------- | -------- | ------- | ------- |
| Docker installed                                | yes      | no      | PENDING |
| Docker Compose installed                        | yes      | no      | PENDING |
| Git installed                                   | yes      | unknown | PENDING |
| Repo cloned                                     | yes      | no      | PENDING |
| Branch main checked out                         | yes      | pending | PENDING |
| staging .env prepared without publishing values | yes      | pending | PENDING |
| LOW_CODE_ADMIN_AUTH_ENABLED=true                | yes      | yes     | PASS    |
| Web-admin URL available                         | yes      | no      | PENDING |
| API gateway URL available                       | yes      | no      | PENDING |
| Low-code service internal URL identified        | yes      | no      | PENDING |

## Domain

Current:

```text
localhost
```

Status:

```text
INVALID_FOR_REMOTE_STAGING
```

Required:

```text
staging.7rights.ru
```

or:

```text
pilot.7rights.ru
```

Temporary fallback:

```text
http://161.104.53.221
```

## Forbidden

* Do not expose PostgreSQL 5432 publicly.
* Do not expose Redis 6379 publicly.
* Do not expose API gateway 8080 publicly.
* Do not expose low-code-service 8088 publicly.
* Do not expose web-admin dev ports 3000/5173 publicly.
* Do not store passwords, tokens, JWT, SSH private keys, .env values, or database credentials in docs.

## Decision

```text
SELECTEL_STAGING_HARDENING_REQUIRED_BEFORE_REMOTE_AUTH_ON_REPEAT
```

## PR-GAP-001 Status

```text
BLOCKED_WAITING_FOR_STAGING_HARDENING_AND_RUNTIME_PREPARATION
```
