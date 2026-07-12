# Low-code Pilot Week-3 Selectel SSH SG Post-Panel Re-Verification Retry 6 Note v0.1

## Summary

Retry #6 on 2026-07-12 after operator SG fix. Trusted SSH restored. Port 22 still publicly open (4/5).

## Decision

```text
SELECTEL_SSH_SG_POST_PANEL_REVERIFICATION_FAILED_PORT_22_STILL_PUBLIC
```

## Pass

* Trusted SSH
* API 200
* 10 containers healthy
* UFW 5432/6379 deny

## Fail

* Non-trusted TCP 22 — 4/5 external nodes connect
* Selectel SG /32 not evidenced
* STG-LIM-003 not closed

## Blocker

```text
BLOCKED_WAITING_FOR_SELECTEL_SG_PANEL_CHANGE
```

## Production-ready

```text
not claimed
```
