# Low-code Pilot Week-3 Bintrans Domain Decision v0.1

## Summary

BINTRANS staging endpoint naming is tracked separately from unrelated external sites. Invalid legacy external domain references from early project naming mistakes have been removed from active documentation.

## Staging endpoint status

Public latin DNS endpoint:

```text
staging.bintrans.ru — NOT VERIFIED / NOT CONFIGURED
```

Repository-selected Cyrillic domain (see DNS checklist):

```text
staging.бинтранс.рф (technical: staging.xn--80abvubqje.xn--p1ai) — status per DNS checklist
```

Dedicated Control Tower staging VM (repository evidence):

```text
VM: bintrans-control-tower-staging
Public IP: 161.104.57.152
Gateway loopback: 127.0.0.1:18080
```

Legacy shared Selectel VPS (historical low-code pilot context):

```text
Public IP: 161.104.53.221
```

Current HTTP access (legacy shared VPS context):

```text
http://161.104.53.221
```

Future HTTPS staging endpoint:

```text
NOT VERIFIED / NOT CONFIGURED — depends on verified DNS + TLS operator action
```

Future API base:

```text
NOT VERIFIED / NOT CONFIGURED
```

## DNS Status

DNS A-record:

```text
pending operator action (see BINTRANS DNS checklist)
```

Required DNS record (when verified):

```text
See docs/LOW_CODE_PILOT_WEEK3_BINTRANS_DNS_CHECKLIST_V0.1.md
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
BINTRANS_STAGING_DNS_PENDING_OPERATOR_ACTION
```
