# Low-code Pilot Week-3 Bintrans DNS Checklist v0.1

## Summary

This checklist defines the operator action required to point the Bintrans staging domain to the Selectel staging server.

## Required DNS Record

| Type | Name                | Value          | TTL                     |
| ---- | ------------------- | -------------- | ----------------------- |
| A    | staging.bintrans.ru | 161.104.53.221 | 300 or provider default |

## Optional Fallback DNS Record

| Type | Name              | Value          | TTL                     |
| ---- | ----------------- | -------------- | ----------------------- |
| A    | pilot.bintrans.ru | 161.104.53.221 | 300 or provider default |

## Operator Action

Operator must create the A-record in the DNS provider / registrar panel.

Do not use deprecated 7rights staging domains for this path:

```text
staging.7rights.ru
pilot.7rights.ru
```

## Verification Commands

After DNS propagation:

```powershell
Resolve-DnsName staging.bintrans.ru
```

Expected:

```text
staging.bintrans.ru -> 161.104.53.221
```

HTTP check before HTTPS:

```powershell
Invoke-WebRequest -UseBasicParsing http://staging.bintrans.ru/health | Select-Object StatusCode
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
| STG-LIM-001 | OPEN_DNS_PENDING_BINTRANS_DOMAIN |
| STG-LIM-002 | OPEN_HTTPS_PENDING_DNS_AND_SSH |
| STG-LIM-003 | OPEN |

## Production-ready

```text
not claimed
```
