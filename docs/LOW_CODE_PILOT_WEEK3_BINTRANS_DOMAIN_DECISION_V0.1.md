# Low-code Pilot Week-3 Bintrans Domain Decision v0.1

## Summary

The staging domain direction has been changed from 7rights to Bintrans.

## Decision

Selected staging domain:

```text
staging.bintrans.ru
```

Fallback staging domain:

```text
pilot.bintrans.ru
```

Deprecated for this staging path:

```text
staging.7rights.ru
pilot.7rights.ru
```

## Target

Selectel staging IP:

```text
161.104.53.221
```

Current HTTP staging endpoint:

```text
http://161.104.53.221
```

Future HTTPS staging endpoint:

```text
https://staging.bintrans.ru
```

Future API base:

```text
https://staging.bintrans.ru/api/v1
```

Future low-code API:

```text
https://staging.bintrans.ru/api/v1/low-code
```

## DNS Status

DNS A-record:

```text
pending operator action
```

Required DNS record:

```text
A staging.bintrans.ru -> 161.104.53.221
```

Optional fallback record:

```text
A pilot.bintrans.ru -> 161.104.53.221
```

## HTTPS Prerequisite (docs-only)

Certbot and Nginx TLS configuration must not be executed until:

* DNS resolves `staging.bintrans.ru` to `161.104.53.221`
* SSH access is available from trusted operator workstation
* Nginx is reachable on port 80 for the staging domain
* Selectel Security Group allows inbound TCP 80 and 443
* STG-LIM-003 SSH /32 restriction is verified closed
* production-ready remains not claimed

See `docs/LOW_CODE_PILOT_WEEK3_BINTRANS_DNS_CHECKLIST_V0.1.md`.

## Production-ready

```text
not claimed
```

## Controlled pilot

```text
active
```

## Decision Status

```text
BINTRANS_STAGING_DOMAIN_SELECTED_DNS_PENDING
```
