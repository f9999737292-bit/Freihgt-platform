# Low-code Pilot Week-3 Bintrans HTTPS / Certbot Preparation Note v0.1

## Summary

HTTPS/Certbot preparation pack updated for Cyrillic `.рф` staging domain. Docs-only. DNS still pending.

Active staging domain: `staging.бинтранс.рф` (technical: `staging.xn--80abvubqje.xn--p1ai`).

## Decision

```text
CYRILLIC_RF_DOMAIN_SELECTED_DNS_PENDING
```

## Pass

* Trusted SSH available
* API health 200 on IP
* SG allows 80/443
* Preparation docs created

## Pending

* DNS A-record for `staging.бинтранс.рф`
* HTTP domain health check
* Certbot execution
* STG-LIM-003 external scan (deferred per operator)

## Production-ready

```text
not claimed
```
