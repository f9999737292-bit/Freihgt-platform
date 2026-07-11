# Low-code Pilot Week-3 Selectel SSH SG Panel Confirmation Note v0.1

## Summary

Panel Confirmation Pack executed as operator. Trusted-path checks pass. Selectel SG panel change remains pending.

## Decision

```text
SELECTEL_SSH_SG_PANEL_CONFIRMATION_REVERIFIED_SG_PANEL_CHANGE_PENDING
```

## Pass

* API health 200
* SSH from trusted operator workstation
* SSH denied without operator private key
* UFW denies 5432/6379 on VM
* 10 runtime containers healthy

## Fail / Pending

* Selectel SG SSH /32 rule not applied in panel
* SSH 22 global exposure in Selectel SG not verified as removed
* Non-trusted IP SSH rejection not tested
* STG-LIM-003 not closed

## Blocker

```text
Selectel control panel not accessible from automation environment — manual operator panel action required
```

## Operator Manual Steps (Selectel panel)

1. Security Group → inbound TCP 22 → trusted operator IP /32
2. Remove broad SSH allow (`0.0.0.0/0`, `::/0`) if present
3. Keep HTTP 80 and HTTPS 443 open
4. Keep PostgreSQL 5432 and Redis 6379 closed in SG
5. Signal re-run of post-panel verification pack

## Production-ready Status

```text
not claimed
```

## Next Pack

```text
Selectel SSH SG Post-Panel Verification Pack v0.1
```
