# Low-code Pilot Week-3 Selectel SSH SG Post-Panel Re-Verification Retry 4 Note v0.1

## Summary

Retry #4 on 2026-07-12 after operator reported SG fix. Port 22 still publicly open. Trusted SSH regressed.

## Decision

```text
SELECTEL_SSH_SG_POST_PANEL_REVERIFICATION_FAILED_PORT_22_STILL_PUBLIC
```

## Pass

* API 200

## Fail

* Trusted SSH — banner exchange timeout
* Non-trusted TCP 22 — 5/5 external nodes connect
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
