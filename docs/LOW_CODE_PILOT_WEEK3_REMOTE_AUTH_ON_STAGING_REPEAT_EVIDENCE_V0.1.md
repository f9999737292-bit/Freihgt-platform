# Low-code Pilot Week-3 Remote Auth-On Staging Repeat Evidence v0.1

## Summary

Remote Auth-On Staging Repeat verification was executed against the Selectel staging API.

The verification completed successfully in read-only GET mode.

## Decision

```text
AUTH_ON_REMOTE_VERIFIED
```

## PR-GAP-001 Status

```text
READY_FOR_REVIEW_REMOTE_AUTH_ON_VERIFIED
```

Production-ready claimed:

```text
no
```

Controlled pilot:

```text
continues
```

## Staging Target

Provider:

```text
Selectel
```

Server IP:

```text
161.104.53.221
```

Staging URL:

```text
http://161.104.53.221
```

API URL:

```text
http://161.104.53.221/api/v1
```

Low-code API URL:

```text
http://161.104.53.221/api/v1/low-code
```

DNS / public domain used:

```text
no
```

HTTPS enabled:

```text
no
```

## Deployment

Deployment path:

```text
/opt/bintrans/freight-platform
```

Deploy commit SHA:

```text
8c8ecfe
```

Repository branch:

```text
main
```

Runtime containers:

```text
10 healthy
```

Health check:

```text
9/9 services OK
```

Migrations:

```text
11/11 applied
```

## Auth-On Runtime Flags

LOW_CODE_ADMIN_AUTH_ENABLED:

```text
true
```

API gateway AUTH_ENABLED:

```text
false
```

CORS_ALLOWED_ORIGINS:

```text
http://161.104.53.221
```

## Identity Context

Tenant ID:

```text
74519f22-ff9b-4a8b-8fff-a958c689682f
```

Tenant code:

```text
dev-bintrans
```

Admin email:

```text
admin@bintrans.local
```

Admin UUID:

```text
744c8983-800b-4ab6-b592-a29c2b3bb4d4
```

Non-admin email:

```text
shipper@bintrans.local
```

Non-admin UUID:

```text
4320aa65-33bd-461c-9458-ed6ad32f05ff
```

Credentials:

```text
provided separately / not stored
```

## Low-code Demo Templates

Published templates:

```text
6
```

Template entity types verified:

```text
TRANSPORT_ORDER
SHIPMENT
BILLING_REGISTER
FREIGHT_REQUEST
DOCUMENT
RFX
```

## Test Method

Mode:

```text
read-only GET
```

Writes executed:

```text
no
```

Forbidden operations executed:

```text
no
```

Secrets captured:

```text
no
```

.env values captured:

```text
no
```

JWT/tokens captured:

```text
no
```

Passwords captured:

```text
no
```

## Verification Matrix

| Test ID      | Actor        | Expected                 | Actual | Result |
| ------------ | ------------ | ------------------------ | ------ | ------ |
| PRE-001      | ops          | gateway health 200       | 200    | PASS   |
| PRE-002      | ops          | low-code via gateway 200 | 200    | PASS   |
| AUTH-STG-001 | admin        | admin templates 200      | 200    | PASS   |
| AUTH-STG-002 | non-admin    | admin templates 403      | 403    | PASS   |
| AUTH-STG-003 | anonymous    | admin templates 401/403  | 401    | PASS   |
| AUTH-STG-004 | admin        | runtime active 200       | 200    | PASS   |
| AUTH-STG-005 | non-admin    | runtime active 200       | 200    | PASS   |
| AUTH-STG-006 | wrong-tenant | 403/404/empty            | 403    | PASS   |
| AUTH-STG-007 | admin        | audit-events 200         | 200    | PASS   |
| AUTH-STG-008 | non-admin    | audit 200 or 403         | 200    | PASS   |

CORE_MATRIX_PASS:

```text
yes
```

FULL_MATRIX_PASS:

```text
yes
```

## Safety Confirmation

Backend code changed:

```text
no
```

Frontend code changed:

```text
no
```

API contracts changed:

```text
no
```

Migrations created:

```text
no
```

Production data used:

```text
no
```

Production-ready claimed:

```text
no
```

## Known Limitations

| Item                                           | Status                                         |
| ---------------------------------------------- | ---------------------------------------------- |
| API is HTTP-only by IP                         | open limitation                                |
| HTTPS / Certbot                                | not configured                                 |
| Public DNS/domain                              | not used                                       |
| SSH 22 restriction via Selectel Security Group | pending                                        |
| Web-admin UI deploy                            | not completed                                  |
| Full demo UI seed-data                         | not executed                                   |
| seed-lowcode-demo custom field values          | skipped because demo entities were not present |

## Closure Recommendation

PR-GAP-001 is ready for review because remote auth-on behavior has been verified against the Selectel staging API with a full passing read-only GET matrix.

Recommended next state:

```text
PR-GAP-001_READY_FOR_OWNER_REVIEW
```

Do not claim production-ready until the remaining staging limitations are reviewed separately.
