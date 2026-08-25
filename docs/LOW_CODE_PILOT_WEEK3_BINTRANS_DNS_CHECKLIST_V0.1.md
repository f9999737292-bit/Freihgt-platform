# Low-code Pilot Week-3 Bintrans DNS Checklist v0.1

## Summary

This checklist defines the operator action required to point the Bintrans staging domain to the Selectel staging server.

Active staging domain migrated from `staging.bintrans.ru` to Cyrillic `.рф` domain.

## Selected Domain

```text
Selected domain: staging.бинтранс.рф
Technical domain: staging.xn--80abvubqje.xn--p1ai
Target IP: 161.104.53.221
DNS status: DNS_PENDING_OPERATOR_ACTION
```

## Required DNS Record

Human-readable:

| Type | Name                | Value          | TTL                     |
| ---- | ------------------- | -------------- | ----------------------- |
| A    | staging.бинтранс.рф | 161.104.53.221 | 300 or provider default |

Technical / punycode:

| Type | Name                              | Value          | TTL                     |
| ---- | --------------------------------- | -------------- | ----------------------- |
| A    | staging.xn--80abvubqje.xn--p1ai | 161.104.53.221 | 300 or provider default |

Equivalent record:

```text
A staging.бинтранс.рф -> 161.104.53.221
A staging.xn--80abvubqje.xn--p1ai -> 161.104.53.221
```

## Deprecated Domains

Do not use for new staging path:

```text
staging.bintrans.ru
pilot.bintrans.ru
```

Invalid erroneous legacy references (never valid BINTRANS endpoints; removed from active docs):

```text
Invalid legacy external domain family — unrelated external site; must not be used for BINTRANS
```

## Operator Action

Operator must create the A-record in the DNS provider / registrar panel for zone `бинтранс.рф` (`xn--80abvubqje.xn--p1ai`).

## Verification Commands

After DNS propagation:

```powershell
nslookup staging.бинтранс.рф
nslookup staging.xn--80abvubqje.xn--p1ai
```

Expected:

```text
staging.бинтранс.рф -> 161.104.53.221
staging.xn--80abvubqje.xn--p1ai -> 161.104.53.221
```

HTTP check before HTTPS (use punycode in commands):

```powershell
Invoke-WebRequest -UseBasicParsing http://staging.xn--80abvubqje.xn--p1ai/health | Select-Object StatusCode
```

Expected:

```text
200
```

## HTTPS Prerequisites

Certbot must not be executed until:

* DNS resolves to 161.104.53.221
* SSH access is available
* Nginx is reachable on port 80
* Selectel Security Group allows 80 and 443
* STG-LIM-003 is not blocking SSH access
* production-ready remains not claimed

## Status

```text
DNS_PENDING_OPERATOR_ACTION
```

## Related Limitations

| ID | Status |
| -- | ------ |
| STG-LIM-001 | OPEN_DNS_PENDING_CYRILLIC_RF_DOMAIN |
| STG-LIM-002 | OPEN_HTTPS_PENDING_DNS_AND_SSH |
| STG-LIM-003 | OPEN |

## Production-ready

```text
not claimed
```
