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

## Scope

This decision is docs-only.

No DNS changes, SSH commands, Certbot commands, Nginx server changes, backend changes, frontend changes, API contract changes, or migrations were executed.

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
