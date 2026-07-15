# Low-code Pilot Week-3 Cyrillic .рф Domain Migration Decision v0.1

## Summary

The active staging domain direction is changed from `bintrans.ru` to `бинтранс.рф`.

Production-ready is not claimed.

## Previous Active Domain

```text
staging.bintrans.ru
```

## New Active Domain

Human-readable:

```text
staging.бинтранс.рф
```

Punycode / technical form:

```text
staging.xn--80abvubqje.xn--p1ai
```

Root punycode:

```text
xn--80abvubqje.xn--p1ai
```

## Target IP

```text
161.104.53.221
```

## Required DNS Record

Human-readable:

```text
A staging.бинтранс.рф -> 161.104.53.221
```

Technical / punycode:

```text
A staging.xn--80abvubqje.xn--p1ai -> 161.104.53.221
```

## Future HTTPS Endpoint

Display:

```text
https://staging.бинтранс.рф
```

Technical:

```text
https://staging.xn--80abvubqje.xn--p1ai
```

## Scope

This is a domain migration decision only.

No DNS changes, Certbot execution, Nginx server changes, backend code changes, frontend code changes, API contract changes, migrations, or production-ready claim were executed.

## Status

```text
CYRILLIC_RF_DOMAIN_SELECTED_DNS_PENDING
```

## Production-ready

```text
not claimed
```
