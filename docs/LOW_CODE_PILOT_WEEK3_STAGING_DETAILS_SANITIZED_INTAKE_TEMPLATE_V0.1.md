# Low-code Pilot Week-3 Staging Details Sanitized Intake Template v0.1

## Summary

Fill this template only after staging server is provisioned.

Do **not** include secrets.

## Remote Staging Details

Provider:

Public IP:

Domain/subdomain:

OS:

CPU:

RAM:

Disk:

SSH user:

SSH key access: yes/no

Sudo/root access: yes/no

## Network

80 open: yes/no

443 open: yes/no

22 restricted by IP: yes/no

PostgreSQL external access closed: yes/no

Redis external access closed: yes/no

Internal service ports external access closed: yes/no

## Runtime

Docker installed: yes/no

Docker Compose installed: yes/no

Repo cloned: yes/no

Branch: main

`.env` staging prepared: yes/no, values not included

`LOW_CODE_ADMIN_AUTH_ENABLED=true`: yes/no

## URLs

Web-admin URL:

API gateway URL:

Low-code service URL, internal only:

## Forbidden Data

Do not include:

- Passwords
- SSH private keys
- JWT
- Tokens
- `.env` values
- Database credentials
- Production data
- Signed legal documents

## Decision

```text
STAGING_DETAILS_SANITIZED_INTAKE_TEMPLATE_READY
```

Reference: `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_DETAILS_INTAKE_FORM_V0.1.md`
